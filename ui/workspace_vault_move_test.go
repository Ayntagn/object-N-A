// spec: FEAT-001 / EV-MYGO-288
package ui

import (
	"testing"

	"cyberwriter/internal/core/workspace"
)

func TestMoveVaultFileToFolderKeepsOpenDraft(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	root := t.TempDir()
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	if err := vault.CreateFolder("Ready"); err != nil {
		t.Fatal(err)
	}
	const saved, draft, target = "# Saved\n", "# Edited\n", "Ready/Note.md"
	note, err := vault.CreateDocument("Note.md", saved)
	if err != nil {
		t.Fatal(err)
	}
	application := &workspaceApplication{}
	app := &workspaceWindow{application: application, preferences: &application.preferences,
		vault: vault, root: root, workspaceTab: &workspaceTab{}, folders: make(map[string]*bool)}
	other := &workspaceWindow{application: application, preferences: &application.preferences,
		vault: vault, root: root, workspaceTab: &workspaceTab{mode: splitMode}}
	application.windows = []*workspaceWindow{app, other}
	other.tabs = []*workspaceTab{other.workspaceTab}
	other.setTabDocument(other.workspaceTab, note)
	buffer := other.draft
	buffer.Set(draft)
	other.recordRecent(note.Path)
	if err := app.moveVaultFileToFolder(note.Path, "Ready"); err != nil {
		t.Fatal(err)
	}
	if other.document.Path != target || other.draft != buffer || !other.dirty() || other.draft.String() != draft {
		t.Fatalf("open draft after move: path=%q, text=%q, dirty=%v", other.document.Path, other.draft.String(), other.dirty())
	}
	if snapshot := other.windowSnapshot(); snapshot.Active != target || len(snapshot.Recent) != 1 || snapshot.Recent[0] != target {
		t.Fatalf("workspace paths after move: %+v", snapshot)
	}
	if doc, err := vault.ReadDocument(target); err != nil || doc.Text != saved {
		t.Fatalf("moving wrote unsaved text to disk: %q, %v", doc.Text, err)
	}
	if !other.saveTab(other.workspaceTab, nil) {
		t.Fatal(other.problem)
	}
	if doc, err := vault.ReadDocument(target); err != nil || doc.Text != draft {
		t.Fatalf("saving relocated draft: %q, %v", doc.Text, err)
	}
}
