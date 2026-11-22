package wysiwyg

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type tableCell struct{ start, end int }

type tableRegion struct {
	start, end int
	rows       [][]tableCell
	delimiter  []tableCell
	align      []parser.Align
}

// TableEdit holds the Markdown cell ranges from the draft when editing starts.
type TableEdit struct {
	draft   *editor.Buffer
	version uint64
	source  string
	region  *tableRegion
}

func (s *Session) TableCount() int { return len(s.tables) }

func (s *Session) EditTable(index int) *TableEdit {
	if index < 0 || index >= len(s.tables) {
		return nil
	}
	edit := s.NewTable()
	edit.region = &s.tables[index]
	return edit
}

func (s *Session) NewTable() *TableEdit {
	s.materializeProjection()
	return &TableEdit{draft: s.draft, version: s.draftVersion, source: s.source}
}

// Rows returns editable Markdown cell text, beginning with the header.
func (t *TableEdit) Rows() [][]string {
	if t.region == nil {
		return [][]string{{"Header 1", "Header 2"}, {"", ""}}
	}
	rows := make([][]string, len(t.region.rows))
	for r, cells := range t.region.rows {
		rows[r] = make([]string, len(cells))
		for c, cell := range cells {
			rows[r][c] = t.source[cell.start:cell.end]
		}
	}
	return rows
}

func (t *TableEdit) Align(column int) parser.Align {
	if t.region != nil && column >= 0 && column < len(t.region.align) {
		return t.region.align[column]
	}
	return parser.AlignNone
}

var errStaleTable = errors.New("The note changed while this table was open. Reopen the table to edit the current version.")
var errInvalidTable = errors.New("The edited cells do not form a table here. Check the cell Markdown or use Edit Source.")

// Apply validates a single table in context and updates the shared draft as
// one undo step. Like Session.Sync, callers apply on the window's UI thread.
func (t *TableEdit) Apply(draft *editor.Buffer, rows [][]string) error {
	if draft != t.draft || draft.Version() != t.version {
		return errStaleTable
	}
	next, _, err := t.editedSource(rows)
	if err != nil || next == t.source {
		return err
	}
	if draft.Version() != t.version {
		return errStaleTable
	}
	draft.Edit(changedRange(t.source, next))
	return nil
}

func (t *TableEdit) editedSource(rows [][]string) (string, tableRegion, error) {
	if len(rows) == 0 || len(rows[0]) == 0 ||
		t.region != nil && (len(rows) < len(t.region.rows) || len(rows[0]) < len(t.region.align)) {
		return "", tableRegion{}, errors.New("Keep the existing table rows and columns when applying changes.")
	}
	columns := len(rows[0])
	values := make([][]string, len(rows))
	for r, row := range rows {
		if len(row) != columns {
			return "", tableRegion{}, errors.New("Every row needs the same number of cells.")
		}
		values[r] = make([]string, columns)
		for c, value := range row {
			if !utf8.ValidString(value) || strings.ContainsAny(value, "\r\n\x00") {
				return "", tableRegion{}, fmt.Errorf("Table row %d, column %d needs a single line of Markdown.", r+1, c+1)
			}
			values[r][c] = escapeTablePipes(strings.Trim(value, " \t"))
		}
	}

	start, end := len(t.source), len(t.source)
	var before, after string
	if t.region != nil {
		start, end = t.region.start, t.region.end
	} else {
		prefix, _, body := workspace.SplitFrontmatter(t.source)
		if body != "" || strings.TrimPrefix(prefix, "\ufeff") != "" {
			switch {
			case strings.HasSuffix(t.source, "\n\n"):
			case strings.HasSuffix(t.source, "\n"):
				before = "\n"
			default:
				before = "\n\n"
			}
		}
		if strings.HasSuffix(t.source, "\n") {
			after = "\n"
		}
	}

	buildNext := func(table string) string {
		return t.source[:start] + before + table + after + t.source[end:]
	}
	preserveLayout := t.region != nil && columns == len(t.region.align)
	table := t.markdown(values, preserveLayout)
	next := buildNext(table)
	tableStart := start + len(before)
	region, ok := t.validTable(next, tableStart, tableStart+len(table), values)
	if !ok && preserveLayout {
		// A cleared cell can make an interior pipe look like an
		// outer one. Try writing explicit boundaries for each row.
		table = t.markdown(values, false)
		next = buildNext(table)
		region, ok = t.validTable(next, tableStart, tableStart+len(table), values)
	}
	if !ok {
		return "", tableRegion{}, errInvalidTable
	}
	return next, region, nil
}

func (t *TableEdit) markdown(rows [][]string, preserveLayout bool) string {
	var b strings.Builder
	if preserveLayout {
		// Copy the whitespace, delimiters and all other bytes between cells.
		pos := t.region.start
		for r, cells := range t.region.rows {
			for c, cell := range cells {
				value := rows[r][c]
				b.WriteString(t.source[pos:cell.start])
				b.WriteString(value)
				// Keep a newly entered backslash from escaping a separator
				// in a compact row such as |first|second|.
				if cell.end < t.region.end && t.source[cell.end] == '|' &&
					strings.HasSuffix(value, `\`) && value != t.source[cell.start:cell.end] {
					b.WriteByte(' ')
				}
				pos = cell.end
			}
		}
		b.WriteString(t.source[pos:t.region.end])
		for _, row := range rows[len(t.region.rows):] {
			b.WriteByte('\n')
			writeTableRow(&b, row)
		}
		return b.String()
	}

	return writeTableMarkdown(rows, t.delimiters(len(rows[0])))
}

func (t *TableEdit) delimiters(columns int) []string {
	delimiter := make([]string, columns)
	for c := range delimiter {
		delimiter[c] = "---"
		if t.region != nil && c < len(t.region.delimiter) {
			cell := t.region.delimiter[c]
			delimiter[c] = t.source[cell.start:cell.end]
		}
	}
	return delimiter
}

func writeTableMarkdown(rows [][]string, delimiter []string) string {
	var b strings.Builder
	writeTableRow(&b, rows[0])
	b.WriteByte('\n')
	writeTableRow(&b, delimiter)
	for _, row := range rows[1:] {
		b.WriteByte('\n')
		writeTableRow(&b, row)
	}
	return b.String()
}

func writeTableRow(b *strings.Builder, row []string) {
	b.WriteByte('|')
	for _, cell := range row {
		b.WriteByte(' ')
		b.WriteString(cell)
		b.WriteString(" |")
	}
}

func (t *TableEdit) validTable(source string, start, end int, rows [][]string) (tableRegion, bool) {
	prefix, _, body := workspace.SplitFrontmatter(source)
	parsed := parser.New(Extensions)
	parsed.Set(body)
	for i := range parsed.Children(0) {
		block := parsed.Block(i)
		if block.Kind != parser.Table || block.Start+len(prefix) != start {
			continue
		}
		table, ok := readTableRegion(parsed, i, len(prefix))
		if !ok || table.end != end || len(table.rows) != len(rows) || len(table.align) != len(rows[0]) {
			return tableRegion{}, false
		}
		for c, align := range table.align {
			if align != t.Align(c) {
				return tableRegion{}, false
			}
		}
		for r, cells := range table.rows {
			for c, cell := range cells {
				if source[cell.start:cell.end] != rows[r][c] {
					return tableRegion{}, false
				}
			}
		}
		return table, true
	}
	return tableRegion{}, false
}

func readTableRegion(parsed *parser.Document, index, base int) (tableRegion, bool) {
	block := parsed.Block(index)
	source := parsed.Source()
	// The AST's end can be inside the last row, before its trailing
	// pipe, padding or overflow cells. Inspect every byte on that line.
	end := len(source)
	if lineEnd := strings.IndexByte(source[block.End:], '\n'); lineEnd >= 0 {
		end = block.End + lineEnd
	}
	markdown := source[block.Start:end]
	if !utf8.ValidString(markdown) || strings.ContainsAny(markdown, "\r\x00") {
		return tableRegion{}, false
	}
	var lines [][]tableCell
	pos := base + block.Start
	for line := range strings.SplitSeq(markdown, "\n") {
		cells := splitTableLine(line, pos)
		if len(cells) != len(block.Align) {
			return tableRegion{}, false
		}
		lines = append(lines, cells)
		pos += len(line) + 1
	}
	if len(lines) < 2 {
		return tableRegion{}, false
	}
	delimiter := lines[1]
	rows := slices.Delete(lines, 1, 2)
	r := 0
	for row := range parsed.Children(index) {
		if r >= len(rows) {
			return tableRegion{}, false
		}
		c := 0
		for cell := range parsed.Children(row) {
			block := parsed.Block(cell)
			if c >= len(rows[r]) || rows[r][c] != (tableCell{base + block.Start, base + block.End}) {
				return tableRegion{}, false
			}
			c++
		}
		if c != len(rows[r]) {
			return tableRegion{}, false
		}
		r++
	}
	if r != len(rows) {
		return tableRegion{}, false
	}
	return tableRegion{
		start: base + block.Start, end: base + end,
		rows: rows, delimiter: delimiter, align: slices.Clone(block.Align),
	}, true
}

func (r tableRegion) relative() tableRegion {
	for _, cells := range append(r.rows, r.delimiter) {
		for col, cell := range cells {
			cells[col] = tableCell{cell.start - r.start, cell.end - r.start}
		}
	}
	r.start, r.end = 0, r.end-r.start
	return r
}

func readTableRegions(parsed *parser.Document, base int) []tableRegion {
	var regions []tableRegion
	for i := range parsed.Children(0) {
		if parsed.Block(i).Kind == parser.Table {
			if region, ok := readTableRegion(parsed, i, base); ok {
				regions = append(regions, region)
			}
		}
	}
	return regions
}

// splitTableLine retains every explicit cell, including trailing overflow
// cells that the rendered table truncates. The AST confirms the ranges.
func splitTableLine(line string, base int) []tableCell {
	left := len(line) - len(strings.TrimLeft(line, " \t"))
	right := len(strings.TrimRight(line, " \t"))
	if left >= right {
		return nil
	}
	start := left
	hasPipe := line[start] == '|'
	if hasPipe {
		start++
	}
	var cells []tableCell
	escaped := false
	for i := start; i < right; i++ {
		if line[i] == '|' && !escaped {
			cells = append(cells, trimmedTableCell(line, base, start, i))
			start = i + 1
			hasPipe = true
		}
		escaped = line[i] == '\\' && !escaped
	}
	if !hasPipe {
		return nil
	}
	if start < right {
		cells = append(cells, trimmedTableCell(line, base, start, right))
	}
	return cells
}

func trimmedTableCell(line string, base, start, end int) tableCell {
	text := line[start:end]
	start += len(text) - len(strings.TrimLeft(text, " \t"))
	end = start + len(strings.Trim(line[start:end], " \t"))
	return tableCell{base + start, base + end}
}

func escapeTablePipes(value string) string {
	if !strings.Contains(value, "|") {
		return value
	}
	var b strings.Builder
	escaped := false
	for i := range len(value) {
		if value[i] == '|' && !escaped {
			b.WriteByte('\\')
		}
		b.WriteByte(value[i])
		escaped = value[i] == '\\' && !escaped
	}
	return b.String()
}
