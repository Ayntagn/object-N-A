// spec: EV-UI-246
package ui

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAIVisualWritebackShowsRawAndCommitsOneUndo(t *testing.T) {
	const heading = "# Heading 🧪\n\n"
	const selected = "SELECT naïve 👩🏽‍💻"
	const remainder = "\n\nOutside sentinel.\n"
	const original = heading + selected + remainder
	const partial = "Reply β **emphasis**"
	const finished = partial + " done."
	for _, replace := range []bool{false, true} {
		name, kept := "insert", selected
		if replace {
			name, kept = "replace", ""
		}
		t.Run(name, func(t *testing.T) {
			app, _, _ := snippetsWorkspace(t, "Visual AI.md", original, wysiwygMode)
			app.preferences.AIStreamIntoEditor = true
			app.ai.replaceSelection = replace
			session := app.wys.session
			doc := session.Document
			plain := doc.Text().String()
			start := utf8.RuneCountInString(plain[:strings.Index(plain, selected)])
			end := start + utf8.RuneCountInString(selected)
			doc.SetSelection(end, start)

			stream := app.beginAIVisualWriteback()
			if stream == nil {
				t.Fatal("no visual writeback")
			}
			preview := stream.previewFor(app, doc)
			if !stream.update(app, partial) || preview == nil || preview != stream.previewFor(app, doc) ||
				!strings.Contains(preview.Text().String(), partial+kept) {
				t.Fatal("stream did not show literal Markdown at the original position")
			}
			if doc.Text().String() != plain || app.draft.String() != original || doc.CanUndo() {
				t.Fatal("stream preview changed the editable document, draft or undo history")
			}
			if !stream.commit(app, finished) || stream.previewFor(app, doc) != nil {
				t.Fatal("the confirmed reply did not replace the preview")
			}
			if got, want := app.draft.String(), heading+finished+kept+remainder; got != want {
				t.Fatalf("draft = %q, want %q", got, want)
			}
			if got := doc.Text().String(); !strings.Contains(got, "Reply β emphasis done."+kept) {
				t.Fatalf("completed visual text = %q", got)
			}
			doc.Undo()
			a, z := doc.Selection()
			if !session.Sync(app.draft) || app.draft.String() != original || doc.CanUndo() || a != end || z != start {
				t.Fatal("one Undo did not restore the document and original selection")
			}
		})
	}
}

func TestAIVisualWritebackStopAndStaleDraft(t *testing.T) {
	const original = "SELECT **formatted** 🙂\n\nOutside sentinel.\n"
	app, _, _ := snippetsWorkspace(t, "Visual stop.md", original, wysiwygMode)
	app.preferences.AIStreamIntoEditor, app.ai.replaceSelection = true, true
	doc := app.wys.session.Document
	doc.SetSelection(0, utf8.RuneCountInString("SELECT formatted 🙂"))
	if got := app.wys.session.SelectionMarkdown(); got != "SELECT **formatted** 🙂\n" {
		t.Fatalf("formatted selection = %q", got)
	}
	empty := app.beginAIVisualWriteback()
	if empty == nil || strings.Contains(empty.previewFor(app, doc).Text().String(), "SELECT") {
		t.Fatal("replace preview did not hide the selection")
	}
	if empty.commit(app, "") || app.draft.String() != original || doc.CanUndo() {
		t.Fatal("ending without output discarded the selection")
	}

	stop := app.beginAIVisualWriteback()
	const partial = "Reply β"
	if !stop.update(app, partial) || !stop.commit(app, partial) || app.draft.String() != partial+"\n\nOutside sentinel.\n" {
		t.Fatal("stop did not commit the received partial reply")
	}
	doc.Undo()
	app.wys.session.Sync(app.draft)
	stale := app.beginAIVisualWriteback()
	doc.InsertText("User edit.")
	if stale.update(app, partial) || stale.commit(app, partial) || !strings.HasPrefix(doc.Text().String(), "User edit.") {
		t.Fatal("a late reply replaced the user's visual edit")
	}
}
