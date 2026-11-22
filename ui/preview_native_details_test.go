// spec: FEAT-WYS-CALLOUT / EV-MYGO-300. HTML disclosures in the native reader preserve Markdown content and copying.
package ui

import (
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestNativeMarkdownDetailsDisclosure(t *testing.T) {
	const source = "前言\n\n<details>\n<summary>练习与答案</summary>\n**答案**\n\n- [参考][ref]\n</details>\n后记\n\n[ref]: https://example.invalid/\n"
	tab := &workspaceTab{document: workspace.Document{Path: "练习.md"}, draft: editor.NewBuffer(source)}
	app := &workspaceWindow{workspaceTab: tab, preferences: &workspacePreferences{}}
	view := mygoui.NewTester(func(c *mygoui.Context) { app.drawMultiFileMarkdown(c, tab) }, 640, 430)
	assertFold := func(open bool) {
		t.Helper()
		if control, found := view.AccessByRole(mygoui.RoleDisclosure, "练习与答案"); !found || control.Expanded != open {
			t.Fatalf("expected details expanded=%v, got %+v", open, control)
		}
		if _, found := view.AccessByRole(mygoui.RoleText, "答案"); found != open {
			t.Fatalf("answer visibility does not match expanded=%v", open)
		}
		for _, surrounding := range []string{"前言", "后记"} {
			if _, found := view.AccessByRole(mygoui.RoleText, surrounding); !found {
				t.Fatalf("surrounding paragraph %q missing", surrounding)
			}
		}
	}
	assertFold(false)
	if err := view.Click("练习与答案"); err != nil {
		t.Fatal(err)
	}
	assertFold(true)
	if _, found := view.AccessByRole(mygoui.RoleLink, "参考"); !found {
		t.Fatal("expanded body lost its reference link to the outer document")
	}
	view.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyC)
	if got := view.Clipboard(); got != source {
		t.Fatalf("Copy as Markdown changed the disclosure source: %q", got)
	}
	if err := view.Click("练习与答案"); err != nil {
		t.Fatal(err)
	}
	assertFold(false)
}
