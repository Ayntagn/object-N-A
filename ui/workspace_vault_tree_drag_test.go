// spec: FEAT-001 / EV-MYGO-179, EV-MYGO-288.
package ui

import (
	"os"
	"path/filepath"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestVaultTreeDragReordersSiblings(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, view, filename := snippetsWorkspace(t, "Root.md", "# Root\n", splitMode)
	root := filepath.Dir(filename)
	if err := os.MkdirAll(filepath.Join(root, "Content", "Sources"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Content/Alpha.md", "Content/Beta.md", "Content/Zulu.md", "Content/Sources/Nested.md", "Outside.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("# Note\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	mountVaultDragTree(t, app, view, vault, root, "Content", "Content/Sources")

	drag := func(source, target string, fraction float32, cancel bool) {
		t.Helper()
		dragVaultTreeRows(t, view, source, target, 32, fraction, cancel)
		if app.document.Path != "Root.md" {
			t.Fatalf("drag switched document to %q", app.document.Path)
		}
	}
	const folder = "Sources, 1"
	requireVaultDragOrder(t, view, folder, "Nested", "Alpha", "Beta", "Zulu")
	drag("Zulu", "Alpha", .25, false)
	requireVaultDragOrder(t, view, folder, "Nested", "Zulu", "Alpha", "Beta")
	drag(folder, "Beta", .75, false)
	requireVaultDragOrder(t, view, "Zulu", "Alpha", "Beta", folder, "Nested")
	drag("Beta", "Zulu", .25, true)
	drag("Alpha", "Outside", .25, false)
	requireVaultDragOrder(t, view, "Zulu", "Alpha", "Beta", folder, "Nested", "Outside")
	if _, err := os.Stat(filepath.Join(root, "Content", "Alpha.md")); err != nil {
		t.Fatalf("dropping onto a file in another folder moved the note: %v", err)
	}
}

func TestVaultTreeDragReordersNestedFolders(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, view, filename := snippetsWorkspace(t, "Root.md", "# Root\n", splitMode)
	root := filepath.Dir(filename)
	files := []string{
		"Other/Archive/Outside.md",
		"Projects/Outline/Draft.md",
		"Projects/References/Sources/Record.md",
	}
	for _, name := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("# "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	expanded := []string{"Other", "Other/Archive", "Projects", "Projects/Outline", "Projects/References", "Projects/References/Sources"}
	mountVaultDragTree(t, app, view, vault, root, expanded...)

	const outline, references, archive = "Outline, 1", "References, 1", "Archive, 1"
	// For the depth-one folder row, x=49 hits the icon and x=66 the name.
	const iconX, nameX = 49, 66
	initial := []string{archive, "Outside", outline, "Draft", references, "Sources, 1", "Record"}
	reordered := []string{archive, "Outside", references, "Sources, 1", "Record", outline, "Draft"}
	requireVaultDragOrder(t, view, initial...)
	dragVaultTreeRows(t, view, references, outline, nameX, .25, false)
	requireVaultDragOrder(t, view, reordered...)
	dragVaultTreeRows(t, view, references, archive, iconX, .75, false)
	requireVaultDragOrder(t, view, reordered...)

	prefs, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	reopened, freshView, _ := snippetsWorkspace(t, "Root.md", "# Root\n", splitMode)
	reopened.preferences = &prefs
	mountVaultDragTree(t, reopened, freshView, vault, root, expanded...)
	requireVaultDragOrder(t, freshView, reordered...)
	dragVaultTreeRows(t, freshView, references, outline, iconX, .75, false)
	requireVaultDragOrder(t, freshView, initial...)
	if app.document.Path != "Root.md" || reopened.document.Path != "Root.md" {
		t.Fatalf("folder drag changed open document to %q or %q", app.document.Path, reopened.document.Path)
	}
	for _, name := range files {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || string(got) != "# "+name+"\n" {
			t.Fatalf("folder drag changed %q: %q, %v", name, got, err)
		}
	}
}

func TestVaultTreeDragMovesFilesIntoFoldersAndRoot(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, view, filename := snippetsWorkspace(t, "Root.md", "# Root\n", splitMode)
	root := filepath.Dir(filename)
	for _, folder := range []string{"Archive", "Content/Empty"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(folder)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	const source, archived, moved = "Content/Draft.md", "Archive/Draft.md", "Content/Empty/Draft.md"
	const draft, archive = "# Working Draft\n", "# Archived Draft\n"
	for path, content := range map[string]string{source: draft, archived: archive} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	mountVaultDragTree(t, app, view, vault, root, "Content")
	checkFile := func(path, want string) {
		t.Helper()
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if want == "" && os.IsNotExist(err) {
			return
		}
		if err != nil || want == "" || string(got) != want {
			t.Fatalf("%s = %q, %v; wanted %q (empty means absent)", path, got, err, want)
		}
	}
	dragVaultTreeRows(t, view, "Draft", "Archive, 1", 32, .5, false)
	if app.problem == "" {
		t.Fatal("drop on the folder containing a same-named file did not report a conflict")
	}
	checkFile(source, draft)
	checkFile(archived, archive)
	dragVaultTreeRows(t, view, "Draft", "Empty, 0", 32, .1, false)
	requireVaultDragOrder(t, view, "Draft", "Empty, 0")
	checkFile(source, draft)
	dragVaultTreeRows(t, view, "Draft", "Empty, 0", 32, .5, false)
	checkFile(moved, draft)
	checkFile(source, "")
	requireVaultDragOrder(t, view, "Empty, 1", "Draft")

	from := vaultDragRowBounds(t, view, "Draft")
	heading, found := view.AccessByRole(mygoui.RoleHeading, filepath.Base(root))
	if !found {
		t.Fatal("missing Vault heading")
	}
	x, y := heading.Bounds.X+heading.Bounds.W/2, heading.Bounds.Y+heading.Bounds.H/2
	view.Press(from.X+32, from.Y+from.H/2)
	view.Move(x, y)
	view.Release(x, y)
	checkFile("Draft.md", draft)
	checkFile(moved, "")
	if app.document.Path != "Root.md" {
		t.Fatalf("file drag changed open document to %q", app.document.Path)
	}
}

func mountVaultDragTree(t *testing.T, app *workspaceWindow, view *mygoui.Tester, vault *workspace.Vault, root string, expanded ...string) {
	t.Helper()
	entries, count, err := readWorkspaceEntries(vault, false)
	if err != nil {
		t.Fatal(err)
	}
	app.vault, app.root, app.vaultVisible = vault, root, true
	app.vaultFileCount = count
	app.folders = make(map[string]*bool)
	for _, folder := range expanded {
		*app.folder(folder) = true
	}
	app.updateEntries(entries)
	view.Frame()
}

func vaultDragRowBounds(t *testing.T, view *mygoui.Tester, label string) mygoui.Rect {
	t.Helper()
	row, found := view.AccessByRole(mygoui.RoleButton, label)
	if !found {
		t.Fatalf("missing Files row %q", label)
	}
	return row.Bounds
}

func requireVaultDragOrder(t *testing.T, view *mygoui.Tester, labels ...string) {
	t.Helper()
	last := float32(-1)
	for _, label := range labels {
		y := vaultDragRowBounds(t, view, label).Y
		if y <= last {
			t.Fatalf("want Files order %q, got %q at y=%g after y=%g", labels, label, y, last)
		}
		last = y
	}
}

func dragVaultTreeRows(t *testing.T, view *mygoui.Tester, source, target string, xOffset, targetYFraction float32, cancel bool) {
	t.Helper()
	from, to := vaultDragRowBounds(t, view, source), vaultDragRowBounds(t, view, target)
	x, y := to.X+xOffset, to.Y+to.H*targetYFraction
	view.Press(from.X+xOffset, from.Y+from.H/2)
	view.Move(x, y)
	if cancel {
		view.Key(0, mygoui.KeyEscape)
	}
	view.Release(x, y)
}
