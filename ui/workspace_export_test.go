// spec: FEAT-HTML-EXPORT / EV-UI-137 / EV-UI-138 / EV-EDITOR-004
package ui

import (
	"strings"
	"testing"

	"cyberwriter/internal/core/exportdoc"
)

func TestExportHTMLIncludesSafeStandaloneContent(t *testing.T) {
	source := "# 中文 & notes\n\n**Draft** [unsafe](javascript:alert(1)). $x^2$\n\n" +
		"[External](https://example.invalid/) [Relative](folder/Note.md#part) [[Missing note|Label]] and [[#Portable Links 中文]].\n\n" +
		"![Missing image](figures/absent.png)\n\n```go\npackage sampler\n```\n\n" +
		"<div class=\"sample-raw\">Original raw <strong>block</strong></div>\n\nAn inline <b>literal</b>.\n\n" +
		"<div class=\"custom math-inline\">Literal math marker</div>\n\n" +
		"\\[pagebreak]\n\n`[pagebreak]`\n\n<script>alert('x')</script>\n"
	layout := exportdoc.DefaultLayout()
	layout.Landscape = true
	layout.LineSpacing = 1.75
	layout.FontFamily = "Georgia"
	layout.FontScale = 140
	doc := exportDocument{source: source, name: "Untitled", kind: documentMarkdown, layout: layout, theme: "synthetic-editorial-black",
		appearance: pdfAppearance{Background: "#fffef0", Text: "#5b4636"}}
	data, err := exportPayload(nil, doc, exportHTML, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := string(data)
	const wikiAnchor = `<a class="wikilink" href="#Portable Links 中文.md"><span class="tag">#Portable</span> Links 中文</a>`
	for _, fragment := range []string{
		"<!doctype html>", "Content-Security-Policy", "<title>中文 &amp; notes</title>",
		`<article class="markdown-body">`, `content="width=684, initial-scale=1.0"`, `data-source-line="1"`,
		`--doc-font-size: 17pt`, `--doc-background: #fffef0`, `--preview-line-height:1.75`,
		`<strong>Draft</strong>`, `<span class="math-inline">x^2</span>`,
		`href="https://example.invalid/" target="_blank" rel="noopener noreferrer"`,
		`href="folder/Note.md#part"`, `href="Missing note.md"`, `src="figures/absent.png"`,
		wikiAnchor,
		`class="hljs language-go"`, `<hr class="page-break">`, `<code>[pagebreak]</code>`,
		`<div class="sample-raw">Original raw <strong>block</strong></div>`,
		`An inline <b>literal</b>.`,
		`<div class="custom">Literal math marker</div>`,
	} {
		if !strings.Contains(result, fragment) {
			t.Errorf("HTML form export is missing %q", fragment)
		}
	}
	if strings.Contains(result, "alert('x')") || strings.Contains(result, `href="javascript:`) ||
		strings.Contains(result, `class="custom math-inline"`) {
		t.Error("HTML export activated a script or an unsafe link")
	}
	shared, err := renderSharedHTML(doc, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shared, `content="width=device-width, initial-scale=1.0"`) ||
		!strings.Contains(shared, `<a href="https://example.invalid/">External</a>`) ||
		!strings.Contains(shared, wikiAnchor) ||
		!strings.Contains(shared, `<div class="sample-raw">Original raw <strong>block</strong></div>`) ||
		strings.Contains(shared, `--doc-background: #fffef0`) || strings.Contains(shared, `--doc-font-size: 17pt`) {
		t.Error("Share HTML did not keep the responsive document theme and link behavior")
	}
	browser, err := browserPreviewMarkup(doc, false)
	if err != nil || browser != shared {
		t.Fatalf("browser preview did not reuse the Share markup: %v", err)
	}
	_, pdfBody := renderPDFHTML(doc)
	if !strings.Contains(pdfBody, `<div class="sample-raw">Original raw <strong>block</strong></div>`) {
		t.Error("PDF preview did not render the allowed HTML block")
	}
}
