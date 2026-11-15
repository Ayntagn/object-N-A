// spec: EV-UI-247 / EV-UI-248
package ui

import (
	"math"
	"slices"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestSourceLineSpacingMenuPersistsAndUpdatesTheEditor(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const original = "# Heading\n\nA paragraph.\n\nAnother line.\n"
	app, editor, _ := snippetsWorkspace(t, "Line Spacing.md", original, splitMode)
	app.preferences.FontSize = 12
	app.preferences.EditorFont = "JetBrains Mono"
	app.code.Select(2, 5)
	editor.Frame()
	selection := app.code.Selection()
	originalHeight := app.code.LineTop(1)
	if math.Abs(originalHeight-16) > 0.5 {
		t.Fatalf("Default line height = %.2f, want the 16-point native baseline", originalHeight)
	}
	settings := mygoui.NewTester(func(c *mygoui.Context) {
		mygoui.Column(c).Padding(20).Children(func() { app.sourceLineSpacingSettings(c) })
	}, 520, 340)
	choose := func(label string, saved float64) {
		t.Helper()
		if err := settings.Click("Line Spacing"); err != nil {
			t.Fatal(err)
		}
		want := []string{"Default", "1.0", "1.15", "1.25", "1.5", "1.75", "2.0", "2.5"}
		if got := settings.Menu(); !slices.Equal(got, want) {
			t.Fatalf("Line Spacing menu = %v, want %v", got, want)
		}
		if err := settings.ChooseMenuItem(label); err != nil {
			t.Fatal(err)
		}
		editor.Frame()
		loaded, err := loadWorkspacePreferences()
		if err != nil || loaded.EditorLineHeight != saved {
			t.Fatalf("saved line height = %g, want %g (%v)", loaded.EditorLineHeight, saved, err)
		}
		if app.code.Selection() != selection || app.draft.String() != original || app.draft.Modified() {
			t.Fatal("line spacing changed the text or selection")
		}
	}
	choose("1.5", 1.5)
	if got, want := app.code.LineTop(1), originalHeight*1.5; math.Abs(got-want) > 1 {
		t.Fatalf("line height = %.2f, want %.2f", got, want)
	}
	choose("1.0", 1)
	if app.code.LineTop(1) != originalHeight {
		t.Fatal("1.0 has a different line height from Default")
	}
	choose("Default", 0)
	if app.code.LineTop(1) != originalHeight {
		t.Fatal("Default did not restore line height")
	}
}
