// spec: EV-UI-082, Properties / File Info.
package ui

import (
	"os"
	"path/filepath"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestPropertiesFileInfoKeepsDetachedPathReadable(t *testing.T) {
	vaultRoot := t.TempDir()
	noteRoot := filepath.Join(t.TempDir(), "private", "runtime-captures", "vault-tag-pills", "native", "Vault", "Content")
	if err := os.MkdirAll(noteRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	vault, err := workspace.Open(vaultRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	storage, err := workspace.Open(noteRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	source := "---\ntags: [zulu-frequency, zzz-contiguous-shared-label]\n---\n# Tag Counts B\n"
	document, err := storage.CreateDocument("Tag Counts B.md", source)
	if err != nil {
		t.Fatal(err)
	}
	tab := &workspaceTab{storage: storage, folderRoot: noteRoot, mode: splitMode}
	app := &workspaceWindow{preferences: &workspacePreferences{}, workspaceTab: tab, tabs: []*workspaceTab{tab},
		blackout: true, vault: vault, root: vaultRoot}
	app.setTabDocument(tab, document)
	tester := mygoui.NewTester(app.propertiesPanel, 348, 620)
	if _, ok := tester.Access("Not in a vault"); !ok {
		t.Fatal("File Info assigned a detached note to the current Vault")
	}
	absolute := filepath.Join(noteRoot, document.Path)
	path, foundPath := tester.Access(absolute)
	copy, foundCopy := tester.AccessByRole(mygoui.RoleButton, "Copy")
	if !foundPath || !foundCopy || path.Bounds.H <= 15 || path.Bounds.X+path.Bounds.W > copy.Bounds.X ||
		copy.Bounds.Y < path.Bounds.Y || copy.Bounds.Y+copy.Bounds.H > path.Bounds.Y+15 || copy.Offscreen {
		t.Fatalf("long path or its copy button is obscured: path=%+v copy=%+v", path, copy)
	}
}
