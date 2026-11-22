// spec: EV-MYGO-299
package markdowndetails

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestProjectDetailsPreservesSourceOffsets(t *testing.T) {
	const source = "before\r\n" +
		"  <DETAILS open=\"false\" class=\"fixture\"><summary>练习 &amp; <b>答案</b></summary>\r\n" +
		"**答案**\r\n<details>\r\n\r\n<summary>Nested</summary>\r\n" +
		"~~~html\r\n</details>\r\n~~~\r\n\r\n- Item\r\n</details>\r\n" +
		"after inner\r\n</details>\r\nafter outer\r\n"
	projected, blocks := Project(source, parser.GFM)
	if len(projected) != len(source) || len(blocks) != 2 {
		t.Fatalf("projection length = %d, blocks = %+v", len(projected), blocks)
	}
	for i := range len(source) {
		if (source[i] == '\r' || source[i] == '\n') && projected[i] != source[i] {
			t.Fatalf("line ending moved at byte %d", i)
		}
	}
	if outer := blocks[0]; outer.Summary != "练习 & 答案" || !outer.Open ||
		outer.Start != strings.Index(source, "<DETAILS") || outer.BodyStart != strings.Index(source, "**答案**") ||
		outer.BodyEnd != strings.LastIndex(source, "</details>") || outer.End != strings.Index(source, "after outer") {
		t.Fatalf("outer disclosure = %+v", outer)
	}
	if inner := blocks[1]; inner.Summary != "Nested" || inner.Open ||
		!strings.Contains(source[inner.BodyStart:inner.BodyEnd], "- Item") {
		t.Fatalf("nested disclosure = %+v", inner)
	}
	doc := parser.New(parser.GFM)
	doc.Set(projected)
	html := doc.HTML()
	for _, want := range []string{"<strong>答案</strong>", "<li>Item</li>", "&lt;/details&gt;", "<p>after inner</p>", "<p>after outer</p>"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in %s", want, html)
		}
	}
}

func TestProjectDetailsLeavesLiteralAndIncompleteMarkup(t *testing.T) {
	const literal = "~~~html\n<details>\n<summary>Literal</summary>\n**sample**\n</details>\n~~~\n\n" +
		"<!--\n<details>\n<summary>Comment</summary>\n</details>\n-->\n\n"
	const complete = "<details>\n<summary>Section</summary>\n\nFirst.\n\n</details>\n\n"
	const incomplete = "<details>\n<summary>Outer</summary>\n<details>\n<summary>Inner</summary>\nBody\n</details>"
	projected, blocks := Project(literal+complete+incomplete, parser.GFM)
	if len(blocks) != 1 || blocks[0].Start != len(literal) || !strings.HasPrefix(projected, literal) || !strings.HasSuffix(projected, incomplete) {
		t.Fatalf("unexpected projection %q, %+v", projected, blocks)
	}
}
