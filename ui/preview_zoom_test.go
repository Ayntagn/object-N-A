// spec: FEAT-001 / EV-UI-009, EV-UI-043, EV-UI-060
package ui

import (
	"testing"

	"cyberwriter/internal/core/workspace"
)

func TestPreviewZoomToolbar(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, window, _ := snippetsWorkspace(t, "Viewport.md", "# Preview viewport\n\nA note.\n", splitMode)
	click := func(label string) {
		t.Helper()
		if err := window.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	checkZoom := func(label string) {
		t.Helper()
		if button, found := window.Access(label); !found || button.Disabled {
			t.Fatalf("zoom reset button %q missing or disabled, zoom=%d", label, app.previewZoom)
		}
	}
	click("Zoom Out")
	checkZoom("75%")
	click("Zoom Out")
	click("Zoom Out")
	checkZoom("25%")
	if button, _ := window.Access("Zoom Out"); !button.Disabled {
		t.Fatal("Zoom Out available at 25%")
	}
	click("25%")
	checkZoom("100%")
	click("Zoom In")
	checkZoom("125%")
	firstTab := app.workspaceTab
	secondTab := &workspaceTab{mode: splitMode}
	app.setTabDocument(secondTab, workspace.Document{Path: "Second.md", Text: "# Another note\n"})
	app.tabs = append(app.tabs, secondTab)
	app.activateTab(secondTab)
	window.Frame()
	checkZoom("125%")
	app.activateTab(firstTab)
	window.Frame()
	checkZoom("125%")
	for step := 125; step < 400; step += 25 {
		click("Zoom In")
	}
	checkZoom("400%")
	if button, _ := window.Access("Zoom In"); !button.Disabled {
		t.Fatal("Zoom In available at 400%")
	}
	saved, err := loadWorkspacePreferences()
	if err != nil || saved.Session.PreviewZoom != 400 {
		t.Fatalf("saved scene zoom = %d, err=%v", saved.Session.PreviewZoom, err)
	}
	app.setMode(previewMode)
	window.Frame()
	checkZoom("400%")
	click("400%")
	checkZoom("100%")
	if button, found := window.Access("Counterclockwise Curved Arrow"); !found || button.Description != "Return to preview" {
		t.Fatalf("return control = %+v, found=%t", button, found)
	}
}
