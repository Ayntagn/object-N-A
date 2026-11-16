package markdownhighlight

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"cyberwriter/internal/core/vaultindex"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// Marker identifies a paired == delimiter by byte offset in a parsed document.
type Marker struct {
	Offset  int
	Opening bool
}

func Markers(doc *parser.Document) []Marker {
	source := doc.Source()
	if !strings.Contains(source, "==") {
		return nil
	}
	var markers []Marker
	for i := 1; i < doc.Len(); i++ {
		inlines := doc.Inlines(i)
		collect(source, inlines, 0, len(inlines), &markers)
	}
	slices.SortFunc(markers, func(a, b Marker) int { return a.Offset - b.Offset })
	return markers
}

// Rewrite creates a rendering projection. It leaves the document and its
// source positions intact and replaces only the recognized delimiter pairs.
func Rewrite(doc *parser.Document, opening, closing string) string {
	source := doc.Source()
	markers := Markers(doc)
	if len(markers) == 0 {
		return source
	}
	var out strings.Builder
	out.Grow(len(source))
	start := 0
	for _, marker := range markers {
		out.WriteString(source[start:marker.Offset])
		if marker.Opening {
			out.WriteString(opening)
		} else {
			out.WriteString(closing)
		}
		start = marker.Offset + 2
	}
	out.WriteString(source[start:])
	return out.String()
}

func collect(source string, in []parser.Inline, start, end int, markers *[]Marker) {
	opening := -1
	for i := start; i < end; i = in[i].Next {
		n := in[i]
		switch n.Kind {
		case parser.Text:
			part := source[n.Start:n.End]
			if !strings.Contains(part, "==") {
				continue
			}
			// Wiki destinations are ordinary text to CommonMark. Keep their
			// bytes intact for the Vault link resolver.
			var links []vaultindex.Wikilink
			if strings.Contains(part, "[[") {
				links = vaultindex.Wikilinks(part)
			}
			for consumed := 0; consumed < len(part); {
				at := strings.Index(part[consumed:], "==")
				if at < 0 {
					break
				}
				at += consumed
				consumed = at + 2
				for len(links) > 0 && links[0].End <= at {
					links = links[1:]
				}
				if len(links) > 0 && links[0].Start <= at {
					consumed = links[0].End
					continue
				}
				offset := n.Start + at
				if !delimiter(source, offset) {
					continue
				}
				before, _ := utf8.DecodeLastRuneInString(source[:offset])
				after, _ := utf8.DecodeRuneInString(source[offset+2:])
				if opening >= 0 && offset > 0 && !unicode.IsSpace(before) {
					*markers = append(*markers, Marker{opening, true}, Marker{offset, false})
					opening = -1
				} else if opening < 0 && offset+2 < len(source) && !unicode.IsSpace(after) {
					opening = offset
				}
			}
		case parser.Emphasis, parser.Strong, parser.Strikethrough, parser.Link:
			collect(source, in, i+1, n.Next, markers)
		}
	}
}

func delimiter(source string, offset int) bool {
	if offset > 0 && source[offset-1] == '=' || offset+2 < len(source) && source[offset+2] == '=' {
		return false
	}
	start := offset
	for start > 0 && source[start-1] == '\\' {
		start--
	}
	return (offset-start)%2 == 0
}
