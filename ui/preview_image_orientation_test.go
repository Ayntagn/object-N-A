// spec: FEAT-WYS-IMAGE / U77 / EV-MYGO-164.
package ui

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestPreviewImageTransformUsesUprightJPEGDimensions(t *testing.T) {
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 120, 200)), nil); err != nil {
		t.Fatal(err)
	}
	// Orientation 6 displays this 120×200 JPEG upright at 200×120.
	exif := []byte{
		0xff, 0xe1, 0, 34, 'E', 'x', 'i', 'f', 0, 0,
		'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0,
		0x12, 0x01, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0,
		0, 0, 0, 0,
	}
	data := append(append([]byte{0xff, 0xd8}, exif...), encoded.Bytes()[2:]...)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "photo.jpg"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	origin := previewResourceOrigin{vault: vault, note: "Note.md"}
	const source = `![Photo](photo.jpg "Caption") <!-- object-na-image: scale=1.00 rotate=90 -->`
	body, _ := renderPreviewDocument(parser.New(webPreviewExtensions), source, "Note",
		origin.resources(), origin.note, false, false, &figureRender{})
	for _, want := range []string{`width:min(100%,120px);aspect-ratio:120/200`,
		`width:166.66666666666666%`, `rotate(90deg)`, `alt="Photo"`, `title="Caption"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("Preview does not use upright JPEG dimensions or preserve the caption. Missing %q", want)
		}
	}
}
