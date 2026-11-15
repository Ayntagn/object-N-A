// spec: FEAT-001 / EV-UI-030, EV-UI-031, EV-UI-070
package workspace_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"cyberwriter/internal/core/workspace"
)

func TestSameNameNotesStaySeparateOnSaveAndReopen(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "Same.md", "# Root\n\nROOT")
	writeFixture(t, dir, "Notes/Same.md", "# Nested\n\nNESTED")
	vault := openFixture(t, dir)
	entries, err := vault.Entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || !entries[0].IsDir || len(entries[0].Children) != 1 ||
		entries[0].Children[0].Path != "Notes/Same.md" || entries[1].Path != "Same.md" {
		t.Fatalf("unexpected file tree: %+v", entries)
	}
	nested, err := vault.ReadDocument("Notes/Same.md")
	if err != nil || nested.Text != "# Nested\n\nNESTED" {
		t.Fatalf("nested note = %q, %v", nested.Text, err)
	}
	root, err := vault.ReadDocument("Same.md")
	if err != nil || root.Text != "# Root\n\nROOT" {
		t.Fatalf("root note = %q, %v", root.Text, err)
	}
	if _, err := vault.SaveDocument(root, root.Text+"7"); err != nil {
		t.Fatal(err)
	}
	reopened := openFixture(t, dir)
	for path, want := range map[string]string{
		"Same.md":       "# Root\n\nROOT7",
		"Notes/Same.md": "# Nested\n\nNESTED",
	} {
		document, err := reopened.ReadDocument(path)
		if err != nil || document.Text != want {
			t.Fatalf("reopened %s = %q, %v; want %q", path, document.Text, err, want)
		}
	}
}

func TestSavePreservesExternalEdits(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "Note.md", "before")
	vault := openFixture(t, dir)
	document, err := vault.ReadDocument("Note.md")
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, dir, "Note.md", "edited outside")
	if _, err := vault.SaveDocument(document, "edited here"); !errors.Is(err, workspace.ErrChangedOnDisk) {
		t.Fatalf("saving stale buffer returned %v", err)
	}
	current, err := vault.ReadDocument("Note.md")
	if err != nil || current.Text != "edited outside" {
		t.Fatalf("disk content = %q, %v", current.Text, err)
	}
}

func TestGraphMarkdownExtensionCanBeEditedAndMoved(t *testing.T) {
	dir := t.TempDir()
	vault := openFixture(t, dir)
	note, err := vault.CreateDocument("Graph Note.markdown", "before")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vault.SaveDocument(note, "after"); err != nil {
		t.Fatal(err)
	}
	moved, err := vault.MoveDocument(note.Path, "Renamed.markdown")
	if err != nil {
		t.Fatal(err)
	}
	if moved.Path != "Renamed.markdown" || moved.Text != "after" {
		t.Fatalf("unexpected moved note: %+v", moved)
	}
}

func TestVaultPathsStayRelative(t *testing.T) {
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, "vault")
	if err := os.Mkdir(vaultDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, dir, "Outside.md", "outside")
	vault := openFixture(t, vaultDir)
	if _, err := vault.ReadDocument("../Outside.md"); err == nil {
		t.Fatal("read accepted a path outside the vault")
	}
}

func writeFixture(t *testing.T, dir, relative, text string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func openFixture(t *testing.T, dir string) *workspace.Vault {
	t.Helper()
	vault, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := vault.Close(); err != nil {
			t.Error(err)
		}
	})
	return vault
}
