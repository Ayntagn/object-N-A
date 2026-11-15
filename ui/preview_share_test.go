// spec: FEAT-001 / EV-UI-043, EV-UI-061
package ui

import (
	"slices"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestPreviewShareAndPDFLayoutUseTheSamePreference(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, window, _ := snippetsWorkspace(t, "Share.md", "# Share fixture\n", splitMode)
	if err := window.Click("Share"); err != nil {
		t.Fatal(err)
	}
	want := []string{"Export as PDF", "Export as Word", "Export as HTML", "-", "Export with Options...", "-", "Open in Browser", "-", "Grid Layout"}
	if got := window.Menu(); !slices.Equal(got, want) {
		t.Fatalf("Share menu = %v", got)
	}
	if err := window.ChooseMenuItem("Grid Layout"); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadWorkspacePreferences()
	if err != nil || !loaded.PDFPreviewGridLayout {
		t.Fatalf("saved PDF grid layout = %t, err=%v", loaded.PDFPreviewGridLayout, err)
	}
	app.pdfPreview.enabled = true
	app.pdfPreview.form = newExportPageForm(app.preferences.Export)
	toolbar := mygoui.NewTester(app.previewToolbar, 1294, 60)
	column, found := toolbar.Access("Column View")
	if !found || column.Description != "Switch to column layout" || column.Bounds.W != 26 || column.Bounds.H != 22 {
		t.Fatalf("PDF column button = %+v", column)
	}
	if err := toolbar.Click("Column View"); err != nil {
		t.Fatal(err)
	}
	if err := toolbar.Click("Share"); err != nil {
		t.Fatal(err)
	}
	if got := toolbar.Menu(); !slices.Equal(got, want) {
		t.Fatalf("Share menu after returning to column layout = %v", got)
	}
}
