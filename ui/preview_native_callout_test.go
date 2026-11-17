// spec: FEAT-WYS-CALLOUT / EV-MYGO-272, EV-MYGO-273, EV-MYGO-274.
package ui

import (
	"testing"

	"cyberwriter/internal/core/callout"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestNativeMarkdownCalloutAndQuoteLines(t *testing.T) {
	const source = "> [!research]-\n> 第一条\n> 第二条\n\n> 上一行\n> 下一行\n\n普通\n段落\n"
	app := &workspaceWindow{preferences: &workspacePreferences{CalloutStyles: []callout.Style{
		{Kind: "research", Name: "研究记录", Icon: "🔬", Color: "#3478f6"},
	}}}
	options := app.withNativeMarkdownCallouts(&markdown.Options{Extensions: parser.GFM})
	view := mygoui.NewTester(func(c *mygoui.Context) { markdown.View(c, source, options) }, 640, 430)
	if _, ok := view.AccessByRole(mygoui.RoleDisclosure, "🔬 研究记录"); !ok {
		t.Fatal("custom callout title and disclosure missing")
	}
	if _, ok := view.AccessByRole(mygoui.RoleText, "第一条\n第二条"); ok {
		t.Fatal("collapsed callout still displays its body")
	}
	if err := view.Click("🔬 研究记录"); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"第一条\n第二条", "上一行\n下一行", "普通 段落"} {
		if _, ok := view.AccessByRole(mygoui.RoleText, text); !ok {
			t.Fatalf("rendered text %q missing", text)
		}
	}
}
