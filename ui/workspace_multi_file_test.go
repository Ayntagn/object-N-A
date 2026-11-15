// spec: FEAT-071 / EV-MYGO-168, EV-MYGO-174. The workspace mounts only
// Markdown, routes edits to each document, and shows compact tiles.
package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestMultiFileWorkspaceRoutesEditsAndEnforcesMarkdown(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, tester, filename := snippetsWorkspace(t, "Alpha.md", "# Alpha\n", splitMode)
	root := filepath.Dir(filename)
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	app.vault, app.root = vault, root
	primary := app.workspaceTab
	if _, err := vault.CreateDocument("Beta.md", "# Beta\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.CreateDocument("Gamma.markdown", "# Gamma\n"); err != nil {
		t.Fatal(err)
	}
	foreign := app.blankTab()
	app.setTabDocument(foreign, workspace.Document{Path: "data.csv", Text: "name,count\n"})
	app.tabs = append(app.tabs, foreign)
	tester.Frame()
	click := func(label string) {
		t.Helper()
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	replace := func(label, markdown string) {
		t.Helper()
		click(label)
		tester.Key(mygoui.Cmd, mygoui.KeyA)
		tester.Type(markdown)
	}
	bounds := func(role mygoui.Role, label string) mygoui.Rect {
		t.Helper()
		item, ok := tester.AccessByRole(role, label)
		if !ok || item.Offscreen {
			t.Fatalf("missing visible %q: %+v", label, item)
		}
		return item.Bounds
	}
	assertCompact := func(firstContent mygoui.Rect, inset float32) {
		t.Helper()
		board := bounds(mygoui.RoleGroup, "Multi-file workspace")
		pane := bounds(mygoui.RoleGroup, "Alpha.md")
		if pane.Y != board.Y || firstContent.Y != pane.Y+inset+1 {
			t.Fatalf("content should start after the existing border and inset: board=%+v, pane=%+v, editor=%+v", board, pane, firstContent)
		}
		if _, ok := tester.Access("Multi-file controls"); ok {
			t.Fatal("the multi-file workspace still mounts a controls row")
		}
		for _, label := range []string{"Source", "Rendered", "Columns", "Rows", "Grid", "Select Alpha.md", "Edit rendered", "Done"} {
			if _, ok := tester.AccessByRole(mygoui.RoleButton, label); ok {
				t.Fatalf("the multi-file workspace still mounts %q", label)
			}
		}
	}
	shortcut := func(key mygoui.Key) { tester.Key(mygoui.Cmd|mygoui.Alt, key) }
	click("Multi-file View")
	app.openVaultFileByPreference("Beta.md", mygoui.Cmd, false)
	beta := app.workspaceTab
	app.openVaultRow("Gamma.markdown", false)
	app.openVaultRow("handout.pdf", false)
	app.openPickedFile(filepath.Join(root, "image.png"))
	app.togglePDFPreview()
	tester.Frame()
	if !app.multiFile.active || len(app.tabs) != 4 || app.pdfPreview.enabled || app.problem != multiFileMarkdownOnly {
		t.Fatal("opening Markdown or rejecting a non-Markdown preview changed the workspace incorrectly")
	}
	if _, ok := tester.Access("Source editor — data.csv"); ok {
		t.Fatal("a pre-existing CSV tab appeared in the Markdown workspace")
	}
	assertCompact(bounds(mygoui.RoleTextField, "Source editor — Alpha.md"), sourceTextTopInset)
	replace("Source editor — Alpha.md", "# Alpha changed\n")
	replace("Source editor — Beta.md", "# Beta changed\n")
	if primary.draft.String() != "# Alpha changed\n" || beta.draft.String() != "# Beta changed\n" ||
		app.workspaceTab != beta || app.mode != splitMode || !app.saveTab(beta, nil) {
		t.Fatalf("independent drafts lost edits: %q / %q, active %s", primary.draft.String(), beta.draft.String(), app.title())
	}
	disk, err := os.ReadFile(filepath.Join(root, "Beta.md"))
	if err != nil || string(disk) != "# Beta changed\n" {
		t.Fatalf("save reached the wrong document: %q, %v", disk, err)
	}

	shortcut(mygoui.Key2)
	if app.multiFile.layout.arrangement != multiFileRows {
		t.Fatal("workspace layout shortcut did not apply with an editor focused")
	}
	shortcut(mygoui.Key4)
	for _, name := range []string{"Alpha.md", "Beta.md", "Gamma.markdown"} {
		if _, ok := tester.Access("Rendered — " + name); !ok {
			t.Fatalf("missing rendered Markdown %q", name)
		}
		if _, ok := tester.Access("Source editor — " + name); ok {
			t.Fatalf("a Source editor for %q remained mounted", name)
		}
	}
	reading, ok := tester.Access("Rendered — Alpha.md")
	if !ok {
		t.Fatal("missing Alpha's reading area")
	}
	assertCompact(reading.Bounds, 0)
	tester.ClickAt(reading.Bounds.X+reading.Bounds.W/2, reading.Bounds.Y+reading.Bounds.H/2)
	if app.workspaceTab != primary {
		t.Fatal("clicking document content did not select its tab")
	}
	tab := bounds(mygoui.RoleTab, "Beta.md")
	tester.ClickAt(tab.X+tab.W/2, tab.Y+tab.H/2)
	shortcut(mygoui.Key5)
	if app.multiFile.editing != beta || !tester.Focused("WYSIWYG editor") {
		t.Fatal("the edit shortcut did not focus the active rendered document")
	}
	replace("WYSIWYG editor", "Visual edit")
	shortcut(mygoui.Key5)
	if app.multiFile.editing != nil || !tester.Focused("Rendered — Beta.md") ||
		!strings.Contains(beta.draft.String(), "Visual edit") || strings.Contains(primary.draft.String(), "Visual edit") {
		t.Fatal("finishing rendered edits did not keep the active file and focus")
	}
	if err := tester.RightClick("Multi-file View"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Grid"); err != nil || app.multiFile.layout.arrangement != multiFileGrid {
		t.Fatalf("the grid context menu did not change the arrangement: %v", err)
	}
	tabBar := bounds(mygoui.RoleTabList, "tab bar")
	tester.ClickAt(tabBar.X+tabBar.W/2, tabBar.Y+tabBar.H+tabBarGap/2)
	if tester.Focused("Rendered — Beta.md") {
		t.Fatal("clicking the blank tab strip gap did not clear editor focus")
	}
	shortcut(mygoui.Key4)
	if app.multiFile.rendered {
		t.Fatal("the view shortcut did not return the full group to Source without editor focus")
	}
	click("Multi-file View")
	if app.multiFile.active || app.mode != splitMode {
		t.Fatal("leaving the workspace did not restore the tab's prior mode")
	}
}

func TestMultiFileEmptyWorkspaceCannotSaveHiddenTab(t *testing.T) {
	const original = "item,count\nalpha,1\n"
	root := t.TempDir()
	file := filepath.Join(root, "Hidden.csv")
	if err := os.WriteFile(file, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	doc, err := vault.ReadTextDocument("Hidden.csv")
	if err != nil {
		t.Fatal(err)
	}
	tab := &workspaceTab{mode: splitMode}
	app := &workspaceWindow{vault: vault, root: root, workspaceTab: tab, tabs: []*workspaceTab{tab}, preferences: &workspacePreferences{}}
	app.setTabDocument(tab, doc)
	tab.draft.Edit(editor.Edit{Start: len(original), End: len(original), Text: "beta,2\n"})
	app.toggleMultiFile()
	if !app.multiFile.active || app.save() || !tab.dirty() {
		t.Fatal("the empty Markdown workspace accepted a save targeting a hidden CSV")
	}
	if disk, err := os.ReadFile(file); err != nil || string(disk) != original {
		t.Fatalf("the hidden CSV was changed: %q, %v", disk, err)
	}
}
