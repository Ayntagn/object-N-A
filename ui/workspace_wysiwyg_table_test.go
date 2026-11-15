// spec: EV-EDITOR-004 / EV-UI-142 / EV-UI-145 / U02
package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestVisualTableChangesStructureThroughHandles(t *testing.T) {
	const source = "# Table\n\n| 名称 | 数量 |\n| --- | ---: |\n| 甲 | 1 |\n| 乙 | 2 |\n\nAfter table.\n"
	app, tester, _ := snippetsWorkspace(t, "Table.md", source, wysiwygMode)
	control := func(role mygoui.Role, name string) mygoui.AccessInfo {
		t.Helper()
		info, ok := tester.AccessByRole(role, name)
		if !ok || info.Offscreen || info.Disabled {
			t.Fatalf("%q is unavailable: %+v", name, info)
		}
		return info
	}
	point := func(info mygoui.AccessInfo) (float32, float32) {
		return info.Bounds.X + info.Bounds.W/2, info.Bounds.Y + info.Bounds.H/2
	}
	first := control(mygoui.RoleTextField, "Row 1, column 1")
	tester.Move(point(first))
	tester.ClickAt(point(control(mygoui.RoleButton, "Column 1 options")))
	control(mygoui.RoleMenu, "Column 1 options")
	tester.ClickAt(point(control(mygoui.RoleMenuItem, "Align Center")))
	if table := app.wys.session.EditTable(0); table == nil || table.Align(0) != parser.AlignCenter {
		t.Fatalf("center alignment left %q", app.draft.String())
	}
	table := control(mygoui.RoleTable, "Markdown table")
	first = control(mygoui.RoleTextField, "Row 1, column 1")
	tester.Move(first.Bounds.X+first.Bounds.W/2, first.Bounds.Y+2)
	tester.ClickAt(point(control(mygoui.RoleButton, "Add row before row 1")))
	if grid := control(mygoui.RoleTable, "Markdown table"); grid.RowCount != table.RowCount+1 {
		t.Fatalf("adding a row left %d rows", grid.RowCount)
	}
	if selected := control(mygoui.RoleTextField, "Row 1, column 1"); !selected.Focused || selected.Value != "" {
		t.Fatalf("inserted row did not receive focus: %+v", selected)
	}
	tester.Key(0, mygoui.KeyTab)
	if next := control(mygoui.RoleTextField, "Row 2, column 1"); !next.Focused || next.Value != "甲" {
		t.Fatalf("Tab from the selected row focused %+v", next)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if grid := control(mygoui.RoleTable, "Markdown table"); grid.RowCount != table.RowCount {
		t.Fatalf("undo left %d rows", grid.RowCount)
	}
	first = control(mygoui.RoleTextField, "Row 1, column 1")
	second := control(mygoui.RoleTextField, "Row 1, column 2")
	tester.Move(point(first))
	handle := control(mygoui.RoleButton, "Column 1 options")
	tester.Press(point(handle))
	tester.Move(point(second))
	tester.Release(point(second))
	if moved := control(mygoui.RoleTextField, "Header, column 2"); !moved.Focused || moved.Value != "名称" {
		t.Fatalf("drag did not move and focus the first column: %+v", moved)
	}
	if got := app.draft.String(); !strings.Contains(got, "| 数量 | 名称 |") || !strings.HasSuffix(got, "\n\nAfter table.\n") {
		t.Fatalf("drag wrote %q", got)
	}
	if moved := app.wys.session.EditTable(0); moved.Align(0) != parser.AlignRight || moved.Align(1) != parser.AlignCenter {
		t.Fatalf("column alignment did not follow its content: %q", app.draft.String())
	}
	row := control(mygoui.RoleTextField, "Row 1, column 2")
	tester.Move(point(row))
	tester.ClickAt(point(control(mygoui.RoleButton, "Row 1 options")))
	tester.ClickAt(point(control(mygoui.RoleMenuItem, "Delete Row")))
	if remaining := control(mygoui.RoleTextField, "Row 1, column 2"); remaining.Value != "乙" {
		t.Fatalf("deleting a row left %+v", remaining)
	}
}

func TestVisualTableEditsInPlaceAndSavesAfterTab(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const original = "# Table\n\n| 名称 | 数量 |\n| --- | ---: |\n| 甲 | 1 |\n| 乙 | 2 |\n"
	const edited = "# Table\n\n| 名称 | 数量 |\n| --- | ---: |\n| 中文甲 | 1 |\n| 乙 | 2 |\n"
	app, tester, file := snippetsWorkspace(t, "Table.md", original, wysiwygMode)
	cell := func(name string) mygoui.AccessInfo {
		t.Helper()
		field, ok := tester.AccessByRole(mygoui.RoleTextField, name)
		if !ok || field.ReadOnly || field.Offscreen {
			t.Fatalf("editable cell %q is missing: %+v", name, field)
		}
		return field
	}
	first := cell("Row 1, column 1")
	if app.modalOpen || app.draft.Modified() {
		t.Fatal("opening WYSIWYG changed the draft or opened a modal")
	}
	tester.ClickAt(first.Bounds.X+16, first.Bounds.Y+first.Bounds.H/2)
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.Type("中文甲")
	tester.Key(0, mygoui.KeyTab)
	if next := cell("Row 1, column 2"); !next.Focused || next.Value != "1" {
		t.Fatalf("Tab did not focus the adjacent number: %+v", next)
	}
	tester.Command("copy")
	if got := tester.Clipboard(); got != "1" {
		t.Fatalf("Tab selected %q, want the full adjacent cell", got)
	}
	tester.Key(mygoui.Shift, mygoui.KeyTab)
	if previous := cell("Row 1, column 1"); !previous.Focused {
		t.Fatalf("Shift+Tab did not focus the previous cell: %+v", previous)
	}
	tester.Key(0, mygoui.KeyTab)
	if got := app.draft.String(); got != edited {
		t.Fatalf("Tab left draft %q, want %q", got, edited)
	}
	tester.Command("undo")
	if got := app.draft.String(); got != original {
		t.Fatalf("undo from adjacent cell left %q", got)
	}
	tester.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ)
	if got := app.draft.String(); got != edited {
		t.Fatalf("redo from adjacent cell left %q", got)
	}
	vault, err := workspace.Open(filepath.Dir(file))
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	document, err := vault.ReadDocument("Table.md")
	if err != nil {
		t.Fatal(err)
	}
	app.document = document
	app.storage, app.folderRoot = vault, filepath.Dir(file)
	app.documentEdited = false // The test has no platform title bar.
	if !app.save() {
		t.Fatalf("save failed: %s", app.problem)
	}
	reopened, err := vault.ReadDocument("Table.md")
	if err != nil || reopened.Text != edited {
		t.Fatalf("reopened %q, error %v", reopened.Text, err)
	}
	app.setTabDocument(app.workspaceTab, reopened)
	tester.Frame()
	if got := cell("Row 1, column 1").Value; got != "中文甲" {
		t.Fatalf("reopened cell = %q", got)
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != edited {
		t.Fatalf("saved file = %q, error %v", data, err)
	}
}

func TestVisualTableInsertUsesCurrentParagraphAndFirstCell(t *testing.T) {
	const source = "# Before\n\nFirst paragraph.\n\n## After\n\nFinal paragraph.\n"
	app, tester, _ := snippetsWorkspace(t, "Insert.md", source, wysiwygMode)
	offset := len("Before\n")
	app.wys.session.Document.SetSelection(offset, offset)
	tester.Frame()
	app.wys.openNewTable(app)
	tester.Frame()
	const table = "| <br /> | <br /> | <br /> |\n| :----- | :----- | :----- |\n" +
		"| <br /> | <br /> | <br /> |\n| <br /> | <br /> | <br /> |"
	const want = "# Before\n\n" + table + "\n\nFirst paragraph.\n\n## After\n\nFinal paragraph.\n"
	if app.modalOpen || app.draft.String() != want {
		t.Fatalf("insert at the active paragraph wrote %q, modal %v", app.draft.String(), app.modalOpen)
	}
	if field, ok := tester.AccessByRole(mygoui.RoleTextField, "Header, column 1"); !ok || !field.Focused || field.Value != "" {
		t.Fatalf("first empty header did not receive focus: %+v", field)
	}
	cell, ok := tester.AccessByRole(mygoui.RoleTextField, "Row 1, column 2")
	if !ok {
		t.Fatal("ordinary body cell is missing")
	}
	tester.ClickAt(cell.Bounds.X+16, cell.Bounds.Y+cell.Bounds.H/2)
	tester.Key(0, mygoui.KeyEnter)
	const suffix = "\n\nFirst paragraph.\n\n## After\n\nFinal paragraph.\n"
	if got := app.draft.String(); got != "# Before\n\n"+table+"\n\n<br />"+suffix {
		t.Fatalf("Enter in a body cell wrote %q", got)
	}
	tester.Type("After table.")
	if got := app.draft.String(); got != "# Before\n\n"+table+"\n\nAfter table."+suffix {
		t.Fatalf("typing after exiting the table wrote %q", got)
	}
}
