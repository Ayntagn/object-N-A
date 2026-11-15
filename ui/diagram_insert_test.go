package ui

// spec: FEAT-DIAGRAM-INSERT / EV-UI-120 / EV-UI-121

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/egoist/mygo/ui/editor"
)

func TestDiagramSourceInsertion(t *testing.T) {
	source := "# Diagram Study\n\nBefore 😀 REPLACE after.\n\nTail paragraph.\n"
	app := &workspaceWindow{workspaceTab: &workspaceTab{mode: sourceMode}}
	app.draft = editor.NewBuffer(source)
	app.code = &editor.State{Buffer: app.draft}
	start := strings.Index(source, "REPLACE")
	app.code.Select(start, start+len("REPLACE"))

	app.insertDiagram(diagramCommands[0])
	got := app.draft.String()
	if !strings.HasPrefix(got, source[:start]+"```mermaid\ngraph TD\n") ||
		!strings.HasSuffix(got, "```"+source[start+len("REPLACE"):]) {
		t.Fatalf("diagram did not replace the selected bytes: %q", got)
	}
	selection := app.code.Selection()
	if selection.Start() != selection.End() || len(utf16.Encode([]rune(got[:selection.Start()]))) != 52 {
		t.Fatalf("caret = %+v, want UTF-16 offset 52", selection)
	}
	app.code.Run(editor.Undo)
	if restored := app.code.Selection(); app.draft.String() != source || restored.Start() != start || restored.End() != start+len("REPLACE") {
		t.Fatalf("undo restored %q at %+v", app.draft.String(), restored)
	}
}

func TestDiagramMenuOrder(t *testing.T) {
	var names []string
	for _, diagram := range diagramCommands {
		names = append(names, diagram.title)
	}
	want := []string{
		"Flowchart", "Sequence Diagram", "Class Diagram", "State Diagram", "ER Diagram", "Gantt Chart",
		"Pie Chart", "Mind Map", "Timeline", "Git Graph", "XY Chart", "C4 Diagram",
		"Sankey Diagram", "Quadrant Chart", "Kanban Board", "Tree Map",
	}
	if !slices.Equal(names, want) {
		t.Fatalf("menu items = %v", names)
	}
}
