// spec: EV-UI-162 / EV-UI-163 / EV-UI-164 / EV-UI-165
package ui

import (
	"strings"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestAPIResponseReplacesOnlyItsAdjacentFence(t *testing.T) {
	const request = "```api[title: Local Weather]\nGET https://example.invalid/fixture\n```"
	const stale = "```api-result\nHTTP 200 OK · 1 ms · 3 B · 2026-11-14 12:00\nold\n```"
	const next = "```api-result\nHTTP 200 OK · 2 ms · 3 B · 2026-11-14 12:01\nnew\n```"
	const before = "---\ntitle: Example\n---\n\n😀 Before\n\n" + request + "\n\n" + stale + "\n\nAfter.\n\n" + request + "\n"
	b := editor.NewBuffer(before)
	target, ok := apiRequestAt(b.Snapshot(), strings.Index(before, request))
	if !ok {
		t.Fatal("request following frontmatter was not found")
	}
	edit, ok := target.resultEdit(b.Snapshot(), next)
	if !ok {
		t.Fatal("adjacent result could not be replaced")
	}
	b.Edit(edit)
	if got, want := b.String(), strings.Replace(before, stale, next, 1); got != want {
		t.Fatalf("request writeback = %q, want %q", got, want)
	}
	if !b.Undo() || b.String() != before {
		t.Fatalf("undo left %q", b.String())
	}
	b.Edit(editor.Edit{Start: target.start, End: target.start + len(request), Text: "Edited away."})
	if _, ok := target.resultEdit(b.Snapshot(), next); ok {
		t.Fatal("a stale response would have been applied to a different request")
	}
	eof := editor.NewBuffer(request + "\n")
	last, ok := apiRequestAt(eof.Snapshot(), 0)
	if !ok {
		t.Fatal("request at EOF was not found")
	}
	insert, ok := last.resultEdit(eof.Snapshot(), next)
	if !ok {
		t.Fatal("could not insert after the final request")
	}
	eof.Edit(insert)
	if got, want := eof.String(), request+"\n\n"+next+"\n"; got != want {
		t.Fatalf("response at EOF = %q, want %q", got, want)
	}
	duplicates := editor.NewBuffer(request + "\n\n" + request + "\n")
	first, ok := apiRequestAt(duplicates.Snapshot(), 0)
	if !ok {
		t.Fatal("first of two identical requests was not found")
	}
	duplicates.Edit(editor.Edit{Start: 0, End: len(request) + 2})
	if _, ok := first.resultEdit(duplicates.Snapshot(), next); ok {
		t.Fatal("the removed request's response would be written beneath its duplicate")
	}
}

func TestAPIRequestInsertionRoutes(t *testing.T) {
	const template = "```api[title: Weather]\nGET https://api.open-meteo.com/v1/forecast?latitude=40.71&longitude=-74.01&current=temperature_2m,weather_code,wind_speed_10m\n```"
	source, _, _ := snippetsWorkspace(t, "Source.md", "😀\n\nReplace\n", sourceMode)
	at := strings.Index(source.draft.String(), "Replace")
	source.code.Select(at, at+len("Replace"))
	action := source.commandPaletteActions()[commandPaletteKey{"BLOCKS", "API Request"}]
	if action == nil {
		t.Fatal("Source palette omitted API Request")
	}
	action()
	if got, want := source.draft.String(), "😀\n\n"+template+"\n"; got != want {
		t.Fatalf("Source insertion = %q", got)
	}
	if s := source.code.Selection(); s.Start() != at+22 || s.End() != at+22 {
		t.Fatalf("Source selection = %+v", s)
	}

	visual, tester, _ := snippetsWorkspace(t, "Visual.md", "", wysiwygMode)
	tester.Type("/api")
	tester.Key(0, mygoui.KeyEnter)
	if got, want := visual.draft.String(), template+"\n\n"; got != want {
		t.Fatalf("WYSIWYG insertion = %q", got)
	}
	if a, z := visual.wys.session.Document.Selection(); a != 0 || z != 1 {
		t.Fatalf("WYSIWYG end-of-note selection = %d…%d", a, z)
	}
}
