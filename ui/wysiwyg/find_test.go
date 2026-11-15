// spec: EV-UI-200
package wysiwyg

import (
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func TestReplaceOverlappingFindRangesKeepsParagraphs(t *testing.T) {
	const source = "banana\n\nKeep this\n"
	draft := editor.NewBuffer(source)
	session := Open(draft)
	matches := FindRanges(session.Document.Text(), "ana")
	if len(matches) != 2 {
		t.Fatalf("find returned %v for overlapping matches", matches)
	}
	ReplaceFindRanges(session.Document, matches, "")
	if !session.Sync(draft) || draft.String() != "b\n\nKeep this\n" {
		t.Fatalf("replace all wrote %q", draft.String())
	}
	session.Document.Undo()
	if !session.Sync(draft) || draft.String() != source {
		t.Fatalf("undo wrote %q", draft.String())
	}
}
