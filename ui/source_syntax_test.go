// spec: FEAT-001 / EV-UI-058
package ui

import (
	"os"
	"slices"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestSourceSyntaxToolbarAndSharedPreference(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const saved = "# Source syntax\n\n**Strong**, *emphasis*, `inline`, and [link](note.md).\n\n```go\n// A short comment.\nfunc greet() string { return \"hello\" }\n```\n"
	app, tester, filename := snippetsWorkspace(t, "Syntax.md", saved, splitMode)
	button, found := tester.Access("Highlight")
	if !found || button.Disabled || !button.Checked {
		t.Fatalf("unexpected initial Highlight control: %+v", button)
	}
	if theme := app.code.Theme; theme.Background != mygoui.Hex("#000000") || theme.TokenColor(editor.TokenHeading) != mygoui.Hex("#ffaaf7") {
		t.Fatal("System did not resolve the Blackout source palette")
	}
	preview, found := tester.Access("Markdown Preview")
	if !found {
		t.Fatal("split preview is missing")
	}
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	selection := app.code.Selection()
	language := app.draft.Language()
	click := func(on *mygoui.Tester, name string) {
		t.Helper()
		if err := on.Click(name); err != nil {
			t.Fatal(err)
		}
	}
	pick := func(on *mygoui.Tester, name string) {
		t.Helper()
		if err := on.ChooseMenuItem(name); err != nil {
			t.Fatal(err)
		}
	}
	click(tester, "Editor Theme")
	choices := tester.Menu()
	if len(choices) != 52 || !slices.Equal(choices[:5], []string{"System", "-", "Dark", "Nabu Dark", "Blackout"}) ||
		!slices.Equal(choices[35:38], []string{"Xcode Dark", "Light", "Nabu Light"}) || choices[len(choices)-1] != "StackOverflow Light" {
		t.Fatalf("unexpected source theme menu: %q", choices)
	}
	pick(tester, "Dracula")
	if app.preferences.EditorTheme != "dracula" || app.code.Theme.Background != mygoui.Hex("#282936") ||
		app.code.Theme.TokenColor(editor.TokenComment) != mygoui.Hex("#626483") {
		t.Fatal("Dracula did not apply its source and comment colors")
	}
	click(tester, "Highlight")
	plain := app.draft.Language()
	button, _ = tester.Access("Highlight")
	if plain.Tokenizer != nil || plain.Highlighter != nil || plain.BlockComment != language.BlockComment ||
		button.Checked || app.sourceHighlightMenuLabel() != "Enable Syntax Highlighting" || app.code.Theme.Background != mygoui.Hex("#282936") {
		t.Fatal("disabling syntax changed the editing language or the selected canvas")
	}
	settings := mygoui.NewTester(app.generalSettings, 800, 600)
	click(settings, "Color Theme")
	pick(settings, "Nabu Light")
	tester.Frame()
	if app.code.Theme.Background != mygoui.Hex("#fdfbf5") || app.code.Theme.TokenColor(editor.TokenHeading) != mygoui.Hex("#9c7b16") ||
		app.draft.Language().Tokenizer != nil {
		t.Fatal("General did not update the same palette while highlighting was disabled")
	}
	preferences, err := loadWorkspacePreferences()
	if err != nil || preferences.EditorTheme != "nabu-light" || !preferences.DisableSyntaxHighlighting {
		t.Fatalf("persisted syntax preferences: %q %v %v", preferences.EditorTheme, preferences.DisableSyntaxHighlighting, err)
	}
	*app.preferences = preferences
	click(tester, "Highlight")
	if app.draft.Language() != language || app.code.Selection() != selection {
		t.Fatal("enabling syntax did not restore the tokenizer or preserve the selection")
	}
	if current, ok := tester.Access("Markdown Preview"); !ok || current.Bounds != preview.Bounds || !tester.HasText("Source syntax") {
		t.Fatal("a source theme changed the Markdown preview")
	}
	if disk, err := os.ReadFile(filename); err != nil || string(disk) != saved || app.draft.String() != saved || app.draft.Modified() {
		t.Fatalf("syntax preference changed the text or save state: %q, %v", disk, err)
	}
}

func TestSourceSyntaxMissingThemeRetainsLastCSS(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, tester, _ := snippetsWorkspace(t, "Syntax.md", "# Source syntax\n", sourceMode)
	for _, theme := range []string{"Nord", "Odsibian"} {
		if err := tester.Click("Editor Theme"); err != nil {
			t.Fatal(err)
		}
		if err := tester.ChooseMenuItem(theme); err != nil {
			t.Fatal(err)
		}
	}
	if app.preferences.EditorTheme != "odsibian" || app.code.Theme.Background != mygoui.Hex("#2e3440") {
		t.Fatal("the missing Odsibian CSS discarded the last successfully applied stylesheet")
	}
	reopened, fresh, _ := snippetsWorkspace(t, "Syntax.md", "# Source syntax\n", sourceMode)
	preferences, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	*reopened.preferences = preferences
	reopened.setTabDocument(reopened.workspaceTab, reopened.document)
	fresh.Frame()
	if reopened.code.Theme.Background != mygoui.Hex("#181914") {
		t.Fatal("the missing first stylesheet did not use Highlightr's initial Pojoaque palette")
	}
}
