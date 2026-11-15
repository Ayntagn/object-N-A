// spec: FEAT-001 / EV-UI-057
package ui

import (
	"os"
	"path/filepath"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestPaneMaximizationRestoresDocumentLayout(t *testing.T) {
	const saved = "# Pane study\n\nSaved text.\n"
	app, tester, filename := snippetsWorkspace(t, "PaneStudy.md", saved, splitMode)
	vault, err := workspace.Open(filepath.Dir(filename))
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	app.vault, app.vaultVisible, app.outlineVisible = vault, true, true
	other := app.blankTab()
	app.setTabDocument(other, workspace.Document{Path: "Second.md", Text: "# Second note\n"})
	app.tabs = append(app.tabs, other)
	primary := app.workspaceTab
	tester.Frame()
	click := func(label string) {
		t.Helper()
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	bounds := func(label string) mygoui.Rect {
		t.Helper()
		node, ok := tester.Access(label)
		if !ok {
			t.Fatalf("%s is missing", label)
		}
		return node.Bounds
	}
	before := map[string]mygoui.Rect{}
	for _, label := range []string{"Source editor", "Markdown Preview", "Vault sidebar", "Outline", "tab bar"} {
		before[label] = bounds(label)
	}
	for _, sample := range []struct{ name, kept, hidden string }{
		{"editor", "Source editor", "Markdown Preview"},
		{"preview", "Markdown Preview", "Source editor"},
	} {
		footer := bounds("GLOW Terminal")
		click("Maximize " + sample.name)
		if app.mode != splitMode || app.focusMode || bounds(sample.kept).W <= before[sample.kept].W {
			t.Fatalf("%s maximization changed the base mode or did not expand", sample.name)
		}
		for _, label := range []string{sample.hidden, "Vault sidebar", "Outline"} {
			if _, ok := tester.Access(label); ok {
				t.Fatalf("%s is still visible with %s maximized", label, sample.name)
			}
		}
		if bounds("tab bar") != before["tab bar"] || bounds("GLOW Terminal") != footer {
			t.Fatalf("tab bar or footer moved with %s maximized", sample.name)
		}
		if sample.name == "editor" {
			tester.Key(mygoui.Cmd, mygoui.KeyA)
			tester.Type("# Pane study\n\nEdited in the expanded source.\n")
			click("Second.md")
			if bounds("Vault sidebar") != before["Vault sidebar"] {
				t.Fatal("the other tab inherited the expanded layout")
			}
			click("PaneStudy.md")
			if app.workspaceTab != primary || bounds(sample.kept).W <= before[sample.kept].W {
				t.Fatal("returning to the tab did not restore its expanded layout")
			}
		}
		click("Exit maximized " + sample.name)
		for label, original := range before {
			if bounds(label) != original {
				t.Fatalf("%s did not restore its original frame", label)
			}
		}
	}
	click(modeControls[wysiwygMode].label)
	wysBefore := bounds("cyberWriter WYSIWYG")
	click("Maximize editor")
	if app.mode != wysiwygMode || bounds("cyberWriter WYSIWYG").W <= wysBefore.W {
		t.Fatal("WYSIWYG maximization did not expand the visual editor")
	}
	click(modeControls[splitMode].label)
	if bounds("Source editor") != before["Source editor"] || bounds("Vault sidebar") != before["Vault sidebar"] {
		t.Fatal("the explicit view choice did not restore the split layout")
	}
	if disk, err := os.ReadFile(filename); err != nil || string(disk) != saved || app.draft.String() != "# Pane study\n\nEdited in the expanded source.\n" {
		t.Fatalf("maximization changed or lost the draft or file: %q, %v", disk, err)
	}
}

func TestPaneMaximizationFromCompactPDFLayout(t *testing.T) {
	app, _, _ := snippetsWorkspace(t, "Preview.md", "# PDF preview\n", splitMode)
	app.outlineVisible = true
	app.pdfPreview.enabled = true
	app.pdfPreview.form = newExportPageForm(app.preferences.Export)
	app.pdfPreview.viewer.payload = []byte("cached PDF")
	tester := mygoui.NewTester(app.previewToolbar, 800, 60)
	if err := tester.Click("PDF Layout"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Maximize preview"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tester.Access("Exit maximized preview"); !ok || !app.pdfPreview.enabled || string(app.pdfPreview.viewer.payload) != "cached PDF" {
		t.Fatal("the compact PDF command did not expand the pane and retain the PDF")
	}
}
