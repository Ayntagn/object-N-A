// spec: FEAT-WYS-CALLOUT / EV-UI-155. Titled callouts and explicit
// quote continuations must render with their own structure in Preview.
package ui

import (
	"strings"
	"testing"

	"cyberwriter/internal/core/callout"
	"cyberwriter/internal/core/exportdoc"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestPreviewCalloutsAndExplicitQuoteLines(t *testing.T) {
	source := "ordinary first\nordinary second\n\n" +
		"> [!note] My **title** &amp; friends\r\n> Content *first*\n> Content second\n" +
		">\n> - [Reference][ref]\n>\n> > [!tip]- Nested\n> > Body\n\n" +
		"> first\n> second\nlazy continuation\n> third\n\n" +
		">[!WARNING]\n> bare header\n\n[ref]: https://example.invalid/\n"
	body, _ := renderPreviewDocument(parser.New(webPreviewExtensions), source, "Callouts", nil, "", false, true, nil)
	for _, want := range []string{
		`class="callout" data-callout="note"`,
		`My <strong>title</strong> &amp; friends</span>`,
		`Content <em>first</em><br>Content second`,
		`<a href="https://example.invalid/">Reference</a>`,
		`<details class="callout" data-callout="tip"`,
		`<summary class="callout-title">`,
		"first<br>second\nlazy continuation<br>third",
		"ordinary first\nordinary second",
		`class="callout" data-callout="warning"`,
		`Warning</span>`,
		`data-source-line="4"`,
		`<p data-source-line="5">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("preview omitted %q in %s", want, body)
		}
	}
	if strings.Contains(body, "[!note]") || strings.Contains(body, "[!WARNING]") || strings.Contains(body, "[!tip]") {
		t.Fatalf("preview exposed a recognized callout marker in %s", body)
	}
}

func TestCalloutStylesInSavedHTMLAndPDF(t *testing.T) {
	pane := workspacePreview{}
	document := exportDocument{
		source: ">[!field-note]+\n> First line\n> Second line\n",
		name:   "Callouts", theme: "modern", kind: documentMarkdown, layout: exportdoc.DefaultLayout(),
		calloutStyleSnapshot: pane.snapshotCalloutStyles([]callout.Style{
			{Kind: "field-note", Name: "Field & Lab", Icon: "🧪", Color: "#7c3aed"},
		}),
	}
	shared, err := renderSharedHTML(document, false)
	if err != nil {
		t.Fatal(err)
	}
	printable, body := renderPDFHTML(document)
	for _, output := range []struct {
		name, markup, container string
	}{
		{"HTML", shared, `<details class="callout" data-callout="field-note" style="--callout-accent:#7c3aed" data-source-line="1" open>`},
		{"PDF", printable, `<aside class="callout" data-callout="field-note" style="--callout-accent:#7c3aed">`},
	} {
		for _, want := range []string{output.container, "🧪", "Field &amp; Lab", "First line<br>Second line", "border-inline-start: 4px solid var(--callout-accent)"} {
			if !strings.Contains(output.markup, want) {
				t.Errorf("%s was missing %q", output.name, want)
			}
		}
	}
	if strings.Contains(body, "<details") {
		t.Fatal("a printed callout hides content inside an interactive fold")
	}
}
