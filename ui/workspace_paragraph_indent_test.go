// spec: user-requested Source paragraph indentation with two U+3000 spaces.
package ui

import (
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func TestAutoIndentParagraphsTypesAndUndoesInSplit(t *testing.T) {
	const initial = "# Heading\n\n"
	app, test, _ := snippetsWorkspace(t, "Prose.md", initial, splitMode)
	app.code.Select(len(initial), len(initial))
	test.Type("平")
	if got := app.draft.String(); got != initial+"平" {
		t.Fatalf("disabled: got %q", got)
	}
	app.code.Run(editor.Undo)
	app.preferences.AutoIndentParagraphs = true
	test.Type("中文")
	if got := app.draft.String(); got != initial+"\u3000\u3000中文" {
		t.Fatalf("enabled: got %q", got)
	}
	if !app.code.Run(editor.Undo) || app.draft.String() != initial || app.code.Cursor() != len(initial) {
		t.Fatalf("undo did not restore paragraph and caret: %q, %d", app.draft.String(), app.code.Cursor())
	}
	test.SetClipboard("粘贴")
	test.Command("paste")
	if got := app.draft.String(); got != initial+"粘贴" {
		t.Fatalf("paste: got %q", got)
	}
}

func TestAutoIndentParagraphsKeepsMarkdownStructure(t *testing.T) {
	const initial = "---\ntitle: Example\n\n---\n\n~~~md\n\n~~~\n\n"
	app, test, _ := snippetsWorkspace(t, "Structure.md", initial, sourceMode)
	app.preferences.AutoIndentParagraphs = true
	app.code.GoTo(editor.Pos{Line: 2})
	test.Type("title")
	app.code.GoTo(editor.Pos{Line: 6})
	test.Type("code")
	app.code.Select(app.draft.Len(), app.draft.Len())
	test.Type("#")
	test.Type(" Heading")
	if got, want := app.draft.String(), "---\ntitle: Example\ntitle\n---\n\n~~~md\ncode\n~~~\n\n# Heading"; got != want {
		t.Fatalf("structure: got %q, want %q", got, want)
	}
}
