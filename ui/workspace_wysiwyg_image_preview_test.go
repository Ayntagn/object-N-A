// spec: FEAT-WYS-IMAGE / EV-MYGO-164 / EV-MYGO-315 / EV-MYGO-317
package ui

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestVisualImagePreviewUsesUprightJPEGSize(t *testing.T) {
	app, _, _, note := imageWorkspace(t, "![Photo](photo.jpg)\n")
	const width, height = 257, 129
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewGray(image.Rect(0, 0, width, height)), nil); err != nil {
		t.Fatal(err)
	}
	exif := []byte{
		0xff, 0xd8, 0xff, 0xe1, 0, 34, 'E', 'x', 'i', 'f', 0, 0,
		'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0,
		0x12, 0x01, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0,
	}
	data := append(exif, encoded.Bytes()[2:]...)
	if err := os.WriteFile(filepath.Join(filepath.Dir(note), "photo.jpg"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	requested := false
	var cache visualImageCache
	bitmap := cache.load(app, "photo.jpg", false, func(w, h int) float32 {
		requested = true
		if w != height || h != width {
			t.Fatalf("shrink received %dx%d, want %dx%d", w, h, height, width)
		}
		return 4
	})
	if bitmap == nil || !requested {
		t.Fatal("WYSIWYG did not request the reduced JPEG")
	}
	if w, h := bitmap.Size(); w != height || h != width {
		t.Fatalf("natural size = %dx%d, want %dx%d", w, h, height, width)
	}
}

func TestVisualPNGPreviewUsesRounded16BitTarget(t *testing.T) {
	var encoded bytes.Buffer
	img := image.NewGray16(image.Rect(0, 0, 161, 129))
	img.Pix[1] = 1
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	preview, err := decodeVisualPNGPreview(encoded.Bytes(), 4)
	if err != nil {
		t.Fatal(err)
	}
	scaled, ok := preview.(*image.RGBA64)
	if !ok || scaled.Bounds() != image.Rect(0, 0, 41, 33) {
		t.Fatalf("expected a 41x33 16-bit preview, got %T", preview)
	}
}
