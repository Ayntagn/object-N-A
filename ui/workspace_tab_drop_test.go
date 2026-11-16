// spec: FEAT-001 / EV-MYGO-262. Drag documents into the title and tab bar.
package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestWorkspaceTabDropKeepsOriginalDraft(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const rootText = "# Original\n"
	app, view, rootFile := snippetsWorkspace(t, "Root.md", rootText, sourceMode)
	root := filepath.Dir(rootFile)
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	inside := filepath.Join(root, "Inside.md")
	external := t.TempDir()
	other := filepath.Join(external, "Other.markdown")
	plain := filepath.Join(external, "Plain.txt")
	rejected := filepath.Join(external, "Ignored.bin")
	files := map[string]string{inside: "# Inside\n", other: "# Other\n", plain: "Some plain text\n", rejected: "not a tab document"}
	for file, content := range files {
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mountVaultDragTree(t, app, view, vault, root)
	t.Cleanup(func() {
		for _, tab := range app.tabs {
			app.releaseTabStorage(tab)
		}
	})

	bounds := func(role mygoui.Role, label string) mygoui.Rect {
		t.Helper()
		item, ok := view.AccessByRole(role, label)
		if !ok || item.Offscreen || item.Bounds.W <= 0 || item.Bounds.H <= 0 {
			t.Fatalf("missing accessible %q: %+v", label, item)
		}
		return item.Bounds
	}
	drop := func(zone mygoui.Rect, paths ...string) {
		t.Helper()
		if !view.DropFiles(zone.X+zone.W/2, zone.Y+zone.H/2, paths...) {
			t.Fatalf("system file drop not accepted at %+v", zone)
		}
	}
	original := app.workspaceTab
	source := bounds(mygoui.RoleTextField, "Source editor")
	view.ClickAt(source.X+20, source.Y+36)
	view.Type("draft")
	draft := original.draft.String()
	if draft == rootText || !original.dirty() {
		t.Fatal("fixture could not make the first tab dirty")
	}

	title := bounds(mygoui.RoleGroup, "Document title")
	row := bounds(mygoui.RoleButton, "Inside")
	x, y := title.X+title.W/2, title.Y+title.H/2
	view.Press(row.X+30, row.Y+row.H/2)
	view.Move(x, y)
	view.Release(x, y)
	if len(app.tabs) != 2 || app.workspaceTab.document.Path != "Inside.md" || app.tabs[0] != original {
		t.Fatalf("Vault drag did not add a tab: active=%q tabs=%d", app.document.Path, len(app.tabs))
	}
	t.Logf("title zone %+v, Vault row %+v", title, row)

	tab := bounds(mygoui.RoleTab, "Inside.md")
	drop(tab, inside, rejected, other, plain)
	if len(app.tabs) != 4 || app.absoluteDocument(app.workspaceTab) != plain || app.mode != splitMode {
		t.Fatalf("file batch did not open the expected tabs: active=%q tabs=%d mode=%d",
			app.absoluteDocument(app.workspaceTab), len(app.tabs), app.mode)
	}
	if !strings.Contains(app.problem, "Ignored.bin") {
		t.Fatalf("the valid items hid the batch error: %q", app.problem)
	}
	bar := bounds(mygoui.RoleTabList, "tab bar")
	resolvedRoot, err := filepath.EvalSymlinks(rootFile)
	if err != nil {
		t.Fatal(err)
	}
	drop(bar, resolvedRoot)
	if app.workspaceTab != original || len(app.tabs) != 4 || original.draft.String() != draft || !original.dirty() {
		t.Fatal("dropping an open file failed to select the existing dirty draft")
	}
	files[rootFile] = rootText
	for file, want := range files {
		if data, err := os.ReadFile(file); err != nil || string(data) != want {
			t.Fatalf("file changed after opening tabs: %s: %v", filepath.Base(file), err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 || len(app.preferences.VaultFileOrders) != 0 {
		t.Fatalf("drop changed the Vault files or sort preferences: entry count %d, error %v", len(entries), err)
	}
	t.Logf("tab drop %+v, four distinct tabs and original draft retained", bar)
}

func TestWorkspaceTitleDropWithNarrowToolbar(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, view, _ := snippetsWorkspace(t, "Root.md", "# Root\n", sourceMode)
	other := filepath.Join(t.TempDir(), "New.md")
	if err := os.WriteFile(other, []byte("# New\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	view.SetSize(720, 420)
	toolbar, ok := view.AccessByRole(mygoui.RoleToolbar, "Main toolbar")
	if !ok || toolbar.Offscreen {
		t.Fatal("minimum-width workspace is missing the main toolbar")
	}
	if _, titleVisible := view.AccessByRole(mygoui.RoleGroup, "Document title"); titleVisible {
		t.Fatal("fixture did not exercise the toolbar fallback")
	}
	bounds := toolbar.Bounds
	if !view.DropFiles(bounds.X+bounds.W/2, bounds.Y+bounds.H/2, other) ||
		len(app.tabs) != 2 || app.absoluteDocument(app.workspaceTab) != other {
		t.Fatalf("toolbar drop did not add a new tab: title %q, tabs %d", app.title(), len(app.tabs))
	}
	t.Cleanup(func() { app.releaseTabStorage(app.workspaceTab) })
	t.Logf("minimum-width toolbar drop zone %+v", bounds)
}
