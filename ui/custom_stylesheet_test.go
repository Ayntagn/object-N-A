// spec: FEAT-CUSTOM-STYLESHEET / EV-UI-126 / EV-UI-127
package ui

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/exportdoc"
)

func TestCustomStylesheetReadsExternalEditsAndRetainsDisabledFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CYBERWRITER_CONFIG_DIR", dir)
	path := filepath.Join(dir, "custom.css")
	app := &workspaceWindow{preferences: &workspacePreferences{UserStylesheetEnabled: true}}
	const before = "  h1 { color: teal; }\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := app.activeCustomStylesheet(); got != strings.TrimSpace(before) {
		t.Fatalf("initial stylesheet = %q", got)
	}
	app.preferences.UserStylesheetEnabled = false
	if got := app.activeCustomStylesheet(); got != "" {
		t.Fatalf("disabled stylesheet = %q", got)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != before {
		t.Fatalf("disabled file = %q, %v", raw, err)
	}
	const after = ".markdown-body { letter-spacing: 1px; }"
	if err := os.WriteFile(path, []byte(after), 0o600); err != nil {
		t.Fatal(err)
	}
	app.preferences.UserStylesheetEnabled = true
	if got := app.activeCustomStylesheet(); got != after {
		t.Fatalf("updated stylesheet = %q", got)
	}
}

func TestCustomStylesheetReachesMarkdownExportsAsCSS(t *testing.T) {
	const css = ".markdown-body h1 { color: teal; }\n@media print { h1 { color: green; } }"
	document := exportDocument{kind: documentMarkdown, source: "# Field Notes", name: "Field Notes",
		theme: "synthetic-editorial-black", layout: exportdoc.DefaultLayout(), customCSS: css}
	shared, err := renderSharedHTML(document, true)
	if err != nil {
		t.Fatal(err)
	}
	pdf, _ := renderPDFHTML(document)
	standalone, err := renderExportHTML(document, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{shared, standalone, pdf} {
		encoded := base64.StdEncoding.EncodeToString([]byte(css))
		if !strings.Contains(output, "data:text/css;base64,"+encoded) ||
			!strings.Contains(output, `class="markdown-body"`) {
			t.Fatal("missing standalone stylesheet or Markdown body selector")
		}
	}
	document.kind, document.source = documentHTML, "<h1>Raw HTML</h1>"
	got, err := browserPreviewMarkup(document, true)
	if err != nil || got != document.source {
		t.Fatalf("HTML browser output = %q, %v", got, err)
	}
}
