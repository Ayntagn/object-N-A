// spec: FEAT-001 / EV-MYGO-295 / EV-MYGO-296
package ui

import (
	"testing"
	"unicode/utf8"

	mygoui "github.com/egoist/mygo/ui"
)

func TestVisualPasteMarkdownTable(t *testing.T) {
	const table = "| 对比   | context            | trapframe    |\n" +
		"| ---- | ------------------ | ------------ |\n" +
		"| 目的   | 内核线程切换             | 保存进入内核前的执行现场 |\n" +
		"| 保存范围 | 一部分内核寄存器           | 更完整的 CPU 状态  |\n" +
		"| 主要场景 | 调度器切换进程            | 系统调用、中断、异常   |\n" +
		"| 保存位置 | `struct proc` 内嵌结构 | 内核栈上的结构      |\n" +
		"| 恢复目标 | 内核执行现场             | 用户态执行现场      |"
	const original = "开头。\n\n替换这段。\n\n结尾。\n"
	app, tester, _ := snippetsWorkspace(t, "Paste.md", original, wysiwygMode)
	app.preferences.AutoIndentParagraphs = true
	start := utf8.RuneCountInString("开头。\n")
	end := start + utf8.RuneCountInString("替换这段。")
	app.wys.session.Document.SetSelection(start, end)
	tester.Frame()
	tester.SetClipboard(table)
	tester.Key(mygoui.Cmd, mygoui.KeyV)

	if grid, ok := tester.AccessByRole(mygoui.RoleTable, "Markdown table"); !ok || grid.RowCount != 6 || grid.ColumnCount != 3 {
		t.Fatalf("paste did not create a six-row, three-column grid: %+v", grid)
	}
	if got, want := app.draft.String(), "开头。\n\n"+table+"\n\n结尾。\n"; got != want {
		t.Fatalf("pasted draft = %q, want %q", got, want)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != original {
		t.Fatalf("undo left %q, want %q", got, original)
	}
}

func TestVisualPasteAutoIndentsProse(t *testing.T) {
	const prose = "UNIX 管道也是用类似的方式实现的。"
	app, tester, _ := snippetsWorkspace(t, "Paste.md", "", wysiwygMode)
	app.preferences.AutoIndentParagraphs = true
	tester.SetClipboard(prose)
	tester.Key(mygoui.Cmd, mygoui.KeyV)
	if got, want := app.draft.String(), "\u3000\u3000"+prose; got != want {
		t.Fatalf("pasted draft = %q, want %q", got, want)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != "" {
		t.Fatalf("undo left %q", got)
	}
}
