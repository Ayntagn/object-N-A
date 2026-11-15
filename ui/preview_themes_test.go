// spec: FEAT-001 / EV-UI-022, EV-UI-063
package ui

import (
	"os"
	"slices"
	"strings"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestPreviewThemeAndAppAppearanceAreIndependent(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const note = "# Theme study\n\n**Original text.**\n"
	app, tester, filename := snippetsWorkspace(t, "ThemeStudy.md", note, splitMode)
	choose := func(control, choice string) {
		t.Helper()
		if err := tester.Click(control); err != nil {
			t.Fatal(err)
		}
		if err := tester.ChooseMenuItem(choice); err != nil {
			t.Fatal(err)
		}
	}
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	selection := app.code.Selection()
	if err := tester.Click("Preview Theme"); err != nil {
		t.Fatal(err)
	}
	want := []string{"System", "Synthetic Editorial", "Blackout", "Night", "PLATO", "Academic", "Apropos", "Sepia", "-", "Custom Theme..."}
	if got := tester.Menu(); !slices.Equal(got, want) {
		t.Fatalf("preview theme menu = %q", got)
	}
	if err := tester.ChooseMenuItem("Night"); err != nil {
		t.Fatal(err)
	}
	if !app.blackout || app.code.Selection() != selection {
		t.Fatal("the document theme changed the app appearance or the source selection")
	}
	choose("Appearance", "System Light")
	if app.blackout {
		t.Fatal("the appearance menu did not select System Light")
	}
	document := app.exportDocument(newExportPageForm(app.preferences.Export).layout)
	markup, err := renderSharedHTML(document, app.blackout)
	if err != nil || !strings.Contains(markup, `data-preview-theme="night"`) {
		t.Fatalf("standalone HTML did not retain the selected Night theme: %v", err)
	}
	settingsFile, err := preferenceFile()
	if err != nil {
		t.Fatal(err)
	}
	if saved, err := os.ReadFile(settingsFile); err != nil || !strings.Contains(string(saved), `"selectedThemeId":"night"`) ||
		!strings.Contains(string(saved), `"appAppearance":"systemLight"`) {
		t.Fatalf("theme preferences did not persist independently: %v", err)
	}
	if disk, err := os.ReadFile(filename); err != nil || string(disk) != note || app.draft.String() != note || app.draft.Modified() {
		t.Fatalf("theme actions changed the note: %q, %v", disk, err)
	}
}

func TestAppearanceKeepsExistingBlackoutPreference(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	file, err := preferenceFile()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(`{"appearance":"dark","blackout":true,"editorThemeRawValue":"nord"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	prefs, err := loadWorkspacePreferences()
	if err != nil || prefs.Appearance != appearanceBlackout || prefs.PreviewThemeID != systemPreviewThemeID || prefs.EditorTheme != "nord" {
		t.Fatalf("loaded appearance=%s preview=%s editor=%s: %v", prefs.Appearance, prefs.PreviewThemeID, prefs.EditorTheme, err)
	}
}
