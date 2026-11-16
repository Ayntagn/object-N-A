// spec: EV-UI-315 / EV-UI-319. Blank and template actions edit the active draft and the
// startup views yield to the Source and Preview of that same document.
package ui

import (
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestSplitStartPageEditsCurrentDraft(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, view, _ := snippetsWorkspace(t, "Empty.md", "", splitMode)
	tab := app.workspaceTab
	app.setTabDocument(tab, workspace.Document{})
	app.activateTab(tab)
	const help = "Let's Get To Work, Together"
	if !view.HasText("Start Writing") || !view.HasText(help) || !view.Focused("Source editor") {
		t.Fatal("empty split did not show both startup views with an editable source")
	}
	heading, _ := view.AccessByRole(mygoui.RoleHeading, help)
	card, _ := view.AccessByRole(mygoui.RoleButton, "Blank, Empty document")
	if err := view.Click("Zoom In"); err != nil {
		t.Fatal(err)
	}
	scaled, _ := view.AccessByRole(mygoui.RoleHeading, help)
	unscaled, _ := view.AccessByRole(mygoui.RoleButton, "Blank, Empty document")
	if app.previewZoom != 125 || scaled.Bounds.H < heading.Bounds.H*1.24 || card.Bounds != unscaled.Bounds {
		t.Fatal("zoom should magnify the welcome preview and keep the template grid in place")
	}
	if err := view.Click("125%"); err != nil {
		t.Fatal(err)
	}
	if err := view.Click("📖 User Guide"); err != nil {
		t.Fatal(err)
	}
	if app.workspaceTab != tab || len(app.tabs) != 1 ||
		!strings.HasPrefix(app.workingText(), "# object-N/A User Guide\n") || view.HasText(help) {
		t.Fatal("User Guide should load Markdown into the active tab with Replace current selected")
	}
	app.newTab()
	tab = app.workspaceTab
	view.Frame()
	if err := view.Click("Blank, Empty document"); err != nil {
		t.Fatal(err)
	}
	if got := app.draft.String(); got != "# " || app.code.Cursor() != 2 || !view.Focused("Source editor") {
		t.Fatalf("Blank produced %q, caret %d", got, app.code.Cursor())
	}
	if app.workspaceTab != tab || view.HasText("Start Writing") || view.HasText(help) {
		t.Fatal("Blank did not return to the editor and preview in the same tab")
	}

	app.newTab()
	view.Frame()
	view.TypeKey(0, mygoui.KeyX, "x")
	if app.draft.String() != "x" || app.title() != "x" || view.HasText("Start Writing") || view.HasText(help) {
		t.Fatal("committed editor text did not replace both startup views")
	}

	app.newTab()
	view.Frame()
	tab = app.workspaceTab
	if err := view.Click("README, Project docs"); err != nil {
		t.Fatal(err)
	}
	markdown := app.draft.String()
	if app.workspaceTab != tab || !strings.HasPrefix(markdown, "# Project Name\n") ||
		app.code.Cursor() != len(markdown) || !view.Focused("Source editor") {
		t.Fatalf("README did not populate the current draft and retain source focus")
	}
}
