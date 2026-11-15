package ui

// spec: EV-UI-025 / EV-DRAW-001

import (
	"testing"
	"time"

	"cyberwriter/internal/core/drawing"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestDrawingTextCreateEditAndReopen(t *testing.T) {
	vault, err := workspace.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vault.Close() })
	file, err := vault.CreateDrawing()
	if err != nil {
		t.Fatal(err)
	}
	d := newWorkspaceDrawing(vault, file)
	app := &workspaceWindow{workspaceTab: &workspaceTab{drawing: d}}
	tester := mygoui.NewTester(app.drawingPane, 720, 480)
	click := func(name string) {
		t.Helper()
		if err := tester.Click(name); err != nil {
			t.Fatal(err)
		}
	}
	field := func() mygoui.AccessInfo {
		t.Helper()
		info, ok := tester.AccessByRole(mygoui.RoleTextField, "Drawing text")
		if !ok || !info.Focused || !info.Multiline {
			t.Fatalf("drawing text field: %+v, found %v", info, ok)
		}
		return info
	}

	const x, y = 160, 160
	const initial = "绘图\nDraft"
	tester.Key(0, mygoui.KeyT)
	tester.ClickAt(x, y)
	field()
	tester.Type("discarded")
	click("Cancel")
	if app.modalOpen || d.dirty() || len(d.scene.Elements()) != 0 {
		t.Fatal("cancel created a text element or left its editor open")
	}
	if canvas, ok := tester.AccessByRole(mygoui.RoleGroup, "Drawing canvas"); !ok || !canvas.Focused {
		t.Fatal("cancel did not restore focus to the drawing canvas")
	}
	tester.ClickAt(x, y)
	field()
	if active, disabled := tester.TextInputActive(); !active || disabled {
		t.Fatal("drawing text field did not activate input methods")
	}
	tester.Compose("绘", 1)
	tester.Type("绘图")
	tester.Key(0, mygoui.KeyEnter)
	tester.Type("Draft")
	click("Add text")
	elements := d.scene.Elements()
	if app.modalOpen || len(elements) != 1 {
		t.Fatalf("adding text left modal open or produced %d elements", len(elements))
	}
	added := elements[0]
	if added.Type != drawing.Text || added.Text != initial || added.X != x || added.Y != y || added.Width <= 0 || added.Height <= 0 {
		t.Fatalf("added text element = %+v", added)
	}

	// Selection double-click edits the existing element. Escape leaves the
	// exact scene and history from before the editor opened.
	saved := d.scene
	tester.Advance(time.Second)
	tester.ClickAt(x+3, y+3)
	tester.ClickAt(x+3, y+3)
	if value := field().Value; value != initial {
		t.Fatalf("double-click opened text %q", value)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.Type("discarded")
	tester.Key(0, mygoui.KeyEscape)
	if app.modalOpen || d.scene != saved || len(d.undo) != 1 {
		t.Fatal("cancel changed the drawing or left its editor open")
	}

	tester.Key(0, mygoui.Key8)
	tester.ClickAt(x+3, y+3)
	field()
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.Type("更新")
	tester.Key(mygoui.Cmd, mygoui.KeyEnter)
	click("Undo")
	if text := d.scene.Elements()[0].Text; text != initial {
		t.Fatalf("undo restored %q", text)
	}
	click("Redo")
	if err := d.save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := loadWorkspaceDrawing(vault, file.Path)
	if err != nil {
		t.Fatal(err)
	}
	edited := reopened.scene.Elements()[0]
	if edited.ID != added.ID || edited.X != x || edited.Y != y || edited.Text != "更新" ||
		edited.OriginalText != "更新" || edited.Version != 2 || !edited.AutoResize || edited.FontFamily != drawing.HelveticaFontFamily {
		t.Fatalf("reopened edited element = %+v", edited)
	}
}
