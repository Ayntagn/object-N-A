// spec: EV-MYGO-293 / PDF Wikilink labels, page fragments, and source round trip.
package wysiwyg

import (
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func TestPDFWikilinkAliasSurvivesVisualEdit(t *testing.T) {
	const target = "资料/02 数制和编码 (7th Edition, z-lib.sk).pdf#page=22"
	const alias = "课件 ASCII 表"
	const token = "[[ " + target + " | " + alias + " ]]"
	const source = "参见 " + token + "。\n"
	draft := editor.NewBuffer(source)
	session := Open(draft)
	if session.Document == nil {
		t.Fatal("the aliased PDF link fell back to read-only preview")
	}
	text := session.Document.Text()
	if got := text.PlainText(); got != "参见 "+alias+"。" {
		t.Fatalf("visual text = %q", got)
	}
	address := text.AttributesAt(len([]rune("参见 "))).Link
	if value, ok := session.WikilinkTarget(address); !ok || value != target {
		t.Fatalf("PDF navigation target = %q (%v)", value, ok)
	}
	session.Document.InsertText("续写")
	if !session.Sync(draft) || draft.String() != "参见 "+token+"。续写\n" {
		t.Fatalf("edited Markdown = %q", draft.String())
	}
	session.Document.Undo()
	if !session.Sync(draft) || draft.String() != source {
		t.Fatalf("undo Markdown = %q", draft.String())
	}
}
