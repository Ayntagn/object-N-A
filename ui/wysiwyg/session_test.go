package wysiwyg

// spec: EV-EDITOR-004 / EV-EDITOR-005 / EV-EDITOR-007 / EV-UI-102

import (
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func TestVisualEditUpdatesCurrentDraftAndUndoRestoresSource(t *testing.T) {
	const original = "\ufeff---\n# 保留原始注释\ntitle: '样例'\n---\n\nTitle\n=====\n\nA😀中B\n"
	draft := editor.NewBuffer(original)
	session := Open(draft)
	if session.Document == nil {
		t.Fatal("普通标题和正文应可编辑")
	}
	if session.Sync(draft) || draft.Modified() || draft.String() != original {
		t.Fatal("进入视觉编辑模式改写了源文")
	}

	session.Document.InsertText("呀")
	const edited = "\ufeff---\n# 保留原始注释\ntitle: '样例'\n---\n\n# Title\n\nA😀中B呀\n"
	if !session.Sync(draft) || draft.String() != edited || !draft.Modified() {
		t.Fatalf("保存前的草稿 = %q", draft.String())
	}
	session.Document.Undo()
	if !session.Sync(draft) || draft.String() != original || draft.Modified() {
		t.Fatalf("撤销后的草稿 = %q", draft.String())
	}

	session.Document.InsertText("只属于旧文件")
	other := editor.NewBuffer("Another document")
	if session.Sync(other) || other.String() != "Another document" {
		t.Fatal("旧编辑器改写了其他文档")
	}
	draft.Set("Source was edited directly")
	if session.Sync(draft) || draft.String() != "Source was edited directly" || session.Current(draft) {
		t.Fatal("旧编辑器覆盖了更新的源码")
	}
}

func TestLossyMarkdownRemainsInReadOnlyPreview(t *testing.T) {
	for name, source := range map[string]string{
		"ragged table":    "| A | B |\n| -- | -- |\n| 甲 | 😀 | extra |\n",
		"table cell HTML": "| A | B |\n| -- | -- |\n| <span style=\"color:red\">甲</span> | 1 |\n",
		"raw HTML":        "<div>保留原始属性</div>\n\n末尾\n",
		"nested diagram":  "> ```mermaid\n> graph TD; A-->B\n> ```\n",
		"wiki target":     "导航到 [[目录/文件|别名]]\n",
		"reference":       "前文\n\n[unused]: https://example.com \"Title\"\n",
		"footnote":        "内容[^note]\n\n[^note]: 注释\n",
	} {
		t.Run(name, func(t *testing.T) {
			draft := editor.NewBuffer(source)
			session := Open(draft)
			if session.Document != nil || session.Body != source {
				t.Fatal("无法往返的 Markdown 应保持完整并使用只读预览")
			}
			if session.Sync(draft) || draft.Modified() || draft.String() != source {
				t.Fatal("只读预览改写了源文")
			}
		})
	}
}

func TestTerminalWikilinkInsertionPreservesSource(t *testing.T) {
	const original = "# Mode Probe\n\n"
	const inserted = "# Mode Probe[[Output Box/Mode Probe]]\n\n"
	draft := editor.NewBuffer(original)
	session := Open(draft)
	end := session.Document.Text().Len()
	session.Document.SetSelection(end, end)
	if !session.InsertMarkdown("[[Output Box/Mode Probe]]\n") {
		t.Fatal("视觉编辑器拒绝了终端 Wikilink")
	}
	if !session.Sync(draft) || draft.String() != inserted {
		t.Fatalf("插入后的 Markdown = %q", draft.String())
	}
	reopened := Open(draft)
	if reopened.Document == nil || reopened.Document.Text().PlainText() != "Mode ProbeOutput Box/Mode Probe" {
		t.Fatal("重新投影后无法编辑带 Wikilink 的标题")
	}
	session.Document.Undo()
	if !session.Sync(draft) || draft.String() != original {
		t.Fatalf("撤销后的 Markdown = %q", draft.String())
	}
}
