// spec: v0.2.0 Source leading spaces and U+3000 paragraph indentation / U75 / EV-MYGO-250, EV-MYGO-251.
package ui

import (
	"strings"
	"testing"

	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
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

func TestAutoIndentParagraphsCommitsChineseCompositionAsOneEdit(t *testing.T) {
	const heading = "# 标题🙂"
	app, test, _ := snippetsWorkspace(t, "Prose.md", heading, sourceMode)
	app.preferences.AutoIndentParagraphs = true
	app.code.Select(len(heading), len(heading))
	test.Key(0, mygoui.KeyEnter)
	test.Key(0, mygoui.KeyEnter)
	const initial = heading + "\n\n"
	test.Compose("ni", 2)
	test.Compose("nihao", 5)
	if got := app.draft.String(); got != initial+"nihao" {
		t.Fatalf("preedit got %q", got)
	}
	test.Type("你好")
	const want = initial + "\u3000\u3000你好"
	if got := app.draft.String(); got != want || app.code.Cursor() != len(want) {
		t.Fatalf("committed paragraph and caret = %q, %d, want %q, %d", got, app.code.Cursor(), want, len(want))
	}
	body, _ := renderPreview(parser.New(webPreviewExtensions), app.draft.String(), "Prose")
	if !strings.Contains(body, "<p>\u3000\u3000你好</p>") {
		t.Fatalf("preview dropped the paragraph indent: %q", body)
	}
	visual := wysiwyg.Open(app.draft)
	if visual.Document == nil || !strings.HasSuffix(visual.Document.Text().PlainText(), "\u3000\u3000你好") {
		t.Fatal("WYSIWYG could not open the indented paragraph with its literal spaces")
	}
	t.Logf("committed paragraph and caret = %q, %d", app.draft.String(), app.code.Cursor())
	test.Command("undo")
	if got := app.draft.String(); got != initial || app.code.Cursor() != len(initial) {
		t.Fatalf("undo got %q at %d", got, app.code.Cursor())
	}
}

// v0.2.0 reported that typing a leading space in Chinese prose indented its wrapped rows.
func TestSourceLeadingSpacesAffectOnlyFirstVisualRow(t *testing.T) {
	paragraph := strings.Repeat("一段足够长的中文正文。", 15)
	app, test, _ := snippetsWorkspace(t, "Soft wrap.md", paragraph, sourceMode)
	test.SetSize(710, 640)
	caret := func() mygoui.Rect {
		t.Helper()
		at, ok := test.TextCaret()
		if !ok || at.H == 0 {
			t.Fatalf("source input has no caret at %d: %+v", app.code.Cursor(), at)
		}
		return at
	}
	app.code.Select(0, 0)
	test.Frame()
	left := caret()
	for range 2 {
		test.TypeKey(0, mygoui.KeySpace, " ")
	}
	first := caret()
	if got := app.draft.String(); got != "  "+paragraph || app.code.Cursor() != 2 || first.X <= left.X {
		t.Fatalf("first row did not retain the manually typed spaces at %d, caret %+v vs %+v", app.code.Cursor(), first, left)
	}
	test.Key(0, mygoui.KeyLeft)
	test.Key(0, mygoui.KeyLeft)
	test.Key(0, mygoui.KeyDown)
	wrapped := caret()
	if app.code.Cursor() <= 2 || wrapped.Y <= left.Y || wrapped.X != left.X {
		t.Fatalf("wrapped row caret = %+v at %d, want below first row at X=%g", wrapped, app.code.Cursor(), left.X)
	}
	t.Logf("initial X=%g, after two spaces X=%g, wrapped start X=%g", left.X, first.X, wrapped.X)
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
