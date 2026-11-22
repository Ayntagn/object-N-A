//go:build darwin || linux

// spec: EV-MYGO-326

package workspace_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"object-na.local/vaultdelta"
)

func TestSidebarDeltaRefreshesHardLinkAliases(t *testing.T) {
	root := t.TempDir()
	write := func(relative, content string) string {
		t.Helper()
		writeFixture(t, root, relative, content)
		return filepath.Join(root, filepath.FromSlash(relative))
	}
	// The alias in Alpha is watched but hidden when the sidebar lists only supported types.
	const sourcePath = "Alpha/Linked/Source.go"
	const hint = "Alpha/Linked"
	source := write(sourcePath, "original\n")
	alias := write("Beta/Alias.md", "old separate file\n")
	write("Alpha/Plain/Note.md", "plain note\n")
	vault := openFixture(t, root)
	previous, err := vault.SidebarEntries(false)
	if err != nil {
		t.Fatal(err)
	}
	refresh := func(dir string, wantFull bool) {
		t.Helper()
		updated, scope, err := vault.RefreshSidebarEntries(previous, false, vaultdelta.Hints(dir))
		if err != nil {
			t.Fatal(err)
		}
		full, err := vault.SidebarEntries(false)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(updated, full) {
			t.Fatalf("hint %s left a stale Vault snapshot: %+v, want %+v", dir, updated, full)
		}
		if scope.Full() != wantFull {
			t.Fatalf("hint %s used a full refresh = %t, want %t", dir, scope.Full(), wantFull)
		}
		previous = updated
	}

	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(alias, source); err != nil {
		t.Fatal(err)
	}
	write(sourcePath, "edited in place through a newly linked name\n")
	refresh(hint, true)

	write("Alpha/Plain/Note.md", "another note in an unrelated branch\n")
	refresh("Alpha/Plain", false)

	write(sourcePath, "another in-place edit before replacing the name\n")
	if err := os.Rename(write("Alpha/Linked/Replacement.go", "replacement\n"), source); err != nil {
		t.Fatal(err)
	}
	refresh(hint, true)
	write(sourcePath, "regular file after the replacement\n")
	refresh(hint, false)
}
