// spec: FEAT-WYS-IMAGE / EV-MYGO-290
package ui

import (
	"image/color"
	"path/filepath"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestVisualWikiImageSurvivesEditsAndReload(t *testing.T) {
	const name = "Pasted image 20260102150405.png"
	const token = "![[" + name + "]]"
	const original = token + "\n\nAfter.\n"
	app, tester, vault, file := imageWorkspace(t, original)
	writeImageFixture(t, filepath.Join(filepath.Dir(file), name), color.RGBA{110, 166, 230, 255})
	tester.Frame()
	doc := app.wys.session.Document
	if doc == nil {
		t.Fatal("the wiki image made the visual editor read-only")
	}
	requireImage := func(height float32) {
		t.Helper()
		picture, ok := tester.AccessByRole(mygoui.RoleImage, name)
		if !ok || picture.Bounds.H != height {
			t.Fatalf("expected loaded wiki bitmap of height %g, got %+v", height, picture)
		}
	}
	requireImage(200)
	end := doc.Text().Len()
	doc.SetSelection(end, end)
	tester.Frame()
	tester.Type(" Text")
	const edited = token + "\n\nAfter. Text\n"
	if got := app.draft.String(); got != edited {
		t.Fatalf("editing beside the wiki image saved %q", got)
	}
	picture, _ := tester.AccessByRole(mygoui.RoleImage, name)
	tester.Move(picture.Bounds.X+picture.Bounds.W/2, picture.Bounds.Y+picture.Bounds.H/2)
	if err := tester.Click("Edit image"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("Rotate Right"); err != nil {
		t.Fatal(err)
	}
	const rotated = token + " <!-- object-na-image: scale=1.00 rotate=90 -->\n\nAfter. Text\n"
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != edited {
		t.Fatalf("undo changed the wiki embed: %q", got)
	}
	tester.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ)
	if !app.save() {
		t.Fatal(app.problem)
	}
	saved, err := vault.ReadDocument("Image.md")
	if err != nil || saved.Text != rotated {
		t.Fatalf("saved %q, error %v", saved.Text, err)
	}
	app.setTabDocument(app.workspaceTab, saved)
	tester.Frame()
	requireImage(120)
}
