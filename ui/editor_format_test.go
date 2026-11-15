package ui

// spec: FEAT-001 / EV-UI-021 / EV-EDITOR-006

import (
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func TestFormatMarkdownPreservesUnicodeSelectionThroughUndo(t *testing.T) {
	state := &editor.State{Buffer: editor.NewBuffer("A😀中B")}
	state.Select(8, 1)

	if !formatMarkdown(state, boldFormat) {
		t.Fatal("formatting was not applied")
	}
	if got := state.Buffer.String(); got != "A**😀中**B" {
		t.Fatalf("formatted text = %q", got)
	}
	if got := state.Selection(); got != (editor.Selection{Anchor: 10, Head: 3}) {
		t.Fatalf("formatted selection = %+v", got)
	}
	if !state.Run(editor.Undo) || state.Buffer.CanUndo() {
		t.Fatal("one undo did not revert the entire formatting command")
	}
	if got := state.Buffer.String(); got != "A😀中B" {
		t.Fatalf("text after undo = %q", got)
	}
	if got := state.Selection(); got != (editor.Selection{Anchor: 8, Head: 1}) {
		t.Fatalf("selection after undo = %+v", got)
	}
}
