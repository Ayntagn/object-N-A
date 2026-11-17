// spec: EV-MYGO-270 / Mixed-language input in the rendered Markdown editor.
package ui

import "testing"

func TestVisualMixedLanguageSpacing(t *testing.T) {
	t.Run("formatted selection", func(t *testing.T) {
		const before = "**甲空乙**"
		app, test, _ := snippetsWorkspace(t, "Spacing.md", before, wysiwygMode)
		doc := app.wys.session.Document
		doc.SetSelection(1, 2)
		test.Frame()
		test.Type("A")
		const want = "**甲 A 乙**"
		if a, z := doc.Selection(); app.draft.String() != want || a != 3 || z != 3 {
			t.Fatalf("typed Markdown = %q at %d:%d, want %q at 3:3", app.draft.String(), a, z, want)
		}
		test.Command("undo")
		if a, z := doc.Selection(); app.draft.String() != before || a != 1 || z != 2 {
			t.Fatalf("undo = %q at %d:%d, want %q at 1:2", app.draft.String(), a, z, before)
		}
		test.Command("redo")
		if a, z := doc.Selection(); app.draft.String() != want || a != 3 || z != 3 {
			t.Fatalf("redo = %q at %d:%d, want %q at 3:3", app.draft.String(), a, z, want)
		}
	})

	t.Run("Chinese composition", func(t *testing.T) {
		const before = "🙂 AlphaBeta"
		const at = 7
		app, test, _ := snippetsWorkspace(t, "Composition.md", before, wysiwygMode)
		doc := app.wys.session.Document
		doc.SetSelection(at, at)
		test.Frame()
		test.Compose("nihao", 5)
		if got := app.draft.String(); got != before {
			t.Fatalf("preedit wrote %q to the Markdown draft", got)
		}
		test.Type("你好")
		const want = "🙂 Alpha 你好 Beta"
		if a, z := doc.Selection(); app.draft.String() != want || a != at+3 || z != at+3 {
			t.Fatalf("IME commit = %q at %d:%d, want %q at %d", app.draft.String(), a, z, want, at+3)
		}
		test.Command("undo")
		if a, z := doc.Selection(); app.draft.String() != before || a != at || z != at {
			t.Fatalf("IME undo = %q at %d:%d, want %q at %d", app.draft.String(), a, z, before, at)
		}
	})

	t.Run("inline code", func(t *testing.T) {
		app, test, _ := snippetsWorkspace(t, "Code.md", "`甲x`", wysiwygMode)
		app.wys.session.Document.SetSelection(1, 1)
		test.Frame()
		test.Type("A")
		if got := app.draft.String(); got != "`甲Ax`" {
			t.Fatalf("typed code = %q, want %q", got, "`甲Ax`")
		}
	})

	t.Run("inline formula", func(t *testing.T) {
		app, test, _ := snippetsWorkspace(t, "Formula.md", "", wysiwygMode)
		for _, typed := range []string{"$", "甲", "x", "$"} {
			test.Type(typed)
		}
		if got := app.draft.String(); got != "$甲x$" {
			t.Fatalf("typed formula = %q, want %q", got, "$甲x$")
		}
	})
}
