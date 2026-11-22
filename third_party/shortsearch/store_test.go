// spec: EV-UI-184 / EV-UI-188 / EV-MYGO-316

package shortsearch

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ncruces/go-sqlite3/driver"
)

func TestLookupFoldedBigrams(t *testing.T) {
	ctx := t.Context()
	store := openStore(t, filepath.Join(t.TempDir(), "indices #100%.sqlite3"))
	documents := []Document{
		{"笔记/港口.md", "1", "\ufeff岬湾港 Qx\nCafe\u0301\na \"quoted\" OR [brackets]\nStraße 👩‍🔬实验。"},
		{"实验.txt", "2", "岬和湾港，Q\nX 及 👩‍🔬实验。"},
		{"case.txt", "3", "xßt，内部\ufeff标志"},
	}
	if err := store.Update(ctx, documents, nil); err != nil {
		t.Fatal(err)
	}
	wantVersions := map[string]string{"笔记/港口.md": "1", "实验.txt": "2", "case.txt": "3"}
	for _, tc := range []struct {
		query string
		paths []string
	}{
		{"岬湾", []string{"笔记/港口.md"}},
		{"qx", []string{"笔记/港口.md"}},
		{"CAFÉ", []string{"笔记/港口.md"}},
		{"STRASSE", []string{"笔记/港口.md"}},
		{"👩‍🔬实", []string{"笔记/港口.md", "实验.txt"}},
		{"\" OR [", []string{"笔记/港口.md"}},
		{"\ufeff标", []string{"case.txt"}},
		{"\ufeff岬", nil},
		{"赣魟", nil},
	} {
		got, err := store.Lookup(ctx, tc.query)
		if err != nil {
			t.Fatalf("Lookup(%q): %v", tc.query, err)
		}
		if !maps.Equal(got.Versions, wantVersions) {
			t.Fatalf("Lookup(%q) versions = %q, want %q", tc.query, got.Versions, wantVersions)
		}
		wantMatches := pathSet(tc.paths...)
		if !maps.Equal(got.Matches, wantMatches) {
			t.Fatalf("Lookup(%q) matches = %v, want %v", tc.query, got.Matches, wantMatches)
		}
	}
	got, err := store.Lookup(ctx, "岬")
	if !errors.Is(err, ErrUnfilterable) || got.Versions != nil || got.Matches != nil {
		t.Fatalf("single-rune query returned (%+v, %v), want no filtering", got, err)
	}
}

func TestLookupLongQuery(t *testing.T) {
	ctx := t.Context()
	store := openStore(t, filepath.Join(t.TempDir(), "index.sqlite3"))
	var phrase strings.Builder
	for r := '一'; r < '一'+512; r++ {
		phrase.WriteRune(r)
	}
	query := phrase.String()
	if err := store.Update(ctx, []Document{{"long.md", "old", "前言" + query + "后记"}}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := store.Lookup(ctx, query)
	if err != nil || !maps.Equal(got.Matches, pathSet("long.md")) {
		t.Fatalf("long query = (%v, %v), want the matching path", got.Matches, err)
	}
}

func TestUpdateReplacesAndRemovesOnDisk(t *testing.T) {
	ctx := t.Context()
	filename := filepath.Join(t.TempDir(), "index.sqlite3")
	store := openStore(t, filename)
	if err := store.Update(ctx, []Document{{"edit.md", "first", "极昼旧文"}, {"gone.txt", "gone", "极昼待删除"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openStore(t, filename)
	versions, err := store.Versions(ctx)
	if err != nil || !maps.Equal(versions, map[string]string{"edit.md": "first", "gone.txt": "gone"}) {
		t.Fatalf("reopened Versions = (%q, %v)", versions, err)
	}
	documents := []Document{{"edit.md", "second", "岬湾新文"}, {"moved.txt", "moved", "重新保存"}}
	if err := store.Update(ctx, documents, []string{"gone.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openStore(t, filename)
	for query, paths := range map[string]map[string]struct{}{"极昼": {}, "岬湾": pathSet("edit.md")} {
		got, err := store.Lookup(ctx, query)
		if err != nil || !maps.Equal(got.Versions, map[string]string{"edit.md": "second", "moved.txt": "moved"}) || !maps.Equal(got.Matches, paths) {
			t.Fatalf("reopened Lookup(%q) = (%+v, %v)", query, got, err)
		}
	}
}

func TestUpdateRollsBackFailedBatch(t *testing.T) {
	ctx := t.Context()
	store := openStore(t, filepath.Join(t.TempDir(), "index.sqlite3"))
	if err := store.Update(ctx, []Document{{"existing.md", "original", "极昼旧文"}}, nil); err != nil {
		t.Fatal(err)
	}
	documents := []Document{{"candidate.md", "new", "极昼新文"}, {"", "invalid", "极昼无路径"}}
	if err := store.Update(ctx, documents, []string{"existing.md"}); err == nil {
		t.Fatal("Update accepted an empty document path")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := store.Update(canceled, documents[:1], []string{"existing.md"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Update returned %v", err)
	}
	if _, err := store.Lookup(canceled, "极昼"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Lookup returned %v", err)
	}
	got, err := store.Lookup(ctx, "极昼")
	if err != nil || !maps.Equal(got.Versions, map[string]string{"existing.md": "original"}) ||
		!maps.Equal(got.Matches, pathSet("existing.md")) {
		t.Fatalf("Lookup after failed batch = (%+v, %v), want original index", got, err)
	}
}

func TestOpenRejectsForeignDatabase(t *testing.T) {
	ctx := t.Context()
	filename := filepath.Join(t.TempDir(), "other.sqlite3")
	other, err := driver.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Close() })
	if _, err := other.ExecContext(ctx, `CREATE TABLE notes (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, filename)
	if store != nil {
		_ = store.Close()
	}
	if !errors.Is(err, ErrIncompatible) {
		t.Fatalf("Open foreign database returned %v, want ErrIncompatible", err)
	}
	var tables int
	if err := other.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema`).Scan(&tables); err != nil || tables != 1 {
		t.Fatalf("foreign database after Open has %d tables, err = %v", tables, err)
	}
}

func TestOpenRejectsTruncatedDatabasePath(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "index.sqlite3")
	store, err := Open(t.Context(), filename+"\x00ignored")
	if store != nil {
		_ = store.Close()
	}
	if _, statErr := os.Stat(filename); !errors.Is(statErr, os.ErrNotExist) || err == nil {
		t.Fatalf("Open with invalid path returned %v and stat of truncated path returned %v", err, statErr)
	}
}

func openStore(t testing.TB, filename string) *Store {
	t.Helper()
	store, err := Open(t.Context(), filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func pathSet(paths ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		set[path] = struct{}{}
	}
	return set
}
