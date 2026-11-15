// spec: EV-UI-048 / EV-UI-150 / EV-UI-151
package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

const testVegaSource = `{
  "data": {"values": [{"name": "甲", "value": 2}, {"name": "乙", "value": 3}]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "name", "type": "nominal"},
    "y": {"field": "value", "type": "quantitative"}
  }
}`

func TestVegaCardEditsCancelCommitAndSave(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const fence = "\x60\x60\x60vega-lite\n" + testVegaSource + "\n\x60\x60\x60"
	const initial = "# Vega\n\n" + fence + "\n\n图后段。包含 __粗体__。\n"
	app, tester, file := snippetsWorkspace(t, "Vega.md", initial, wysiwygMode)
	if app.wys.session.Document == nil {
		t.Fatal("a top-level Vega-Lite fence must stay in the editable visual document")
	}
	click := func(info mygoui.AccessInfo) {
		t.Helper()
		tester.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	}
	stage, ok := tester.AccessByRole(mygoui.RoleStatus, "Vega preview unavailable")
	if !ok {
		t.Fatal("headless chart stage is missing")
	}
	click(stage)
	a, z := app.wys.session.Document.Selection()
	if a != len("Vega\n") || z != a+1 || app.wys.diagramEdit.diagram != nil {
		t.Fatalf("stage click left selection %d…%d, editor open=%t", a, z, app.wys.diagramEdit.diagram != nil)
	}
	open := func(want string) {
		t.Helper()
		card, ok := tester.AccessByRole(mygoui.RoleGroup, "Vega chart")
		if !ok || card.Offscreen {
			t.Fatalf("Vega card is missing: %+v", card)
		}
		tester.Move(card.Bounds.X+card.Bounds.W/2, card.Bounds.Y+card.Bounds.H/2)
		button, ok := tester.AccessByRole(mygoui.RoleButton, "Edit source")
		if !ok || button.Disabled || button.Offscreen {
			t.Fatalf("Edit source is unavailable: %+v", button)
		}
		click(button)
		if input, ok := tester.AccessByRole(mygoui.RoleTextField, "Vega source"); !ok || !input.Focused || input.Value != want {
			t.Fatalf("source field = %+v, want focused with original value", input)
		}
	}
	replace := func(value string) {
		tester.Key(mygoui.Cmd, mygoui.KeyA)
		tester.Type(value)
	}
	open(testVegaSource)
	before, _ := tester.AccessByRole(mygoui.RoleTextField, "Vega source")
	grip, ok := tester.AccessByRole(mygoui.RoleSplitter, "Resize Vega source")
	if !ok {
		t.Fatal("Vega source resize grip is missing")
	}
	x, y := grip.Bounds.X+grip.Bounds.W/2, grip.Bounds.Y+grip.Bounds.H/2
	tester.Press(x, y)
	tester.Move(x, y+36)
	tester.Release(x, y+36)
	if resized, _ := tester.AccessByRole(mygoui.RoleTextField, "Vega source"); !resized.Focused || resized.Bounds.H < before.Bounds.H+35 || app.draft.String() != initial {
		t.Fatalf("resizing changed focus or Markdown, field height %v -> %v", before.Bounds.H, resized.Bounds.H)
	}
	replace("cancel this change")
	tester.Key(0, mygoui.KeyEscape)
	if app.draft.String() != initial {
		t.Fatalf("Esc left %q", app.draft.String())
	}
	open(testVegaSource)
	replace("not valid JSON")
	tester.Key(mygoui.Cmd, mygoui.KeyEnter)
	invalid := strings.Replace(initial, testVegaSource, "not valid JSON", 1)
	if app.draft.String() != invalid {
		t.Fatalf("invalid JSON was lost: %q", app.draft.String())
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if app.draft.String() != initial {
		t.Fatalf("undo left %q", app.draft.String())
	}
	tester.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ)
	if app.draft.String() != invalid {
		t.Fatalf("redo left %q", app.draft.String())
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	open(testVegaSource)
	updated := strings.Replace(testVegaSource, `"value": 3`, `"value": 4`, 1) + "  \t\n"
	replace(updated)
	// Press a sibling paragraph while the source field has focus.
	body, ok := tester.AccessByRole(mygoui.RoleTextField, "WYSIWYG editor")
	if !ok {
		t.Fatal("body editor is missing")
	}
	boxes := app.wys.session.Document.VisibleParagraphs()
	last := boxes[len(boxes)-1]
	tester.ClickAt(body.Bounds.X+184, body.Bounds.Y+last.Top+last.Height/2)
	want := strings.Replace(initial, testVegaSource, updated, 1)
	if app.draft.String() != want {
		t.Fatalf("blur changed fence or lost whitespace: %q", app.draft.String())
	}
	vault, err := workspace.Open(filepath.Dir(file))
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	app.document, err = vault.ReadDocument("Vega.md")
	if err != nil {
		t.Fatal(err)
	}
	app.storage, app.folderRoot = vault, filepath.Dir(file)
	app.documentEdited = false // The tester has no platform title bar.
	if !app.save() {
		t.Fatalf("save failed: %s", app.problem)
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != want {
		t.Fatalf("saved %q, error %v", data, err)
	}
	reopened, err := vault.ReadDocument("Vega.md")
	if err != nil {
		t.Fatal(err)
	}
	app.setTabDocument(app.workspaceTab, reopened)
	tester.Frame()
	open(updated)
}

func TestVisualBlockMenuInsertsVegaChart(t *testing.T) {
	const initial = "Before.\n\n<br />\n\nAfter.\n"
	app, tester, _ := snippetsWorkspace(t, "Charts.md", initial, wysiwygMode)
	start := len("Before.\n")
	app.wys.session.Document.SetSelection(start, start)
	tester.Frame()
	tester.Type("/vega")
	tester.Key(0, mygoui.KeyEnter)
	item, ok := tester.AccessByRole(mygoui.RoleMenuItem, "Bar Chart")
	if !ok || item.Offscreen || item.Disabled {
		t.Fatalf("Bar Chart menu item is unavailable: %+v", item)
	}
	tester.ClickAt(item.Bounds.X+30, item.Bounds.Y+item.Bounds.H/2)
	if app.draft.String() != "Before.\n\n"+chartCommands[0].markdown()+"\n\nAfter.\n" {
		t.Fatalf("chart submenu wrote %q", app.draft.String())
	}
	if _, ok := tester.AccessByRole(mygoui.RoleGroup, "Vega chart"); !ok {
		t.Fatal("inserted chart is not an editable visual card")
	}

	empty, _, _ := snippetsWorkspace(t, "Empty.md", "", wysiwygMode)
	empty.insertChart(chartCommands[0])
	if got := empty.draft.String(); got != chartCommands[0].markdown()+"\n\n" {
		t.Fatalf("empty note left %q", got)
	}
	if a, z := empty.wys.session.Document.Selection(); a != 2 || z != 2 {
		t.Fatalf("empty note selection = %d…%d, want caret below chart", a, z)
	}
}
