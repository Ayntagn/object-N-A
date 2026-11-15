// spec: EV-UI-247 / EV-UI-263, continuous Source spelling checks.
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
