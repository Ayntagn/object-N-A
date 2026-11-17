// spec: FEAT-WYS-CALLOUT / EV-UI-154 / 用户反馈的显式引用行合并。
package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"cyberwriter/ui/wysiwyg"
)

func TestVisualQuoteLinesKeepTheirSourceBoundaries(t *testing.T) {
	const initial = "普通段落\n仍是同一段\n\n> **第一行\n> 第二行**\n> 第三行\n\n> [!WORK-PLAN]- 排期\n> 步骤一\n仍在这一行\n> 步骤二\n"
	const visible = "普通段落 仍是同一段\n第一行\n第二行\n第三行\n\ufffc排期\n步骤一 仍在这一行\n步骤二"
	app, tester, _ := snippetsWorkspace(t, "Quote lines.md", initial, wysiwygMode)
	document := app.wys.session.Document
	if document == nil {
		t.Fatal("quoted lines made the document read-only")
	}
	if got := document.Text().String(); got != visible {
		t.Fatalf("visual lines = %q, want %q", got, visible)
	}
	for _, change := range [][2]string{{"第二行", "第二行修改"}, {"步骤二", "第二个步骤"}} {
		current := document.Text().String()
		before, _, found := strings.Cut(current, change[0])
		if !found {
			t.Fatalf("missing %q in %q", change[0], current)
		}
		at := utf8.RuneCountInString(before)
		document.SetSelection(at, at+utf8.RuneCountInString(change[0]))
		tester.Frame()
		tester.Type(change[1])
	}
	const want = "普通段落 仍是同一段\n\n> **第一行\n> 第二行修改**\n> 第三行\n\n> [!WORK-PLAN]- 排期\n> 步骤一 仍在这一行\n> 第二个步骤\n"
	if got := app.draft.String(); got != want {
		t.Fatalf("written Markdown = %q, want %q", got, want)
	}
	reopened := wysiwyg.Open(app.draft)
	if reopened.Document == nil || reopened.Document.Text().String() != document.Text().String() {
		t.Fatal("reopening changed the quoted lines")
	}
}
