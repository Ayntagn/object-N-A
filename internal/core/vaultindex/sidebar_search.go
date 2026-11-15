package vaultindex

import (
	"context"
	"iter"
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
	"github.com/go-text/typesetting/segmenter"
	"golang.org/x/text/cases"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

type SidebarSearchHit struct {
	Entry   workspace.Entry
	Snippet string
}

// TrimSidebarSearchQuery removes Foundation whitespace at the field's edges.
func TrimSidebarSearchQuery(query string) string {
	return strings.TrimFunc(query, func(r rune) bool {
		return foundationWhitespace(r) || strings.ContainsRune(sidebarSearchNewlines, r)
	})
}

// SidebarSearchQueryLength counts graphemes in the trimmed search field.
func SidebarSearchQueryLength(query string) int {
	return sidebarGraphemeCount(TrimSidebarSearchQuery(query))
}

// SearchSidebar matches a contiguous query against Vault entries. File names,
// paths and tags rank before body-only hits. Body matches read saved source.
// A nil Index permits search without tags while the index is being built.
func (index *Index) SearchSidebar(ctx context.Context, vault *workspace.Vault, entries []workspace.Entry, query, graphTag string) []SidebarSearchHit {
	query = TrimSidebarSearchQuery(query)
	// A typed query takes over the results while the selected tag stays visible.
	if query != "" {
		graphTag = ""
	}
	if ctx.Err() != nil || (index == nil && graphTag != "") {
		return nil
	}
	folded := foldSidebarSearch(query)
	tagQuery := norm.NFC.String(strings.ToLower(query))
	searchBody := vault != nil && sidebarGraphemeCount(query) >= 2
	// Root collation approximates Foundation's localized comparison across platforms.
	names := collate.New(language.Und, collate.IgnoreCase)
	var primary, bodyOnly []SidebarSearchHit
	var walk func([]workspace.Entry)
	walk = func(entries []workspace.Entry) {
		if ctx.Err() != nil {
			return
		}
		ordered := slices.Clone(entries)
		slices.SortStableFunc(ordered, func(a, b workspace.Entry) int { return names.CompareString(a.Name, b.Name) })
		for _, entry := range ordered {
			if ctx.Err() != nil {
				return
			}
			if entry.IsDir {
				walk(entry.Children)
				continue
			}
			var indexed *note
			if index != nil {
				if id, ok := index.byPath[entry.Path]; ok {
					indexed = &index.notes[id]
				}
			}
			if graphTag != "" && (indexed == nil || !slices.Contains(indexed.graphTags, graphTag)) {
				continue
			}
			metadata := sidebarSearchContains(entry.Name, folded) || sidebarSearchContains(entry.Path, folded)
			if !metadata && indexed != nil {
				metadata = slices.ContainsFunc(indexed.graphTags, func(tag string) bool {
					return strings.Contains(norm.NFC.String(tag), tagQuery)
				})
			}
			hit := SidebarSearchHit{Entry: entry}
			if searchBody && sidebarSearchableBody(entry.Path) {
				if document, err := vault.ReadTextDocument(entry.Path); err == nil && ctx.Err() == nil {
					source := strings.TrimPrefix(document.Text, "\ufeff")
					if start, end := sidebarSearchRange(source, folded); start >= 0 {
						hit.Snippet = sidebarSearchSnippet(source, start, end)
					}
				}
			}
			if metadata {
				primary = append(primary, hit)
			} else if hit.Snippet != "" {
				bodyOnly = append(bodyOnly, hit)
			}
		}
	}
	walk(entries)
	if ctx.Err() != nil {
		return nil
	}
	return append(primary, bodyOnly...)
}

const sidebarSearchNewlines = "\n\v\f\r\u0085\u2028\u2029"

func sidebarGraphemeCount(text string) int {
	var segments segmenter.Segmenter
	if err := segments.InitWithString(text); err != nil {
		return 0
	}
	count := 0
	for range sidebarGraphemes(&segments) {
		count++
	}
	return count
}

func sidebarGraphemes(segments *segmenter.Segmenter) iter.Seq2[int, int] {
	return func(yield func(start, end int) bool) {
		it := segments.GraphemeIterator()
		if !it.Next() {
			return
		}
		current := it.Grapheme()
		start := current.OffsetInBytes
		for it.Next() {
			next := it.Grapheme()
			// typesetting v0.3.5 can miss a join in adjacent pictographs.
			// With those clusters isolated, its regular GB11 rule applies.
			if current.Text[len(current.Text)-1] != '\u200d' || !sidebarEmojiJoin(current.Text, next.Text[0]) {
				if !yield(start, next.OffsetInBytes) {
					return
				}
				start = next.OffsetInBytes
			}
			current = next
		}
		yield(start, current.OffsetInBytes+current.LengthInBytes)
	}
}

func sidebarEmojiJoin(left []rune, right rune) bool {
	if len(left) < 2 || !(unicode.IsSymbol(right) || unicode.IsPunct(right)) {
		return false
	}
	var segments segmenter.Segmenter
	joined := make([]rune, len(left)+1)
	copy(joined, left)
	joined[len(left)] = right
	segments.Init(joined)
	it := segments.GraphemeIterator()
	return it.Next() && !it.Next()
}

func sidebarSearchableBody(relative string) bool {
	switch strings.ToLower(path.Ext(relative)) {
	case ".md", ".markdown", ".txt":
		return true
	}
	return false
}

var sidebarCaseFold = cases.Fold()

func foldSidebarSearch(text string) string {
	return norm.NFC.String(sidebarCaseFold.String(text))
}

func sidebarSearchContains(source, query string) bool {
	start, _ := sidebarSearchRange(source, query)
	return start >= 0
}

// sidebarSearchRange maps a case-folded, canonically normalized match back to
// its exact original bytes. A match cannot split a grapheme or a case expansion.
func sidebarSearchRange(source, query string) (int, int) {
	if query == "" {
		return 0, 0
	}
	if strings.IndexFunc(source, func(r rune) bool { return r > unicode.MaxASCII }) < 0 &&
		(!strings.ContainsAny(query, "\r\n") || !strings.Contains(source, "\r\n")) {
		if start := strings.Index(strings.ToLower(source), query); start >= 0 {
			return start, start + len(query)
		}
		return -1, -1
	}
	folded := foldSidebarSearch(source)
	match := strings.Index(folded, query)
	if match < 0 {
		return -1, -1
	}
	var segments segmenter.Segmenter
	if err := segments.InitWithString(source); err != nil {
		return -1, -1
	}
	// Case expansions can produce overlapping candidates. Search from each
	// boundary so a partial match at "sß" cannot hide the following "ß".
	foldedAt := 0
	var active []sidebarMatchBoundary
	for start, end := range sidebarGraphemes(&segments) {
		if foldedAt == match {
			active = append(active, sidebarMatchBoundary{after: match + len(query), original: start})
		}
		foldedAt += len(foldSidebarSearch(source[start:end]))
		if foldedAt > len(folded) {
			return -1, -1
		}
		for len(active) > 0 && active[0].after <= foldedAt {
			if active[0].after == foldedAt {
				return active[0].original, end
			}
			active = active[1:]
		}
		if match < foldedAt {
			match = strings.Index(folded[foldedAt:], query)
			if match >= 0 {
				match += foldedAt
			} else {
				match = len(folded)
			}
		}
		if match == len(folded) && len(active) == 0 {
			break
		}
	}
	return -1, -1
}

type sidebarMatchBoundary struct {
	after    int
	original int
}

func sidebarSearchSnippet(source string, start, end int) string {
	const contextSize = 30
	lineStart, lineEnd := 0, len(source)
	if before := strings.LastIndexAny(source[:start], sidebarSearchNewlines); before >= 0 {
		_, size := utf8.DecodeRuneInString(source[before:])
		lineStart = before + size
	}
	if after := strings.IndexAny(source[end:], sidebarSearchNewlines); after >= 0 {
		lineEnd = end + after
	}
	line := source[lineStart:lineEnd]
	start, end = start-lineStart, end-lineStart
	var segments segmenter.Segmenter
	if err := segments.InitWithString(line); err != nil {
		return ""
	}
	var preceding [contextSize]int
	first, last := start, end
	before, after := 0, 0
	for at, next := range sidebarGraphemes(&segments) {
		if at < start {
			preceding[before%contextSize] = at
			before++
		} else if at >= end {
			after++
			if after > contextSize {
				break
			}
			last = next
		}
	}
	if before > 0 {
		first = preceding[max(0, before-contextSize)%contextSize]
	}
	text := strings.TrimFunc(line[first:last], foundationWhitespace)
	if before > contextSize {
		text = "…" + text
	}
	if after > contextSize {
		text += "…"
	}
	return strings.Clone(text)
}
