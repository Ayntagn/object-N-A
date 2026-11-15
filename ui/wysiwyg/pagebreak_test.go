// spec: EV-UI-114 / EV-EDITOR-004
package wysiwyg

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func TestVisualPageBreakSplitsSelectionAndRoundTrips(t *testing.T) {
	const original = "# Page Break Probe\n\nBefore marker.\n\nAfter marker.\n"
	const inserted = "# Page Break Probe\n\nBe\n\n\\[pagebreak]\n\nre marker.\n\nAfter marker.\n"
	draft := editor.NewBuffer(original)
	session := Open(draft)
	start := strings.Index(session.Document.Text().PlainText(), "Before") + 2
	session.Document.SetSelection(start, start+2)
	session.InsertPageBreak()
	if !session.Sync(draft) || draft.String() != inserted {
		t.Fatalf("inserted source = %q", draft.String())
	}
	reopened := Open(draft)
	if reopened.Document == nil {
		t.Fatal("reopening the page break disabled visual editing")
	}
	if visual := reopened.Document.Text().Markdown(); !strings.Contains(visual, "![PAGE BREAK](cyberwriter-page-break:") || reopened.Sync(draft) {
		t.Fatalf("reopened visual Markdown = %q", visual)
	}
	session.Document.Undo()
	if !session.Sync(draft) || draft.String() != original {
		t.Fatalf("undo source = %q", draft.String())
	}
	if anchor, caret := session.Document.Selection(); anchor != start || caret != start+2 {
		t.Fatalf("undo selection = %d:%d", anchor, caret)
	}
}
