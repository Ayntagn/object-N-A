// spec: FEAT-WYS-IMAGE / EV-EDITOR-004 / EV-UI-147 / EV-UI-156 / EV-UI-157
package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func writeImageFixture(t *testing.T, file string, ink color.RGBA) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 120, 200))
	for y := range 200 {
		for x := range 120 {
			picture.SetRGBA(x, y, ink)
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, picture); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func imageWorkspace(t *testing.T, content string) (*workspaceWindow, *mygoui.Tester, *workspace.Vault, string) {
	t.Helper()
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, tester, file := snippetsWorkspace(t, "Image.md", content, wysiwygMode)
	root := filepath.Dir(file)
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vault.Close() })
	app.document, err = vault.ReadDocument("Image.md")
	if err != nil {
		t.Fatal(err)
	}
	app.storage, app.folderRoot = vault, root
	return app, tester, vault, file
}

func TestVisualImageCaptionResizeAndReopen(t *testing.T) {
	const original = "# Image\n\n![旧 alt](portrait.png \"Old\")\n\nAfter __bold__.\n"
	app, tester, vault, file := imageWorkspace(t, original)
	writeImageFixture(t, filepath.Join(filepath.Dir(file), "portrait.png"), color.RGBA{110, 166, 230, 255})
	tester.Frame()
	doc := app.wys.session.Document
	if doc == nil {
		t.Fatal("image made the note read-only")
	}
	paragraph := strings.Index(doc.Text().String(), "After")
	at := len([]rune(doc.Text().String()[:paragraph])) + len("After")
	doc.SetSelection(at, at)
	tester.Frame()
	tester.Type(" plus")
	const withBody = "# Image\n\n![旧 alt](portrait.png \"Old\")\n\nAfter plus **bold**.\n"
	if !strings.Contains(app.draft.String(), "![旧 alt](portrait.png \"Old\")") {
		t.Fatalf("edit outside image changed its Markdown: %q", app.draft.String())
	}
	if err := tester.Click("Write Image Caption"); err != nil {
		t.Fatal(err)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.Type("New caption")
	tester.Advance(time.Second + time.Millisecond)
	tester.Frame()
	if got := app.draft.String(); got != strings.Replace(withBody, "\"Old\"", "\"New caption\"", 1) {
		t.Fatalf("caption writeback left %q", got)
	}
	picture, ok := tester.AccessByRole(mygoui.RoleImage, "New caption")
	if !ok || picture.Offscreen {
		t.Fatalf("image AX = %+v", picture)
	}
	tester.Move(picture.Bounds.X+picture.Bounds.W/2, picture.Bounds.Y+picture.Bounds.H-1)
	handle, ok := tester.AccessByRole(mygoui.RoleButton, "Resize image")
	if !ok || handle.Offscreen {
		t.Fatalf("resize handle AX = %+v", handle)
	}
	x, y := handle.Bounds.X+handle.Bounds.W/2, handle.Bounds.Y+handle.Bounds.H/2
	tester.Press(x, y)
	tester.Move(x, y+60)
	tester.Release(x, y+60)
	const resized = "# Image\n\n![1.30](portrait.png \"New caption\")\n\nAfter plus **bold**.\n"
	if got := app.draft.String(); got != resized {
		t.Fatalf("resize left %q", got)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if !strings.Contains(app.draft.String(), "![旧 alt](portrait.png \"New caption\")") {
		t.Fatalf("undo resize left %q", app.draft.String())
	}
	tester.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ)
	app.documentEdited = false
	if !app.save() {
		t.Fatal(app.problem)
	}
	saved, err := vault.ReadDocument("Image.md")
	if err != nil || saved.Text != resized {
		t.Fatalf("saved %q, error %v", saved.Text, err)
	}
	app.setTabDocument(app.workspaceTab, saved)
	tester.Frame()
	if picture, ok := tester.AccessByRole(mygoui.RoleImage, "New caption"); !ok || picture.Bounds.H != 260 {
		t.Fatalf("reopened image AX = %+v", picture)
	}
}

func TestVisualImageSlashAndRelativeLink(t *testing.T) {
	app, tester, _, file := imageWorkspace(t, "Before.\n\nAfter.\n")
	writeImageFixture(t, filepath.Join(filepath.Dir(file), "portrait.png"), color.RGBA{130, 180, 80, 255})
	doc := app.wys.session.Document
	doc.SetSelection(len("Before."), len("Before."))
	tester.Frame()
	tester.Key(0, mygoui.KeyEnter)
	tester.Type("/image")
	tester.Key(0, mygoui.KeyEnter)
	const empty = "Before.\n\n![1.00]()\n\nAfter.\n"
	if got := app.draft.String(); got != empty {
		t.Fatalf("slash insertion left %q", got)
	}
	if upload, ok := tester.AccessByRole(mygoui.RoleButton, "Upload file"); !ok || upload.Offscreen {
		t.Fatalf("empty image upload AX = %+v", upload)
	}
	if err := tester.Click("Image link"); err != nil {
		t.Fatal(err)
	}
	tester.Type("portrait.png")
	if app.draft.String() != empty {
		t.Fatalf("unconfirmed link wrote %q", app.draft.String())
	}
	tester.Key(0, mygoui.KeyEnter)
	want := strings.Replace(empty, "()", "(portrait.png)", 1)
	if got := app.draft.String(); got != want {
		t.Fatalf("confirmed link left %q", got)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != empty {
		t.Fatalf("undo link left %q", got)
	}
	tester.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ)
	if got := app.draft.String(); got != want {
		t.Fatalf("redo link left %q", got)
	}
}

func TestVisualImageImportAndTemplateRoutes(t *testing.T) {
	t.Run("source template", func(t *testing.T) {
		app, _, _ := snippetsWorkspace(t, "Image.md", "😀文字", sourceMode)
		app.code.Select(0, len("😀文字"))
		app.insertImage()
		if got, caret := app.draft.String(), app.code.Selection().Head; got != "![😀文字](image-url)" || caret != len("![😀文字](") {
			t.Fatalf("source Image left %q at %d", got, caret)
		}
		app, tester, _ := snippetsWorkspace(t, "Image.md", "", sourceMode)
		tester.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyK)
		if got, caret := app.draft.String(), app.code.Selection().Head; got != "![alt text](image-url)" || caret != 2 {
			t.Fatalf("source Image shortcut left %q at %d", got, caret)
		}
	})
	t.Run("visual template and durable upload", func(t *testing.T) {
		app, tester, vault, file := imageWorkspace(t, "# Image\n\n![1.00]()\n\nBeforeAfter.\n")
		doc := app.wys.session.Document
		value := doc.Text().String()
		split := len([]rune(value[:strings.Index(value, "After")]))
		doc.SetSelection(split, split)
		tester.Frame()
		app.insertImage()
		tester.Frame()
		const withTemplate = "# Image\n\n![1.00]()\n\nBefore\n\n![1.00](image-url)\n\nAfter.\n"
		if got := app.draft.String(); got != withTemplate {
			t.Fatalf("visual template left %q", got)
		}
		name := "图 #1.png"
		external := filepath.Join(t.TempDir(), name)
		data := writeImageFixture(t, external, color.RGBA{200, 110, 80, 255})
		if err := os.Mkdir(filepath.Join(filepath.Dir(file), "images"), 0o700); err != nil {
			t.Fatal(err)
		}
		existing := filepath.Join(filepath.Dir(file), "images", name)
		prior := writeImageFixture(t, existing, color.RGBA{80, 110, 200, 255})
		field, ok := tester.AccessByRole(mygoui.RoleTextField, "Image link")
		if !ok || !tester.DropFiles(field.Bounds.X+field.Bounds.W/2, field.Bounds.Y+field.Bounds.H/2, external) {
			t.Fatal("image did not drop onto the empty card")
		}
		want := strings.Replace(withTemplate, "![1.00]()", "![1.00](images/%E5%9B%BE%20%231-1.png)", 1)
		if got := app.draft.String(); got != want {
			t.Fatalf("import left %q, problem %q", got, app.problem)
		}
		for path, bytesOnDisk := range map[string][]byte{existing: prior, filepath.Join(filepath.Dir(existing), "图 #1-1.png"): data} {
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, bytesOnDisk) {
				t.Fatalf("%s bytes differ, error %v", path, err)
			}
		}
		app.documentEdited = false
		if !app.save() {
			t.Fatal(app.problem)
		}
		saved, err := vault.ReadDocument("Image.md")
		if err != nil || saved.Text != want {
			t.Fatalf("saved %q, error %v", saved.Text, err)
		}
		app.setTabDocument(app.workspaceTab, saved)
		tester.Frame()
		if info, ok := tester.AccessByRole(mygoui.RoleImage, "图 #1-1.png"); !ok || info.Bounds.H != 200 {
			t.Fatalf("reopened imported image AX = %+v", info)
		}
	})
}
