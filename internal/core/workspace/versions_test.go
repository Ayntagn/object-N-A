// spec: FEAT-001 / EV-UI-028, EV-UI-030
package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNoteVersionsPersistByPathAndFollowMove(t *testing.T) {
	config := t.TempDir()
	root := t.TempDir()
	note := filepath.Join(root, "Note.md")
	other := filepath.Join(root, "Other", "Note.md")
	renamed := filepath.Join(root, "Renamed.md")
	store := VersionHistory{Directory: config}
	for _, entry := range []struct{ path, text string }{
		{note, "# 第一版\r\n😀\r\n"},
		{other, "# Other note\n"},
		{renamed, "# Stale history at destination\n"},
		{note, "# Second version\n"},
	} {
		if err := store.Record(entry.path, entry.text); err != nil {
			t.Fatal(err)
		}
	}

	reopened := VersionHistory{Directory: config}
	assertVersions := func(path string, want ...string) {
		t.Helper()
		versions, err := reopened.List(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(versions) != len(want) {
			t.Fatalf("history for %s has %d versions, want %d", path, len(versions), len(want))
		}
		for i, version := range versions {
			got, err := reopened.Read(path, version)
			if err != nil || got != want[i] {
				t.Fatalf("history for %s version %d = %q, %v, want %q", path, i, got, err, want[i])
			}
		}
	}
	assertVersions(note, "# Second version\n", "# 第一版\r\n😀\r\n")
	assertVersions(other, "# Other note\n")
	if err := reopened.Move(note, renamed); err != nil {
		t.Fatal(err)
	}
	assertVersions(note)
	assertVersions(renamed, "# Second version\n", "# 第一版\r\n😀\r\n")
	if err := reopened.Retire(renamed); err != nil {
		t.Fatal(err)
	}
	assertVersions(renamed)

	archived, err := filepath.Glob(filepath.Join(config, "retired", "*", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	var contents []string
	for _, name := range archived {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		contents = append(contents, string(data))
	}
	slices.Sort(contents)
	if want := []string{"# Second version\n", "# Stale history at destination\n", "# 第一版\r\n😀\r\n"}; !slices.Equal(contents, want) {
		t.Fatalf("archived history = %q, want %q", contents, want)
	}
}
