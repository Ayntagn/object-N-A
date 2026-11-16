// spec: FEAT-001 / EV-UI-247
package ui

import (
	"testing"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestVaultOpenPreferencesPersist(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	prefs, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	app := &workspaceWindow{preferences: &prefs, blackout: true}
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		mygoui.Column(c).Fill().Children(func() { app.drawVaultOpenPreferences(c) })
	}, 640, 150)
	for _, label := range []string{"Replace current", "New window"} {
		if item, ok := tester.AccessByRole(mygoui.RoleRadio, label); !ok || item.Bounds.W != 120 || item.Bounds.H != 24 {
			t.Fatalf("%s radio button = %+v", label, item)
		}
	}
	for _, label := range []string{"New window", "Drawings Open Separately"} {
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	prefs, err = loadWorkspacePreferences()
	if err != nil || !prefs.SidebarClickNewWindow || prefs.DrawingsOpenSeparately {
		t.Fatalf("reopened settings: new window=%v, separate drawings=%v, err=%v",
			prefs.SidebarClickNewWindow, prefs.DrawingsOpenSeparately, err)
	}
}

func TestVaultOpenPreferencesRouteTreeSearchAndDrawing(t *testing.T) {
	root := t.TempDir()
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	note := func(name string) workspace.Document {
		t.Helper()
		doc, err := vault.CreateDocument(name, "# "+name+"\n")
		if err != nil {
			t.Fatal(err)
		}
		return doc
	}
	current, replace, shared := note("Current.md"), note("Replace.md"), note("Shared.md")
	file, err := vault.CreateDrawing()
	if err != nil {
		t.Fatal(err)
	}
	application := &workspaceApplication{preferences: workspacePreferences{DrawingsOpenSeparately: true, ShowExtension: true}}
	app := &workspaceWindow{application: application, preferences: &application.preferences,
		vault: vault, root: root, workspaceTab: &workspaceTab{mode: splitMode}}
	other := &workspaceWindow{application: application, preferences: &application.preferences,
		vault: vault, root: root, workspaceTab: &workspaceTab{mode: splitMode}}
	application.windows = []*workspaceWindow{app, other}
	app.tabs = []*workspaceTab{app.workspaceTab}
	app.setTabDocument(app.workspaceTab, current)
	sharedTab, drawingTab := other.blankTab(), other.blankTab()
	other.setTabDocument(sharedTab, shared)
	other.setTabDrawing(drawingTab, newWorkspaceDrawing(vault, file))
	other.tabs = []*workspaceTab{sharedTab, drawingTab}

	replaceEntry := workspace.Entry{Name: replace.Path, Path: replace.Path}
	sharedEntry := workspace.Entry{Name: shared.Path, Path: shared.Path}
	drawingEntry := workspace.Entry{Name: file.Path, Path: file.Path}
	app.index.data, err = vaultindex.Build(vault, []workspace.Entry{replaceEntry, sharedEntry, drawingEntry})
	if err != nil {
		t.Fatal(err)
	}
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		tree := mygoui.TreeBase(c)
		tree.Root.Children(func() {
			colors := app.vaultSectionColors(c)
			app.fileRow(c, tree, replaceEntry, colors, true)
			app.searchResultRow(c, tree, vaultindex.SidebarSearchHit{Entry: sharedEntry}, colors)
			app.fileRow(c, tree, drawingEntry, colors, true)
		})
	}, 390, 120)
	click := func(name string, mods mygoui.Modifiers) {
		t.Helper()
		if err := tester.ClickWith(mods, name); err != nil {
			t.Fatal(err)
		}
	}
	click(replace.Path, 0)
	if app.document.Path != replace.Path || len(app.tabs) != 1 {
		t.Fatal("ordinary click should replace the active tab")
	}
	origin := app.workspaceTab
	for _, newWindow := range []bool{false, true} {
		application.preferences.SidebarClickNewWindow = newWindow
		other.activateTab(drawingTab)
		mods := mygoui.Cmd
		if newWindow {
			mods = 0
		}
		click(shared.Path, mods)
		if app.workspaceTab != origin || app.document.Path != replace.Path || other.workspaceTab != sharedTab || len(other.tabs) != 2 {
			t.Fatalf("New window=%v, modifiers=%v: existing document was not reused", newWindow, mods)
		}
	}

	application.preferences.SidebarClickNewWindow = false
	click(file.Path, 0)
	if other.workspaceTab != drawingTab || app.document.Path != replace.Path {
		t.Fatal("separate drawing should reuse its open window")
	}
	other.activateTab(sharedTab)
	other.tabs = []*workspaceTab{sharedTab}
	application.preferences.DrawingsOpenSeparately = false
	click(file.Path, 0)
	if app.drawing == nil || app.workspaceTab != origin {
		t.Fatal("drawing with separation off should replace the current document")
	}
	application.preferences.DrawingsOpenSeparately = true
	app.openDrawingWikiLink(file.Path, "[[Shared]]", 0)
	if app.drawing == nil || app.workspaceTab != origin {
		t.Fatal("separate linked note should leave the drawing open")
	}
	application.preferences.DrawingsOpenSeparately = false
	app.openDrawingWikiLink(file.Path, "[[Shared]]", 0)
	if app.drawing != nil || app.document.Path != shared.Path || len(app.tabs) != 1 {
		t.Fatal("linked note should use the current tab when separation is off")
	}
}
