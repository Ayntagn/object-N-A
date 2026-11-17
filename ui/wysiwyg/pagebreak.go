package wysiwyg

import (
	"crypto/rand"
	"strings"

	"cyberwriter/internal/core/exportdoc"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

const PageBreakLabel = "PAGE BREAK"

func (s *Session) pageBreakObject() *richtext.Attachment {
	if s.pageBreak == nil {
		s.pageBreak = &richtext.Attachment{
			Name: PageBreakLabel, URL: "cyberwriter-page-break:" + rand.Text(),
			Width: 600, Height: 36, Draw: drawPageBreak,
		}
	}
	return s.pageBreak
}

// IsPageBreak identifies a visual marker by its session-owned attachment.
func (s *Session) IsPageBreak(text richtext.Text, offset int) bool {
	return s.pageBreak != nil && text.AttributesAt(offset).Attachment == s.pageBreak
}

// InsertPageBreak replaces the visual selection with a block between the
// surviving paragraph fragments. The preceding fragment keeps its style.
func (s *Session) InsertPageBreak() {
	object := s.pageBreakObject()
	s.Document.Edit(func(edit *richtext.Edit) {
		anchor, caret := edit.Selection()
		start, end := min(anchor, caret), max(anchor, caret)
		var style richtext.ParagraphStyle
		for offset, paragraph := range edit.Text().Paragraphs() {
			if offset > start {
				break
			}
			style = paragraph.Style
		}
		edit.Replace(start, end, richtext.TextOf(richtext.Paragraph{Style: style}, attachmentParagraph(object), richtext.Paragraph{}))
		edit.Select(start, start)
	})
}

func (s *Session) projectMarkdown(doc *parser.Document, source string) (richtext.Text, string, map[string]visualWikilink, bool) {
	return s.projectVisualMarkdown(doc, source, true)
}

func (s *Session) projectVisualMarkdown(doc *parser.Document, source string, withCallouts bool) (richtext.Text, string, map[string]visualWikilink, bool) {
	var callouts map[string][]richtext.Paragraph
	var calloutLinks map[string]visualWikilink
	if withCallouts {
		var ok bool
		source, callouts, calloutLinks, ok = s.projectCallouts(doc, source)
		if !ok {
			return richtext.Text{}, "", nil, false
		}
		if len(callouts) > 0 {
			doc = parser.New(Extensions)
			doc.Set(source)
		}
	}
	source, blocks := projectBlockParagraphs(doc, source)
	if source != doc.Source() {
		doc = parser.New(Extensions)
		doc.Set(source)
	}
	source, formulas := projectMath(doc, source)
	if source != doc.Source() {
		doc = parser.New(Extensions)
		doc.Set(source)
	}
	source, images := projectImages(doc, source, formulas)
	var projected strings.Builder
	start := 0
	for _, marker := range exportdoc.PageBreaks(source) {
		projected.WriteString(source[start:marker.Start])
		projected.WriteString("[" + PageBreakLabel + "](" + s.pageBreakObject().URL + ")")
		start = marker.End
	}
	if start != 0 {
		projected.WriteString(source[start:])
		source = projected.String()
	}
	if source != doc.Source() {
		doc = parser.New(Extensions)
		doc.Set(source)
	}
	if start == 0 && len(blocks) == 0 && len(formulas) == 0 && len(images) == 0 && len(callouts) == 0 {
		return projectWithWikilinks(doc, source, nil, nil, !withCallouts)
	}
	text, _, links, ok := projectWithWikilinks(doc, source, formulas, images, !withCallouts)
	if !ok {
		return richtext.Text{}, "", nil, false
	}
	if links == nil {
		links = calloutLinks
	} else {
		for key, target := range calloutLinks {
			links[key] = target
		}
	}
	var paragraphs []richtext.Paragraph
	for _, paragraph := range text.Paragraphs() {
		if len(paragraph.Runs) == 1 {
			link := paragraph.Runs[0].Attributes.Link
			if group := callouts[link]; group != nil {
				paragraphs = append(paragraphs, group...)
				continue
			}
			if s.pageBreak != nil && link == s.pageBreak.URL {
				paragraph = attachmentParagraph(s.pageBreak)
			} else if block, ok := blocks[link]; ok {
				paragraph = block
			}
		}
		for i, run := range paragraph.Runs {
			if object := run.Attributes.Attachment; object != nil {
				if formula := formulas[object.URL]; formula != nil {
					paragraph.Runs[i].Attributes = formula.attributes(run.Attributes)
				} else if picture := images[object.URL]; picture != nil {
					paragraph.Runs[i].Attributes.Attachment = picture.object
				}
			}
		}
		paragraphs = append(paragraphs, paragraph)
	}
	text = richtext.TextOf(paragraphs...)
	if len(formulas) != 0 {
		text = tagFormulaColors(text)
	}
	return text, visualMarkdown(text), links, true
}

func (s *Session) restorePageBreaks(markdown string) string {
	if s.pageBreak == nil {
		return markdown
	}
	marker := "![" + PageBreakLabel + "](" + s.pageBreak.URL + ")"
	if !strings.Contains(markdown, marker) {
		return markdown
	}
	var out strings.Builder
	for i, line := range strings.Split(markdown, "\n") {
		if i > 0 {
			out.WriteByte('\n')
		}
		// RichText adds a space after an empty heading marker.
		if marks, ok := strings.CutSuffix(line, " "); ok && len(marks) > 0 && len(marks) <= 6 && strings.Trim(marks, "#") == "" {
			line = marks
		}
		out.WriteString(strings.ReplaceAll(line, marker, `\`+exportdoc.PageBreakMarker))
	}
	return out.String()
}

func drawPageBreak(p *mygoui.Painter, bounds mygoui.Rect) {
	center := bounds.Y + bounds.H/2
	ink := mygoui.RGB(127, 127, 127)
	for offset := float32(0); offset < bounds.W; offset += 12 {
		p.Fill(mygoui.Rect{X: bounds.X + offset, Y: center - .5, W: min(6, bounds.W-offset), H: 1}, ink.Alpha(.45), 0)
	}
	badge := mygoui.Rect{W: min(92, bounds.W), H: 18}
	badge.X, badge.Y = bounds.X+(bounds.W-badge.W)/2, center-badge.H/2
	p.Fill(badge, mygoui.RGB(255, 255, 255), 9)
	p.Stroke(badge, ink.Alpha(.35), 9, 1)
	label := mygoui.ShapeText(PageBreakLabel, mygoui.Font{Size: 10, Weight: 600}, 0)
	w, h := label.Size()
	p.Clip(badge, 9, func() { p.TextLayout(label, badge.X+(badge.W-w)/2, center-h/2, ink) })
}
