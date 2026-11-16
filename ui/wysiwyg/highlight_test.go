// spec: FEAT-001 / Milkdown highlight mark in re/ui/milkdown-schema-and-plugins.md.
package wysiwyg

import (
	"testing"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestVisualHighlightReadsMarkdown(t *testing.T) {
	const source = "前文 ==中文== 和 **==要点==**。\n\n`==code==`\n"
	draft := editor.NewBuffer(source)
	session := Open(draft)
	if session.Document == nil {
		t.Fatal("含高亮标记的正文应可编辑")
	}
	text := session.Document.Text()
	if got := text.PlainText(); got != "前文 中文 和 要点。\n==code==" {
		t.Fatalf("视觉正文 = %q", got)
	}
	color := mygoui.RGBA(255, 214, 92, .32)
	if a := text.AttributesAt(3); a.Background != color || a.Bold() {
		t.Fatalf("普通高亮属性 = %+v", a)
	}
	if a := text.AttributesAt(8); a.Background != color || !a.Bold() {
		t.Fatalf("粗体高亮属性 = %+v", a)
	}
	if a := text.AttributesAt(12); a.Background.A != 0 || !a.Code {
		t.Fatalf("代码属性 = %+v", a)
	}
	if session.Sync(draft) || draft.Modified() || draft.String() != source {
		t.Fatal("进入视觉编辑模式改写了已有标记")
	}
	session.Document.SetSelection(0, 0)
	session.Document.InsertText("补")
	if !session.Sync(draft) || draft.String() != "补"+source {
		t.Fatalf("编辑正文后丢失了已有标记，Markdown = %q", draft.String())
	}
}

func TestVisualHighlightTogglesAndUndoesWithBoldIntact(t *testing.T) {
	const original = "# 章\n\n说明 **关键** 😀。\n"
	const highlighted = "# 章\n\n说明 **==关键==** 😀。\n"
	draft := editor.NewBuffer(original)
	session := Open(draft)
	if session.Document == nil {
		t.Fatal("正文应可编辑")
	}
	session.Document.SetSelection(7, 5)
	if !ToggleHighlight(session.Document) || !session.Sync(draft) || draft.String() != highlighted {
		t.Fatalf("高亮后的 Markdown = %q", draft.String())
	}
	if anchor, caret := session.Document.Selection(); anchor != 7 || caret != 5 {
		t.Fatalf("高亮后的选区 = %d:%d", anchor, caret)
	}
	reopened := Open(draft)
	if reopened.Document == nil {
		t.Fatal("带高亮的正文重开后应可编辑")
	}
	if a := reopened.Document.Text().AttributesAt(5); a.Background != mygoui.RGBA(255, 214, 92, .32) || !a.Bold() {
		t.Fatalf("重开的背景与粗体属性 = %+v", a)
	}
	if !ToggleHighlight(session.Document) || !session.Sync(draft) || draft.String() != original {
		t.Fatalf("取消高亮后的 Markdown = %q", draft.String())
	}
	session.Document.Undo()
	if !session.Sync(draft) || draft.String() != highlighted {
		t.Fatalf("撤销后的 Markdown = %q", draft.String())
	}
}
