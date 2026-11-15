// spec: EV-EDITOR-004 / EV-EDITOR-007 / EV-UI-145 / U02
package wysiwyg

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestGoldenTableCellEditPersistsWithAlignment(t *testing.T) {
	const original = "\ufeff---\r\n# 保留原始注释\r\ntitle: '表格'\r\n---\r\n\r\n# Table\r\n\r\n" +
		"| 名称 | 数量 |\r\n| --- | ---: |\r\n| 甲 | 1 |\r\n| 乙 | 2 |\r\n\r\n外部 [[笔记|别名]]  \r\n"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Table.md"), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	doc, err := vault.ReadDocument("Table.md")
	if err != nil {
		t.Fatal(err)
	}
	draft := editor.NewBuffer(doc.Text)
	session := Open(draft)
	if session.Document != nil || session.TableCount() != 1 {
		t.Fatal("混合正文应保留只读预览并提供一个可编辑表格")
	}
	table := session.EditTable(0)
	rows := table.Rows()
	rows[1][0] = "中文甲"
	if err := table.Apply(draft, rows); err != nil {
		t.Fatal(err)
	}
	if !draft.Undo() || draft.Modified() || draft.CanUndo() || !draft.Redo() {
		t.Fatal("整个表格编辑应能一步撤销并重做")
	}
	var output bytes.Buffer
	if _, err := draft.WriteTo(&output); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.SaveDocument(doc, output.String()); err != nil {
		t.Fatal(err)
	}
	reopened, err := vault.ReadDocument("Table.md")
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Replace(original, "| 甲 |", "| 中文甲 |", 1); reopened.Text != want {
		t.Fatalf("重开源文 = %q，期望 %q", reopened.Text, want)
	}
	edited := Open(editor.NewBuffer(reopened.Text)).EditTable(0)
	if edited == nil || edited.Rows()[1][0] != "中文甲" || edited.Align(1) != parser.AlignRight {
		t.Fatal("重开后应保留完整单元格及右对齐")
	}
}

func TestTableCanGrowWithEscapedPipesAndCode(t *testing.T) {
	draft := editor.NewBuffer("| 左列 | 右列 |\n| :---- | ----: |\n| 原有\\|文字 | `x\\|y` |\n\n后文\n")
	table := Open(draft).EditTable(0)
	if table == nil {
		t.Fatal("带转义管道符的表格应可编辑")
	}
	rows := table.Rows()
	rows[0] = append(rows[0], "新增")
	rows[1] = append(rows[1], "A | B")
	rows = append(rows, []string{"`左|右`", "", "😀"})
	if err := table.Apply(draft, rows); err != nil {
		t.Fatal(err)
	}
	const want = "| 左列 | 右列 | 新增 |\n| :---- | ----: | --- |\n| 原有\\|文字 | `x\\|y` | A \\| B |\n" +
		"| `左\\|右` |  | 😀 |\n\n后文\n"
	if draft.String() != want {
		t.Fatalf("扩展后的表格 = %q，期望 %q", draft.String(), want)
	}
}

func TestTableCanClearCellsWithoutOuterPipes(t *testing.T) {
	draft := editor.NewBuffer("A | B\n--- | ---:\n1 | 2\n\nAfter\n")
	table := Open(draft).EditTable(0)
	if table == nil {
		t.Fatal("省略首尾管道符的表格应可编辑")
	}
	rows := table.Rows()
	rows[0][0], rows[1][1] = "", ""
	if err := table.Apply(draft, rows); err != nil {
		t.Fatal(err)
	}
	const want = "|  | B |\n| --- | ---: |\n| 1 |  |\n\nAfter\n"
	if draft.String() != want {
		t.Fatalf("清空单元格后的表格 = %q，期望 %q", draft.String(), want)
	}
}

func TestTableInsertionKeepsFrontmatterAndUndo(t *testing.T) {
	const original = "\ufeff---\ntitle: '表格'\n---"
	draft := editor.NewBuffer(original)
	table := Open(draft).NewTable()
	if err := table.Apply(draft, [][]string{{"名称", "数量"}, {"苹果", ""}}); err != nil {
		t.Fatal(err)
	}
	const want = original + "\n\n| 名称 | 数量 |\n| --- | --- |\n| 苹果 |  |"
	if draft.String() != want || Open(draft).TableCount() != 1 {
		t.Fatalf("插入后的草稿 = %q", draft.String())
	}
	if !draft.Undo() || draft.String() != original {
		t.Fatal("撤销插入应保留原样的 frontmatter")
	}
}

func TestUnsafeAndStaleTableEditsKeepSource(t *testing.T) {
	for _, source := range []string{
		"| A | B |\n| --- | --- |\n| 1 | 2 | ignored |\n",
		"> | A | B |\n> | --- | --- |\n> | 1 | 2 |\n",
	} {
		draft := editor.NewBuffer(source)
		if session := Open(draft); session.TableCount() != 0 || session.Document != nil {
			t.Fatalf("不规则或嵌套表格应继续用源码编辑：%q", source)
		}
	}
	fenced := editor.NewBuffer("```\nunfinished\n")
	insert := Open(fenced).NewTable()
	if err := insert.Apply(fenced, insert.Rows()); err == nil || fenced.Modified() {
		t.Fatal("向未关闭的代码块写入表格应被拒绝")
	}
	const valid = "| A | B |\n| --- | --- |\n| 1 | 2 |\n"
	draft := editor.NewBuffer(valid)
	table := Open(draft).EditTable(0)
	rows := table.Rows()
	rows[1][0] = "changed"
	other := editor.NewBuffer(valid)
	if err := table.Apply(other, rows); err == nil || other.Modified() {
		t.Fatal("旧表格编辑不应写入同内容的其他草稿")
	}
	const changed = valid + "\nsource change"
	draft.Set(changed)
	if err := table.Apply(draft, rows); err == nil || draft.String() != changed {
		t.Fatal("旧表格编辑不应覆盖新草稿版本")
	}
}

func TestVisualTableRemovalPreservesSourceAndUndo(t *testing.T) {
	const before = "\ufeff---\ntitle: '表格'\n---\n\nTitle\n=====\n\n"
	const table = "| Name |\n| :---- |\n| 甲 |"
	const after = "\n\nSuffix *keeps* marks.\n\n"
	const original = before + table + after
	draft := editor.NewBuffer(original)
	session := Open(draft)
	if session.Document == nil {
		t.Fatal("表格夹具应能在正文编辑")
	}
	offset := len("Title\n")
	current, ok := session.TableAt(offset)
	if !ok {
		t.Fatal("正文应包含表格")
	}
	updated, err := session.ChangeTableCell(offset, current, 1, 0, "中文甲")
	if err != nil || !session.Sync(draft) {
		t.Fatalf("前置单元格编辑失败：%v", err)
	}
	edited := before + strings.Replace(table, "甲", "中文甲", 1) + after
	if got := draft.String(); got != edited {
		t.Fatalf("单元格编辑得到 %q，期望 %q", got, edited)
	}
	removed, err := session.ChangeTableStructure(offset, updated, DeleteTableColumn, 0, 0, parser.AlignNone)
	if err != nil || removed != nil || !session.Sync(draft) {
		t.Fatalf("删除唯一列应移除表格：%v", err)
	}
	if got := draft.String(); got != before+after {
		t.Fatalf("删除表格得到 %q，期望 %q", got, before+after)
	}
	if anchor, caret := session.Document.Selection(); anchor != offset || caret != offset || session.Document.Text().Slice(caret, caret+1).String() != "S" {
		t.Fatalf("删除后光标应移至后文开头，得到 %d:%d", anchor, caret)
	}
	for _, want := range []string{edited, original} {
		session.Document.Undo()
		if !session.Sync(draft) || draft.String() != want {
			t.Fatalf("撤销得到 %q，期望 %q", draft.String(), want)
		}
		restored, ok := session.TableAt(offset)
		if !ok || restored.ID() != current.ID() {
			t.Fatal("撤销应恢复同一表格的旧版本")
		}
	}
	for _, want := range []string{edited, before + after} {
		session.Document.Redo()
		if !session.Sync(draft) || draft.String() != want {
			t.Fatalf("重做得到 %q，期望 %q", draft.String(), want)
		}
	}
}
