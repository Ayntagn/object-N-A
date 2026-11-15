// spec: FEAT-001 / vault-file-order, explicit sibling order in object-N/A.
package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestVaultCustomFileOrder(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CYBERWRITER_CONFIG_DIR", config)
	app, view, rootNote := snippetsWorkspace(t, "Root.md", "# Root\n", splitMode)
	root := filepath.Dir(rootNote)
	if err := os.MkdirAll(filepath.Join(root, "Content", "Sources"), 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"Root.md": "# Root\n", "Content/Alpha.md": "# Alpha\n", "Content/Zulu.md": "# Zulu\n",
		"Content/Sources/Nested.md": "# Nested\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	show := func(app *workspaceWindow, view *mygoui.Tester) {
		t.Helper()
		entries, count, err := readWorkspaceEntries(vault, false)
		if err != nil {
			t.Fatal(err)
		}
		app.vault, app.root, app.vaultVisible = vault, root, true
		app.vaultFileCount = count
		if app.folders == nil {
			app.folders = make(map[string]*bool)
		}
		*app.folder("Content") = true
		app.updateEntries(entries)
		view.Frame()
	}
	checkOrder := func(view *mygoui.Tester, labels ...string) {
		t.Helper()
		last := -1
		for _, label := range labels {
			row, ok := view.AccessByRole(mygoui.RoleButton, label)
			if !ok || row.PosInSet <= last {
				t.Fatalf("want row order %q; %q at %d, found=%v", labels, label, row.PosInSet, ok)
			}
			last = row.PosInSet
		}
	}
	chooseOrder := func(view *mygoui.Tester, label, action string) {
		t.Helper()
		if err := view.RightClick(label); err != nil {
			t.Fatal(err)
		}
		if err := view.ChooseMenuItem("Sort Order", action); err != nil {
			t.Fatal(err)
		}
	}
	const folder = "Sources, 1"
	show(app, view)
	checkOrder(view, folder, "Alpha", "Zulu", "Root")
	chooseOrder(view, "Zulu", "Move Up")
	checkOrder(view, folder, "Zulu", "Alpha", "Root")
	if err := view.Click(folder); err != nil {
		t.Fatal(err)
	}
	view.Key(mygoui.Shift, mygoui.KeyF10)
	if err := view.ChooseMenuItem("Sort Order", "Move Down"); err != nil {
		t.Fatalf("keyboard context menu: %v", err)
	}
	checkOrder(view, "Zulu", folder, "Nested", "Alpha", "Root")

	filename := filepath.Join(config, "settings.json")
	prior, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CYBERWRITER_CONFIG_DIR", filename)
	chooseOrder(view, "Alpha", "Move Up")
	if content, err := os.ReadFile(filename); err != nil || !bytes.Equal(content, prior) ||
		!strings.Contains(app.problem, "Could not save file order") {
		t.Fatalf("failed write should leave preferences intact: %v, %q", err, app.problem)
	}
	checkOrder(view, "Zulu", folder, "Nested", "Alpha", "Root")
	t.Setenv("CYBERWRITER_CONFIG_DIR", config)

	files["Content/Aardvark.md"] = "# Aardvark\n"
	if err := os.WriteFile(filepath.Join(root, "Content", "Aardvark.md"), []byte(files["Content/Aardvark.md"]), 0o600); err != nil {
		t.Fatal(err)
	}
	reloaded, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	reopened, fresh, _ := snippetsWorkspace(t, "Root.md", files["Root.md"], splitMode)
	reopened.preferences = &reloaded
	show(reopened, fresh)
	checkOrder(fresh, "Zulu", folder, "Alpha", "Aardvark", "Root")
	for _, action := range []string{"Use Custom Order", "Use Custom Order", "Reset Custom Orders"} {
		if err := fresh.Click("More"); err != nil {
			t.Fatal(err)
		}
		if err := fresh.ChooseMenuItem("File Order", action); err != nil {
			t.Fatal(err)
		}
		if action == "Use Custom Order" && reopened.preferences.VaultFileOrders[root].Enabled {
			checkOrder(fresh, "Zulu", folder, "Alpha", "Aardvark", "Root")
		} else {
			checkOrder(fresh, folder, "Aardvark", "Alpha", "Zulu", "Root")
		}
	}
	reset, err := loadWorkspacePreferences()
	if err != nil || len(reset.VaultFileOrders) != 0 {
		t.Fatalf("reset order was not persisted: %v, %v", err, reset.VaultFileOrders)
	}
	for name, want := range files {
		if data, err := os.ReadFile(filepath.Join(root, name)); err != nil || string(data) != want {
			t.Fatalf("sorting changed vault file %q: %q, %v", name, data, err)
		}
	}
}
