// spec: FEAT-001 / EV-MYGO-219. Render quoted shell here-documents as strings.
package ui

import (
	"fmt"
	"html"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func TestShellAliasFenceHighlightsQuotedHereDocument(t *testing.T) {
	const hereDoc = "<<'NOTES'\n# 中文 is literal\nNOTES"
	const comment = "# outside"
	const source = "cat " + hereDoc + "\n" + comment + "\n"
	for _, language := range []string{"zsh", "ksh"} {
		t.Run(language, func(t *testing.T) {
			markup := previewMarkup{codeTheme: previewCodeTheme(true)}
			if !markup.writeHighlightedFence(language, "", source) {
				t.Fatal("shell fence is missing syntax highlighting")
			}
			for _, token := range []struct {
				text string
				kind editor.TokenKind
			}{{hereDoc, editor.TokenString}, {comment, editor.TokenComment}} {
				color := markup.codeTheme.TokenColor(token.kind)
				span := fmt.Sprintf(`<span style="color:#%02x%02x%02x">%s</span>`, color.R, color.G, color.B, html.EscapeString(token.text))
				if !strings.Contains(markup.out.String(), span) {
					t.Errorf("wrong syntax color for %q in %s", token.text, markup.out.String())
				}
			}
		})
	}
}
