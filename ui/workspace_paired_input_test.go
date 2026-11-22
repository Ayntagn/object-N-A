// spec: Markdown pair completion / EV-MYGO-297.
package ui

import (
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestSourcePairedInputTypesNestedWikilink(t *testing.T) {
	const prefix = "🙂"
	app, test, _ := snippetsWorkspace(t, "Pairs.md", prefix, splitMode)
	app.code.Select(len(prefix), len(prefix))
	test.Type("[")
	if got, at := app.draft.String(), app.code.Cursor(); got != prefix+"[]" || at != len(prefix)+1 {
		t.Fatalf("opening bracket got %q at %d; want a pair with the caret inside", got, at)
	}
	test.Type("[")
	test.Type("Page")
	test.Type("]")
	test.Type("]")
	if got, at := app.draft.String(), app.code.Cursor(); got != prefix+"[[Page]]" || at != len(got) {
		t.Fatalf("typing and stepping over both closers got %q at %d", got, at)
	}
	test.Type("（")
	test.Key(0, mygoui.KeyBackspace)
	if got := app.draft.String(); got != prefix+"[[Page]]" {
		t.Fatalf("paired backspace got %q", got)
	}
	test.Command("undo")
	if got, at := app.draft.String(), app.code.Cursor(); got != prefix+"[[Page]]（）" || at != len(prefix+"[[Page]]（") {
		t.Fatalf("undoing paired backspace got %q at %d", got, at)
	}
}

func TestSourcePairedInputKeepsLiteralPunctuation(t *testing.T) {
	const prefix = "don"
	app, test, _ := snippetsWorkspace(t, "Pairs.md", prefix, sourceMode)
	app.code.Select(len(prefix), len(prefix))
	test.Type("'")
	test.Type("t")
	test.Type("\\")
	test.Type("(")
	if got := app.draft.String(); got != "don't\\(" {
		t.Fatalf("apostrophe and escaped opener changed to %q", got)
	}
	test.SetClipboard("[")
	test.Command("paste")
	if got := app.draft.String(); got != "don't\\([" {
		t.Fatalf("paste complemented a literal bracket: %q", got)
	}
}

func TestSourcePairedInputPreservesParagraphIndent(t *testing.T) {
	const heading = "# 示例\n\n"
	app, test, _ := snippetsWorkspace(t, "Pairs.md", heading, sourceMode)
	app.preferences.AutoIndentParagraphs = true
	app.code.Select(len(heading), len(heading))
	test.Type("“")
	if got, at := app.draft.String(), app.code.Cursor(); got != heading+paragraphIndent+"“”" || at != len(heading+paragraphIndent+"“") {
		t.Fatalf("paired punctuation at the start of an indented paragraph got %q at %d", got, at)
	}
}

func TestVisualPairedInputPreservesFormattingAndSelection(t *testing.T) {
	const before = "甲**🙂**乙"
	app, test, _ := snippetsWorkspace(t, "Pairs.md", before, wysiwygMode)
	doc := app.wys.session.Document
	doc.SetSelection(1, 2)
	test.Frame()
	test.Type("「")
	const want = "甲**「🙂」**乙"
	if a, z := doc.Selection(); app.draft.String() != want || a != 2 || z != 3 {
		t.Fatalf("wrapping formatted selection got %q at %d:%d, want %q at 2:3", app.draft.String(), a, z, want)
	}
	test.Command("undo")
	if a, z := doc.Selection(); app.draft.String() != before || a != 1 || z != 2 {
		t.Fatalf("undo got %q at %d:%d, want %q at 1:2", app.draft.String(), a, z, before)
	}
	test.Command("redo")
	test.Key(0, mygoui.KeyRight)
	test.Type("」")
	if a, z := doc.Selection(); app.draft.String() != want || a != 4 || z != 4 {
		t.Fatalf("stepping over closer got %q at %d:%d, want %q at 4:4", app.draft.String(), a, z, want)
	}
}

func TestVisualPairedInputInCodeCard(t *testing.T) {
	app, test, _ := snippetsWorkspace(t, "Pairs.md", "", wysiwygMode)
	for range 3 {
		test.TypeKey(0, mygoui.KeyBackquote, "`")
	}
	test.Type("go")
	test.Key(0, mygoui.KeyEnter)
	if !test.Focused("go source") {
		t.Fatal("new code block did not receive focus")
	}
	test.Type("fmt.Println")
	test.Type("(")
	if got, want := app.draft.String(), "```go\nfmt.Println()\n```\n\n"; got != want {
		t.Fatalf("typing '(' in the code block got %q, want %q", got, want)
	}
	test.Type("1")
	test.Type(")")
	if got, want := app.draft.String(), "```go\nfmt.Println(1)\n```\n\n"; got != want {
		t.Fatalf("typing through the existing ')' got %q, want %q", got, want)
	}
}
