package wysiwyg

import (
	"crypto/rand"
	"strings"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

const diagramLabel = "Mermaid diagram"
const diagramPlaceholderHeight = 190
const vegaLabel = "Vega chart"
const vegaAttachmentScheme = "cyberwriter-vega:"
const vegaCaretID = "cyberwriter-vega-caret"

// Diagram carries a visual diagram's payload and the original fenced bytes.
// A user edit creates a new payload, leaving earlier undo steps intact.
type Diagram struct {
	Source, Info string
	markdown     string
	id           string
	object       *richtext.Attachment
}

func newDiagram(source, info, markdown string) *Diagram {
	return newDiagramVersion(source, info, markdown, rand.Text())
}

func newDiagramVersion(source, info, markdown, id string) *Diagram {
	diagram := &Diagram{Source: source, Info: info, markdown: markdown, id: id}
	label, url := diagramLabel, "cyberwriter-mermaid:"+rand.Text()
	if diagram.VegaMode() != "" {
		label = vegaLabel
		url = vegaAttachmentScheme + id + ":" + rand.Text()
	}
	diagram.object = &richtext.Attachment{
		Name: label, URL: url,
		Width: 1, Height: diagramPlaceholderHeight, Data: diagram,
	}
	return diagram
}

// VegaMode recognizes the two chart fences. A chart specification's schema
// can override the mode later, without changing the language saved in the note.
func VegaMode(info string) string {
	mode := strings.ToLower(strings.TrimSpace(info))
	if mode == "vega" || mode == "vega-lite" {
		return mode
	}
	return ""
}

func (d *Diagram) VegaMode() string { return VegaMode(d.Info) }

func isVegaBlock(block *parser.Block) bool {
	return block.Parent == 0 && block.Kind == parser.CodeBlock && block.Fenced && !block.Open && VegaMode(block.Info()) != ""
}

func (d *Diagram) paragraph() richtext.Paragraph {
	return attachmentParagraph(d.object)
}

// ReserveHeight changes display metrics without changing the undoable payload.
func (d *Diagram) ReserveHeight(height float32) {
	d.object.Height = height
}

func (d *Diagram) Height() float32 { return d.object.Height }

// DiagramAt returns a standalone diagram starting at the document offset.
func (s *Session) DiagramAt(offset int) (*Diagram, bool) {
	if s.Document == nil {
		return nil, false
	}
	return diagramAt(s.Document.Text(), offset)
}

// InsertVegaChart replaces the selection with a root-level card. A split
// paragraph keeps its style on both sides, with the caret after the card.
func (s *Session) InsertVegaChart(source string) bool {
	const info = "vega-lite"
	diagram := newDiagram(source, info, fencedDiagramMarkdown(source, info))
	return s.insertStandalone(diagram.object, vegaCaretID, false)
}

func (s *Session) insertStandalone(object *richtext.Attachment, caretID string, selectBlock bool) bool {
	if s.Document == nil {
		return false
	}
	a, z := s.Document.Selection()
	return s.insertStandaloneAt(object, caretID, selectBlock, min(a, z), max(a, z))
}

func (s *Session) insertStandaloneAt(object *richtext.Attachment, caretID string, selectBlock bool, start, end int) bool {
	if s.Document == nil || !s.Current(s.draft) {
		return false
	}
	text := s.Document.Text()
	paragraphStart, _ := text.ParagraphRange(start)
	_, paragraphEnd := text.ParagraphRange(end)
	for offset := paragraphStart; offset <= end; {
		style := text.ParagraphStyleAt(offset)
		if style.List != richtext.NoList || style.Quote || style.CodeBlock {
			return false
		}
		_, finish := text.ParagraphRange(offset)
		offset = finish + 1
	}
	var paragraphs []richtext.Paragraph
	blockStart := start
	if start > paragraphStart {
		paragraphs = append(paragraphs, richtext.Paragraph{Style: text.ParagraphStyleAt(start)})
		blockStart++
	}
	paragraphs = append(paragraphs, attachmentParagraph(object), richtext.Paragraph{})
	trailingEmpty := end == text.Len()
	if end == paragraphEnd && !trailingEmpty {
		end++ // Join the trailing empty fragment to the existing next paragraph.
	}
	s.Document.Edit(func(edit *richtext.Edit) {
		edit.Replace(start, end, richtext.TextOf(paragraphs...))
		edit.FormatParagraphs(blockStart, blockStart, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
		caret := blockStart + 2
		if trailingEmpty {
			edit.FormatParagraphs(caret, caret, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{ID: caretID} })
		}
		if selectBlock {
			edit.Select(blockStart, blockStart+1)
		} else {
			edit.Select(caret, caret)
		}
	})
	return true
}

func diagramAt(text richtext.Text, offset int) (*Diagram, bool) {
	if offset < 0 || offset >= text.Len() {
		return nil, false
	}
	attachment := text.AttributesAt(offset).Attachment
	if attachment == nil {
		return nil, false
	}
	d, ok := attachment.Data.(*Diagram)
	return d, ok
}

// ChangeDiagram replaces the payload at the current offset in one undo step.
func (s *Session) ChangeDiagram(offset int, previous *Diagram, source, info string) bool {
	diagram, ok := s.DiagramAt(offset)
	if !ok || diagram != previous || !s.Current(s.draft) || diagram.Source == source && diagram.Info == info {
		return false
	}
	next := newDiagramVersion(source, info, fencedDiagramMarkdown(source, info), previous.id)
	next.ReserveHeight(previous.object.Height)
	s.Document.Edit(func(edit *richtext.Edit) {
		edit.ReplaceString(offset, offset+1, string(richtext.ObjectReplacement), richtext.Attributes{Attachment: next.object})
	})
	return true
}

// patchFencedSource applies a source-editor transaction to its fence
// only. The full rich-text serializer may normalize Markdown elsewhere.
func (s *Session) patchFencedSource(markdown string, text richtext.Text) (string, bool) {
	if !strings.Contains(markdown, vegaAttachmentScheme) && !strings.Contains(markdown, queryAttachmentScheme) && !strings.Contains(markdown, apiAttachmentScheme) {
		return "", false
	}
	change := changedRange(s.lastMarkdown, markdown)
	for offset := range text.Paragraphs() {
		object := text.AttributesAt(offset).Attachment
		if object == nil {
			continue
		}
		var id, raw, scheme string
		var isFence func(*parser.Block) bool
		switch block := object.Data.(type) {
		case *Diagram:
			if block.VegaMode() == "" {
				continue
			}
			id, raw, scheme, isFence = block.id, block.markdown, vegaAttachmentScheme, isVegaBlock
		case *QueryBlock:
			id, raw, scheme, isFence = block.id, block.markdown, queryAttachmentScheme, isRootQueryFence
		case *APIBlock:
			isFence = isRootAPIRequest
			if block.Result {
				isFence = isRootAPIResult
			}
			id, raw, scheme = block.id, block.markdown, apiAttachmentScheme
		default:
			continue
		}
		prefix := "![" + object.Name + "](" + scheme
		marker := attachmentMarkdown(object)
		start := strings.Index(markdown, marker)
		if start < 0 || change.Start < start || change.Start+len(change.Text) > start+len(marker) {
			continue
		}
		end := start + len(marker) - len(change.Text) + change.End - change.Start
		if end > len(s.lastMarkdown) || end <= start {
			return "", false
		}
		old, ok := strings.CutPrefix(s.lastMarkdown[start:end], prefix+id+":")
		if _, okEnd := strings.CutSuffix(old, ")"); !ok || !okEnd {
			return "", false
		}
		return s.replaceFence(strings.Count(markdown[:start], prefix), raw, isFence)
	}
	return "", false
}

func (s *Session) replaceFence(index int, markdown string, isFence func(*parser.Block) bool) (string, bool) {
	prefix, _, body := workspace.SplitFrontmatter(s.source)
	doc := parser.New(Extensions)
	doc.Set(body)
	for i := 1; i < doc.Len(); i++ {
		b := doc.Block(i)
		if !isFence(b) {
			continue
		}
		if index == 0 {
			start := strings.LastIndexByte(body[:b.Start], '\n') + 1
			// The replacement already includes its own opener indentation.
			end := b.End
			if end > b.Start && body[end-1] == '\n' {
				end--
			}
			return prefix + body[:start] + markdown + body[end:], true
		}
		index--
	}
	return "", false
}

func fencedDiagramMarkdown(source, info string) string {
	fence := "```"
	for strings.Contains(source, fence) {
		fence += "`"
	}
	return fence + info + "\n" + source + "\n" + fence
}
