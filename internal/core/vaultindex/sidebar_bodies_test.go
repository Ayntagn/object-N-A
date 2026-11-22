// spec: EV-UI-184 / EV-UI-188 / EV-MYGO-316
package vaultindex

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"cyberwriter/internal/core/workspace"
)

func TestSidebarBodyIndexMatchesSavedTextAfterExternalEdit(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "Notes.md")
	original := []byte("body still mentions 航线 and Cafe\u0301, xß and sß.\n")
	if err := os.WriteFile(file, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "航线.txt"), []byte("\ufeff\ufeff航线存档"), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	entries, err := vault.SidebarEntries(true)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cache := filepath.Join(t.TempDir(), "search.db")
	if err := RefreshSidebarBodyIndex(ctx, vault, entries, cache); err != nil {
		t.Fatal(err)
	}
	if vault.HasIndexedTextVersions() {
		if info, err := os.Stat(cache); err != nil || info.Size() == 0 {
			t.Fatalf("missing saved body index: %v", err)
		}
	}
	var index *Index
	check := func(query string) {
		t.Helper()
		want := index.SearchSidebar(ctx, vault, entries, query, "")
		got := index.SearchSidebarWithBodyIndex(ctx, vault, entries, query, "", cache)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("query %q returned %#v, want %#v", query, got, want)
		}
	}
	const queryWithRetainedBOM = "\ufeff航"
	if hits := index.SearchSidebar(ctx, vault, entries, queryWithRetainedBOM, ""); len(hits) != 1 || hits[0].Entry.Path != "航线.txt" {
		t.Fatalf("exact search did not match the second BOM: %#v", hits)
	}
	for _, query := range []string{"航线", "CAFÉ", "ss", "港口", queryWithRetainedBOM} {
		check(query)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	edited := []byte("body still mentions 港口 and Cafe\u0301, xß and sß.\n")
	if err := os.WriteFile(file, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(file, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	check("港口")
	check("航线")
	if err := RefreshSidebarBodyIndex(ctx, vault, entries, cache); err != nil {
		t.Fatal(err)
	}
	check("港口")
	check("航线")
}
