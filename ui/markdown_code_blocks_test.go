// spec: FEAT-071. A language badge in Rendered copies the fenced code body.
package ui

import (
	"strings"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestMultiFileRenderedCodeBadgeCopiesFenceBody(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const goCode = "package main\n\n\t// 中文\n"
	const powershell = "$notes = Get-ChildItem -Path \"$HOME\"\n\t# 中文\n\n"
	const unknownCode = "  line one\n第二行\n\n"
	const source = "```Go example\n" + goCode + "```\n\n```pwsh\n" + powershell + "```\n\n~~~MadeUpLang\n" + unknownCode + "~~~\n"
	app, tester, _ := snippetsWorkspace(t, "Code.md", source, splitMode)
	if err := tester.Click("Multi-file View"); err != nil {
		t.Fatal(err)
	}
	tester.Key(mygoui.Cmd|mygoui.Alt, mygoui.Key4)
	if !tester.HasText(strings.TrimSuffix(powershell, "\n")) {
		t.Fatal("rendered PowerShell source is missing")
	}
	for _, fence := range []struct{ language, code string }{{"Go", goCode}, {"pwsh", powershell}, {"MadeUpLang", unknownCode}} {
		if err := tester.Click("Copy " + fence.language + " code"); err != nil {
			t.Fatal(err)
		}
		if got := tester.Clipboard(); got != fence.code {
			t.Fatalf("copy %s: got %q, want %q", fence.language, got, fence.code)
		}
	}
	tester.SetClipboard("keyboard check")
	tester.Key(0, mygoui.KeySpace)
	if got := tester.Clipboard(); got != unknownCode {
		t.Fatalf("keyboard copy: got %q, want %q", got, unknownCode)
	}
	if app.draft.Modified() || app.draft.String() != source {
		t.Fatalf("copy changed the document: %q", app.draft.String())
	}
}
