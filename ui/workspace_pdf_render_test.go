// spec: EV-MYGO-267
package ui

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"cyberwriter/internal/core/exportdoc"
	"github.com/egoist/mygo/ui/editor"
)

func TestPDFMapleOperatorFeaturesPreserveCopyableText(t *testing.T) {
	doc := exportDocument{
		theme: "academic", editorFont: sourceFontDefaultFamily, hasEditorFont: true,
		layout: exportdoc.DefaultLayout(), source: "Copy -> => >= != && // == <=.",
	}
	document, _ := renderPDFHTML(doc)
	calt := `"calt",`
	if runtime.GOOS == "darwin" {
		calt = `"calt" 0,`
		if !strings.Contains(document, "font-variant-ligatures: no-contextual") {
			t.Error("Quartz capture did not disable contextual substitutions")
		}
	}
	for _, feature := range []string{calt, `"kern" 0`, `"cv02"`, `"ss11"`} {
		if !strings.Contains(document, feature) {
			t.Errorf("PDF is missing the Maple Mono feature %s", feature)
		}
	}
	if !strings.Contains(resolveDocumentEditorFont(sourceFontDefaultFamily).cssFeatures, `"calt",`) {
		t.Error("PDF settings mutated the Markdown preview font features")
	}
}

func TestPDFUsesEditorFontAndHighlightsCode(t *testing.T) {
	source := "# Sample\n\nText\n\n" +
		"```go\npackage sample\n// Preserve colors on print.\nvar greeting = \"hello\"\n```\n\n" +
		"```bash\necho \"hello\"\n```\n\n" +
		"```mermaid\nflowchart TD\nA --> B\n```\n"
	app, _, _ := snippetsWorkspace(t, "Sample.md", source, splitMode)
	app.preferences.EditorFont = "Fira Code"
	layout := exportdoc.DefaultLayout()
	fontSnapshot := app.exportDocument(layout)
	doc := fontSnapshot
	// Code background is independent of the paper. A dark block on a light
	// page needs the dark syntax palette in both preview and print.
	doc.theme = "modern"
	doc.appearance.CodeBackground = "#2d2d2d"
	document, body := renderPDFHTML(doc)
	for _, variable := range []string{"--doc-body-font", "--doc-heading-font", "--doc-code-font"} {
		if !strings.Contains(document, variable+`: "Fira Code",`) {
			t.Errorf("PDF %s does not use the selected editor font", variable)
		}
	}
	if !strings.Contains(document, `@font-face { font-family: "Fira Code";`) {
		t.Error("PDF is missing the selected bundled font")
	}
	palette := previewCodeTheme(true)
	keyword := palette.TokenColor(editor.TokenKeyword)
	str := palette.TokenColor(editor.TokenString)
	for _, fragment := range []string{
		`class="hljs language-go"`,
		fmt.Sprintf(`style="color:#%02x%02x%02x">package</span>`, keyword.R, keyword.G, keyword.B),
		fmt.Sprintf(`style="color:#%02x%02x%02x">&#34;hello&#34;</span>`, str.R, str.G, str.B),
		`class="hljs language-bash"`,
		mermaidBlockStart,
	} {
		if !strings.Contains(body, fragment) {
			t.Errorf("PDF body is missing %q: %s", fragment, body)
		}
	}
	if strings.Contains(body, "language-mermaid") || strings.Contains(body, "preview-code-copy") {
		t.Errorf("PDF added syntax or copy controls to the diagram: %s", body)
	}

	app.preferences.EditorFont = ""
	systemDocument := app.exportDocument(layout)
	if systemDocument == fontSnapshot {
		t.Fatal("PDF snapshot did not change when switching to System Mono")
	}
	systemHTML, _ := renderPDFHTML(systemDocument)
	if !strings.Contains(systemHTML, "--doc-body-font: monospace;") {
		t.Error("An explicit System Mono preference should be inherited")
	}

	doc.layout.FontFamily = "Georgia"
	overrideHTML, _ := renderPDFHTML(doc)
	if !strings.Contains(overrideHTML, "--doc-code-font: Georgia,") ||
		strings.Contains(overrideHTML, `@font-face { font-family: "Fira Code";`) {
		t.Error("An explicit PDF font should take precedence over the editor font")
	}
}
