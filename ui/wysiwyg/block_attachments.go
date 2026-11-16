package wysiwyg

import (
	"crypto/rand"
	"strings"

	"cyberwriter/internal/core/mermaid"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

const DividerLabel = "Divider"
const dividerCaretID = "cyberwriter-inserted-divider-caret"
const reopenedDividerCaretID = "cyberwriter-reopened-divider-caret"
const dividerMarkdown = "***"

type divider struct{ markdown string }

func newDivider(markdown string) *richtext.Attachment {
	return &richtext.Attachment{
		Name: DividerLabel, URL: "cyberwriter-divider:" + rand.Text(),
		// MyGo scales wide attachments to the editor's inner width. The
		// painter keeps the rule one point thick at every viewport size.
		Width: 4096, Height: 13, Data: divider{markdown: markdown},
		Draw: func(p *mygoui.Painter, r mygoui.Rect) {
			r.Y += (r.H - 1) / 2
			r.H = 1
			p.Fill(r, p.Theme().Text.Alpha(.10), 0)
		},
	}
}

// IsDivider identifies the separator attachment at the given rune offset.
func (s *Session) IsDivider(text richtext.Text, offset int) bool {
	if offset < 0 || offset >= text.Len() {
		return false
	}
	object := text.AttributesAt(offset).Attachment
	if object == nil {
		return false
	}
	_, ok := object.Data.(divider)
	return ok
}

// InsertDivider replaces the block-menu selection with a thematic break.
func (s *Session) InsertDivider() bool {
	start, end, ok := s.blockMenuSelection()
	if !ok {
		return false
	}
	s.Document.Edit(func(edit *richtext.Edit) {
		paragraph := attachmentParagraph(newDivider(dividerMarkdown))
		atEnd := end == edit.Text().Len()
		if atEnd {
			edit.Replace(start, end, richtext.TextOf(paragraph, richtext.Paragraph{}))
			edit.FormatParagraphs(start+2, start+2, func(p *richtext.ParagraphStyle) {
				*p = richtext.ParagraphStyle{ID: dividerCaretID}
			})
			edit.Format(start+1, start+2, func(a *richtext.Attributes) { *a = richtext.Attributes{} })
			edit.Select(start, start+1)
		} else {
			edit.Replace(start, end, richtext.TextOf(paragraph))
			edit.Select(start+2, start+2)
		}
		edit.FormatParagraphs(start, start, func(p *richtext.ParagraphStyle) { *p = paragraph.Style })
	})
	return true
}

func attachmentParagraph(object *richtext.Attachment) richtext.Paragraph {
	return richtext.Paragraph{Runs: []richtext.Run{{
		Text: string(richtext.ObjectReplacement), Attributes: richtext.Attributes{Attachment: object},
	}}}
}

// Project Markdown blocks as temporary links so MyGo can lay out the rest
// of the document through its ordinary rich-text parser.
func projectBlockParagraphs(doc *parser.Document, source string) (string, map[string]richtext.Paragraph) {
	var blocks map[string]richtext.Paragraph
	var projected strings.Builder
	start := 0
	for i := 1; i < doc.Len(); i++ {
		block := doc.Block(i)
		if block.Parent != 0 && block.Kind != parser.HTMLBlock {
			continue
		}
		begin := block.Start
		end := block.End
		var address, label string
		var paragraph richtext.Paragraph
		switch block.Kind {
		case parser.CodeBlock:
			if block.Fenced && !block.Open {
				if isRootQueryFence(block) {
					begin = strings.LastIndexByte(source[:begin], '\n') + 1
					query := newQueryBlock(strings.TrimSuffix(doc.Lines(i), "\n"), strings.TrimSuffix(source[begin:end], "\n"), rand.Text())
					address, label, paragraph = query.object.URL, query.object.Name, attachmentParagraph(query.object)
					break
				}
				if isRootAPIRequest(block) || isRootAPIResult(block) {
					begin = strings.LastIndexByte(source[:begin], '\n') + 1
					api := newAPIBlock(strings.TrimSuffix(doc.Lines(i), "\n"), block.Info(), strings.TrimSuffix(source[begin:end], "\n"), rand.Text())
					address, label, paragraph = api.object.URL, api.object.Name, attachmentParagraph(api.object)
					break
				}
				_, isMermaid := mermaid.Background(block.Info())
				isVega := isVegaBlock(block)
				if isMermaid || isVega {
					if isVega {
						begin = strings.LastIndexByte(source[:begin], '\n') + 1
					}
					raw := source[begin:end]
					diagram := newDiagram(strings.TrimSuffix(doc.Lines(i), "\n"), block.Info(), strings.TrimSuffix(raw, "\n"))
					address, label, paragraph = diagram.object.URL, diagram.object.Name, diagram.paragraph()
				}
			}
		case parser.Table:
			if region, ok := readTableRegion(doc, i, 0); ok && supportsVisualTable(doc, i, region.end) {
				end = region.end
				table := newVisualTable(source[region.start:end], region.relative(), "", 0)
				address, label, paragraph = table.object.URL, tableLabel, attachmentParagraph(table.object)
			}
		case parser.ThematicBreak:
			begin = strings.LastIndexAny(source[:begin], "\r\n") + 1
			object := newDivider(source[begin:end])
			address, label, paragraph = object.URL, DividerLabel, attachmentParagraph(object)
		case parser.HTMLBlock:
			if style, offset, ok := emptyMenuBlock(doc, i); ok {
				address, label = "cyberwriter-empty-paragraph:"+rand.Text(), "Empty paragraph"
				begin, paragraph.Style = offset, style
			}
		}
		if address == "" {
			continue
		}
		if blocks == nil {
			blocks = make(map[string]richtext.Paragraph)
		}
		blocks[address] = paragraph
		projected.WriteString(source[start:begin])
		projected.WriteString("\n\n[" + label + "](" + address + ")\n\n")
		start = end
	}
	if blocks == nil {
		return source, nil
	}
	projected.WriteString(source[start:])
	return projected.String(), blocks
}

// Empty menu blocks can appear as HTML in the Markdown tree. Mask an empty
// block's entire source range so MyGo checks the surrounding document alone.
func emptyMenuBlock(doc *parser.Document, i int) (richtext.ParagraphStyle, int, bool) {
	block := doc.Block(i)
	style := richtext.ParagraphStyle{ID: tableExitID}
	if strings.TrimSpace(doc.Source()[block.Start:block.End]) != tableEmptyParagraphMarkdown {
		return style, 0, false
	}
	if block.Parent == 0 {
		return style, block.Start, true
	}
	parent := doc.Block(block.Parent)
	if i != block.Parent+1 || block.Next != parent.Next {
		return style, 0, false
	}
	if parent.Kind == parser.Quote && parent.Parent == 0 {
		style.Quote = true
		return style, parent.Start, true
	}
	if parent.Kind == parser.Item {
		list := doc.Block(parent.Parent)
		if list.Parent == 0 && (list.Marker == '*' || list.Ordered && list.Marker == '.') {
			switch {
			case parent.Ordered:
				style.List = richtext.Numbers
			case parent.Task != parser.NoTask:
				style.List = richtext.Tasks
				style.Checked = parent.Task == parser.Checked
			default:
				style.List = richtext.Bullets
			}
			return style, parent.Start, true
		}
	}
	return style, 0, false
}

func standaloneBlockAttachment(object *richtext.Attachment) bool {
	if object == nil {
		return false
	}
	switch value := object.Data.(type) {
	case *Diagram, *QueryBlock, *APIBlock, *Table, divider:
		return true
	case *Image:
		return value.Block
	case *Formula:
		return value.Block
	}
	return false
}

func standaloneBlockAt(text richtext.Text, offset int) bool {
	return offset >= 0 && offset < text.Len() && standaloneBlockAttachment(text.AttributesAt(offset).Attachment)
}

// Retain block boundaries in the Markdown even if Backspace joined the
// visual paragraphs around an attachment.
func splitAttachmentParagraphs(text richtext.Text) richtext.Text {
	var paragraphs []richtext.Paragraph
	changed := false
	for _, paragraph := range text.Paragraphs() {
		if calloutID(paragraph.Style) != "" {
			paragraphs = append(paragraphs, paragraph)
			continue
		}
		start := 0
		for i, run := range paragraph.Runs {
			object := run.Attributes.Attachment
			if !standaloneBlockAttachment(object) {
				continue
			}
			if start < i {
				paragraphs = append(paragraphs, richtext.Paragraph{Style: paragraph.Style, Runs: paragraph.Runs[start:i]})
			}
			paragraphs = append(paragraphs, attachmentParagraph(object))
			start = i + 1
			changed = changed || len(paragraph.Runs) != 1 || paragraph.Style != (richtext.ParagraphStyle{})
		}
		if start == 0 || start < len(paragraph.Runs) {
			paragraphs = append(paragraphs, richtext.Paragraph{Style: paragraph.Style, Runs: paragraph.Runs[start:]})
		}
	}
	if changed {
		return richtext.TextOf(paragraphs...)
	}
	return text
}

func restoreAttachments(markdown string, text richtext.Text) string {
	var replacements []string
	seen := make(map[*richtext.Attachment]bool)
	for _, run := range text.Runs(0, text.Len()) {
		object := run.Attributes.Attachment
		if object == nil || seen[object] {
			continue
		}
		var raw string
		switch value := object.Data.(type) {
		case *Diagram:
			raw = value.markdown
		case *QueryBlock:
			raw = value.markdown
		case *APIBlock:
			raw = value.markdown
		case *Table:
			raw = value.markdown
		case divider:
			raw = value.markdown
		case *Formula:
			raw = value.markdown
		case *Image:
			raw = value.markdown
		case *Callout:
			// The title marker is presentation only. A partial selection
			// can contain it without containing the card.
		default:
			continue
		}
		seen[object] = true
		replacements = append(replacements, attachmentMarkdown(object), raw)
	}
	if len(replacements) == 0 {
		return markdown
	}
	return strings.NewReplacer(replacements...).Replace(markdown)
}

func attachmentMarkdown(object *richtext.Attachment) string {
	return "![" + object.Name + "](" + object.URL + ")"
}
