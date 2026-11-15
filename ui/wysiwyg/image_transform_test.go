// spec: FEAT-WYS-IMAGE / EV-UI-156. Reset must override an existing numeric
// alt ratio without changing the original Markdown token.
package wysiwyg

import (
	"testing"

	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestImageTransformResetOverridesLegacyRatio(t *testing.T) {
	const token = `![1.25](portrait.png "Old caption")`
	draft := editor.NewBuffer(token + "\n")
	s := Open(draft)
	if s.Document == nil {
		t.Fatal("legacy image lost visual editing")
	}
	picture, ok := ImageAt(s.Document.Text(), 0)
	if !ok || picture.Ratio != 1.25 {
		t.Fatalf("legacy image = %+v", picture)
	}
	if _, ok := s.TransformImage(0, picture, ImageTransform{Ratio: 1}); !ok || !s.Sync(draft) {
		t.Fatal("could not reset the legacy ratio")
	}
	const want = token + " <!-- object-na-image: scale=1.00 rotate=0 -->\n"
	if got := draft.String(); got != want {
		t.Fatalf("reset left %q, want %q", got, want)
	}
	reopened := Open(draft)
	if reopened.Document == nil {
		t.Fatal("saved reset lost visual editing")
	}
	picture, ok = ImageAt(reopened.Document.Text(), 0)
	if !ok || picture.Ratio != 1 || picture.Alt != "1.25" || picture.Caption != "Old caption" {
		t.Fatalf("reopened image = %+v", picture)
	}
}

func TestImageTransformIgnoresReferencesWithLinkPunctuationInAlt(t *testing.T) {
	const source = `![注 \]( 保留][pic] <!-- object-na-image: scale=1.00 rotate=90 -->` + "\n\n[pic]: portrait.png\n"
	doc := parser.New(Extensions)
	doc.Set(source)
	in := doc.Inlines(1)
	if len(in) == 0 || in[0].Kind != parser.Image || in[0].Dest != "portrait.png" {
		t.Fatalf("fixture did not parse as a reference image: %+v", in)
	}
	if _, _, ok := ReadBlockImageTransform(doc.Source(), in); ok {
		t.Fatal("reference image consumed a transform comment")
	}
}
