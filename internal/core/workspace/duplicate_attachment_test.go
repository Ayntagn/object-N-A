// spec: EV-UI-182
package workspace_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDuplicateAttachmentPublishesIndependentBinaryCopies(t *testing.T) {
	dir := t.TempDir()
	vault := openFixture(t, dir)
	const source = "Nested/Menu Raster.PNG"
	const content = "\x89PNG\r\n\x1a\n\x00\xff\x80"
	writeFixture(t, dir, source, content)
	absolute := func(relative string) string { return filepath.Join(dir, filepath.FromSlash(relative)) }
	if err := os.Chmod(absolute(source), 0o640); err != nil {
		t.Fatal(err)
	}
	modified := time.Date(2026, 11, 14, 10, 30, 0, 0, time.UTC)
	if err := os.Chtimes(absolute(source), time.Time{}, modified); err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(absolute(source))
	if err != nil {
		t.Fatal(err)
	}
	const first = "Nested/Menu Raster copy.PNG"
	const second = "Nested/Menu Raster copy 2.PNG"
	const occupied = "first copy has its own bytes"
	for _, want := range []string{first, second} {
		got, err := vault.DuplicateAttachment(source)
		if err != nil || got != want {
			t.Fatalf("duplicate path = %q, %v; want %q", got, err, want)
		}
		data, err := os.ReadFile(absolute(got))
		if err != nil || string(data) != content {
			t.Fatalf("duplicate bytes = %x, %v; want %x", data, err, content)
		}
		info, err := os.Stat(absolute(got))
		if err != nil {
			t.Fatal(err)
		}
		if os.SameFile(original, info) || original.Mode().Perm() != info.Mode().Perm() || !original.ModTime().Equal(info.ModTime()) {
			t.Fatalf("duplicate inode independence or metadata mismatch: source %v, copy %v", original, info)
		}
		if got == first {
			writeFixture(t, dir, first, occupied)
		}
	}
	if data, err := os.ReadFile(absolute(first)); err != nil || string(data) != occupied {
		t.Fatalf("existing copy = %q, %v; want %q", data, err, occupied)
	}
	entries, err := os.ReadDir(filepath.Dir(absolute(source)))
	if err != nil || len(entries) != 3 {
		t.Fatalf("expected source and two copies, got %v, %v", entries, err)
	}
}
