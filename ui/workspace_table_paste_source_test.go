// spec: FEAT-001 / EV-MYGO-295
package ui

import (
	"strings"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestSourcePasteMarkdownTableCreatesSeparateBlock(t *testing.T) {
	const original = "前面的正文。"
	const table = "| 对比 | context | trapframe |\n" +
		"| ---- | ------- | --------- |\n" +
		"| 目的 | 内核线程切换 | 保存进入内核前的执行现场 |\n" +
		"| 保存位置 | " + "\x60struct proc\x60" + " 内嵌结构 | 内核栈上的结构 |"
	app, view, _ := snippetsWorkspace(t, "Table.md", original, splitMode)
	app.preferences.AutoIndentParagraphs = true
	app.code.Select(len(original), len(original))
	view.SetClipboard(table)
	view.Key(mygoui.Cmd, mygoui.KeyV)

	want := original + "\n\n" + table
	if got := app.draft.String(); got != want {
		t.Fatalf("paste = %q, want %q", got, want)
	}
	body, _ := renderPreview(parser.New(webPreviewExtensions), want, "Table")
	if !strings.Contains(body, "<table>") || !strings.Contains(body, "<code>struct proc</code>") {
		t.Fatalf("preview did not preserve the table and code cell: %s", body)
	}
	view.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != original {
		t.Fatalf("undo = %q, want %q", got, original)
	}
}
