// spec: FEAT-001 / EV-MYGO-304. Reused preview blocks keep fresh markup, source lines and code-copy targets.
package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestLivePreviewBlocksMatchCompleteRender(t *testing.T) {
	const introduction = "# Repeat\n\nStart of the article.\n\n"
	const finish = "# Repeat\n\n[An external link][a]\n\n[a]: https://example.invalid/one\n\n" +
		"> A quoted $x^2$.\n>\n> - A list item.\n\n" +
		"```go\nfunc main() {}\n```\n\n```mermaid\nflowchart LR\n A --> B\n```\n"
	const prose = "正文 $x$、==重点== 与 `VALUE`。\n\n"
	long := introduction + strings.Repeat(prose, longPreviewBytes/len(prose)+1) + finish
	doc := parser.New(webPreviewExtensions)
	var cache previewRenderCache
	for i, source := range []string{
		long,
		strings.Replace(long, "Start of", "A new paragraph.\n\nStart of", 1),
		strings.Replace(long, "invalid/one", "invalid/two", 1),
		long + "\n<details>\n<summary>Summary</summary>\n\n**Body**\n\n</details>\n",
	} {
		newMarkup := func(parsed *parser.Document) previewMarkup {
			return previewMarkup{doc: parsed, charts: true, codeTheme: previewCodeTheme(true),
				codeCopies: &previewCodeCopies{revision: uint64(i + 1)}, liveAPIFences: true}
		}
		live := newMarkup(doc)
		result := live.renderLive(source, "Fixture", &cache, false)
		fresh := newMarkup(parser.New(webPreviewExtensions))
		want, title := fresh.render(source, "Fixture", true)
		if got := result.fullHTML(); got != want || result.title != title || !result.math || !result.mermaid {
			t.Fatalf("revision %d differs from the complete render (HTML %d vs %d bytes, title %q vs %q)", i+1, len(got), len(want), result.title, title)
		}
		if i == 1 && (result.reused == 0 || result.fenceHits != 1) {
			t.Fatalf("line insertion reused %d paragraphs and %d code bodies", result.reused, result.fenceHits)
		}
		if i == 3 && result.blocks != nil {
			t.Fatal("a disclosure spanning parser blocks must ship a complete body")
		}
	}
}
