// Package markdowndetails identifies Markdown sections delimited by HTML
// details and summary tags.
package markdowndetails

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Block describes one complete disclosure block in the original source.
// Start and Close point to the opening and closing tags. The body excludes
// both tag lines, and End follows the closing line and its line ending.
type Block struct {
	Start, Close, BodyStart, BodyEnd, End int
	Summary                               string
	Open                                  bool
}

type delimiter struct {
	line, at, end, next int
	summary             string
	open, close         bool
}

// Project replaces the matched envelope lines with inert HTML declarations
// so the Markdown parser reads the body normally. The caller identifies the
// two declarations by Start and Close and renders controls for them. The
// projection preserves source byte offsets and line endings.
func Project(source string, extensions parser.Extensions) (string, []Block) {
	if !strings.Contains(source, "<") {
		return source, nil
	}
	delimiters := readDelimiters(source)
	if len(delimiters) < 2 {
		return source, nil
	}
	projected := []byte(source)
	for _, marker := range delimiters {
		marker.mask(projected)
	}
	doc := parser.New(extensions)
	doc.Set(string(projected))
	var nesting, group []int
	keep := make([]bool, len(delimiters))
	for i := range doc.Children(0) {
		block := doc.Block(i)
		if block.Kind != parser.HTMLBlock {
			continue
		}
		markerIndex, ok := slices.BinarySearchFunc(delimiters, block.Start, func(mark delimiter, position int) int { return mark.at - position })
		if !ok || block.End != delimiters[markerIndex].end {
			continue
		}
		if !delimiters[markerIndex].close {
			nesting = append(nesting, markerIndex)
			continue
		}
		if len(nesting) == 0 {
			continue
		}
		opening := nesting[len(nesting)-1]
		nesting = nesting[:len(nesting)-1]
		group = append(group, opening, markerIndex)
		if len(nesting) == 0 {
			for _, part := range group {
				keep[part] = true
			}
			group = group[:0]
		}
	}
	var blocks []Block
	nesting = nesting[:0]
	for i, marker := range delimiters {
		if !keep[i] {
			copy(projected[marker.at:marker.end], source[marker.at:marker.end])
			continue
		}
		if !marker.close {
			nesting = append(nesting, len(blocks))
			blocks = append(blocks, Block{Start: marker.at, BodyStart: marker.next, Summary: marker.summary, Open: marker.open})
			continue
		}
		block := &blocks[nesting[len(nesting)-1]]
		block.Close, block.BodyEnd, block.End = marker.at, marker.line, marker.next
		nesting = nesting[:len(nesting)-1]
	}
	if len(blocks) == 0 {
		return source, nil
	}
	return string(projected), blocks
}

func (marker delimiter) mask(projected []byte) {
	for i := marker.at; i < marker.end; i++ {
		if projected[i] != '\r' && projected[i] != '\n' {
			projected[i] = ' '
		}
	}
	// A declaration can span the summary line without making it blank. It
	// does not end an enclosing HTML comment the way a comment marker would.
	copy(projected[marker.at:], "<!D")
	projected[marker.end-1] = '>'
}

func readDelimiters(source string) []delimiter {
	var found []delimiter
	for line := 0; line < len(source); {
		end, next := lineBounds(source, line)
		at := line
		for at < end && at-line < 3 && source[at] == ' ' {
			at++
		}
		if marker, ok := readDelimiter(source, delimiter{line: line, at: at, end: end, next: next}); ok {
			found = append(found, marker)
			next = marker.next
		}
		line = next
	}
	return found
}

func readDelimiter(source string, marker delimiter) (delimiter, bool) {
	line := source[marker.at:marker.end]
	if !strings.HasPrefix(line, "<") {
		return delimiter{}, false
	}
	tokens := html.NewTokenizer(strings.NewReader(line))
	kind := tokens.Next()
	if kind != html.StartTagToken && kind != html.EndTagToken {
		return delimiter{}, false
	}
	token := tokens.Token()
	if token.DataAtom != atom.Details {
		return delimiter{}, false
	}
	rest := strings.Trim(line[len(tokens.Raw()):], " \t")
	if kind == html.EndTagToken {
		marker.close = true
		return marker, rest == ""
	}
	if rest == "" {
		// HTML accepts empty lines between the opening tag and the summary.
		for start := marker.next; start < len(source) && rest == ""; {
			marker.end, marker.next = lineBounds(source, start)
			rest = strings.Trim(source[start:marker.end], " \t")
			start = marker.next
		}
	}
	var ok bool
	marker.summary, ok = readSummary(rest)
	marker.open = slices.ContainsFunc(token.Attr, func(attr html.Attribute) bool { return attr.Key == "open" })
	return marker, ok
}

func readSummary(line string) (string, bool) {
	tokens := html.NewTokenizer(strings.NewReader(line))
	if tokens.Next() != html.StartTagToken || tokens.Token().DataAtom != atom.Summary {
		return "", false
	}
	var title strings.Builder
	for remaining := line[len(tokens.Raw()):]; ; remaining = remaining[len(tokens.Raw()):] {
		switch tokens.Next() {
		case html.ErrorToken:
			return "", false
		case html.TextToken:
			title.Write(tokens.Text())
		case html.EndTagToken:
			if tokens.Token().DataAtom == atom.Summary {
				return strings.Join(strings.Fields(title.String()), " "), strings.Trim(remaining[len(tokens.Raw()):], " \t") == ""
			}
		}
	}
}

func lineBounds(source string, start int) (end, next int) {
	length := strings.IndexAny(source[start:], "\r\n")
	if length < 0 {
		return len(source), len(source)
	}
	end = start + length
	next = end + 1
	if source[end] == '\r' && next < len(source) && source[next] == '\n' {
		next++
	}
	return end, next
}
