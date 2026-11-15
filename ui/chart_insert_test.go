package ui

// spec: EV-UI-147 / EV-UI-151

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/egoist/mygo/ui/editor"
)

func TestChartSourceInsertion(t *testing.T) {
	const original = "Before 😀 REPLACE after."
	app := &workspaceWindow{workspaceTab: &workspaceTab{mode: sourceMode}}
	app.draft = editor.NewBuffer(original)
	app.code = &editor.State{Buffer: app.draft}
	start := strings.Index(original, "REPLACE")
	end := start + len("REPLACE")
	app.code.Select(start, end)

	app.insertChart(chartCommands[0])

	got := app.draft.String()
	prefix := original[:start] + "```vega-lite\n"
	suffix := "\n```" + original[end:]
	body, foundPrefix := strings.CutPrefix(got, prefix)
	body, foundSuffix := strings.CutSuffix(body, suffix)
	if !foundPrefix || !foundSuffix || !json.Valid([]byte(body)) {
		t.Fatalf("expected a JSON fence replacing the selection, got %q", got)
	}
	selection := app.code.Selection()
	if selection.Start() != selection.End() || len(utf16.Encode([]rune(got[:selection.Start()]))) != 23 {
		t.Fatalf("caret = %+v, want UTF-16 offset 23", selection)
	}
	app.code.Run(editor.Undo)
	if restored := app.code.Selection(); app.draft.String() != original || restored.Start() != start || restored.End() != end {
		t.Fatalf("undo restored %q at %+v", app.draft.String(), restored)
	}
}
