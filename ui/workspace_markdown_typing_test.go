// spec: Markdown typing assistance / EV-MYGO-270, EV-MYGO-271.
package ui

import (
	"testing"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestMarkdownTypingCommitsMixedLanguageOnce(t *testing.T) {
	const initial = "🙂使用API"
	app, test, _ := snippetsWorkspace(t, "Prose.md", initial, splitMode)
	at := len("🙂使用")
	app.code.Select(at, at)
	test.Compose("ni", 2)
	test.Compose("nihao", 5)
	if got := app.draft.String(); got != "🙂使用nihaoAPI" {
		t.Fatalf("preedit = %q", got)
	}
	test.Type("你好Go中")
	const committed = "🙂使用你好 Go 中 API"
	const caret = len("🙂使用你好 Go 中")
	if got := app.draft.String(); got != committed || app.code.Cursor() != caret {
		t.Fatalf("commit = %q at %d, want %q at %d", got, app.code.Cursor(), committed, caret)
	}
	test.Command("undo")
	if got := app.draft.String(); got != initial || app.code.Cursor() != at {
		t.Fatalf("undo = %q at %d", got, app.code.Cursor())
	}
	test.Command("redo")
	if got := app.draft.String(); got != committed || app.code.Cursor() != caret {
		t.Fatalf("redo = %q at %d", got, app.code.Cursor())
	}
	test.Command("undo")
	test.SetClipboard("你好Go")
	test.Command("paste")
	if got := app.draft.String(); got != "🙂使用你好GoAPI" {
		t.Fatalf("paste = %q", got)
	}
}

func TestMarkdownTypingKeepsCaretInsideMixedLanguageSpacing(t *testing.T) {
	app, test, _ := snippetsWorkspace(t, "Prose.md", "中文", sourceMode)
	app.code.Select(len("中"), len("中"))
	test.TypeKey(0, mygoui.KeyG, "G")
	const spaced = "中 G 文"
	const caret = len("中 G")
	if got := app.draft.String(); got != spaced || app.code.Cursor() != caret {
		t.Fatalf("typed = %q at %d, want %q at %d", got, app.code.Cursor(), spaced, caret)
	}
	app.code.Run(editor.Undo)
	if app.draft.String() != "中文" || app.code.Cursor() != len("中") {
		t.Fatalf("undo = %q at %d", app.draft.String(), app.code.Cursor())
	}
	app.code.Run(editor.Redo)
	test.TypeKey(0, mygoui.KeyO, "o")
	if got := app.draft.String(); got != "中 Go 文" {
		t.Fatalf("typing after redo = %q", got)
	}
}

func TestMarkdownTypingContinuesAndExitsSplitList(t *testing.T) {
	const initial = "9. 一项"
	app, test, _ := snippetsWorkspace(t, "List.md", initial, splitMode)
	app.code.Select(len(initial), len(initial))
	test.Key(0, mygoui.KeyEnter)
	const continued = initial + "\n10. "
	if got := app.draft.String(); got != continued || app.code.Cursor() != len(continued) {
		t.Fatalf("first Enter = %q at %d", got, app.code.Cursor())
	}
	test.Key(0, mygoui.KeyEnter)
	test.Type("普通段落")
	if got := app.draft.String(); got != initial+"\n\n普通段落" {
		t.Fatalf("text after empty list item = %q", got)
	}
	document := parser.Parse(app.draft.String())
	var last parser.Kind
	for index := range document.Children(0) {
		last = document.Block(index).Kind
	}
	if last != parser.Paragraph {
		t.Fatalf("exited list still renders following text inside markup: %v", last)
	}
}
