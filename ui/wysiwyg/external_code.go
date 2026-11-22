package wysiwyg

import (
	"strings"

	"cyberwriter/internal/core/markdownfence"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

// ExternalCodeFenceAt maps an editable code card or paragraph to the raw
// Markdown fence in the current draft. The caller flushes pending edits first.
func (s *Session) ExternalCodeFenceAt(visualOffset int) (markdownfence.Fence, bool) {
	if !s.externalCodeCurrent() {
		return markdownfence.Fence{}, false
	}
	text := s.Document.Text()
	if visualOffset < 0 || visualOffset > text.Len() {
		return markdownfence.Fence{}, false
	}
	start, _ := text.ParagraphRange(visualOffset)
	formula, attachment := FormulaAt(text, start)
	if attachment && (!formula.Block || formula.IsLaTeX()) ||
		!attachment && !text.ParagraphStyleAt(start).CodeBlock {
		return markdownfence.Fence{}, false
	}
	doc, base := s.externalCodeDocument()
	if attachment {
		for _, card := range externalCodeCards(doc, text) {
			if card.offset == start {
				return markdownfence.Find(s.source, base+card.source.start)
			}
		}
		return markdownfence.Fence{}, false
	}
	offset, ok := plainCodeSourceOffset(doc, text, start)
	if !ok {
		return markdownfence.Fence{}, false
	}
	return markdownfence.Find(s.source, base+offset)
}

// ApplyExternalCodeFence updates the attachment and its exact Markdown together
// so the visual document can undo an external edit.
func (s *Session) ApplyExternalCodeFence(fence markdownfence.Fence, replacement, code string) bool {
	if !s.externalCodeCurrent() || fence.Start < 0 || fence.End < fence.Start ||
		fence.End > len(s.source) || s.source[fence.Start:fence.End] != fence.Source || replacement == fence.Source {
		return false
	}
	text := s.Document.Text()
	doc, base := s.externalCodeDocument()
	for _, card := range externalCodeCards(doc, text) {
		f := card.formula
		if !f.Block || f.IsLaTeX() || base+card.source.start != fence.Start || base+card.source.end != fence.End {
			continue
		}
		normalized := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(code)
		next := newFormula(f.ID, strings.TrimSuffix(normalized, "\n"), f.Language, replacement, true)
		next.SetVisual(f.object.Width, f.Height(), f.Descent(), nil)
		attrs := next.attributes(text.AttributesAt(card.offset))
		object := string(richtext.ObjectReplacement)
		preview := splitAttachmentParagraphs(text.ReplaceString(card.offset, card.offset+1, object, attrs))
		patched, ok := s.patchFormulaSource(visualMarkdown(preview), preview)
		if !ok || patched != s.source[:fence.Start]+replacement+s.source[fence.End:] {
			return false
		}
		a, z := s.Document.Selection()
		s.Document.Edit(func(edit *richtext.Edit) {
			edit.ReplaceString(card.offset, card.offset+1, object, attrs)
			edit.Select(a, z)
		})
		return s.Sync(s.draft)
	}
	return false
}

func (s *Session) externalCodeCurrent() bool {
	if s.Document == nil || !s.Current(s.draft) || s.Document.Version() != s.visualVersion {
		return false
	}
	s.materializeProjection()
	return true
}

func (s *Session) externalCodeDocument() (*parser.Document, int) {
	prefix, _, body := workspace.SplitFrontmatter(s.source)
	doc := parser.New(Extensions)
	doc.Set(body)
	return doc, len(prefix)
}

type codeCardRegion struct {
	offset  int
	formula *Formula
	source  mathRegion
}

func externalCodeCards(doc *parser.Document, text richtext.Text) []codeCardRegion {
	regions := mathRegions(doc)
	cards := make([]codeCardRegion, 0, len(regions))
	for offset, run := range text.Runs(0, text.Len()) {
		if run.Attributes.Attachment == nil {
			continue
		}
		f, ok := run.Attributes.Attachment.Data.(*Formula)
		if !ok {
			continue
		}
		i := len(cards)
		if i >= len(regions) || regions[i].language != f.Language || regions[i].source != f.Source ||
			regions[i].block != f.Block {
			return nil
		}
		cards = append(cards, codeCardRegion{offset: offset, formula: f, source: regions[i]})
	}
	if len(cards) != len(regions) {
		return nil
	}
	return cards
}

func plainCodeSourceOffset(doc *parser.Document, text richtext.Text, selected int) (int, bool) {
	count, target := 0, 0
	groupStart, start, end := 0, 0, 0
	id := ""
	for offset, paragraph := range text.Paragraphs() {
		style := paragraph.Style
		if !style.CodeBlock {
			id = ""
			continue
		}
		if style.ID == "" {
			return 0, false
		}
		if style.ID != id {
			count++
			id, groupStart = style.ID, offset
		}
		if offset == selected {
			target, start = count, groupStart
		}
		if target == count {
			_, end = text.ParagraphRange(offset)
		}
	}
	if target == 0 {
		return 0, false
	}
	code := text.Slice(start, end).String()
	index := 0
	offset := -1
	for i := 1; i < doc.Len(); i++ {
		block := doc.Block(i)
		if block.Kind != parser.CodeBlock {
			continue
		}
		bash := block.Info() == "bash" && block.Parent == 0 && block.Fenced && !block.Open
		if block.Info() != "" && !bash {
			continue
		}
		index++
		if index == target && strings.TrimSuffix(doc.Lines(i), "\n") == code {
			offset = block.Start
		}
	}
	return offset, index == count && offset >= 0
}
