package wysiwyg

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/richtext"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// FindRange is a range of visible text in a rich-text document, in runes.
type FindRange struct{ Start, End int }

// FindRanges searches within text blocks. Formatting runs remain part of the
// same string, and inline objects break the text at their insertion point.
// EV-UI-200 records the case-insensitive, literal, overlapping matches.
func FindRanges(text richtext.Text, query string) []FindRange {
	if query == "" || strings.ContainsAny(query, "\n\x00\ufffc") {
		return nil
	}
	lower := cases.Lower(language.Und)
	query = lower.String(query)
	var matches []FindRange
	start := 0
	for paragraph := range strings.SplitSeq(text.String(), "\n") {
		for fragment := range strings.SplitSeq(paragraph, "\ufffc") {
			matches = appendFindRanges(matches, fragment, start, query, lower)
			start += utf8.RuneCountInString(fragment) + 1
		}
	}
	return matches
}

func appendFindRanges(matches []FindRange, text string, base int, query string, lower cases.Caser) []FindRange {
	folded := lower.String(text)
	next := strings.Index(folded, query)
	if next < 0 {
		return matches
	}
	// Rare lowercase expansions, such as İ, still point to whole runes in
	// the original text when the folded spelling is wider.
	var original []int
	if utf8.RuneCountInString(folded) != utf8.RuneCountInString(text) {
		offset := base
		for _, r := range text {
			for range lower.String(string(r)) {
				original = append(original, offset)
			}
			offset++
		}
	}
	cursor, offset := 0, 0
	length := utf8.RuneCountInString(query)
	for next >= 0 {
		at := cursor + next
		offset += utf8.RuneCountInString(folded[cursor:at])
		match := FindRange{base + offset, base + offset + length}
		if original != nil {
			match = FindRange{original[offset], original[offset+length-1] + 1}
		}
		if len(matches) == 0 || matches[len(matches)-1] != match {
			matches = append(matches, match)
		}
		_, size := utf8.DecodeRuneInString(folded[at:])
		cursor = at + size
		offset++
		next = strings.Index(folded[cursor:], query)
	}
	return matches
}

// ReplaceFindRanges applies the snapshot's matches from right to left in one
// editor transaction. Insertion takes the formatting of the first character.
func ReplaceFindRanges(doc *richtext.Document, matches []FindRange, replacement string) {
	if len(matches) == 0 {
		return
	}
	anchor, caret := doc.Selection()
	selected := FindRange{min(anchor, caret), max(anchor, caret)}
	doc.Edit(func(edit *richtext.Edit) {
		var replacedSelection bool
		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			body := edit.Text()
			if match.Start >= body.Len() {
				continue
			}
			// An overlapping match can have shortened this paragraph.
			// Keep the change on this side of its paragraph separator.
			_, blockEnd := body.ParagraphRange(match.Start)
			end := min(match.End, blockEnd)
			if match.Start >= end {
				continue
			}
			edit.ReplaceString(match.Start, end, replacement, body.AttributesAt(match.Start))
			replacedSelection = replacedSelection || match == selected
		}
		if replacedSelection {
			a, z := edit.Selection()
			edit.Select(max(a, z), max(a, z))
		}
	})
}
