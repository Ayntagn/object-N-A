// spec: FEAT-001 / EV-MYGO-298
package ui

import (
	"strings"
	"testing"

	"cyberwriter/internal/core/markdownfence"
	"github.com/egoist/mygo/ui/editor"
)

func TestExternalCodeAppliesToTheOriginalFence(t *testing.T) {
	const block = "~~~go\nprintln(1)\n~~~"
	const between = "\n\n另一段\n\n"
	original := block + between + block + "\n"
	draft := editor.NewBuffer(original)
	fence, ok := markdownfence.Find(original, len(block)+len(between)+7)
	if !ok {
		t.Fatal("fixture fence was not found")
	}
	edit := externalCodeEdit{draft, draft.Snapshot(), fence}
	const prefix = "新正文\n\n"
	draft.Edit(editor.Edit{Text: prefix})
	if err := edit.apply("println(2)\n", nil); err != nil {
		t.Fatal(err)
	}
	if want := prefix + block + between + strings.Replace(block, "println(1)", "println(2)", 1) + "\n"; draft.String() != want {
		t.Fatalf("updated note = %q, want %q", draft.String(), want)
	}
	draft.Undo()
	if draft.String() != prefix+original {
		t.Fatalf("one undo left %q", draft.String())
	}
}

func TestExternalCodeRetainsConcurrentEditsToTheFence(t *testing.T) {
	const original = "正文\n\n```go\nprintln(1)\n```\n"
	draft := editor.NewBuffer(original)
	fence, ok := markdownfence.Find(original, strings.Index(original, "println"))
	if !ok {
		t.Fatal("fixture fence was not found")
	}
	edit := externalCodeEdit{draft, draft.Snapshot(), fence}
	local := strings.Replace(original, "println(1)", "println(3)", 1)
	draft.Set(local)
	if err := edit.apply("println(2)\n", nil); err == nil || draft.String() != local {
		t.Fatalf("external edit overwrote a concurrent code change: %q, %v", draft.String(), err)
	}
}
