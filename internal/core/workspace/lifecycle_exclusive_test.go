// regression: note creation without atomic rename or hard links.
package workspace

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/iotest"
)

func TestExclusiveLifecycleFileKeepsOccupiedAndRemovesPartial(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.WriteFile("Occupied.md", []byte("existing note"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusiveLifecycleFile(root, "Occupied.md", strings.NewReader("replacement"), 0o600); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("exclusive creation over a note returned %v", err)
	}
	if got, err := root.ReadFile("Occupied.md"); err != nil || string(got) != "existing note" {
		t.Fatalf("occupied note = %q, %v", got, err)
	}
	interrupted := errors.New("fixture write interrupted")
	content := io.MultiReader(strings.NewReader("partial text"), iotest.ErrReader(interrupted))
	if err := writeExclusiveLifecycleFile(root, "Partial.md", content, 0o600); !errors.Is(err, interrupted) {
		t.Fatalf("interrupted creation returned %v", err)
	}
	if _, err := root.Lstat("Partial.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("interrupted creation left a note: %v", err)
	}
}
