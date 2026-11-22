// spec: EV-UI-188 / EV-UI-189 / EV-MYGO-253

package workspace_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	"object-na.local/vaultdelta"
)

func TestSidebarDeltaMatchesFullRefresh(t *testing.T) {
	root := t.TempDir()
	native := func(relative string) string { return filepath.Join(root, filepath.FromSlash(relative)) }
	write := func(relative, body string) {
		t.Helper()
		file := native(relative)
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const source = "Alpha/Leaf/First.md"
	write(source, "[[Shared]] #old")
	write("Beta/Shared.md", "## Another note")
	write("Beta/Keep.markdown", "[[First]]")
	write("Alpha/Leaf/.secret/hidden.md", "hidden text")
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	previous, err := vault.SidebarEntries(false)
	if err != nil {
		t.Fatal(err)
	}
	initial := previous
	var index *vaultindex.Index
	index, err = index.RefreshMetadata(vault, previous)
	if err != nil {
		t.Fatal(err)
	}
	refresh := func(scope vaultdelta.Scope, wantFull bool) {
		t.Helper()
		updated, used, err := vault.RefreshSidebarEntries(previous, false, scope)
		if err != nil || used.Full() != wantFull {
			t.Fatalf("RefreshSidebarEntries(%q) used full = %t, want %t, error = %v", scope.Directories(), used.Full(), wantFull, err)
		}
		full, err := vault.SidebarEntries(false)
		if err != nil || !reflect.DeepEqual(updated, full) {
			t.Fatalf("delta differs from full ordered tree\n delta: %+v\n full: %+v\n err: %v", updated, full, err)
		}
		baseline, err := index.RefreshMetadata(vault, full)
		if err != nil {
			t.Fatal(err)
		}
		next, err := index.RefreshMetadata(vault, updated)
		if err != nil || !reflect.DeepEqual(next.Graph(), baseline.Graph()) || !slices.Equal(next.Tags(source), baseline.Tags(source)) {
			t.Fatalf("metadata differs from a full ordered refresh: %v", err)
		}
		previous, index = updated, next
	}
	info, err := os.Stat(native(source))
	if err != nil {
		t.Fatal(err)
	}
	write(source, "[[Shared]] #new")
	if err := os.Chtimes(native(source), info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	refresh(vaultdelta.Hints("Alpha/Leaf"), false)
	if got := index.Tags(source); !slices.Equal(got, []string{"new"}) {
		t.Fatalf("same mtime save tags = %q, want new", got)
	}
	if err := os.Rename(native(source), native("Beta/Moved.md")); err != nil {
		t.Fatal(err)
	}
	refresh(vaultdelta.Hints("Alpha/Leaf").Merge(vaultdelta.Hints("Beta")), false)
	if !slices.ContainsFunc(initial[0].Children[0].Children, func(entry workspace.Entry) bool { return entry.Path == source }) {
		t.Fatal("the older snapshot lost the original file during the rename")
	}
	if err := os.Rename(native("Alpha/Leaf"), native("Alpha/Renamed")); err != nil {
		t.Fatal(err)
	}
	refresh(vaultdelta.Hints("Alpha/Leaf"), true)
	if err := os.Rename(native("Beta"), native("BETA")); err != nil {
		t.Fatal(err)
	}
	refresh(vaultdelta.Hints("Beta"), true)
}
