// spec: FEAT-001 / EV-UI-027, EV-UI-030, EV-UI-031
package workspace_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"cyberwriter/internal/core/workspace"
)

// regression: creating a Vault note on a volume without hard links.
func TestCreateVaultNotePublishesInParent(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "Untitled 1.md", "separate note")
	writeFixture(t, dir, "Notes/Untitled.md", "existing note")
	vault := openFixture(t, dir)
	if _, err := vault.CreateDocument("Notes/Untitled.md", "replacement"); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("creating an existing note returned %v", err)
	}
	const newText = "# 新笔记\r\n"
	if _, err := vault.CreateDocument("Notes/Untitled 1.md", newText); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"Untitled 1.md":       "separate note",
		"Notes/Untitled.md":   "existing note",
		"Notes/Untitled 1.md": newText,
	} {
		contents, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil || string(contents) != want {
			t.Fatalf("note %q = %q, %v; want %q", name, contents, err, want)
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "Notes"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("note directory contains unexpected entries: %v, %v", entries, err)
	}
}

func TestCreateAndDuplicatePreserveExistingNotes(t *testing.T) {
	dir := t.TempDir()
	vault := openFixture(t, dir)
	if err := vault.CreateFolder("Notes"); err != nil {
		t.Fatal(err)
	}
	note, err := vault.CreateDocument("Notes/Note.md", "# 中文\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vault.CreateDocument(note.Path, "replacement"); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("creating an existing note returned %v", err)
	}
	if _, err := vault.SaveDocument(note, "# saved\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, filepath.FromSlash(note.Path)), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.CreateDocument("Notes/Note copy.md", "existing copy"); err != nil {
		t.Fatal(err)
	}
	copy, err := vault.DuplicateDocument(note.Path)
	if err != nil {
		t.Fatal(err)
	}
	if copy.Path == note.Path || copy.Path == "Notes/Note copy.md" || filepath.ToSlash(filepath.Dir(copy.Path)) != "Notes" {
		t.Fatalf("duplicate path = %q", copy.Path)
	}
	mode := func(path string) fs.FileMode {
		t.Helper()
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		return info.Mode().Perm()
	}
	if got, want := mode(copy.Path), mode(note.Path); got != want {
		t.Fatalf("duplicate permissions = %o; want %o", got, want)
	}
	for path, want := range map[string]string{
		note.Path:            "# saved\r\n",
		"Notes/Note copy.md": "existing copy",
		copy.Path:            "# saved\r\n",
	} {
		document, err := vault.ReadDocument(path)
		if err != nil || document.Text != want {
			t.Fatalf("note %q = %q, %v; want %q", path, document.Text, err, want)
		}
	}
}

func TestMoveRetainsDraftConflictAndTrashRetainsFile(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "Notes/Note.md", "before")
	writeFixture(t, dir, "Note.md", "occupied")
	vault := openFixture(t, dir)
	opened, err := vault.ReadDocument("Notes/Note.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vault.MoveDocument(opened.Path, "Note.md"); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("moving over an existing note returned %v", err)
	}
	if occupied, err := vault.ReadDocument("Note.md"); err != nil || occupied.Text != "occupied" {
		t.Fatalf("occupied note = %q, %v", occupied.Text, err)
	}
	if err := vault.CreateFolder("Other"); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, dir, opened.Path, "edited outside")
	moved, err := vault.MoveDocument(opened.Path, "Other/Moved.md")
	if err != nil || moved.Path != "Other/Moved.md" || moved.Text != "edited outside" {
		t.Fatalf("moved note = %+v, %v", moved, err)
	}
	if _, err := vault.ReadDocument(opened.Path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("original path returned %v", err)
	}
	if _, err := vault.SaveDocument(opened.Relocated(moved), "edit in draft"); !errors.Is(err, workspace.ErrChangedOnDisk) {
		t.Fatalf("saving the relocated stale buffer returned %v", err)
	}
	if err := vault.TrashDocument(moved.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.ReadDocument(moved.Path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("trashed path returned %v", err)
	}
	trash := filepath.Join(dir, workspace.TrashDirectory)
	items, err := os.ReadDir(trash)
	if err != nil || len(items) != 1 {
		t.Fatalf("trash contains %v, %v", items, err)
	}
	data, err := os.ReadFile(filepath.Join(trash, items[0].Name(), "Other", "Moved.md"))
	if err != nil || string(data) != "edited outside" {
		t.Fatalf("recoverable note = %q, %v", data, err)
	}
	entries, err := vault.Entries()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Path == workspace.TrashDirectory {
			t.Fatal("recovery directory appears among normal Vault entries")
		}
	}
}

func TestVaultMutationsRejectTraversalAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "Real/Note.md", "protected")
	vault := openFixture(t, dir)
	if _, err := vault.CreateDocument("../Escape.md", "escaped"); err == nil {
		t.Fatal("creation accepted traversal")
	}
	if err := os.Symlink("Real", filepath.Join(dir, "Alias")); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	if _, err := vault.MoveDocument("Real/Note.md", "Alias/Moved.md"); err == nil {
		t.Fatal("move accepted a symlink directory")
	}
	if err := os.Symlink("Real/Note.md", filepath.Join(dir, "Alias.md")); err != nil {
		t.Skipf("file symlink unavailable: %v", err)
	}
	if err := vault.TrashDocument("Alias.md"); err == nil {
		t.Fatal("trash accepted a symlink source")
	}
	note, err := vault.ReadDocument("Real/Note.md")
	if err != nil || note.Text != "protected" {
		t.Fatalf("protected note = %q, %v", note.Text, err)
	}
}
