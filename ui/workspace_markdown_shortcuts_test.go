// spec: FEAT-001 / EV-MYGO-133 and the batch Snippets editor request
package ui

import (
	"slices"
	"testing"

	"cyberwriter/internal/core/mdshortcuts"
	mygoui "github.com/egoist/mygo/ui"
)

func TestMarkdownShortcutsSettingsEditsAndSavesSnippets(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	legacy := mdshortcuts.Rule{
		Trigger: ";legacy", Replacement: "Legacy ${0:entry}", Scope: mdshortcuts.Text,
		Automatic: false, Enabled: true,
	}
	prefs := workspacePreferences{MarkdownShortcuts: mdshortcuts.Config{Rules: []mdshortcuts.Rule{legacy}}}
	app := &workspaceWindow{preferences: &prefs, settingsTab: "Shortcuts"}
	settings := mygoui.NewTester(app.appearanceSettings, 880, 892)
	legacyRules, err := mdshortcuts.ParseSnippets(snippetsSettingsField(t, settings).Value)
	if err != nil || !slices.Equal(legacyRules, []mdshortcuts.Rule{legacy}) {
		t.Fatalf("legacy snippets = %+v, error %v", legacyRules, err)
	}

	clickSnippetsSetting(t, settings, "Enable Markdown Shortcuts")
	const source = "[\n  // Keep this group with the user's notes.\n" +
		"  {\"trigger\":\";sig\",\"replacement\":\"**Signed** $0\",\"enabled\":true},\n" +
		"  {\"trigger\":\";later\",\"replacement\":\"Later\",\"enabled\":false}\n]\n"
	fillSnippetsSettings(t, settings, source)
	clickSnippetsSetting(t, settings, "General")
	clickSnippetsSetting(t, settings, "Shortcuts")
	if got := snippetsSettingsField(t, settings).Value; got != source {
		t.Fatalf("draft after changing Settings pages = %q", got)
	}
	clickSnippetsSetting(t, settings, "Save Snippets")
	loaded, err := loadWorkspacePreferences()
	if err != nil || !loaded.MarkdownShortcuts.Enabled || loaded.MarkdownShortcuts.SnippetSource != source ||
		len(loaded.MarkdownShortcuts.Rules) != 2 || loaded.MarkdownShortcuts.Rules[1].Enabled {
		t.Fatalf("saved preferences = %+v, %v", loaded.MarkdownShortcuts, err)
	}
	const invalid = "[\n  {\"trigger\": \";broken\", \"replacement\": }\n]\n"
	fillSnippetsSettings(t, settings, invalid)
	clickSnippetsSetting(t, settings, "Save Snippets")
	broken := snippetsSettingsField(t, settings)
	if broken.Value != invalid || !broken.Focused || !settings.HasText("line 2, column") {
		t.Fatalf("invalid snippet draft = %q, focused = %t, labels = %v", broken.Value, broken.Focused, settings.Texts())
	}
	unchanged, err := loadWorkspacePreferences()
	if err != nil || unchanged.MarkdownShortcuts.SnippetSource != source ||
		!slices.Equal(unchanged.MarkdownShortcuts.Rules, loaded.MarkdownShortcuts.Rules) {
		t.Fatalf("invalid input reached the active settings: %+v, %v", unchanged.MarkdownShortcuts, err)
	}
	note, editor, _ := snippetsWorkspace(t, "Saved snippets.md", "", sourceMode)
	note.preferences.MarkdownShortcuts = app.preferences.MarkdownShortcuts
	editor.Type(";sig")
	editor.Key(0, mygoui.KeyTab)
	if got := note.draft.String(); got != "**Signed** " {
		t.Fatalf("compiled rules after invalid Settings edit expanded to %q", got)
	}
}

func TestMarkdownShortcutsSettingsRejectsStaleDraft(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	prefs := &workspacePreferences{}
	first := mygoui.NewTester((&workspaceWindow{preferences: prefs}).drawMarkdownShortcutsSettings, 660, 620)
	second := mygoui.NewTester((&workspaceWindow{preferences: prefs}).drawMarkdownShortcutsSettings, 660, 620)
	const saved = "[{\"trigger\":\";first\",\"replacement\":\"Winning edit\"}]\n"
	const draft = "[{\"trigger\":\";second\",\"replacement\":\"Other edit\"}]\n"
	fillSnippetsSettings(t, second, draft)
	fillSnippetsSettings(t, first, saved)
	clickSnippetsSetting(t, first, "Save Snippets")
	second.Frame()
	clickSnippetsSetting(t, second, "Save Snippets")
	if prefs.MarkdownShortcuts.SnippetSource != saved || snippetsSettingsField(t, second).Value != draft ||
		!second.HasText("another Settings window") {
		t.Fatal("stale save overwrote the shared snippets or the local draft")
	}
	clickSnippetsSetting(t, second, "Load Saved Snippets")
	if got := snippetsSettingsField(t, second).Value; got != saved {
		t.Fatalf("reloaded snippets = %q", got)
	}
}

func snippetsSettingsField(t *testing.T, test *mygoui.Tester) mygoui.AccessInfo {
	t.Helper()
	field, ok := test.AccessByRole(mygoui.RoleTextField, "Snippets")
	if !ok {
		t.Fatal("the Snippets editor is missing")
	}
	return field
}

func fillSnippetsSettings(t *testing.T, test *mygoui.Tester, value string) {
	t.Helper()
	field := snippetsSettingsField(t, test)
	test.ClickAt(field.Bounds.X+10, field.Bounds.Y+field.Bounds.H/2)
	test.Key(mygoui.Cmd, mygoui.KeyA)
	test.Type(value)
}

func clickSnippetsSetting(t *testing.T, test *mygoui.Tester, name string) {
	t.Helper()
	if err := test.Click(name); err != nil {
		t.Fatal(err)
	}
}

func TestMarkdownShortcutsThroughSourceInput(t *testing.T) {
	app, tester, _ := snippetsWorkspace(t, "Shortcuts.md", "", sourceMode)
	app.preferences.MarkdownShortcuts = mdshortcuts.Config{Enabled: true, Rules: []mdshortcuts.Rule{
		{Trigger: ";item", Replacement: `# ${0:Title} / ${1:Description}`, Enabled: true},
		{Trigger: "ff", Replacement: `\frac{$0}{$1}`, Scope: mdshortcuts.Math, Enabled: true, Automatic: true},
	}}
	tester.Frame()
	tester.Type(";item")
	if got := app.draft.String(); got != ";item" {
		t.Fatalf("a manual shortcut expanded during typing: %q", got)
	}
	tester.Key(0, mygoui.KeyTab)
	selection := app.code.Selection()
	if got := app.draft.Slice(selection.Start(), selection.End()); got != "Title" {
		t.Fatalf("first tab stop = %q from %q", got, app.draft.String())
	}
	tester.Type("Example")
	tester.Key(0, mygoui.KeyTab)
	selection = app.code.Selection()
	if got := app.draft.Slice(selection.Start(), selection.End()); got != "Description" {
		t.Fatalf("second tab stop = %q, draft %q", got, app.draft.String())
	}
	tester.Type("Detail")
	tester.Key(0, mygoui.KeyTab)
	tester.Key(0, mygoui.KeyEnter)
	for _, key := range []string{"$", "f", "f"} {
		tester.Type(key)
	}
	if got := app.draft.String(); got != "# Example / Detail\n$\\frac{}{}" {
		t.Fatalf("auto math and text shortcut produced %q", got)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != "# Example / Detail\n$f" {
		t.Fatalf("Undo did not remove just the math expansion: %q", got)
	}
	tester.Type("f")
	if got := app.draft.String(); got != "# Example / Detail\n$\\frac{}{}" {
		t.Fatalf("math shortcut did not expand after undo: %q", got)
	}
}
