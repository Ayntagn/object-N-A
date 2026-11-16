// spec: FEAT-001 / EV-UI-247 / EV-UI-263 / EV-MYGO-257, continuous Source spelling checks.
package ui

import (
	"strings"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestSourceSpellingUsesByteRanges(t *testing.T) {
	const text = "😀 A mispelled note and anothr word."
	second := strings.Index(text, "anothr")
	ranges := []sourceSpellingRange{
		{Location: 1, Length: 1}, // Inside the emoji's UTF-16 surrogate pair.
		{Location: 5, Length: 9},
		{Location: uint(mygoui.UTF16Len(text[:second])), Length: 6},
		{Location: ^uint(0), Length: 1},
	}
	diagnostics := sourceSpellingDiagnostics(text, ranges)
	if len(diagnostics) != 2 || diagnostics[0].Start != 7 || diagnostics[0].End != 16 ||
		diagnostics[1].Start != second || diagnostics[1].End != second+6 {
		t.Fatalf("spelling diagnostics = %+v", diagnostics)
	}
}

func TestSourceSpellingExcludesMarkdownCode(t *testing.T) {
	const text = "\ufeff---\r\ntitle: `metadata`\r\n---\r\n" +
		"😀 mispelled `qzxblorf`.\r\n\r\n" +
		"> ~~~c\r\n> #include <stdio.h>\r\n> int qxz /* 😀 中 */\r\n> ~~~\r\n\r\n" +
		"    indentedword\r\n\r\n中文 anothr.\r\n\r\n```unknown\r\nstillopen"
	input := sourceSpellingInput(text)
	for _, code := range []string{"qzxblorf", "#include <stdio.h>", "qxz", "indentedword", "stillopen"} {
		if strings.Contains(input, code) {
			t.Errorf("spelling input included code %q", code)
		}
	}
	for _, prose := range []string{"title: `metadata`", "😀 mispelled", "中文 anothr."} {
		if !strings.Contains(input, prose) {
			t.Errorf("spelling input lost %q", prose)
		}
	}
	if mygoui.UTF16Len(input) != mygoui.UTF16Len(text) || strings.Count(input, "\r\n") != strings.Count(text, "\r\n") {
		t.Fatal("the spelling input changed the source offsets or line endings")
	}
	offset := strings.Index(input, "anothr")
	if offset < 0 {
		t.Fatal("the spelling input lost the word after the code blocks")
	}
	ranges := []sourceSpellingRange{{Location: uint(mygoui.UTF16Len(input[:offset])), Length: 6}}
	diagnostics := sourceSpellingDiagnostics(text, ranges)
	start := strings.Index(text, "anothr")
	if len(diagnostics) != 1 || diagnostics[0].Start != start || diagnostics[0].End != start+6 {
		t.Fatalf("spelling diagnostics after code = %+v, want byte range %d:%d", diagnostics, start, start+6)
	}
}

func TestSourceSpellingExcludesMarkdownMath(t *testing.T) {
	const text = "😀 中文 mispelled before.\r\n\r\n# Heading\r\n$$\r\n\\begin{aligned}\r\n" +
		"x &= \\mathrm{m}\\cdot y \\Longrightarrow \\text{😀 中}\r\n\\end{aligned}\r\n$$\r\n\r\n" +
		"Display $$\\sqrtt{x}$$ in prose.\r\n\r\n> \\[\r\n> \\arccot{x}\r\n> \\]\r\n\r\n" +
		"中文 anothr after.\r\n\r\n$$\r\n\\approx{y}"
	input := sourceSpellingInput(text)
	for _, cmd := range []string{`\begin`, `\mathrm`, `\cdot`, `\Longrightarrow`, `\sqrtt`, `\arccot`, `\approx`} {
		if strings.Contains(input, cmd) {
			t.Errorf("spelling input included formula command %q", cmd)
		}
	}
	start := strings.Index(input, "anothr")
	if start < 0 || !strings.Contains(input, "mispelled before.") || !strings.Contains(input, "in prose.") ||
		mygoui.UTF16Len(input[:start]) != mygoui.UTF16Len(text[:strings.Index(text, "anothr")]) ||
		mygoui.UTF16Len(input) != mygoui.UTF16Len(text) || strings.Count(input, "\r\n") != strings.Count(text, "\r\n") {
		t.Fatal("excluding formulas changed prose, source offsets, or line endings")
	}
}

func TestSourceSpellingDropsResultsFromOldEditsAndDisabledChecks(t *testing.T) {
	if !sourceSpellAvailable {
		t.Skip("this platform has no spelling provider")
	}
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	preferences, err := loadWorkspacePreferences()
	if err != nil || !preferences.SpellCheckEnabled {
		t.Fatalf("loaded Spell Check = %t, error %v", preferences.SpellCheckEnabled, err)
	}
	const text = "😀 A mispelled word."
	draft := editor.NewBuffer(text)
	code := &editor.State{Buffer: draft}
	tab := &workspaceTab{draft: draft, code: code}
	app := &workspaceWindow{
		workspaceTab: tab, tabs: []*workspaceTab{tab}, preferences: &preferences,
	}
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		app.applySourceSpellCheck(c, app.workspaceTab)
		editor.View(c, code).Fill()
	}, 330, 240)
	tester.Advance(sourceSpellPause)
	first := tab.sourceSpell
	result := sourceSpellingDiagnostics(text, []sourceSpellingRange{{Location: 5, Length: 9}})
	app.publishSourceSpelling(tab, first, result)
	if len(code.Diagnostics()) != 1 {
		t.Fatal("the current spelling result was not applied")
	}

	tester.ClickAt(300, 14)
	tester.Type(" Additional text.")
	app.publishSourceSpelling(tab, first, result)
	if draft.Version() == first.version || len(code.Diagnostics()) != 0 {
		t.Fatal("the older check marked the newer draft")
	}

	tester.Advance(sourceSpellPause)
	second := tab.sourceSpell
	app.publishSourceSpelling(tab, second, result)
	settings := mygoui.NewTester(app.sourceSpellSettings, 330, 60)
	if err := settings.Click("Spell Check"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	app.publishSourceSpelling(tab, second, result)
	if len(code.Diagnostics()) != 0 || tab.sourceSpell != nil {
		t.Fatal("disabling Spell Check left diagnostics or accepted a late result")
	}
	saved, err := loadWorkspacePreferences()
	if err != nil || saved.SpellCheckEnabled {
		t.Fatalf("saved Spell Check = %t, error %v", saved.SpellCheckEnabled, err)
	}
}
