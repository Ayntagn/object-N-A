// spec: FEAT-001 / EV-UI-009 / EV-UI-194 / EV-UI-197
package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestVaultTreeFolderAndFileRows(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, tester, original := snippetsWorkspace(t, "Root.md", "# Root\n", splitMode)
	root := filepath.Dir(original)
	if err := os.MkdirAll(filepath.Join(root, "Content", "Nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{
		"Content/Alpha Body.md",
		"Content/CwNeedle File And Body.md",
		"Content/cwneedle File Only.md",
		"Content/Nested/Beta.md",
		"Content/Zulu Body.md",
	} {
		if err := os.WriteFile(filepath.Join(root, relative), []byte("# Note\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	entries, count, err := readWorkspaceEntries(vault, false)
	if err != nil {
		t.Fatal(err)
	}
	app.vault, app.root, app.vaultVisible = vault, root, true
	app.vaultFileCount = count
	app.folders = make(map[string]*bool)
	app.updateEntries(entries)
	tester.Frame()

	const content = "Content, 5"
	folder, found := tester.AccessByRole(mygoui.RoleButton, content)
	if !found || folder.Expanded || folder.Bounds.W != 340 || folder.Bounds.H != 24 {
		t.Fatalf("collapsed folder row = %+v, found = %v", folder, found)
	}
	if err := tester.Click(content); err != nil {
		t.Fatal(err)
	}
	folder, found = tester.AccessByRole(mygoui.RoleButton, content)
	if !found || !folder.Expanded {
		t.Fatalf("expanded folder row = %+v, found = %v", folder, found)
	}
	if _, found := tester.AccessByRole(mygoui.RoleButton, "Nested, 1"); !found {
		t.Fatal("nested folder is missing")
	}
	var order []string
	for _, label := range []string{"Nested, 1", "Alpha Body", "CwNeedle File And Body", "cwneedle File Only", "Zulu Body"} {
		row, found := tester.AccessByRole(mygoui.RoleButton, label)
		if !found || row.PosInSet != len(order)+2 {
			t.Fatalf("sidebar rows %v → %q at %d, found = %v", order, label, row.PosInSet, found)
		}
		order = append(order, label)
	}
	if err := tester.RightClick(content); err != nil {
		t.Fatal(err)
	}
	if menu := tester.Menu(); !slices.Contains(menu, "New Folder…") || !slices.Contains(menu, "New Document Here…") {
		t.Fatalf("folder menu = %v", menu)
	}
	tester.CloseMenu()
	tester.Key(0, mygoui.KeyLeft)
	if _, found := tester.AccessByRole(mygoui.RoleButton, "Alpha Body"); found {
		t.Fatal("Left left the folder expanded")
	}
	tester.Key(0, mygoui.KeyRight)
	leaf, found := tester.AccessByRole(mygoui.RoleButton, "Alpha Body")
	if !found || leaf.Bounds.W != 340 || leaf.Bounds.H != 24 {
		t.Fatalf("reopened file row = %+v, found = %v", leaf, found)
	}
	if err := tester.RightClick("Alpha Body"); err != nil {
		t.Fatal(err)
	}
	if menu := tester.Menu(); !slices.Contains(menu, "Open in New Tab") {
		t.Fatalf("file menu = %v", menu)
	}
	tester.CloseMenu()
	if err := tester.Click("Alpha Body"); err != nil {
		t.Fatal(err)
	}
	if app.document.Path != "Content/Alpha Body.md" {
		t.Fatalf("opened %q", app.document.Path)
	}
	// MyGo exposes AccessSelected as Checked on an AX button.
	if leaf, found := tester.AccessByRole(mygoui.RoleButton, "Alpha Body"); !found || !leaf.Checked {
		t.Fatalf("active file row = %+v, found = %v", leaf, found)
	}
}
