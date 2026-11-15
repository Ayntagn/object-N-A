// spec: EV-UI-147 / EV-UI-148 / EV-UI-149
package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestVisualBlockMenuFiltersAndInsertsTable(t *testing.T) {
	const source = "Before table.\n\nAfter table.\n"
	app, tester, _ := snippetsWorkspace(t, "Blocks.md", source, wysiwygMode)
	document := app.wys.session.Document
	end := len("Before table.")
	document.SetSelection(end, end)
	tester.Frame()
	tester.Key(0, mygoui.KeyEnter)
	tester.TypeKey(0, mygoui.KeySlash, "/")
	option := func(label string) mygoui.AccessInfo {
		t.Helper()
		info, ok := tester.AccessByRole(mygoui.RoleMenuItem, label)
		if !ok || !info.Checked || info.Offscreen {
			t.Fatalf("menu selection %q = %+v", label, info)
		}
		return info
	}
	option("Text")
	tester.Key(0, mygoui.KeyRight)
	option("Bullet List")
	tester.Key(0, mygoui.KeyRight)
	option("Image")
	tester.Key(0, mygoui.KeyLeft)
	option("Task List")
	tester.Type("TAB")
	option("Table")
	if _, ok := tester.AccessByRole(mygoui.RoleMenuItem, "Table of Contents"); !ok {
		t.Fatal("substring filter omitted Table of Contents")
	}
	tester.Type("xyz")
	if _, open := tester.AccessByRole(mygoui.RoleMenu, "Insert block"); open {
		t.Fatal("a query with no matches showed a menu")
	}
	for range 3 {
		tester.Key(0, mygoui.KeyBackspace)
	}
	option("Table")
	tester.Key(0, mygoui.KeyEscape)
	if _, open := tester.AccessByRole(mygoui.RoleMenu, "Insert block"); open ||
		app.draft.String() != "Before table.\n\n/TAB\n\nAfter table.\n" {
		t.Fatalf("Escape left popup=%v, Markdown=%q", open, app.draft.String())
	}
	tester.Type("l")
	option("Table")
	tester.Key(0, mygoui.KeyEnter)
	if header, ok := tester.AccessByRole(mygoui.RoleTextField, "Header, column 1"); !ok || !header.Focused || header.Value != "" {
		t.Fatalf("Table did not focus the first empty header: %+v", header)
	}
	if got := app.draft.String(); !strings.HasPrefix(got, "Before table.\n\n| <br /> | <br /> | <br /> |\n") ||
		!strings.HasSuffix(got, "\n\nAfter table.\n") || strings.Contains(got, "/TABl") {
		t.Fatalf("Table replacement left %q", got)
	}
}

func TestVisualBlockGutterAddsAndMovesBlock(t *testing.T) {
	const source = "# Alpha heading\n\nFirst paragraph with **bold** and 😀.\n\nSecond paragraph.\n\n## Omega heading\n\nFinal paragraph.\n"
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, tester, file := snippetsWorkspace(t, "Blocks.md", source, wysiwygMode)
	document := app.wys.session.Document
	editor, ok := tester.AccessByRole(mygoui.RoleTextField, "WYSIWYG editor")
	if !ok {
		t.Fatal("editor missing")
	}
	button := func(name string) mygoui.AccessInfo {
		t.Helper()
		info, ok := tester.AccessByRole(mygoui.RoleButton, name)
		if !ok || info.Offscreen || info.Disabled {
			t.Fatalf("gutter button %q = %+v", name, info)
		}
		return info
	}
	point := func(b mygoui.Rect) (float32, float32) { return b.X + b.W/2, b.Y + b.H/2 }
	boxes := document.VisibleParagraphs()
	tester.Move(editor.Bounds.X+240, editor.Bounds.Y+boxes[1].Top+boxes[1].Height/2)
	tester.ClickAt(point(button("Add block").Bounds))
	if _, open := tester.AccessByRole(mygoui.RoleMenu, "Insert block"); !open {
		t.Fatal("gutter add did not open block menu")
	}
	tester.Key(0, mygoui.KeyEscape)
	if got := app.draft.String(); got != strings.Replace(source, "\n\nSecond", "\n\n<br />\n\nSecond", 1) {
		t.Fatalf("Escape after adding left %q", got)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if app.draft.String() != source {
		t.Fatalf("undo of gutter insertion left %q", app.draft.String())
	}
	boxes = document.VisibleParagraphs()
	tester.Move(editor.Bounds.X+240, editor.Bounds.Y+boxes[1].Top+boxes[1].Height/2)
	tester.ClickAt(point(button("Add block").Bounds))
	tester.Type("tab")
	tester.Key(0, mygoui.KeyEnter)
	if header, ok := tester.AccessByRole(mygoui.RoleTextField, "Header, column 1"); !ok || !header.Focused {
		t.Fatalf("gutter query did not enter the table header: %+v", header)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != strings.Replace(source, "\n\nSecond", "\n\ntab\n\nSecond", 1) {
		t.Fatalf("undo of the table replacement left %q", got)
	}
	// Restore the fixture before exercising the independent drag path.
	for range 2 {
		tester.Key(mygoui.Cmd, mygoui.KeyZ)
	}
	if app.draft.String() != source {
		t.Fatalf("undo of the typed gutter query and new paragraph left %q", app.draft.String())
	}
	boxes = document.VisibleParagraphs()
	tester.Move(editor.Bounds.X+240, editor.Bounds.Y+boxes[1].Top+boxes[1].Height/2)
	tester.Press(point(button("Move block").Bounds))
	x, y := editor.Bounds.X+240, editor.Bounds.Y+boxes[0].Top+2
	tester.Move(x, y)
	tester.Key(0, mygoui.KeyEscape)
	tester.Release(x, y)
	if app.draft.String() != source {
		t.Fatalf("canceled drag left %q", app.draft.String())
	}
	tester.Move(editor.Bounds.X+240, editor.Bounds.Y+boxes[1].Top+boxes[1].Height/2)
	tester.Press(point(button("Move block").Bounds))
	tester.Move(x, y)
	tester.Release(x, y)
	moved := "First paragraph with **bold** and 😀.\n\n# Alpha heading\n\nSecond paragraph.\n\n## Omega heading\n\nFinal paragraph.\n"
	if app.draft.String() != moved {
		t.Fatalf("drag left %q", app.draft.String())
	}
	a, z := document.Selection()
	if got := document.Text().Slice(a, z).String(); got != "First paragraph with bold and 😀." {
		t.Fatalf("moved selection = %q", got)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if app.draft.String() != source {
		t.Fatalf("undo of drag left %q", app.draft.String())
	}
	tester.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ)
	vault, err := workspace.Open(filepath.Dir(file))
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	app.document, err = vault.ReadDocument("Blocks.md")
	if err != nil {
		t.Fatal(err)
	}
	app.storage, app.folderRoot = vault, filepath.Dir(file)
	app.documentEdited = false // The tester has no platform title bar.
	if !app.save() {
		t.Fatalf("save failed: %s", app.problem)
	}
	saved, err := os.ReadFile(file)
	if err != nil || string(saved) != moved {
		t.Fatalf("saved %q, error %v", saved, err)
	}
	reopened, err := vault.ReadDocument("Blocks.md")
	if err != nil {
		t.Fatal(err)
	}
	app.setTabDocument(app.workspaceTab, reopened)
	tester.Frame()
	if got := app.wys.session.Document.Text().String(); !strings.HasPrefix(got, "First paragraph with bold and 😀.\nAlpha heading\n") {
		t.Fatalf("reopened visual order: %q", got)
	}
}

func TestVisualBlockMenuOpensMermaidSubmenu(t *testing.T) {
	const source = "Before table.\n\n<br />\n\nAfter table.\n"
	app, tester, _ := snippetsWorkspace(t, "Diagrams.md", source, wysiwygMode)
	document := app.wys.session.Document
	start := len("Before table.\n")
	document.SetSelection(start, start)
	tester.Frame()
	tester.Type("/mermaid")
	tester.Key(0, mygoui.KeyEnter)
	if _, open := tester.AccessByRole(mygoui.RoleMenu, "Mermaid Diagram"); !open || app.draft.String() != source {
		t.Fatalf("Mermaid submenu = %v, draft = %q", open, app.draft.String())
	}
	flowchart, ok := tester.AccessByRole(mygoui.RoleMenuItem, "Flowchart")
	if !ok || flowchart.Offscreen || flowchart.Disabled {
		t.Fatalf("Flowchart is unavailable: %+v", flowchart)
	}
	x, y := flowchart.Bounds.X+30, flowchart.Bounds.Y+flowchart.Bounds.H/2
	tester.Press(x, y)
	tester.Release(x, y)
	if got := app.draft.String(); !strings.HasPrefix(got, "Before table.\n\n\x60\x60\x60mermaid\ngraph TD\n") ||
		!strings.HasSuffix(got, "\n\nAfter table.\n") || strings.Count(got, "\x60\x60\x60mermaid") != 1 {
		t.Fatalf("choosing Flowchart left %q", got)
	}
}
