// spec: FEAT-001 / EV-MYGO-298

package wysiwyg

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func TestExternalCodeFenceAtFindsTheSecondIdenticalCard(t *testing.T) {
	const raw = "~~~go\npackage main\n~~~"
	const source = "---\ntitle: Demo\n---\n\n" + raw + "\n\n$1+1$\n\n" + raw + "\n"
	draft := editor.NewBuffer(source)
	s := Open(draft)
	if s.Document == nil {
		t.Fatal("fixture should have an editable rich-text projection")
	}
	text := s.Document.Text()
	var cards []int
	for offset := range text.Paragraphs() {
		if f, ok := FormulaAt(text, offset); ok && f.Block {
			cards = append(cards, offset)
		}
	}
	if len(cards) != 2 {
		t.Fatalf("expected two identical code cards, got %v", cards)
	}
	second, ok := s.ExternalCodeFenceAt(cards[1] + 1)
	if !ok || second.Start != strings.LastIndex(source, raw) || second.Source != raw || second.Code != "package main\n" {
		t.Fatalf("second card = %+v, %t", second, ok)
	}
	f, _ := FormulaAt(text, cards[1])
	const code = "package example\r\n\r\n"
	replacement, err := second.Replacement(code)
	if err != nil || !s.ApplyExternalCodeFence(second, replacement, code) {
		t.Fatalf("could not apply named code edit: %v", err)
	}
	next, ok := FormulaAt(s.Document.Text(), cards[1])
	want := source[:second.Start] + "~~~go\npackage example\n\n~~~" + source[second.End:]
	if !ok || next.ID != f.ID || next.Source != "package example\n" || draft.String() != want {
		t.Fatalf("card = %+v; draft = %q; want %q", next, draft.String(), want)
	}
	s.Document.Undo()
	if !s.Sync(draft) || draft.String() != source {
		t.Fatalf("undo = %q; want %q", draft.String(), source)
	}
	draft.Set("another note")
	if _, ok := s.ExternalCodeFenceAt(cards[1]); ok {
		t.Fatal("a stale visual session mapped a code fence")
	}
}

func TestExternalCodeFenceAtFindsAPlainMultilineCodeBlock(t *testing.T) {
	const bash = "```bash\nfirst\nsecond\n```"
	const plain = "```\nplain\n```"
	const source = "Prelude\n\n    legacy()\n\n" + bash + "\n\n" + plain + "\n"
	s := Open(editor.NewBuffer(source))
	if s.Document == nil {
		t.Fatal("fixture should have an editable rich-text projection")
	}
	text := s.Document.Text()
	var offsets []int
	for offset, paragraph := range text.Paragraphs() {
		if paragraph.Style.CodeBlock {
			offsets = append(offsets, offset)
		}
	}
	if len(offsets) != 4 {
		t.Fatalf("expected four code paragraphs, got %v", offsets)
	}
	if _, ok := s.ExternalCodeFenceAt(offsets[0]); ok {
		t.Fatal("indented code was identified as an editable fence")
	}
	for _, target := range []struct {
		offset int
		raw    string
	}{
		{offsets[2], bash},
		{offsets[3], plain},
	} {
		fence, ok := s.ExternalCodeFenceAt(target.offset)
		if !ok || fence.Start != strings.Index(source, target.raw) || fence.Source != target.raw {
			t.Fatalf("plain code = %+v, %t", fence, ok)
		}
	}
}
