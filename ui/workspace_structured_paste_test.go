// spec: FEAT-001 / EV-MYGO-303. A copied Markdown list retains its items and inline code.
package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/transfer"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

const goTestConventions = "Go 的测试文件命名约定：\n\n" +
	"- 测试文件：`xxx_test.go`\n" +
	"- 测试函数：`func TestXxx(t *testing.T)`\n" +
	"- 基准测试：`func BenchmarkXxx(b *testing.B)`\n" +
	"- 示例函数：`func ExampleXxx()`"

func TestVisualPasteMarkdownStructures(t *testing.T) {
	const before, after = "保留前文。", "保留后文。"
	app, view, _ := snippetsWorkspace(t, "Paste.md", before+after, wysiwygMode)
	app.preferences.AutoIndentParagraphs = true
	position := utf8.RuneCountInString(before)
	app.wys.session.Document.SetSelection(position, position)
	view.Frame()
	view.SetClipboard(goTestConventions)
	view.Key(mygoui.Cmd, mygoui.KeyV)

	const want = before + "\n\n" + paragraphIndent + goTestConventions + "\n\n" + after
	if got := app.draft.String(); got != want {
		t.Fatalf("pasted visual Markdown = %q, want %q", got, want)
	}
	items, code := 0, 0
	for _, paragraph := range app.wys.session.Document.Text().Paragraphs() {
		if paragraph.Style.List == richtext.Bullets {
			items++
			for _, run := range paragraph.Runs {
				if run.Attributes.Code {
					code++
				}
			}
		}
	}
	if items != 4 || code != 4 {
		t.Fatalf("visual paste made %d list items with %d inline code runs", items, code)
	}
	view.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != before+after {
		t.Fatalf("undo left %q", got)
	}
}

func TestSplitPastePreservesClipboardStructures(t *testing.T) {
	const before, after = "保留前文。", "保留后文。"
	const browserHTML = `<p>Go 的测试文件命名约定：</p><ul>
		<li>测试文件：<code>xxx_test.go</code></li>
		<li>测试函数：<code>func TestXxx(t *testing.T)</code></li>
		<li>基准测试：<code>func BenchmarkXxx(b *testing.B)</code></li>
		<li>示例函数：<code>func ExampleXxx()</code></li></ul>`
	for _, test := range []struct {
		name string
		data transfer.Data
	}{
		{"Markdown as plain text", transfer.TextData(goTestConventions)},
		{"Explicit Markdown", transfer.New(transfer.NewItem(transfer.Bytes(transfer.Markdown, []byte(goTestConventions)),
			transfer.Bytes(transfer.Text, []byte("Plain fallback"))))},
		{"Web selection", transfer.New(transfer.NewItem(transfer.Bytes(transfer.HTML, []byte(browserHTML)),
			transfer.Bytes(transfer.Text, []byte(strings.NewReplacer("`", "", "- ", "").Replace(goTestConventions)))))},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, view, _ := snippetsWorkspace(t, "Paste.md", before+after, splitMode)
			app.code.Select(len(before), len(before))
			view.SetClipboardData(test.data)
			view.Key(mygoui.Cmd, mygoui.KeyV)
			const want = before + "\n\n" + goTestConventions + "\n\n" + after
			if got := app.draft.String(); got != want {
				t.Fatalf("pasted split Markdown = %q, want %q", got, want)
			}
			body, _ := renderPreview(parser.New(webPreviewExtensions), app.draft.String(), "Paste")
			if strings.Count(body, "<li>") != 4 || strings.Count(body, "<code>") != 4 {
				t.Fatalf("preview lost the list or code styling: %s", body)
			}
		})
	}
}

func TestVisualPasteMarkdownBlockSequence(t *testing.T) {
	const markdown = "## 示例\n\n> 运行测试前保存。\n\n1. 第一项\n2. 第二项\n\n" +
		"- [x] 已完成\n\n```go\nfunc Example() {}\n```\n\n| 名称 | 命令 |\n| --- | --- |\n| 测试 | `go test` |"
	app, view, _ := snippetsWorkspace(t, "Paste.md", "", wysiwygMode)
	view.SetClipboard(markdown)
	view.Key(mygoui.Cmd, mygoui.KeyV)
	body, _ := renderPreview(parser.New(webPreviewExtensions), app.draft.String(), "Paste")
	for _, markup := range []string{"<h2", "<blockquote", `<ol start="1"`, `type="checkbox"`, "<pre", "<table", "<code>go test</code>"} {
		if !strings.Contains(body, markup) {
			t.Fatalf("paste lost %q: %s", markup, app.draft.String())
		}
	}
	if !strings.Contains(app.draft.String(), "```go\nfunc Example() {}\n```") {
		t.Fatalf("pasted fence lost its Go language or body: %q", app.draft.String())
	}
	if grid, ok := view.AccessByRole(mygoui.RoleTable, "Markdown table"); !ok || grid.RowCount != 2 || grid.ColumnCount != 2 {
		t.Fatalf("pasted table grid = %+v", grid)
	}
}
