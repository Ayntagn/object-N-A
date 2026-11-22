// spec: FEAT-WYS-IMAGE / U77 / EV-MYGO-164 / EV-MYGO-313.
package ui

import (
	"bytes"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
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
	assets := &previewImageFrame{}
	resources := origin.resources()
	resources.fileImages = assets
	body, _ := renderPreviewDocument(parser.New(webPreviewExtensions), source, "Note",
		origin.resources(), origin.note, false, false, &figureRender{})
	liveBody, _ := renderPreviewDocument(parser.New(webPreviewExtensions), source, "Note",
		resources, origin.note, false, false, &figureRender{})
	for _, want := range []string{`width:min(100%,120px);aspect-ratio:120/200`,
		`width:166.66666666666666%`, `rotate(90deg)`, `alt="Photo"`, `title="Caption"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("Preview does not use upright JPEG dimensions or preserve the caption. Missing %q", want)
		}
		if !strings.Contains(liveBody, want) {
			t.Fatalf("Live preview resource lost %q", want)
		}
	}

	file := assets.files["photo.jpg"]
	if file.route == "" || !strings.Contains(liveBody, previewImageOrigin+file.route) || strings.Contains(liveBody, "data:image") {
		t.Fatal("The live preview should refer to the verified image without inlining its bytes")
	}
	unchanged := &previewImageFrame{previous: assets.files}
	same, ok := unchanged.add(vault, "photo.jpg", 4<<20)
	if !ok || same.route != file.route {
		t.Fatal("A text edit must preserve the resource URL of an unchanged attachment")
	}
	page, otherPage := new(mygo.Page), new(mygo.Page)
	registry := previewImageRegistry{pages: make(map[*mygo.Page]map[string]previewLocalImage)}
	registry.publish(page, assets.routes)
	get := func(from *mygo.Page, route string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, previewImageScheme+"://localhost"+route, nil)
		response := httptest.NewRecorder()
		registry.serve(response, request, from)
		return response
	}
	if response := get(page, file.route); response.Code != http.StatusOK ||
		response.Header().Get("Content-Type") != "image/jpeg" || !bytes.Equal(response.Body.Bytes(), data) {
		t.Fatal("The resource should return the same attachment bytes to its preview page")
	}
	if response := get(otherPage, file.route); response.Code != http.StatusNotFound {
		t.Fatal("An unrelated page must not read the attachment URL")
	}

	filename := filepath.Join(root, "photo.jpg")
	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Clone(data)
	changed[30] = 1 // same JPEG and dimensions, with EXIF Orientation 1
	if err := os.WriteFile(filename, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filename, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if response := get(page, file.route); response.Code != http.StatusNotFound {
		t.Fatal("The resource must not serve a different file version at an old URL")
	}
	reloaded := &previewImageFrame{previous: assets.files}
	current, ok := reloaded.add(vault, "photo.jpg", 4<<20)
	if !ok || current.route == file.route || current.width != 120 || current.height != 200 {
		t.Fatalf("A same-size rewrite must get a fresh URL and geometry: %+v", current)
	}
	registry.publish(page, reloaded.routes)
	registry.publish(page, nil)
	if response := get(page, current.route); response.Code != http.StatusNotFound {
		t.Fatal("Closing the preview must revoke its image URLs")
	}
}
