package wysiwyg

import (
	"crypto/rand"
	"strings"

	"cyberwriter/internal/core/callout"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

const calloutPrefix = "cyberwriter-callout:"
const (
	CalloutTitleTag = "cyberwriter-callout-title"
	CalloutBodyTag  = "cyberwriter-callout-body"
	CalloutIconTag  = "cyberwriter-callout-icon"
)

// Callout keeps the source header and the original body independently of
// the attributed paragraphs that the visual editor can edit and undo.
type Callout struct {
	id, Kind, heading, rawTitle, rawBody string
	title, body                          string
	icon                                 *richtext.Attachment
}

type calloutRegion struct {
	start, end           int
	kind, heading, title string
	quotedBody, body     string
}

func readCalloutHeader(line string) (kind string, titleStart int) {
	header, ok := callout.ParseHeader(line)
	if !ok {
		return "", 0
	}
	return header.Kind, header.TitleOffset
}

func readCalloutRegions(doc *parser.Document) []calloutRegion {
	var out []calloutRegion
	source := doc.Source()
	for index := range doc.Children(0) {
		block := doc.Block(index)
		if block.Kind != parser.Quote {
			continue
		}
		start := strings.LastIndexByte(source[:block.Start], '\n') + 1
		end := min(block.End, len(source))
		raw := strings.TrimSuffix(source[start:end], "\n")
		head, body, hasBody := strings.Cut(raw, "\n")
		offset := quotePrefixLength(head)
		if offset == 0 {
			continue
		}
		kind, titleStart := readCalloutHeader(head[offset:])
		if kind == "" {
			continue
		}
		titleStart += offset
		region := calloutRegion{start: start, end: start + len(raw), kind: kind, heading: head[:titleStart], title: head[titleStart:]}
		if hasBody {
			region.quotedBody = "\n" + body
			region.body = projectCalloutBody(body)
		}
		out = append(out, region)
	}
	return out
}

func quotePrefixLength(line string) int {
	n := len(line) - len(strings.TrimLeft(line, " \t"))
	if n >= len(line) || line[n] != '>' {
		return 0
	}
	n++
	if n < len(line) && (line[n] == ' ' || line[n] == '\t') {
		n++
	}
	return n
}

func calloutTitle(source string) string {
	// A leading ordinary character keeps Markdown inline on the title
	// line even when the title starts with a heading or list marker.
	doc := parser.New(parser.GFM)
	doc.Set("a " + source)
	var title strings.Builder
	for _, token := range doc.Inlines(1) {
		if token.Kind == parser.Text {
			title.WriteString(token.Text)
		}
	}
	return strings.TrimSpace(strings.TrimPrefix(title.String(), "a"))
}

func newCallout(region calloutRegion, body richtext.Text) *Callout {
	id := calloutPrefix + rand.Text()
	c := &Callout{
		id: id, Kind: region.kind, heading: region.heading, rawTitle: region.title, rawBody: region.quotedBody,
		title: calloutTitle(region.title), body: visualMarkdown(body),
	}
	c.icon = &richtext.Attachment{
		Name: "Callout", URL: id, Data: c, Width: 21, Height: 16,
		Draw: func(*mygoui.Painter, mygoui.Rect) {},
	}
	return c
}

func (c *Callout) ID() string { return c.id }

func (c *Callout) SetIcon(width, ascent float32, draw func(*mygoui.Painter, mygoui.Rect)) bool {
	changed := c.icon.Width != width || c.icon.Height != ascent
	c.icon.Width, c.icon.Height, c.icon.Draw = width, ascent, draw
	return changed
}

func (c *Callout) paragraphs(body richtext.Text) []richtext.Paragraph {
	titleAttributes := richtext.Attributes{Tag: CalloutTitleTag}
	marker := richtext.Attributes{Tag: CalloutIconTag, Attachment: c.icon}
	title := richtext.Paragraph{Style: richtext.ParagraphStyle{Quote: true, ID: c.id},
		Runs: []richtext.Run{{Text: string(richtext.ObjectReplacement), Attributes: marker}}}
	if c.title != "" {
		title.Runs = append(title.Runs, richtext.Run{Text: c.title, Attributes: titleAttributes})
	}
	parts := []richtext.Paragraph{title}
	for _, p := range body.Paragraphs() {
		p.Style = calloutChildStyle(c.id, p.Style)
		for i := range p.Runs {
			tagCalloutBody(&p.Runs[i].Attributes)
		}
		parts = append(parts, p)
	}
	return parts
}

func calloutID(style richtext.ParagraphStyle) string {
	if !style.Quote || !strings.HasPrefix(style.ID, calloutPrefix) {
		return ""
	}
	id, _, _ := strings.Cut(style.ID, "/")
	return id
}

func calloutChildStyle(id string, style richtext.ParagraphStyle) richtext.ParagraphStyle {
	role := "body"
	if style.Quote {
		role = "quote"
	}
	style.ID = id + "/" + role + "/" + style.ID
	style.Quote = true
	return style
}

func CalloutParagraphID(text richtext.Text, offset int) string {
	return calloutID(text.ParagraphStyleAt(offset))
}

func CalloutAt(text richtext.Text, offset int) *Callout {
	if offset < 0 || offset > text.Len() || !text.ParagraphStyleAt(offset).Quote {
		return nil
	}
	start, end := text.ParagraphRange(offset)
	for _, run := range text.Runs(start, end) {
		if card := calloutObject(run.Attributes); card != nil {
			return card
		}
	}
	return nil
}

func calloutObject(attrs richtext.Attributes) *Callout {
	if attrs.Attachment != nil {
		card, _ := attrs.Attachment.Data.(*Callout)
		return card
	}
	return nil
}

func tagCalloutBody(attrs *richtext.Attributes) {
	if attrs.Tag == "" && attrs.Color.A == 0 && attrs.Attachment == nil && attrs.Link == "" && !attrs.Code {
		attrs.Tag = CalloutBodyTag
	}
}

// PrepareCalloutTyping continues the card's visual styling through a
// committed insertion, including at the edges where MyGo drops app tags.
func (s *Session) PrepareCalloutTyping() {
	doc := s.Document
	a, z := doc.Selection()
	text := doc.Text()
	start, end := text.ParagraphRange(min(a, z))
	if max(a, z) > end || CalloutParagraphID(text, start) == "" {
		return
	}
	attrs := doc.TypingAttributes()
	if CalloutAt(text, start) != nil {
		if calloutObject(text.AttributesAt(start)) != nil && min(a, z) == start {
			doc.SetSelection(max(a, start+1), max(z, start+1))
		}
		attrs.Tag = CalloutTitleTag
	} else {
		tagCalloutBody(&attrs)
	}
	doc.SetTypingAttributes(attrs)
}

func (s *Session) projectCallouts(doc *parser.Document, source string) (string, map[string][]richtext.Paragraph, map[string]visualWikilink, bool) {
	regions := readCalloutRegions(doc)
	if len(regions) == 0 {
		return source, nil, nil, true
	}
	var out strings.Builder
	groups := make(map[string][]richtext.Paragraph, len(regions))
	links := make(map[string]visualWikilink)
	start := 0
	for _, region := range regions {
		parsed := parser.New(Extensions)
		parsed.Set(region.body)
		body, _, bodyLinks, ok := s.projectVisualMarkdown(parsed, region.body, false)
		if !ok {
			return "", nil, nil, false
		}
		for key, target := range bodyLinks {
			links[key] = target
		}
		c := newCallout(region, body)
		groups[c.id] = c.paragraphs(body)
		out.WriteString(source[start:region.start])
		out.WriteString("\n\n[Callout](" + c.id + ")\n\n")
		start = region.end
	}
	out.WriteString(source[start:])
	return out.String(), groups, links, true
}

// visualCalloutMarkdown supplies the outer quote for the current title
// and body. Untouched header and body bytes are taken from the loaded note.
func visualCalloutMarkdown(c *Callout, title richtext.Paragraph, body []richtext.Paragraph) string {
	title.Style = richtext.ParagraphStyle{}
	var titleRuns []richtext.Run
	for _, run := range title.Runs {
		if calloutObject(run.Attributes) != nil {
			continue
		}
		titleRuns = append(titleRuns, run)
	}
	title.Runs = titleRuns
	titleText := richtext.TextOf(title)
	titleMarkdown := c.rawTitle
	if titleText.String() != c.title {
		titleMarkdown = titleText.Markdown()
	}
	paragraphs := append([]richtext.Paragraph(nil), body...)
	for i := range paragraphs {
		style := &paragraphs[i].Style
		_, child, _ := strings.Cut(style.ID, "/")
		kind, previousID, _ := strings.Cut(child, "/")
		style.Quote = kind == "quote"
		style.ID = previousID
		if len(paragraphs[i].Runs) == 0 && !style.CodeBlock && style.Heading == 0 && style.List == richtext.NoList {
			style.ID = tableExitID
		}
	}
	bodyText := richtext.TextOf(paragraphs...)
	bodyMarkdown := visualMarkdown(bodyText)
	if bodyMarkdown == c.body || c.body == "" && bodyMarkdown == tableEmptyParagraphMarkdown {
		return c.heading + titleMarkdown + c.rawBody
	}
	var out strings.Builder
	out.WriteString(c.heading + titleMarkdown)
	if bodyMarkdown == "" {
		return out.String()
	}
	if first, inserted := strings.CutSuffix(bodyMarkdown, "\n\n"+c.body); inserted &&
		c.body != "" && strings.HasPrefix(c.rawBody, "\n> ") &&
		first != tableEmptyParagraphMarkdown && !strings.Contains(first, "\n") {
		// The bridge places a new first paragraph before the original
		// body lines without normalizing their quote prefixes.
		out.WriteString("\n>\n> " + restoreAttachments(first, bodyText) + c.rawBody)
		return out.String()
	}
	// New callouts have the empty quote line Crepe emits. For existing
	// blocks preserve whether the source used that line or direct adjacency.
	if c.rawBody == "" || strings.HasPrefix(c.rawBody, "\n>\n") || strings.HasPrefix(c.rawBody, "\n> \n") {
		out.WriteString("\n>")
	}
	for _, line := range strings.Split(restoreAttachments(bodyMarkdown, bodyText), "\n") {
		out.WriteString("\n>")
		if line != "" {
			out.WriteByte(' ')
			out.WriteString(line)
		}
	}
	return out.String()
}

func markdownWithCallouts(paragraphs []richtext.Paragraph) (string, bool) {
	var parts []string
	start := 0
	for i := 0; i < len(paragraphs); i++ {
		title := paragraphs[i]
		var c *Callout
		for _, run := range title.Runs {
			if c = calloutObject(run.Attributes); c != nil {
				break
			}
		}
		if c == nil {
			continue
		}
		if i > start {
			parts = append(parts, visualMarkdown(richtext.TextOf(paragraphs[start:i]...)))
		}
		end := i + 1
		for end < len(paragraphs) && calloutID(paragraphs[end].Style) == c.id {
			if p := paragraphs[end]; len(p.Runs) != 0 && p.Runs[0].Attributes.Attachment == c.icon {
				break
			}
			end++
		}
		parts = append(parts, visualCalloutMarkdown(c, title, paragraphs[i+1:end]))
		start, i = end, end-1
	}
	if len(parts) == 0 {
		return "", false
	}
	if start < len(paragraphs) {
		parts = append(parts, visualMarkdown(richtext.TextOf(paragraphs[start:]...)))
	}
	return strings.Join(parts, "\n\n"), true
}

// patchCalloutSource applies a card-only change to the existing draft
// instead of reserializing the surrounding Markdown.
func (s *Session) patchCalloutSource(markdown string, text richtext.Text) (string, bool) {
	if !strings.Contains(markdown, "[!") || !strings.Contains(s.lastMarkdown, "[!") {
		return "", false
	}
	parse := func(source string) []calloutRegion {
		doc := parser.New(Extensions)
		doc.Set(source)
		return readCalloutRegions(doc)
	}
	old, next := parse(s.lastMarkdown), parse(markdown)
	if len(old) != len(next) {
		return "", false
	}
	change := changedRange(s.lastMarkdown, markdown)
	for i, previous := range old {
		current := next[i]
		if change.Start < previous.start || change.End > previous.end || change.Start+len(change.Text) > current.end {
			continue
		}
		head, _, body := workspace.SplitFrontmatter(s.source)
		original := parse(body)
		if len(original) != len(old) {
			return "", false
		}
		rangeInDraft := original[i]
		updated := restoreWikilinks(s.restorePageBreaks(restoreAttachments(markdown[current.start:current.end], text)), s.wikilinks)
		return head + body[:rangeInDraft.start] + updated + body[rangeInDraft.end:], true
	}
	return "", false
}

// InsertCallout replaces the selected paragraphs with a callout and
// leaves its body in the same rich-text document and undo transaction.
func (s *Session) InsertCallout(kind, title, body string) bool {
	return s.insertCallout(kind, title, body, nil)
}

// WrapCalloutSelection applies the toolbar template. The native Markdown
// template quotes the first paragraph only. A partial selection within one
// paragraph opens the inserted slice into that paragraph.
func (s *Session) WrapCalloutSelection(kind, title string) bool {
	if s.Document == nil || !s.Current(s.draft) {
		return false
	}
	doc := s.Document
	a, z := doc.Selection()
	start, end := min(a, z), max(a, z)
	text := doc.Text()
	first, last := text.ParagraphRange(start)
	body := s.SelectionMarkdown()
	if body == "" {
		body = "Content"
	}
	if end <= last && (start != first || end != last) {
		paragraphs := []richtext.Paragraph{{Runs: []richtext.Run{{Text: title}}}}
		if start == end {
			paragraphs = append(paragraphs, richtext.Paragraph{Runs: []richtext.Run{{Text: body}}})
		} else {
			for _, selected := range text.Slice(start, end).Paragraphs() {
				selected.Style = richtext.ParagraphStyle{}
				paragraphs = append(paragraphs, selected)
			}
		}
		doc.Insert(richtext.TextOf(paragraphs...))
		return true
	}
	firstBody, remainder, multiple := strings.Cut(body, "\n\n")
	var tail []richtext.Paragraph
	if multiple {
		parsed := parser.New(Extensions)
		parsed.Set(strings.TrimSuffix(remainder, "\n"))
		tailText, _, links, ok := s.projectMarkdown(parsed, parsed.Source())
		if !ok {
			return false
		}
		s.rememberWikilinks(links)
		for _, p := range tailText.Paragraphs() {
			tail = append(tail, p)
		}
	}
	return s.insertCallout(kind, title, firstBody, tail)
}

func (s *Session) insertCallout(kind, title, body string, tail []richtext.Paragraph) bool {
	if s.Document == nil || !s.Current(s.draft) {
		return false
	}
	a, z := s.Document.Selection()
	start, end := min(a, z), max(a, z)
	text := s.Document.Text()
	first, _ := text.ParagraphRange(start)
	last, lastEnd := text.ParagraphRange(end)
	if end == last && start != end && end > 0 {
		_, lastEnd = text.ParagraphRange(end - 1)
	}
	body = strings.TrimSuffix(body, "\n")
	parsed := parser.New(Extensions)
	parsed.Set(body)
	bodyText, _, links, ok := s.projectVisualMarkdown(parsed, body, false)
	if !ok {
		return false
	}
	s.rememberWikilinks(links)
	if bodyText.Len() == 0 {
		body = tableEmptyParagraphMarkdown
	}
	var raw strings.Builder
	raw.WriteString("\n>")
	for _, line := range strings.Split(body, "\n") {
		raw.WriteString("\n>")
		if line != "" {
			raw.WriteString(" " + line)
		}
	}
	c := newCallout(calloutRegion{kind: kind, heading: "> [!" + kind + "] ", title: title, quotedBody: raw.String()}, bodyText)
	group := c.paragraphs(bodyText)
	var paragraphs []richtext.Paragraph
	if start != first {
		paragraphs = append(paragraphs, richtext.Paragraph{Style: text.ParagraphStyleAt(start)})
	}
	paragraphs = append(paragraphs, group...)
	paragraphs = append(paragraphs, tail...)
	insert := richtext.TextOf(paragraphs...)
	caret := start + insert.Len()
	if end < text.Len() && (len(tail) == 0 || end == lastEnd) {
		if end == lastEnd {
			end++
		}
		insert = richtext.TextOf(append(paragraphs, richtext.Paragraph{})...)
	}
	s.Document.Edit(func(edit *richtext.Edit) {
		edit.Replace(start, end, insert)
		for offset, p := range insert.Paragraphs() {
			if calloutID(p.Style) != "" {
				edit.FormatParagraphs(start+offset, start+offset, func(style *richtext.ParagraphStyle) { *style = p.Style })
			}
		}
		edit.Select(caret, caret)
	})
	return true
}

// CalloutEnter handles the two structural boundaries in the shared editor.
func (s *Session) CanCalloutEnter() bool {
	doc := s.Document
	a, z := doc.Selection()
	text := doc.Text()
	p, q := text.ParagraphRange(min(a, z))
	if max(a, z) > q {
		return false
	}
	style := text.ParagraphStyleAt(p)
	if calloutID(style) != "" {
		return true
	}
	if !style.Quote || a != z || style.List != richtext.NoList || style.CodeBlock ||
		p > 0 && text.ParagraphStyleAt(p-1).Quote {
		return false
	}
	kind, _ := readCalloutHeader(strings.TrimSpace(text.Slice(p, q).String()))
	return kind != ""
}

func (s *Session) CalloutEnter() bool {
	doc := s.Document
	a, z := doc.Selection()
	start, end := min(a, z), max(a, z)
	text := doc.Text()
	p, q := text.ParagraphRange(start)
	if end > q {
		return false
	}
	style := text.ParagraphStyleAt(start)
	if id := calloutID(style); id != "" {
		title := CalloutAt(text, p) != nil
		if title {
			start, end = max(p+1, start), max(p+1, end)
		}
		attrs := doc.TypingAttributes()
		attrs.Attachment = nil
		if title {
			attrs = richtext.Attributes{Tag: CalloutBodyTag}
		}
		doc.Edit(func(edit *richtext.Edit) {
			edit.ReplaceString(start, end, "\n", attrs)
			edit.FormatParagraphs(p, p, func(next *richtext.ParagraphStyle) { *next = style })
			if title {
				style = calloutChildStyle(id, richtext.ParagraphStyle{})
			}
			style.Checked = false
			edit.FormatParagraphs(start+1, start+1, func(next *richtext.ParagraphStyle) { *next = style })
			if title {
				edit.Format(start+1, start+1+q-end, func(a *richtext.Attributes) {
					if a.Tag == CalloutTitleTag {
						a.Tag = CalloutBodyTag
					}
				})
			}
			edit.Select(start+1, start+1)
		})
		if title {
			doc.SetTypingAttributes(attrs)
		}
		return true
	}
	line := strings.TrimSpace(text.Slice(p, q).String())
	kind, at := readCalloutHeader(line)
	if kind == "" {
		return false
	}
	doc.SetSelection(p, q)
	return s.InsertCallout(kind, strings.TrimSpace(line[at:]), "")
}

func (s *Session) CanCalloutBackspace() bool {
	a, z := s.Document.Selection()
	if a != z {
		return false
	}
	text := s.Document.Text()
	p, _ := text.ParagraphRange(z)
	id := CalloutParagraphID(text, p)
	if id == "" {
		return false
	}
	if CalloutAt(text, p) != nil {
		return z == p || z == p+1 && calloutObject(text.AttributesAt(p)) != nil
	}
	return z == p && p > 0 && CalloutParagraphID(text, p-1) == id && CalloutAt(text, p-1) != nil
}

// CalloutBackspace consumes Backspace at the beginning of a title and
// joins the first body paragraph to its title at the inner boundary.
func (s *Session) CalloutBackspace() {
	if !s.CanCalloutBackspace() {
		return
	}
	doc := s.Document
	_, caret := doc.Selection()
	text := doc.Text()
	p, q := text.ParagraphRange(caret)
	if CalloutAt(text, p) != nil {
		return
	}
	doc.Edit(func(edit *richtext.Edit) {
		edit.ReplaceString(p-1, p, "", richtext.Attributes{})
		edit.Format(p-1, q-1, func(a *richtext.Attributes) {
			if a.Tag == "" || a.Tag == CalloutBodyTag {
				a.Tag = CalloutTitleTag
			}
		})
		edit.Select(p-1, p-1)
	})
}
