// spec: FEAT-001 / EV-MYGO-301. HTML in Markdown uses the same rendering policy across reading and export.
package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestPreviewRendersSupportedNoteHTML(t *testing.T) {
	source := "# HTML Fixture\n\n" +
		"Inline <span style=\"color: #c33\">**Red**</span> <kbd>⌘S</kbd> and `<u>example</u>`.\n\n" +
		"<div class=\"example\" style=\"text-align: center\">Block **literal**</div>\n\n" +
		"<table><tr><th colspan=\"2\">Merged HTML cell</th></tr><tr><td>One</td><td>Two</td></tr></table>\n\n" +
		`<svg viewBox="0 0 20 20"><circle cx="10" cy="10" r="8"/></svg> beside <math><mi>x</mi></math>.` + "\n\n" +
		"<!-- Hidden fixture comment -->\n\n<details>\n<summary>Existing fold</summary>\n**Fold body**\n</details>\n"
	body, _ := renderPreviewDocument(parser.New(webPreviewExtensions), source, "", nil, "", false, true, nil)
	for _, want := range []string{
		`<span style="color:`, `<strong>Red</strong></span> <kbd>⌘S</kbd>`,
		`<code>&lt;u&gt;example&lt;/u&gt;</code>`,
		`<div class="example" style="text-align:`, `>Block **literal**</div>`,
		`<th colspan="2">Merged HTML cell</th>`, `<td>One</td><td>Two</td>`,
		`<svg viewBox="0 0 20 20">`, `<circle cx="10" cy="10" r="8"`, `<math><mi>x</mi></math>`,
		`<summary>Existing fold</summary><p`, `<strong>Fold body</strong>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("preview missing %q in %s", want, body)
		}
	}
	if strings.Contains(body, "Hidden fixture comment") || strings.Contains(body, "<strong>literal</strong>") {
		t.Fatalf("comment leaked or HTML block interpreted as Markdown: %s", body)
	}
}

func TestNoteHTMLMediaAndPrintPolicy(t *testing.T) {
	const source = `<video controls src="https://example.invalid/clip.mp4"></video>

<iframe src="https://example.invalid/embed" title="Embedded page" width="640"></iframe>
`
	body, _ := renderPreview(parser.New(webPreviewExtensions), source, "Media")
	if !strings.Contains(body, `<video controls=`) || !strings.Contains(body, `src="https://example.invalid/clip.mp4"`) ||
		!strings.Contains(body, `sandbox="allow-scripts allow-presentation"`) ||
		!strings.Contains(body, `src="https://example.invalid/embed"`) ||
		!strings.Contains(previewContentPolicy, `frame-src https: http:`) {
		t.Fatalf("reading preview missing bounded media or iframe: %s", body)
	}
	_, print := renderPDFHTML(exportDocument{source: source, kind: documentMarkdown})
	if strings.Contains(print, "<iframe") || strings.Contains(print, "<video") ||
		!strings.Contains(print, `href="https://example.invalid/embed"`) {
		t.Fatalf("print output should link to the embed: %s", print)
	}
}
