// spec: FEAT-WYS-IMAGE / EV-UI-156. The image edit controls extend the
// verified caption and local link persistence without changing either token.
package ui

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestVisualImageTransformKeepsOriginalAndReopens(t *testing.T) {
	const token = "![旧 alt](<images/图%20原件.png> '图注')"
	const original = "# Image\n\n" + token + "\n\nAfter __bold__.\n"
	app, tester, vault, file := imageWorkspace(t, original)
	folder := filepath.Join(filepath.Dir(file), "images")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	asset := filepath.Join(folder, "图 原件.png")
	data := writeImageFixture(t, asset, color.RGBA{90, 130, 200, 255})
	tester.Frame()
	click := func(label string) {
		t.Helper()
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	imageSize := func(w, h float32) {
		t.Helper()
		if info, ok := tester.AccessByRole(mygoui.RoleImage, "图注"); !ok || info.Bounds.W != w || info.Bounds.H != h {
			t.Fatalf("image bounds = %+v, want %.0f × %.0f", info.Bounds, w, h)
		}
	}
	assertSource := func(want string) {
		t.Helper()
		if got := app.draft.String(); got != want {
			t.Fatalf("Markdown = %q, want %q", got, want)
		}
	}
	edit := func(action string) {
		t.Helper()
		click("Edit image")
		click(action)
	}

	imageSize(120, 200)
	click("图注")
	edit("Rotate Right")
	imageSize(200, 120)
	const clockwise = "# Image\n\n" + token + " <!-- object-na-image: scale=1.00 rotate=90 -->\n\nAfter __bold__.\n"
	assertSource(clockwise)
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	assertSource(original)
	imageSize(120, 200)
	tester.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ)
	assertSource(clockwise)
	imageSize(200, 120)

	edit("Rotate Left")
	assertSource(original)
	edit("Rotate Left")
	const counterclockwise = "# Image\n\n" + token + " <!-- object-na-image: scale=1.00 rotate=270 -->\n\nAfter __bold__.\n"
	assertSource(counterclockwise)
	click("Edit image")
	field, ok := tester.AccessByRole(mygoui.RoleTextField, "Scale (%)")
	if !ok || field.Offscreen {
		t.Fatalf("scale field = %+v", field)
	}
	tester.ClickAt(field.Bounds.X+field.Bounds.W/2, field.Bounds.Y+field.Bounds.H/2)
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.Type("150")
	tester.Key(0, mygoui.KeyEnter)
	const scaled = "# Image\n\n" + token + " <!-- object-na-image: scale=1.50 rotate=270 -->\n\nAfter __bold__.\n"
	assertSource(scaled)
	imageSize(300, 180)
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	assertSource(counterclockwise)
	tester.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ)
	assertSource(scaled)

	app.documentEdited = false
	if !app.save() {
		t.Fatal(app.problem)
	}
	saved, err := vault.ReadDocument("Image.md")
	if err != nil || saved.Text != scaled {
		t.Fatalf("saved Markdown = %q, error %v", saved.Text, err)
	}
	app.setTabDocument(app.workspaceTab, saved)
	tester.Frame()
	imageSize(300, 180)
	tester.SetSize(320, 831)
	tester.Frame()
	narrow, ok := tester.AccessByRole(mygoui.RoleImage, "图注")
	if !ok || narrow.Bounds.W >= 300 || narrow.Bounds.W < 1 ||
		narrow.Bounds.H < narrow.Bounds.W*.6-1 || narrow.Bounds.H > narrow.Bounds.W*.6+1 {
		t.Fatalf("rotated image did not fit the narrow editor: %+v", narrow)
	}
	tester.SetSize(1294, 831)
	tester.Frame()

	preview, _ := renderPreviewDocument(parser.New(webPreviewExtensions), saved.Text, "Image",
		app.previewOrigin().resources(), app.document.Path, false, true, &figureRender{})
	for _, want := range []string{"data:image/png;base64,", `alt="旧 alt"`, `title="图注"`,
		"width:min(100%,300px)", "aspect-ratio:200/120", "rotate(270deg)"} {
		if !strings.Contains(preview, want) {
			t.Errorf("Preview lacks %q: %s", want, preview)
		}
	}
	if strings.Contains(preview, "object-na-image:") {
		t.Fatal("Preview exposes image metadata as text")
	}

	click("图注")
	edit("Reset Image")
	assertSource(original)
	imageSize(120, 200)
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	assertSource(scaled)
	if got, err := os.ReadFile(asset); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("image bytes changed, error %v", err)
	}
}
