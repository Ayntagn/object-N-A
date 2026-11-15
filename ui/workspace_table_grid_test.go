// spec: EV-UI-144
package ui

import (
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestTableGridInSourceReplacesSelection(t *testing.T) {
	const prefix = "Before 😀.\n\n"
	const selected = "replace"
	const suffix = "\n\nAfter.\n"
	const source = prefix + selected + suffix
	app, tester, _ := snippetsWorkspace(t, "Table.md", source, sourceMode)
	app.code.Select(len(prefix), len(prefix)+len(selected))
	tester.Frame()
	click := func(label string) {
		t.Helper()
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	click("Table")
	anchor := tableGridAccess(t, tester, mygoui.RoleButton, "Table")
	popup := tableGridAccess(t, tester, mygoui.RolePopup, "Insert Table")
	grid := tableGridAccess(t, tester, mygoui.RoleGrid, "Table dimensions")
	if !anchor.Expanded || popup.Bounds.W != 189 || popup.Bounds.H != 167 ||
		popup.Bounds.Y < anchor.Bounds.Y+anchor.Bounds.H || grid.RowCount != 6 || grid.ColumnCount != 8 {
		t.Fatalf("dimensions picker bounds or metadata = %+v, %+v", popup.Bounds, grid)
	}
	tableGridAccess(t, tester, mygoui.RoleStatus, "Sweep to size, click to insert")
	cell := tableGridAccess(t, tester, mygoui.RoleCell, "2 × 3")
	tester.Move(cell.Bounds.X+cell.Bounds.W/2, cell.Bounds.Y+cell.Bounds.H/2)
	tester.Move(popup.Bounds.X+6, popup.Bounds.Y+6)
	tableGridAccess(t, tester, mygoui.RoleStatus, "2 × 3  (first row = header)")
	click("2 × 3")
	const want = prefix + "| Column 1 | Column 2 | Column 3 |\n| --- | --- | --- |\n|   |   |   |\n" + suffix
	if got := app.draft.String(); got != want {
		t.Fatalf("grid wrote %q, want %q", got, want)
	}
	if editor := tableGridAccess(t, tester, mygoui.RoleTextField, "Source editor"); !editor.Focused {
		t.Fatal("grid did not return focus to the source editor")
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != source {
		t.Fatalf("undo after grid wrote %q", got)
	}

	app.code.Select(len(prefix), len(prefix)+len(selected))
	tester.Frame()
	click("Add")
	if err := tester.ChooseMenuItem("Table"); err != nil {
		t.Fatal(err)
	}
	const fixed = "| Header 1 | Header 2 | Header 3 |\n|----------|----------|----------|\n" +
		"| Cell 1   | Cell 2   | Cell 3   |\n| Cell 4   | Cell 5   | Cell 6   |"
	if got := app.draft.String(); got != prefix+fixed+suffix || app.code.Cursor() != len(prefix)+2 {
		t.Fatalf("Add > Table wrote %q, caret = %d", got, app.code.Cursor())
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != source {
		t.Fatalf("undo after fixed table wrote %q", got)
	}
}

func TestTableGridInWYSIWYGUsesKeyboardSelection(t *testing.T) {
	const prefix = "# Before\n\n"
	const selected = "replace"
	const suffix = "\n\n## After\n"
	const source = prefix + selected + suffix
	app, tester, _ := snippetsWorkspace(t, "Visual.md", source, wysiwygMode)
	offset := len("Before\n")
	app.wys.session.Document.SetSelection(offset, offset+len(selected))
	tester.Frame()
	open := func() {
		t.Helper()
		if err := tester.Click("Table"); err != nil {
			t.Fatal(err)
		}
		tableGridAccess(t, tester, mygoui.RoleGrid, "Table dimensions")
	}
	open()
	tester.Key(0, mygoui.KeyEscape)
	if anchor := tableGridAccess(t, tester, mygoui.RoleButton, "Table"); anchor.Expanded || app.draft.String() != source {
		t.Fatal("Escape did not dismiss the picker without editing")
	}
	if editor := tableGridAccess(t, tester, mygoui.RoleTextField, "WYSIWYG editor"); !editor.Focused {
		t.Fatal("Escape did not return to the visual editor")
	}
	open()
	tester.Key(0, mygoui.KeyRight)
	tester.Key(0, mygoui.KeyRight)
	tester.Key(0, mygoui.KeyDown)
	if cell := tableGridAccess(t, tester, mygoui.RoleCell, "2 × 3"); !cell.Focused || !cell.Selected {
		t.Fatalf("keyboard selection not reflected in accessibility: %+v", cell)
	}
	tester.Key(0, mygoui.KeyEnter)
	const want = prefix + "| Column 1 | Column 2 | Column 3 |\n| --- | --- | --- |\n|   |   |   |" + suffix
	if got := app.draft.String(); got != want {
		t.Fatalf("grid wrote %q, want %q", got, want)
	}
	if header := tableGridAccess(t, tester, mygoui.RoleTextField, "Header, column 1"); !header.Focused || header.Value != "Column 1" {
		t.Fatalf("first header did not receive focus: %+v", header)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != source {
		t.Fatalf("undo after grid wrote %q", got)
	}

	app.wys.session.Document.SetSelection(offset, offset+len(selected))
	tester.Frame()
	if err := tester.Click("Add"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Table"); err != nil {
		t.Fatal(err)
	}
	const fixed = "| Header 1 | Header 2 | Header 3 |\n| -------- | -------- | -------- |\n" +
		"| Cell 1   | Cell 2   | Cell 3   |\n| Cell 4   | Cell 5   | Cell 6   |"
	if got := app.draft.String(); got != prefix+fixed+suffix {
		t.Fatalf("Add > Table wrote %q", got)
	}
	last := tableGridAccess(t, tester, mygoui.RoleTextField, "Row 2, column 3")
	if !last.Focused || last.Value != "Cell 6" {
		t.Fatalf("fixed table did not focus Cell 6: %+v", last)
	}
	tester.Type("!")
	if got := tableGridAccess(t, tester, mygoui.RoleTextField, "Row 2, column 3").Value; got != "Cell 6!" {
		t.Fatalf("typing at fixed table caret wrote %q", got)
	}
}

func tableGridAccess(t *testing.T, tester *mygoui.Tester, role mygoui.Role, label string) mygoui.AccessInfo {
	t.Helper()
	info, ok := tester.AccessByRole(role, label)
	if !ok || info.Offscreen {
		t.Fatalf("missing visible %q: %+v", label, info)
	}
	return info
}
