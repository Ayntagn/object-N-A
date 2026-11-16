// spec: FEAT-001 / EV-UI-142. WYSIWYG highlight shares the existing table edit path.
package ui

import (
	"os"
	"path/filepath"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestVisualHighlightInTableCellPreservesEscapedPipe(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const source = "# 章\n\n| 名称 | 数量 |\n| --- | ---: |\n| **甲\\|乙** | 1 |\n\n其它正文。\n"
	const marked = "# 章\n\n| 名称 | 数量 |\n| --- | ---: |\n| **==甲\\|乙==** | 1 |\n\n其它正文。\n"
	app, _, file := snippetsWorkspace(t, "Highlight.md", source, wysiwygMode)
	tester := mygoui.NewTester(func(c *mygoui.Context) { app.wys.draw(c, app) }, 1294, 831)
	cell, ok := tester.AccessByRole(mygoui.RoleTextField, "Row 1, column 1")
	if !ok || cell.Value != "甲|乙" {
		t.Fatalf("表格单元格 = %+v", cell)
	}
	tester.ClickAt(cell.Bounds.X+16, cell.Bounds.Y+cell.Bounds.H/2)
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	if err := tester.Click("Highlight"); err != nil {
		t.Fatal(err)
	}
	if got := app.draft.String(); got != marked {
		t.Fatalf("高亮后的表格 Markdown = %q, problem %q", got, app.problem)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != source {
		t.Fatalf("撤销高亮后的表格 Markdown = %q", got)
	}
	tester.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ)
	if got := app.draft.String(); got != marked {
		t.Fatalf("重做高亮后的表格 Markdown = %q", got)
	}
	vault, err := workspace.Open(filepath.Dir(file))
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	app.document, err = vault.ReadDocument("Highlight.md")
	if err != nil {
		t.Fatal(err)
	}
	app.storage, app.folderRoot = vault, filepath.Dir(file)
	if !app.save() {
		t.Fatalf("保存表格失败：%s", app.problem)
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != marked {
		t.Fatalf("保存的文件 = %q, error %v", data, err)
	}
	document, err := vault.ReadDocument("Highlight.md")
	if err != nil {
		t.Fatal(err)
	}
	app.setTabDocument(app.workspaceTab, document)
	tester.Frame()
	cell, ok = tester.AccessByRole(mygoui.RoleTextField, "Row 1, column 1")
	if !ok || cell.Value != "甲|乙" || cell.ReadOnly {
		t.Fatalf("重新投影后的单元格 = %+v", cell)
	}
	tester.ClickAt(cell.Bounds.X+16, cell.Bounds.Y+cell.Bounds.H/2)
	if selected := app.wys.focusedCell; selected == nil || !selected.document.Text().AttributesAt(0).Bold() ||
		selected.document.Text().AttributesAt(0).Background != mygoui.RGBA(255, 214, 92, .32) {
		t.Fatal("重开的单元格没有保留背景与粗体")
	}
}
