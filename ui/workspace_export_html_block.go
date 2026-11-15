package ui

import (
	"io"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// EV-UI-137: standalone Markdown output keeps ordinary HTML formatting blocks.
// Validate and serialize the complete fragment before writing to the document.
func standaloneHTMLBlock(source string) (string, bool) {
	tokens := html.NewTokenizer(strings.NewReader(source))
	var markup strings.Builder
	var open []atom.Atom
	for {
		kind := tokens.Next()
		token := tokens.Token()
		switch kind {
		case html.ErrorToken:
			return markup.String(), tokens.Err() == io.EOF && len(tokens.Raw()) == 0 && len(open) == 0
		case html.TextToken:
		case html.StartTagToken, html.SelfClosingTagToken:
			if !standaloneFormattingTag(token) {
				return "", false
			}
			if token.DataAtom == atom.Br || token.DataAtom == atom.Hr {
				break
			}
			if kind == html.SelfClosingTagToken {
				return "", false
			}
			open = append(open, token.DataAtom)
		case html.EndTagToken:
			if len(open) == 0 || open[len(open)-1] != token.DataAtom {
				return "", false
			}
			open = open[:len(open)-1]
		default:
			return "", false
		}
		markup.WriteString(token.String())
	}
}

func standaloneFormattingTag(token html.Token) bool {
	switch token.DataAtom {
	case atom.Div, atom.P, atom.Blockquote, atom.Pre, atom.Br, atom.Hr,
		atom.Span, atom.B, atom.Strong, atom.I, atom.Em, atom.U, atom.S,
		atom.Code, atom.Kbd, atom.Mark, atom.Small, atom.Sub, atom.Sup:
	default:
		return false
	}
	for _, attr := range token.Attr {
		if attr.Key != "class" {
			return false
		}
		for _, class := range strings.Fields(attr.Val) {
			// These classes mark nodes for the bundled Markdown renderers.
			switch class {
			case "math-inline", "math-display", "mermaid":
				return false
			}
		}
	}
	return true
}
