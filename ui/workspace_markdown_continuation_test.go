package ui

import (
	"strings"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// spec: FEAT-001 / EV-MYGO-271 / Markdown source Enter continuation and empty marker exit.
func TestSourceMarkdownContinuation(t *testing.T) {
	enter := mygoui.InputEvent{Kind: mygoui.InputKeyDown, Key: mygoui.KeyEnter}
	tests := []struct {
		name, source, continued, exited string
	}{
		{"quote", "> 引用", "> 引用\n> ", "> 引用\n\n"},
		{"bullet", "- 内容", "- 内容\n- ", "- 内容\n\n"},
		{"numbered", "9. 内容", "9. 内容\n10. ", "9. 内容\n\n"},
		{"quoted bullet", "> - 内容", "> - 内容\n> - ", "> - 内容\n> \n> "},
		{"nested quote", "> > 内容", "> > 内容\n> > ", "> > 内容\n> \n> "},
		{"nested list", "1. 父项\n   - 子项", "1. 父项\n   - 子项\n   - ", "1. 父项\n   - 子项\n2. "},
		{"nested list on same line", "- - 内容", "- - 内容\n  - ", "- - 内容\n- "},
		{"list quote", "- > 内容", "- > 内容\n  > ", "- > 内容\n- "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code := &editor.State{Buffer: editor.NewBuffer(test.source)}
			code.Select(len(test.source), len(test.source))
			if !continueSourceMarkdownLine(code, enter) || code.Buffer.String() != test.continued || code.Cursor() != len(test.continued) {
				t.Fatalf("continued to %q with caret %d", code.Buffer.String(), code.Cursor())
			}
			if !continueSourceMarkdownLine(code, enter) || code.Buffer.String() != test.exited || code.Cursor() != len(test.exited) {
				t.Fatalf("exited to %q with caret %d", code.Buffer.String(), code.Cursor())
			}
			code.Run(editor.Undo)
			if got := code.Buffer.String(); got != test.continued || code.Cursor() != len(test.continued) {
				t.Fatalf("exit undo = %q with caret %d, want %q with caret %d", got, code.Cursor(), test.continued, len(test.continued))
			}
			code.Run(editor.Undo)
			if got := code.Buffer.String(); got != test.source {
				t.Fatalf("continuation undo = %q, want %q", got, test.source)
			}
		})
	}
}

func TestSourceMarkdownContinuationPreservesLiteralContent(t *testing.T) {
	enter := mygoui.InputEvent{Kind: mygoui.InputKeyDown, Key: mygoui.KeyEnter}
	tests := []string{
		"```text\n- literal",
		"> ~~~text\n> - literal",
		"    - literal",
		"$$\n> literal",
		"---\n- literal\n---",
		"+++\n- literal",
		"ordinary prose\n2. not a list",
		"ordinary prose\n    > lazy prose",
	}
	for _, source := range tests {
		t.Run(source, func(t *testing.T) {
			code := &editor.State{Buffer: editor.NewBuffer(source)}
			line := 1
			if code.Buffer.LineCount() == 1 {
				line = 0
			}
			caret := code.Buffer.Snapshot().LineEnd(line)
			code.Select(caret, caret)
			if continueSourceMarkdownLine(code, enter) || code.Buffer.String() != source {
				t.Fatalf("modified literal content to %q", code.Buffer.String())
			}
		})
	}
}

func TestSourceMarkdownContinuationPreservesCRLF(t *testing.T) {
	code := &editor.State{Buffer: editor.NewBuffer("Intro\r\n\r\n- item")}
	caret := code.Buffer.Snapshot().Len()
	code.Select(caret, caret)
	if !continueSourceMarkdownLine(code, mygoui.InputEvent{Kind: mygoui.InputKeyDown, Key: mygoui.KeyEnter}) {
		t.Fatal("Enter did not continue list")
	}
	var saved strings.Builder
	if _, err := code.Buffer.WriteTo(&saved); err != nil {
		t.Fatal(err)
	}
	if got, want := saved.String(), "Intro\r\n\r\n- item\r\n- "; got != want {
		t.Fatalf("saved %q, want %q", got, want)
	}
}
