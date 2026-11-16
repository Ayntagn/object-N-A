package ui

// spec: FEAT-001 / EV-UI-021 / EV-EDITOR-006

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
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

// spec: EV-UI-009 / EV-EDITOR-007. Inline Highlight in the v0.2.0 regression.
func TestFormatHighlightRendersSelectedTextAndKeepsCodeLiteral(t *testing.T) {
	const source = "首句要标亮的是 **选中的词**。\n\n`==inline==`\n\n```c\nint equal = left == right;\n```\n"
	const selection = "**选中的词**"
	state := &editor.State{Buffer: editor.NewBuffer(source)}
	start := strings.Index(source, selection)
	state.Select(start, start+len(selection))
	if !formatMarkdown(state, highlightFormat) {
		t.Fatal("Highlight action did not apply")
	}
	marked := state.Buffer.String()
	if want := strings.Replace(source, selection, "=="+selection+"==", 1); marked != want {
		t.Fatalf("source = %q, want %q", marked, want)
	}
	body, _ := renderPreview(parser.New(webPreviewExtensions), marked, "")
	if !strings.Contains(body, "<mark class=\"cw-highlight\"><strong>选中的词</strong></mark>") ||
		!strings.Contains(body, "<code>==inline==</code>") || strings.Count(body, "<mark ") != 1 {
		t.Fatalf("highlighted preview = %s", body)
	}
	if !formatMarkdown(state, highlightFormat) || state.Buffer.String() != source {
		t.Fatalf("second action did not remove the marks, source = %q", state.Buffer.String())
	}
}
