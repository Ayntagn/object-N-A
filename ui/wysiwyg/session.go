// Package wysiwyg projects Markdown drafts into MyGo's rich text editor.
package wysiwyg

import (
	"maps"
	"regexp"
	"strings"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

const Extensions = parser.GFM | parser.ExtAlerts | parser.ExtFootnotes | parser.ExtMath | parser.ExtDollarMath

// Session binds a visual document to one versioned Markdown draft.
type Session struct {
	// Document is nil when the body cannot round-trip through rich text.
	Document   *richtext.Document
	Properties string
	Body       string

	draft            *editor.Buffer
	draftVersion     uint64
	visualVersion    uint64
	original, source string
	prefix, suffix   string
	initialMarkdown  string
	lastMarkdown     string
	tables           []tableRegion
	tableProjection  string
	visualTables     []*Table
	wikilinks        map[string]string
	pageBreak        *richtext.Attachment
}

func Open(draft *editor.Buffer) *Session {
	snapshot := draft.Snapshot()
	source := snapshot.String()
	prefix, properties, body := workspace.SplitFrontmatter(source)
	// Keep the original space between the properties and the body, and
	// the original terminal newlines, outside the visual document.
	start := len(body) - len(strings.TrimLeft(body, "\n"))
	prefix += body[:start]
	inner := strings.TrimRight(body[start:], "\n")
	session := &Session{
		Body: body, Properties: properties, draft: draft, draftVersion: snapshot.Version(),
		original: source, source: source, prefix: prefix, suffix: body[start+len(inner):],
	}
	parsed := parser.New(Extensions)
	parsed.Set(inner)
	session.tables = readTableRegions(parsed, len(prefix))
	visual, markdown, links, ok := session.projectMarkdown(parsed, inner)
	if !ok {
		return session
	}
	if strings.HasSuffix(inner, dividerMarkdown) && session.suffix == "\n\n" &&
		session.IsDivider(visual, visual.Len()-1) {
		// A final divider's source includes an empty paragraph for the caret.
		visual = visual.Append("\n", richtext.Attributes{})
		end := visual.Len()
		visual = visual.FormatParagraphs(end, end, func(p *richtext.ParagraphStyle) {
			*p = richtext.ParagraphStyle{ID: reopenedDividerCaretID}
		})
		markdown = visualMarkdown(visual)
	}
	if f, ok := FormulaAt(visual, visual.Len()-1); ok && f.Block && session.suffix == "\n\n" {
		visual = visual.Append("\n", richtext.Attributes{})
		end := visual.Len()
		visual = visual.FormatParagraphs(end, end, func(p *richtext.ParagraphStyle) {
			*p = richtext.ParagraphStyle{ID: mathCaretID}
		})
		markdown = visualMarkdown(visual)
	}
	if p, ok := ImageAt(visual, visual.Len()-1); ok && p.Block && session.suffix == "\n\n" {
		visual = visual.Append("\n", richtext.Attributes{})
		end := visual.Len()
		visual = visual.FormatParagraphs(end, end, func(style *richtext.ParagraphStyle) { style.ID = imageCaretID })
		markdown = visualMarkdown(visual)
	}
	if _, ok := queryAt(visual, visual.Len()-1); ok && session.suffix == "\n\n" {
		visual = visual.Append("\n", richtext.Attributes{})
		end := visual.Len()
		visual = visual.FormatParagraphs(end, end, func(style *richtext.ParagraphStyle) { style.ID = queryCaretID })
		markdown = visualMarkdown(visual)
	}
	if _, ok := apiAt(visual, visual.Len()-1); ok && session.suffix == "\n\n" {
		visual = visual.Append("\n", richtext.Attributes{})
		end := visual.Len()
		visual = visual.FormatParagraphs(end, end, func(style *richtext.ParagraphStyle) { style.ID = apiCaretID })
		markdown = visualMarkdown(visual)
	}
	session.Document = richtext.NewDocument(visual)
	session.visualVersion = session.Document.Version()
	session.initialMarkdown, session.lastMarkdown = markdown, markdown
	session.tableProjection, session.visualTables = tableProjection(markdown, visual)
	session.wikilinks = links
	return session
}

// InsertMarkdown pastes Markdown into the visual selection as one undo step.
func (s *Session) InsertMarkdown(source string) bool {
	if s.Document == nil {
		return false
	}
	anchor, caret := s.Document.Selection()
	return s.InsertMarkdownAt(min(anchor, caret), max(anchor, caret), source)
}

// InsertMarkdownAt writes to a rune range in one step without moving the
// current selection before recording that step.
func (s *Session) InsertMarkdownAt(start, end int, source string) bool {
	if s.Document == nil || start < 0 || start > end || end > s.Document.Text().Len() {
		return false
	}
	parsed := parser.New(Extensions)
	parsed.Set(source)
	text, _, links, ok := s.projectMarkdown(parsed, source)
	if !ok {
		return false
	}
	s.rememberWikilinks(links)
	s.Document.Edit(func(edit *richtext.Edit) {
		paste := text
		firstIsBlock := standaloneBlockAt(text, 0) || CalloutParagraphID(text, 0) != ""
		lastIsBlock := standaloneBlockAt(text, text.Len()-1) || CalloutParagraphID(text, text.Len()) != ""
		if firstIsBlock || lastIsBlock {
			var paragraphs []richtext.Paragraph
			if firstIsBlock && start > 0 && edit.Text().Slice(start-1, start).String() != "\n" {
				paragraphs = append(paragraphs, richtext.Paragraph{Style: edit.Text().ParagraphStyleAt(start)})
			}
			for _, paragraph := range text.Paragraphs() {
				paragraphs = append(paragraphs, paragraph)
			}
			if lastIsBlock {
				paragraphs = append(paragraphs, richtext.Paragraph{})
			}
			paste = richtext.TextOf(paragraphs...)
		}
		edit.Replace(start, end, paste)
		// MyGo assigns fresh IDs when it pastes paragraphs.
		for offset, paragraph := range paste.Paragraphs() {
			if id := paragraph.Style.ID; id != "" {
				edit.FormatParagraphs(start+offset, start+offset, func(style *richtext.ParagraphStyle) {
					if style.CodeBlock || isTableExit(paragraph.Style) || calloutID(paragraph.Style) != "" {
						style.ID = id
					}
				})
			}
		}
		edit.Select(start+paste.Len(), start+paste.Len())
	})
	return true
}

func projectMarkdown(parsed *parser.Document, source string, formulas map[string]*Formula, images map[string]*Image) (richtext.Text, string, bool) {
	projected, highlights := projectHighlights(parsed)
	if !supported(projected, formulas, images) {
		return richtext.Text{}, "", false
	}
	visual := richtext.ParseMarkdown(projected.Source())
	if projected != parsed {
		var ok bool
		visual, ok = applyHighlightDelimiters(visual, highlights)
		if !ok {
			return richtext.Text{}, "", false
		}
	}
	visual, ok := tagCodeFences(parsed, tagEmptyHeadings(visual))
	if !ok {
		return richtext.Text{}, "", false
	}
	markdown := visualMarkdown(visual)
	if source != markdown {
		projection := parser.New(Extensions)
		projection.Set(markdown)
		before, after := projected.HTML(), projection.HTML()
		if projected != parsed {
			strip := strings.NewReplacer(highlights.open, "", highlights.close, "")
			before = strip.Replace(before)
			after = strip.Replace(projectHighlightDelimiters(projection, highlights).HTML())
		}
		if softBreaksAsSpaces(before) != softBreaksAsSpaces(after) {
			return richtext.Text{}, "", false
		}
	}
	return visual, markdown, true
}

func (s *Session) Current(draft *editor.Buffer) bool {
	return s.draft == draft && s.draftVersion == draft.Version()
}

// RefreshFrontmatter keeps the rich editor and its selection when only the
// source metadata changed. The caller syncs pending rich-text edits first.
func (s *Session) RefreshFrontmatter(draft *editor.Buffer) {
	if s.draft != draft || s.Document == nil {
		return
	}
	snapshot := draft.Snapshot()
	next := snapshot.String()
	_, _, previousBody := workspace.SplitFrontmatter(s.source)
	prefix, properties, body := workspace.SplitFrontmatter(next)
	leading := len(body) - len(strings.TrimLeft(body, "\n"))
	if strings.TrimLeft(previousBody, "\n") != body[leading:] || !strings.HasPrefix(s.original, s.prefix) {
		return
	}
	prefix += body[:leading]
	s.original = prefix + s.original[len(s.prefix):]
	s.prefix, s.Properties, s.Body = prefix, properties, body
	s.source, s.draftVersion = next, snapshot.Version()
	s.readDraftTables()
}

// Sync applies a visual edit to the active draft before it can be saved
// or replaced. An old session cannot write through a newer source edit.
func (s *Session) Sync(draft *editor.Buffer) bool {
	if s.Document == nil || !s.Current(draft) {
		return false
	}
	version := s.Document.Version()
	if version == s.visualVersion {
		return false
	}
	s.visualVersion = version
	text := splitAttachmentParagraphs(s.Document.Text())
	markdown := visualMarkdown(text)
	projection, tables := tableProjection(markdown, text)
	next, patched := s.patchTableSource(projection, tables)
	if markdown == s.initialMarkdown {
		next = s.original
	} else if fencePatch, ok := s.patchFencedSource(markdown, text); ok {
		next = fencePatch
	} else if formulaPatch, ok := s.patchFormulaSource(markdown, text); ok {
		next = formulaPatch
	} else if calloutPatch, ok := s.patchCalloutSource(markdown, text); ok {
		next = calloutPatch
	} else if imagePatch, ok := s.patchImageSource(markdown, text); ok {
		next = imagePatch
	} else if !patched {
		body := restoreWikilinks(s.restorePageBreaks(restoreAttachments(markdown, text)), s.wikilinks)
		suffix := s.suffix
		end := text.Len()
		caretID := text.ParagraphStyleAt(end).ID
		if formula, ok := FormulaAt(text, end-2); caretID == mathCaretID && ok && formula.Block && strings.HasSuffix(body, "\n\n") {
			suffix = ""
		} else if chart, ok := diagramAt(text, end-2); caretID == vegaCaretID && ok && chart.VegaMode() != "" && strings.HasSuffix(body, "\n\n") {
			suffix = ""
		} else if _, ok := queryAt(text, end-2); caretID == queryCaretID && ok && strings.HasSuffix(body, "\n\n") {
			suffix = ""
		} else if _, ok := apiAt(text, end-2); caretID == apiCaretID && ok && strings.HasSuffix(body, "\n\n") {
			suffix = ""
		} else if picture, ok := ImageAt(text, end-2); caretID == imageCaretID && ok && picture.Block && strings.HasSuffix(body, "\n\n") {
			suffix = ""
		} else if (caretID == dividerCaretID || caretID == reopenedDividerCaretID) &&
			s.IsDivider(text, end-2) && strings.HasSuffix(body, "\n\n") {
			// The trailing caret paragraph already supplies the final two LFs.
			suffix = ""
		} else if standaloneBlockAt(text, end-2) && strings.HasSuffix(text.String(), "\n") {
			// A trailing empty paragraph already emits a line break.
			body = strings.TrimSuffix(body, "\n")
		}
		if caretID == reopenedDividerCaretID && suffix == "\n\n" {
			suffix = "\n"
		}
		if body != "" && strings.TrimPrefix(s.prefix, "\ufeff") != "" && !strings.HasSuffix(s.prefix, "\n") {
			body = "\n\n" + body
		}
		next = s.prefix + body + suffix
	}
	s.lastMarkdown = markdown
	s.tableProjection, s.visualTables = projection, tables
	if next == s.source {
		return false
	}
	draft.Edit(changedRange(s.source, next))
	s.source, s.draftVersion = next, draft.Version()
	if len(s.tables) != 0 || len(tables) != 0 {
		s.readDraftTables()
	}
	return true
}

func (s *Session) readDraftTables() {
	prefix, _, body := workspace.SplitFrontmatter(s.source)
	parsed := parser.New(Extensions)
	parsed.Set(body)
	s.tables = readTableRegions(parsed, len(prefix))
}

func (s *Session) rememberWikilinks(links map[string]string) {
	if s.wikilinks == nil {
		s.wikilinks = links
	} else {
		maps.Copy(s.wikilinks, links)
	}
}

// SelectionMarkdown returns the current visual selection as Markdown,
// with the terminal LF emitted by the native visual selection serializer.
func (s *Session) SelectionMarkdown() string {
	a, z := s.Document.Selection()
	if a == z {
		return ""
	}
	text := s.Document.Text().Slice(min(a, z), max(a, z))
	value := restoreWikilinks(s.restorePageBreaks(restoreAttachments(visualMarkdown(text), text)), s.wikilinks)
	return strings.TrimRight(value, "\n") + "\n"
}

// A reference definition can disappear from the parser's rendered tree,
// including when no inline uses it.
var referenceDefinition = regexp.MustCompile(`(?m)^[ \t]*(?:>[ \t]*|(?:[-+*]|[0-9]{1,9}[.)])[ \t]+)*\[[^\n]+\]:`)

func supported(doc *parser.Document, formulas map[string]*Formula, images map[string]*Image) bool {
	source := doc.Source()
	start := 0
	for i := 1; i < doc.Len(); i++ {
		block := doc.Block(i)
		switch block.Kind {
		case parser.Table, parser.HTMLBlock, parser.FootnoteDefinition, parser.MathBlock:
			return false
		case parser.Quote:
			if block.Alert != parser.NoAlert {
				return false
			}
		case parser.CodeBlock:
			plainBash := block.Info() == "bash" && block.Parent == 0 && block.Fenced && !block.Open
			if (block.Info() != "" && !plainBash) || referenceDefinition.MatchString(source[start:block.Start]) {
				return false
			}
			start = block.End
		case parser.Paragraph, parser.Heading:
			for _, inline := range doc.Inlines(i) {
				if inline.Kind == parser.Image && (formulas[inline.Dest] != nil || images[inline.Dest] != nil) {
					continue
				}
				if !supportsVisualInline(inline) {
					return false
				}
			}
		}
	}
	return !referenceDefinition.MatchString(source[start:])
}

func supportsVisualInline(inline parser.Inline) bool {
	switch inline.Kind {
	case parser.FootnoteRef, parser.Math, parser.Image:
		return false
	case parser.Text:
		text := strings.ToLower(inline.Text)
		return !strings.Contains(text, "[[") &&
			!strings.Contains(text, "[pagebreak]") && !strings.Contains(text, "==")
	}
	return true
}

// In CommonMark's HTML output, a soft line break is interchangeable
// with a space. Fenced code is preserved separately by RichText.
func softBreaksAsSpaces(html string) string {
	return strings.ReplaceAll(html, "\n", " ")
}

// changedRange leaves the draft's surrounding bytes and source selections
// intact. A shared prefix or suffix can end partway through a UTF-8 rune.
func changedRange(before, after string) editor.Edit {
	start := 0
	for start < min(len(before), len(after)) && before[start] == after[start] {
		start++
	}
	for start > 0 && start < len(before) && !utf8.RuneStart(before[start]) {
		start--
	}
	oldEnd, newEnd := len(before), len(after)
	for oldEnd > start && newEnd > start && before[oldEnd-1] == after[newEnd-1] {
		oldEnd--
		newEnd--
	}
	for oldEnd < len(before) && !utf8.RuneStart(before[oldEnd]) {
		oldEnd++
		newEnd++
	}
	return editor.Edit{Start: start, End: oldEnd, Text: after[start:newEnd]}
}
