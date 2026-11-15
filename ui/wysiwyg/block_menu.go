package wysiwyg

import (
	"crypto/rand"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/richtext"
)

// FormatBlockMenuSelection replaces the query or empty paragraph with an
// editable block. The query and formatting change share one undo step.
func (s *Session) FormatBlockMenuSelection(style richtext.ParagraphStyle) bool {
	start, end, ok := s.blockMenuSelection()
	if !ok {
		return false
	}
	style.ID = tableExitID
	if style.CodeBlock {
		style.ID = compactFencePrefix + rand.Text()
	}
	s.Document.Edit(func(edit *richtext.Edit) {
		text := edit.Text()
		if end < text.Len() {
			edit.Format(end, end+1, func(a *richtext.Attributes) { *a = richtext.Attributes{} })
		}
		if start != end {
			if end == text.Len() {
				edit.Format(start, end, func(a *richtext.Attributes) { *a = richtext.Attributes{} })
			}
			edit.ReplaceString(start, end, "", richtext.Attributes{})
		}
		edit.FormatParagraphs(start, start, func(p *richtext.ParagraphStyle) { *p = style })
		edit.Select(start, start)
	})
	return true
}

func (s *Session) blockMenuSelection() (start, end int, ok bool) {
	if s.Document == nil || !s.Current(s.draft) {
		return 0, 0, false
	}
	anchor, caret := s.Document.Selection()
	start, end = min(anchor, caret), max(anchor, caret)
	text := s.Document.Text()
	p, q := text.ParagraphRange(start)
	return start, end, start == p && end == q && (start == end || strings.HasPrefix(text.Slice(start, end).String(), "/"))
}

// InsertBlockParagraph adds a plain editable paragraph after the paragraph
// starting at after. For a multi-paragraph block, after is its final paragraph.
func (s *Session) InsertBlockParagraph(after int) bool {
	if !s.canEditBlockOrder() {
		return false
	}
	text := s.Document.Text()
	if after < 0 || after > text.Len() {
		return false
	}
	start, end := text.ParagraphRange(after)
	if start != after || end < text.Len() && SameBlockGroup(text.ParagraphStyleAt(start), text.ParagraphStyleAt(end+1)) {
		return false
	}
	style := text.ParagraphStyleAt(start)
	s.Document.Edit(func(edit *richtext.Edit) {
		empty := richtext.Paragraph{Style: richtext.ParagraphStyle{ID: tableExitID}}
		edit.Replace(end, end, richtext.TextOf(richtext.Paragraph{Style: style}, empty))
		// Splitting a paragraph may strip the IDs of the two halves.
		edit.FormatParagraphs(start, start, func(p *richtext.ParagraphStyle) { *p = style })
		edit.FormatParagraphs(end+1, end+1, func(p *richtext.ParagraphStyle) { *p = empty.Style })
		// The split moves the old newline's typing attributes into the
		// empty paragraph. Formatting the inserted break at the end of the
		// document clears the final caret's typing attributes too.
		plain := end + 1
		if end == text.Len() {
			plain = end
		}
		edit.Format(plain, end+2, func(a *richtext.Attributes) { *a = richtext.Attributes{} })
		edit.Select(end+1, end+1)
	})
	return true
}

// MoveBlock moves a single ordinary paragraph or heading to an insertion
// boundary in the original paragraph order, then selects the moved text.
func (s *Session) MoveBlock(fromIndex, toIndex int) bool {
	if !s.canEditBlockOrder() {
		return false
	}
	text := s.Document.Text()
	count := text.ParagraphCount()
	if fromIndex < 0 || fromIndex >= count || toIndex < 0 || toIndex > count ||
		toIndex == fromIndex || toIndex == fromIndex+1 {
		return false
	}
	first := min(fromIndex, toIndex)
	last := max(fromIndex, toIndex-1)
	var affected []richtext.Paragraph
	start, end := 0, text.Len()
	var sourceLength int
	var previous richtext.ParagraphStyle
	index := 0
	for offset, paragraph := range text.Paragraphs() {
		if index == toIndex && index > 0 && SameBlockGroup(previous, paragraph.Style) {
			return false
		}
		if index == first {
			start = offset
		}
		if index == fromIndex {
			style := paragraph.Style
			if style.List != richtext.NoList || style.Indent != 0 || style.Quote || style.CodeBlock || len(paragraph.Runs) == 0 {
				return false
			}
			for _, run := range paragraph.Runs {
				if run.Attributes.Attachment != nil {
					return false
				}
				sourceLength += utf8.RuneCountInString(run.Text)
			}
		}
		if index >= first && index <= last {
			affected = append(affected, paragraph)
		}
		if index == last {
			_, end = text.ParagraphRange(offset)
		}
		// Also read the paragraph following the range to validate a drop
		// boundary at its start when moving forward.
		if index >= max(last, toIndex) {
			break
		}
		previous = paragraph.Style
		index++
	}
	from, destination := fromIndex-first, toIndex-first
	if from < destination {
		destination--
	}
	moved := affected[from]
	affected = slices.Insert(slices.Delete(affected, from, from+1), destination, moved)
	s.Document.Edit(func(edit *richtext.Edit) {
		edit.Replace(start, end, richtext.TextOf(affected...))
		offset, selection := start, start
		for i, paragraph := range affected {
			// Text.Replace takes its boundary IDs from the original range.
			// Restore each paragraph's own style, including its app ID.
			edit.FormatParagraphs(offset, offset, func(p *richtext.ParagraphStyle) { *p = paragraph.Style })
			if i == destination {
				selection = offset
			}
			_, end := edit.Text().ParagraphRange(offset)
			offset = end + 1
		}
		edit.Select(selection, selection+sourceLength)
	})
	return true
}

func (s *Session) canEditBlockOrder() bool {
	return s.Document != nil && s.Current(s.draft) && s.Document.Version() == s.visualVersion
}

// SameBlockGroup identifies an unsafe boundary between adjacent paragraphs
// that may belong to the same list, quote or code block.
func SameBlockGroup(left, right richtext.ParagraphStyle) bool {
	if a, b := calloutID(left), calloutID(right); a != "" || b != "" {
		return a != "" && a == b
	}
	if left.Quote && right.Quote {
		return true
	}
	if left.CodeBlock && right.CodeBlock {
		return left.ID == "" || right.ID == "" || left.ID == right.ID
	}
	return (left.List != richtext.NoList || left.Indent > 0) && (right.List != richtext.NoList || right.Indent > 0)
}
