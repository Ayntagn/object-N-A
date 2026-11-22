// spec: Source leading spaces and U+3000 paragraph indentation / U75 / EV-MYGO-250, EV-MYGO-251, EV-MYGO-296.
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
	const prose = "UNIX 管道用于连接进程。"
	const rest = "\n\n    printf('code')\n\n　　已缩进的段落。"
	test.SetClipboard(prose + rest)
	test.Command("paste")
	if got, want := app.draft.String(), initial+paragraphIndent+prose+rest; got != want {
		t.Fatalf("paste: got %q, want %q", got, want)
	}
	test.Command("undo")
	if got := app.draft.String(); got != initial || app.code.Cursor() != len(initial) {
		t.Fatalf("paste undo got %q at %d, want %q at %d", got, app.code.Cursor(), initial, len(initial))
	}
}

func TestAutoIndentParagraphsConvertsFourthSpaceBeforeChineseProse(t *testing.T) {
	const heading = "# 示例🙂\n\n"
	const prose = "UNIX 管道用于连接进程。"
	app, test, _ := snippetsWorkspace(t, "Prose.md", heading+prose, splitMode)
	app.preferences.AutoIndentParagraphs = true
	app.code.Select(len(heading), len(heading))
	for range 4 {
		test.TypeKey(0, mygoui.KeySpace, " ")
	}
	if got, want := app.draft.String(), heading+paragraphIndent+prose; got != want {
		t.Fatalf("manual indentation: got %q, want %q", got, want)
	}
	body, _ := renderPreview(parser.New(webPreviewExtensions), app.draft.String(), "Prose")
	if !strings.Contains(body, "<p>"+paragraphIndent+prose+"</p>") {
		t.Fatalf("preview did not render the indented paragraph: %q", body)
	}
	test.Command("undo")
	if got, want := app.draft.String(), heading+"   "+prose; got != want || app.code.Cursor() != len(heading)+3 {
		t.Fatalf("undo got %q at %d, want %q at %d", got, app.code.Cursor(), want, len(heading)+3)
	}
}

func TestAutoIndentParagraphsConvertsManualSpacesAtFirstProseInput(t *testing.T) {
	const initial = "# 示例🙂\n\n"
	app, test, _ := snippetsWorkspace(t, "Prose.md", initial, sourceMode)
	app.preferences.AutoIndentParagraphs = true
	app.code.Select(len(initial), len(initial))
	for range 4 {
		test.TypeKey(0, mygoui.KeySpace, " ")
	}
	test.Compose("ni", 2)
	test.Type("你好")
	if got, want := app.draft.String(), initial+paragraphIndent+"你好"; got != want {
		t.Fatalf("committed prose: got %q, want %q", got, want)
	}
	test.Command("undo")
	if got, want := app.draft.String(), initial+"    "; got != want || app.code.Cursor() != len(want) {
		t.Fatalf("undo got %q at %d, want %q at %d", got, app.code.Cursor(), want, len(want))
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
	test.SetClipboard("code\n\n最后一行")
	test.Command("paste")
	app.code.GoTo(editor.Pos{Line: 7})
	for range 4 {
		test.TypeKey(0, mygoui.KeySpace, " ")
	}
	test.Type("注释")
	app.code.Select(app.draft.Len(), app.draft.Len())
	test.Type("#")
	test.Type(" Heading")
	if got, want := app.draft.String(), "---\ntitle: Example\ntitle\n---\n\n~~~md\ncode\n    注释\n最后一行\n~~~\n\n# Heading"; got != want {
		t.Fatalf("structure: got %q, want %q", got, want)
	}
}
