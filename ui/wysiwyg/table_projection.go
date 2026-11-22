package wysiwyg

import (
	"crypto/rand"
	"strings"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

const tableLabel = "Markdown table"
const tableEmptyParagraphMarkdown = "<br />"
const tableExitID = "cyberwriter-empty-paragraph"
const tableCaretID = "cyberwriter-table-caret"

// Table is one version of a projected Markdown table. Its content stays
// immutable so the rich-text history can restore that version.
type Table struct {
	id, markdown string
	region       tableRegion
	object       *richtext.Attachment
	removal      *tableRemoval
}

// A removed table remains in the document's undo history. Keep its source
// position for restoration when the rest of the visual document is unchanged.
type tableRemoval struct {
	start         int
	without       string
	before, after string
}

func newVisualTable(markdown string, region tableRegion, id string, height float32) *Table {
	version := rand.Text()
	if id == "" {
		id = version
	}
	if height <= 0 {
		height = 1 + 43.5*float32(len(region.rows))
	}
	table := &Table{id: id, markdown: markdown, region: region}
	table.object = &richtext.Attachment{
		Name: tableLabel, URL: "cyberwriter-table:" + version,
		Width: 1, Height: height, Data: table,
		// The inline cell editors paint the table within the reserved box.
		Draw: func(*mygoui.Painter, mygoui.Rect) {},
	}
	return table
}

func (t *Table) ID() string       { return t.id }
func (t *Table) RowCount() int    { return len(t.region.rows) }
func (t *Table) ColumnCount() int { return len(t.region.align) }
func (t *Table) Height() float32  { return t.object.Height }

func (t *Table) ReserveHeight(height float32) { t.object.Height = height }

// Cell returns the original Markdown of the header or data cell.
func (t *Table) Cell(row, col int) string {
	if row < 0 || row >= t.RowCount() || col < 0 || col >= t.ColumnCount() {
		return ""
	}
	cell := t.region.rows[row][col]
	return t.markdown[cell.start:cell.end]
}

func (t *Table) Align(col int) parser.Align {
	if col < 0 || col >= t.ColumnCount() {
		return parser.AlignNone
	}
	return t.region.align[col]
}

func (s *Session) TableAt(offset int) (*Table, bool) {
	if s.Document == nil {
		return nil, false
	}
	return visualTableAt(s.Document.Text(), offset)
}

func visualTableAt(text richtext.Text, offset int) (*Table, bool) {
	if offset < 0 || offset >= text.Len() {
		return nil, false
	}
	object := text.AttributesAt(offset).Attachment
	if object == nil {
		return nil, false
	}
	table, ok := object.Data.(*Table)
	return table, ok
}

// ChangeTableCell uses the same GFM serializer as the table form, then
// replaces the attachment in a single rich-text undo step.
func (s *Session) ChangeTableCell(offset int, previous *Table, row, col int, value string) (*Table, error) {
	current, ok := s.TableAt(offset)
	if !ok || current != previous || !s.Current(s.draft) || row < 0 || row >= current.RowCount() || col < 0 || col >= current.ColumnCount() {
		return nil, errStaleTable
	}
	codec := TableEdit{source: current.markdown, region: &current.region}
	rows := codec.Rows()
	rows[row][col] = value
	markdown, region, err := codec.editedSource(rows)
	if err != nil {
		return nil, err
	}
	if markdown == current.markdown {
		return current, nil
	}
	next := newVisualTable(markdown, region, current.id, current.Height())
	s.Document.Edit(func(edit *richtext.Edit) {
		replaceVisualTable(edit, offset, next)
	})
	return next, nil
}

func replaceVisualTable(edit *richtext.Edit, offset int, table *Table) {
	edit.ReplaceString(offset, offset+1, string(richtext.ObjectReplacement), richtext.Attributes{Attachment: table.object})
}

func (s *Session) removeVisualTable(offset int, table *Table) {
	s.Sync(s.draft)
	s.materializeProjection()
	var removal *tableRemoval
	if len(s.tables) == len(s.visualTables) {
		for i, current := range s.visualTables {
			region := s.tables[i]
			if current == table && s.source[region.start:region.end] == table.markdown {
				removal = &tableRemoval{
					start:   region.start,
					without: s.source[:region.start] + s.source[region.end:],
					before:  s.tableProjection,
				}
				break
			}
		}
	}
	s.Document.Edit(func(edit *richtext.Edit) {
		text := edit.Text()
		start, end := text.ParagraphRange(offset)
		if start != offset || end != offset+1 {
			// A surrounding text edit can join a table to a paragraph.
			start, end = offset, offset+1
		} else if end < text.Len() {
			end++
		}
		edit.ReplaceString(start, end, "", richtext.Attributes{})
		edit.Select(start, start)
	})
	if removal != nil {
		text := splitAttachmentParagraphs(s.Document.Text())
		removal.after, _ = tableProjection(visualMarkdown(text), text)
	}
	table.removal = removal
}

// InsertTable places the Crepe default 3-by-3 table at the visual selection.
// The returned offset is the table's rune position for focusing its header.
func (s *Session) InsertTable() (int, *Table) {
	const row = "| <br /> | <br /> | <br /> |"
	const markdown = row + "\n| :----- | :----- | :----- |\n" + row + "\n" + row
	return s.InsertMarkdownTable(markdown)
}

// InsertMarkdownTable replaces the visual selection with a standalone table.
// Its returned rune offset identifies the attachment for focusing a cell.
func (s *Session) InsertMarkdownTable(markdown string) (int, *Table) {
	if s.Document == nil || !s.Current(s.draft) {
		return -1, nil
	}
	markdown = strings.Trim(markdown, "\n")
	region, ok := singleTableRegion(markdown)
	if !ok {
		return -1, nil
	}
	table := newVisualTable(markdown, region, "", 0)
	offset := 0
	s.Document.Edit(func(edit *richtext.Edit) {
		anchor, caret := edit.Selection()
		start, end := min(anchor, caret), max(anchor, caret)
		text := edit.Text()
		paragraphStart, _ := text.ParagraphRange(start)
		_, paragraphEnd := text.ParagraphRange(end)
		var paragraphs []richtext.Paragraph
		if start > paragraphStart {
			paragraphs = append(paragraphs, richtext.Paragraph{Style: text.ParagraphStyleAt(start)})
		}
		offset = start + len(paragraphs)
		paragraphs = append(paragraphs, attachmentParagraph(table.object))
		if end < paragraphEnd {
			paragraphs = append(paragraphs, richtext.Paragraph{Style: text.ParagraphStyleAt(end)})
		}
		edit.Replace(start, end, richtext.TextOf(paragraphs...))
		edit.Select(offset, offset)
	})
	return offset, table
}

// MoveAfterTable places the caret in the next paragraph, adding an editable
// empty paragraph when the table is the last block.
func (s *Session) MoveAfterTable(offset int) {
	if _, ok := s.TableAt(offset); !ok || !s.Current(s.draft) {
		return
	}
	s.Document.Edit(func(edit *richtext.Edit) {
		moveAfterTable(edit, offset)
	})
}

func moveAfterTable(edit *richtext.Edit, offset int) {
	text := edit.Text()
	_, end := text.ParagraphRange(offset)
	if end == text.Len() {
		edit.ReplaceString(end, end, "\n", richtext.Attributes{})
		edit.FormatParagraphs(end+1, end+1, func(style *richtext.ParagraphStyle) {
			*style = richtext.ParagraphStyle{ID: tableCaretID}
		})
	}
	edit.Select(end+1, end+1)
}

// ExitTable inserts an editable body paragraph after the table and puts a
// collapsed caret in it. Its ID preserves an empty paragraph on save.
func (s *Session) ExitTable(offset int) {
	if _, ok := s.TableAt(offset); !ok || !s.Current(s.draft) {
		return
	}
	s.Document.Edit(func(edit *richtext.Edit) {
		end := offset + 1
		text := edit.Text()
		_, paragraphEnd := text.ParagraphRange(end)
		paragraphs := []richtext.Paragraph{{}, {}}
		if end < paragraphEnd {
			paragraphs = append(paragraphs, richtext.Paragraph{Style: text.ParagraphStyleAt(end)})
		}
		edit.Replace(end, end, richtext.TextOf(paragraphs...))
		edit.FormatParagraphs(end+1, end+1, func(style *richtext.ParagraphStyle) {
			*style = richtext.ParagraphStyle{ID: tableExitID}
		})
		edit.Select(end+1, end+1)
	})
}

func isTableExit(style richtext.ParagraphStyle) bool { return style.ID == tableExitID }

// The table attachment masks its cells from the surrounding document's
// round-trip check. Keep unsupported cell markup in the Markdown editor.
func supportsVisualTable(doc *parser.Document, index, end int) bool {
	source := doc.Source()[doc.Block(index).Start:end]
	if strings.Contains(source, "==") {
		table := parser.New(doc.Extensions())
		table.Set(source)
		projected, _ := projectHighlights(table)
		if projected != table && projected.Len() > 1 && projected.Block(1).Kind == parser.Table {
			doc, index = projected, 1
		}
	}
	for row := range doc.Children(index) {
		for cell := range doc.Children(row) {
			for _, inline := range doc.Inlines(cell) {
				if !supportsVisualInline(inline) || inline.Kind == parser.Link && inline.Title != "" {
					return false
				}
				if inline.Kind == parser.HTML && !supportsTableHTML(inline.Text) {
					return false
				}
			}
		}
	}
	return true
}

// ParseTableCell keeps the table's pipe escapes and inline formatting when
// projecting its source to an editable rich-text field.
func ParseTableCell(source string) richtext.Text {
	if source == tableEmptyParagraphMarkdown {
		return richtext.Text{}
	}
	const header = "Cell\n"
	markdown := "| Cell |\n| --- |\n| " + source + " |"
	var text richtext.Text
	if strings.Contains(source, "==") {
		doc := parser.New(Extensions)
		doc.Set(markdown)
		projected, highlights := projectHighlights(doc)
		if projected != doc {
			marked, ok := applyHighlightDelimiters(richtext.ParseMarkdown(projected.Source()), highlights)
			if ok {
				text = marked
			}
		}
	}
	if text.Len() == 0 {
		text = richtext.ParseMarkdown(markdown)
	}
	if !strings.HasPrefix(text.String(), header) {
		return richtext.NewText(source)
	}
	return text.Slice(len(header), text.Len())
}

// TableCellMarkdown serializes a cell on one Markdown line. The existing
// table transaction escapes pipes and validates the reconstructed table.
func TableCellMarkdown(text richtext.Text) string {
	parts := make([]string, 0, text.ParagraphCount())
	for _, paragraph := range text.Paragraphs() {
		paragraph.Style = richtext.ParagraphStyle{}
		parts = append(parts, markdownWithHighlights(richtext.TextOf(paragraph), []richtext.Paragraph{paragraph}))
	}
	if value := strings.Join(parts, "<br />"); value != "" {
		return value
	}
	return tableEmptyParagraphMarkdown
}

func supportsTableHTML(tag string) bool {
	// These tags are emitted by MyGo's Markdown writer for line breaks
	// and marks that need HTML delimiters. Attributes are source-only.
	switch strings.ToLower(tag) {
	case "<br>", "<br/>", "<br />", "<strong>", "</strong>", "<em>", "</em>", "<del>", "</del>", "<u>", "</u>":
		return true
	}
	return false
}

// SupportsMarkdownTable reports whether the complete source can be inserted
// as a standalone table without losing cells.
func SupportsMarkdownTable(source string) bool {
	_, ok := singleTableRegion(source)
	return ok
}

func singleTableRegion(source string) (tableRegion, bool) {
	parsed := parser.New(Extensions)
	parsed.Set(source)
	for i := range parsed.Children(0) {
		if parsed.Block(i).Kind != parser.Table {
			break
		}
		region, ok := readTableRegion(parsed, i, 0)
		if ok && region.start == 0 && region.end == len(source) {
			return region, true
		}
		break
	}
	return tableRegion{}, false
}

// Table payloads have distinct attachment URLs for undo. Their stable IDs
// let Sync identify a change confined to tables at existing positions.
func tableProjection(markdown string, text richtext.Text) (string, []*Table) {
	if start, end := text.ParagraphRange(text.Len()); start == end && start > 0 && text.ParagraphStyleAt(start).ID == tableCaretID {
		// The trailing paragraph exists for the caret after last-row deletion.
		markdown = strings.TrimSuffix(markdown, "\n\n")
	}
	var tables []*Table
	var replacements []string
	seen := make(map[*Table]bool)
	for _, run := range text.Runs(0, text.Len()) {
		object := run.Attributes.Attachment
		if object == nil {
			continue
		}
		table, ok := object.Data.(*Table)
		if !ok {
			continue
		}
		tables = append(tables, table)
		if !seen[table] {
			seen[table] = true
			replacements = append(replacements, attachmentMarkdown(object), "!["+tableLabel+"](cyberwriter-table-id:"+table.id+")")
		}
	}
	if len(tables) > 0 {
		markdown = strings.NewReplacer(replacements...).Replace(markdown)
	}
	return markdown, tables
}

// patchTableSource preserves every byte outside the validated table ranges,
// including Markdown that the rich-text writer would otherwise normalize.
func (s *Session) patchTableSource(projection string, tables []*Table) (string, bool) {
	if len(tables) != len(s.visualTables) {
		return s.patchTableRemoval(projection, tables)
	}
	if projection != s.tableProjection || len(tables) == 0 || len(tables) != len(s.tables) {
		return "", false
	}
	var next strings.Builder
	start := 0
	for i, table := range tables {
		region := s.tables[i]
		previous := s.visualTables[i]
		if table.id != previous.id || s.source[region.start:region.end] != previous.markdown {
			return "", false
		}
		next.WriteString(s.source[start:region.start])
		next.WriteString(table.markdown)
		start = region.end
	}
	next.WriteString(s.source[start:])
	return next.String(), true
}

func (s *Session) patchTableRemoval(projection string, tables []*Table) (string, bool) {
	previousProjection, remainingProjection := s.tableProjection, projection
	previousTables, remainingTables := s.visualTables, tables
	restoring := len(tables) > len(s.visualTables)
	if restoring {
		previousProjection, remainingProjection = remainingProjection, previousProjection
		previousTables, remainingTables = remainingTables, previousTables
	}
	if len(previousTables) != len(remainingTables)+1 {
		return "", false
	}
	index := 0
	for index < len(remainingTables) && previousTables[index] == remainingTables[index] {
		index++
	}
	table := previousTables[index]
	removal := table.removal
	if removal == nil || previousProjection != removal.before || remainingProjection != removal.after {
		return "", false
	}
	with := removal.without[:removal.start] + table.markdown + removal.without[removal.start:]
	if restoring && s.source == removal.without {
		return with, true
	}
	if !restoring && s.source == with {
		return removal.without, true
	}
	return "", false
}
