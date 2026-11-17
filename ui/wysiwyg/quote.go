package wysiwyg

import (
	"strings"

	"cyberwriter/internal/core/callout"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// Remove the callout's outer marker. A lazy continuation still belongs to
// the same visual line, even though the remaining body uses physical lines.
func projectCalloutBody(quoted string) string {
	var body strings.Builder
	var lazy map[int]bool
	for i, line := range strings.Split(quoted, "\n") {
		prefix := quotePrefixLength(line)
		if i > 0 {
			if prefix == 0 {
				if lazy == nil {
					lazy = make(map[int]bool)
				}
				lazy[body.Len()] = true
			}
			body.WriteByte('\n')
		}
		body.WriteString(line[prefix:])
	}
	source := body.String()
	if len(lazy) != 0 {
		doc := parser.New(Extensions)
		doc.Set(source)
		joined := []byte(source)
		for i := 1; i < doc.Len(); i++ {
			if doc.Block(i).Kind != parser.Paragraph {
				continue
			}
			for _, n := range doc.Inlines(i) {
				if n.Kind == parser.SoftBreak && lazy[n.Start] {
					joined[n.Start] = ' '
				}
			}
		}
		source = string(joined)
	}
	return strings.Trim(source, "\n")
}

func quoteSoftBreaks(doc *parser.Document, calloutBody bool) func(int, parser.Inline) bool {
	return func(index int, br parser.Inline) bool {
		if doc.Block(index).Kind != parser.Paragraph {
			return false
		}
		depth := 0
		for parent := doc.Block(index).Parent; parent > 0; parent = doc.Block(parent).Parent {
			if doc.Block(parent).Kind == parser.Quote {
				depth++
			}
		}
		if depth == 0 {
			// A callout body has already had its outer quote prefix stripped.
			return calloutBody
		}
		return callout.ExplicitQuoteBreak(doc.Source()[br.Start:br.End], depth)
	}
}
