// spec: FEAT-001 / EV-MYGO-299
package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestPreviewDetailsRendersMarkdownBody(t *testing.T) {
	source := "# 课堂笔记\n\n<details>\n<summary>练习与答案</summary>\n\n" +
		"**答案**和[参考][ref]。\n\n</details>\n接着写。\n\n" +
		"<details open><summary>展开 &amp; 复习</summary>\n- **第一项**\n</details>\n下一段。\n\n" +
		"[ref]: https://example.invalid/\n"
	body, title := renderPreviewDocument(parser.New(webPreviewExtensions), source, "", nil, "", false, true, nil)
	for _, want := range []string{
		`<details class="markdown-details" data-source-line="3"><summary>练习与答案</summary>`,
		`<p data-source-line="6"><strong>答案</strong>和<a href="https://example.invalid/">参考</a>。</p>`,
		`</details><p data-source-line="9">接着写。</p>`,
		`open><summary>展开 &amp; 复习</summary><ul`,
		`<strong>第一项</strong>`,
		`</details><p data-source-line="14">下一段。</p>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in %s", want, body)
		}
	}
	if title != "课堂笔记" || strings.Contains(body, "raw-markup") {
		t.Fatalf("wrong title or unparsed markup: %q, %s", title, body)
	}
}

func TestPDFDetailsShowsAnswers(t *testing.T) {
	_, body := renderPDFHTML(exportDocument{
		source: "<details>\n<summary>练习与答案</summary>\n**答案**\n</details>\n",
		kind:   documentMarkdown,
	})
	if strings.Contains(body, "<details") || !strings.Contains(body, "练习与答案") ||
		!strings.Contains(body, "<strong>答案</strong>") {
		t.Fatalf("print output should show the title and answer: %s", body)
	}
}
