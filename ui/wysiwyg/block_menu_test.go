// spec: EV-UI-147
package wysiwyg

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/richtext"
)

func TestBlockMenuEmptyBlockRoundTrip(t *testing.T) {
	const before, after = "Before table.\n\n", "\n\nAfter table.\n"
	for _, tc := range []struct {
		query, want string
		style       richtext.ParagraphStyle
	}{
		{"/text", "<br />", richtext.ParagraphStyle{}},
		{"/heading 1", "#", richtext.ParagraphStyle{Heading: 1}},
		{"/quote", "> <br />", richtext.ParagraphStyle{Quote: true}},
		{"/bullet", "* <br />", richtext.ParagraphStyle{List: richtext.Bullets}},
		{"/ordered", "1. <br />", richtext.ParagraphStyle{List: richtext.Numbers}},
		{"/task", "* [ ] <br />", richtext.ParagraphStyle{List: richtext.Tasks}},
		{"/code", "\x60\x60\x60\n\x60\x60\x60", richtext.ParagraphStyle{CodeBlock: true}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			draft := editor.NewBuffer(before + tc.query + after)
			session := Open(draft)
			start := len(before) - 1 // A single separator in RichText.
			session.Document.SetSelection(start, start+len(tc.query))
			if !session.FormatBlockMenuSelection(tc.style) || !session.Sync(draft) || draft.String() != before+tc.want+after {
				t.Fatalf("command wrote %q", draft.String())
			}
			if a, z := session.Document.Selection(); a != start || z != start {
				t.Fatalf("new block selection = %d–%d", a, z)
			}
			reopened := Open(draft)
			if reopened.Document == nil {
				t.Fatal("saved block did not reopen for editing")
			}
			style := reopened.Document.Text().ParagraphStyleAt(start)
			style.ID = ""
			if style != tc.style {
				t.Fatalf("reopened style = %+v, want %+v", style, tc.style)
			}
		})
	}
}

func TestBlockMenuDividerAtEnd(t *testing.T) {
	const before = "After table.\n\n"
	draft := editor.NewBuffer(before + "/divider\n")
	session := Open(draft)
	start := len(before) - 1
	session.Document.SetSelection(start, start+len("/divider"))
	if !session.InsertDivider() || !session.Sync(draft) || draft.String() != before+"***\n\n" {
		t.Fatalf("EOF divider wrote %q", draft.String())
	}
	if a, z := session.Document.Selection(); a != start || z != start+1 {
		t.Fatalf("EOF divider selection = %d–%d", a, z)
	}
	reopened := Open(draft)
	if reopened.Document == nil || !reopened.IsDivider(reopened.Document.Text(), start) {
		t.Fatal("saved divider did not reopen for editing")
	}
	end := reopened.Document.Text().Len()
	reopened.Document.SetSelection(end, end)
	reopened.Document.InsertText("New paragraph")
	if !reopened.Sync(draft) || !strings.HasPrefix(draft.String(), before+"***\n\nNew paragraph") {
		t.Fatalf("typing after reopened divider wrote %q", draft.String())
	}
}
