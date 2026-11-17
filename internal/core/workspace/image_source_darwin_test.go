// spec: FEAT-NOTE-DROP / EV-MYGO-284
package workspace

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestImportImageFromSearchOnlyDirectory(t *testing.T) {
	var screenshot bytes.Buffer
	if err := png.Encode(&screenshot, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	filename := filepath.Join(source, "Screenshot.png")
	if err := os.WriteFile(filename, screenshot.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(source, 0o100); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(source, 0o700) })
	if directory, err := os.OpenRoot(source); err == nil {
		directory.Close()
		t.Skip("the test process can open a directory without read permission")
	}

	vault, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	if _, err := vault.CreateTextDocument("note.md", "original"); err != nil {
		t.Fatal(err)
	}
	const want = "images/Screenshot.png"
	destination, err := vault.ImportImage("note.md", filename)
	if err != nil || destination != want {
		t.Fatalf("destination = %q, error = %v, want %q", destination, err, want)
	}
	data, _, err := vault.ReadPreviewImage(destination, maxImageBytes)
	if err != nil || !bytes.Equal(data, screenshot.Bytes()) {
		t.Fatalf("imported screenshot differs from the original, error = %v", err)
	}
}
