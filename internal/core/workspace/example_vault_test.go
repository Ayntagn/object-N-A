// spec: EV-UI-091 / EV-UI-092
package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestRestoreOpenDraftRecreatesOnlyAMissingNote(t *testing.T) {
	root := t.TempDir()
	vault, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vault.Close() })
	const filename = "Draft.md"
	pending, err := vault.PrepareDocumentRecreation(filename)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := vault.SaveDocument(pending, "unsaved text\n")
	if err != nil || saved.Path != filename || saved.Text != "unsaved text\n" {
		t.Fatalf("recreated note = %#v, %v", saved, err)
	}
	if _, err := vault.SaveDocument(pending, "second writer\n"); !errors.Is(err, ErrChangedOnDisk) {
		t.Fatalf("save to recreated path = %v, want ErrChangedOnDisk", err)
	}
	read, err := vault.ReadDocument(filename)
	if err != nil || read.Text != saved.Text {
		t.Fatalf("note after competing save = %#v, %v", read, err)
	}
}

func TestExampleRestoreStagesBeforeReplacingUserFiles(t *testing.T) {
	parent := t.TempDir()
	source := fstest.MapFS{
		"Welcome.md":        {Data: []byte("# Welcome\n")},
		"data/readings.csv": {Data: []byte("day,reading\n1,42\n")},
	}
	install, err := PlanExampleVaultRestore(parent)
	if err != nil || install.Existing() {
		t.Fatalf("install plan = %#v, %v", install, err)
	}
	builtin, err := BuiltinExampleVault()
	if err != nil {
		t.Fatal(err)
	}
	if installed, err := install.Apply(builtin); err != nil || !installed {
		t.Fatalf("install = %v, %v", installed, err)
	}

	extra := filepath.Join(install.Root(), "Draft.md")
	if err := os.WriteFile(extra, []byte("local draft"), 0o600); err != nil {
		t.Fatal(err)
	}
	restore, err := PlanExampleVaultRestore(parent)
	if err != nil || !restore.Existing() {
		t.Fatalf("restore plan = %#v, %v", restore, err)
	}
	broken := fstest.MapFS{
		"Welcome.md": {Data: []byte("# Incomplete new sample\n")},
		"link":       {Mode: fs.ModeSymlink, Data: []byte("Welcome.md")},
	}
	if restored, err := restore.Apply(broken); err == nil || restored {
		t.Fatalf("broken source = %v, %v", restored, err)
	}
	if data, err := os.ReadFile(extra); err != nil || string(data) != "local draft" {
		t.Fatalf("draft after failed staging = %q, %v", data, err)
	}

	if restored, err := restore.Apply(source); err != nil || !restored {
		t.Fatalf("restore = %v, %v", restored, err)
	}
	if _, err := os.Stat(extra); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("draft after confirmed restore = %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(restore.Root(), "data", "readings.csv")); err != nil || string(data) != "day,reading\n1,42\n" {
		t.Fatalf("restored readings = %q, %v", data, err)
	}
}
