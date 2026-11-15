package wysiwyg

// spec: FEAT-DIAGRAM-INSERT / EV-UI-120 / EV-UI-121

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/richtext"
)

func TestDiagramVisualRoundTrip(t *testing.T) {
	const fence = "```mermaid\ngraph TD\n    A[First step] --> B[New finish]\n```"
	const original = "# Diagram Study\n\nTail paragraph.\n"
	draft := editor.NewBuffer(original)
	s := Open(draft)
	end := s.Document.Text().Len()
	s.Document.SetSelection(end, end)
	if !s.InsertMarkdown(fence) || !s.Sync(draft) {
		t.Fatal("visual Mermaid insertion failed")
	}
	if got := draft.String(); got != strings.TrimSuffix(original, "\n")+"\n\n"+fence+"\n\n" {
		t.Fatalf("source after insertion = %q", got)
	}
	s.Document.Undo()
	if !s.Sync(draft) || draft.String() != original {
		t.Fatalf("visual undo restored %q", draft.String())
	}
	s.Document.Redo()
	s.Sync(draft)
	s = Open(draft)
	if s.Document == nil {
		t.Fatal("could not reopen the visual diagram")
	}
	var found bool
	for offset := range s.Document.Text().Paragraphs() {
		if diagram, ok := s.DiagramAt(offset); ok {
			found = true
			if diagram.Source != "graph TD\n    A[First step] --> B[New finish]" {
				t.Fatalf("reopened source = %q", diagram.Source)
			}
			if !s.ChangeDiagram(offset, diagram, diagram.Source, "mermaid[bg:none]") || !s.Sync(draft) {
				t.Fatal("could not update the diagram background")
			}
			if !strings.Contains(draft.String(), "```mermaid[bg:none]\n"+diagram.Source) {
				t.Fatalf("background edit changed the source: %q", draft.String())
			}
			if s.ChangeDiagram(offset, diagram, "stale update", "mermaid") {
				t.Fatal("a stale card replaced the updated diagram")
			}
			break
		}
	}
	if !found {
		t.Fatal("no visual diagram paragraph")
	}
}

func TestDiagramFenceSurvivesJoinedParagraphs(t *testing.T) {
	const fence = "```mermaid\ngraph TD\n    A --> B\n```"
	const original = "Before\n\n" + fence + "\n\nAfter\n"
	draft := editor.NewBuffer(original)
	s := Open(draft)
	if s.Document == nil {
		t.Fatal("could not project diagram")
	}
	offset := len("Before\n")
	if _, ok := s.DiagramAt(offset); !ok {
		t.Fatal("no diagram after the first paragraph")
	}
	s.Document.Edit(func(edit *richtext.Edit) {
		edit.ReplaceString(offset+1, offset+2, "", richtext.Attributes{})
		edit.ReplaceString(offset-1, offset, "", richtext.Attributes{})
	})
	s.Sync(draft)
	if draft.String() != original {
		t.Fatalf("joining the surrounding paragraphs damaged the fence: %q", draft.String())
	}
}
