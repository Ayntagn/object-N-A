// spec: FEAT-001 / EV-UI-022, EV-UI-063, EV-MYGO-266
package ui

import (
	"os"
	"slices"
	"strings"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestMarkdownPreviewUsesChosenEditorFont(t *testing.T) {
	theme, _ := findPreviewTheme("night")
	for _, choice := range []struct {
		preference, native, css, bundled string
	}{
		{"Space Grotesk", "Space Grotesk, monospace", `"Space Grotesk", monospace`, `font-family: "Space Grotesk"`},
		{"", "monospace", "monospace", ""},
	} {
		preview := theme.withEditorFont(choice.preference)
		style := preview.markdownTheme(100, 0)
		if style.Font != choice.native || style.CodeFont != choice.native {
			t.Fatalf("native Markdown font = %q for body, %q for code, want %q", style.Font, style.CodeFont, choice.native)
		}
		css := preview.cssVariables()
		for _, variable := range []string{"--doc-body-font", "--doc-heading-font", "--doc-code-font"} {
			if !strings.Contains(css, variable+": "+choice.css+";") {
				t.Errorf("web Markdown %s did not use %q", variable, choice.css)
			}
		}
		if fonts := preview.fontFaces(); (choice.bundled == "" && fonts != "") ||
			(choice.bundled != "" && !strings.Contains(fonts, choice.bundled)) {
			t.Errorf("unexpected WebView font registration for %q", choice.preference)
		}
	}
}

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
