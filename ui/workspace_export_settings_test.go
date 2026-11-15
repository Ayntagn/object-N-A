// spec: EV-UI-022 / EV-UI-247
package ui

import (
	"slices"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestExportSettingsPageNumbersAndLocation(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	prefs, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	prefs.Export.LastDirectory = t.TempDir()
	if err := prefs.write(); err != nil {
		t.Fatal(err)
	}
	prefs, err = loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	app := &workspaceWindow{preferences: &prefs, settingsTab: "Export", blackout: true}
	tester := mygoui.NewTester(app.appearanceSettings, 880, 860)
	if label := newExportPageForm(prefs.Export).pagination.NumberLabel(0); label != "1" {
		t.Fatalf("an existing preference file must default to numbered pages, got %q", label)
	}
	for _, control := range []string{"Page Numbers", "Remember Location"} {
		if err := tester.Click(control); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	if label := newExportPageForm(stored.Export).pagination.NumberLabel(0); label != "" {
		t.Fatalf("a disabled Page Numbers setting still produces a footer number %q", label)
	}
	if stored.Export.RememberLocation || stored.Export.LastDirectory != "" {
		t.Fatal("disabling Remember Location did not clear the saved directory")
	}
}

func TestExportLineSpacingChoicesAndLegacyValue(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	prefs := workspacePreferences{Export: workspaceExportPreferences{LineSpacing: "2×"}}
	app := &workspaceWindow{preferences: &prefs, settingsTab: "Export"}
	tester := mygoui.NewTester(app.appearanceSettings, 880, 860)
	spacing, ok := tester.Access("Line Spacing")
	if !ok || spacing.Value != "2.0" || prefs.Export.spacing() != 2 {
		t.Fatalf("legacy spacing has value %q and multiplier %g", spacing.Value, prefs.Export.spacing())
	}
	if err := tester.Click("Line Spacing"); err != nil {
		t.Fatal(err)
	}
	choices := tester.Menu()
	if want := []string{"Theme default", "1.0", "1.15", "1.25", "1.5", "1.75", "2.0", "2.5"}; !slices.Equal(choices, want) {
		t.Fatalf("Line Spacing choices are %q, want %q", choices, want)
	}
	if err := tester.ChooseMenuItem("1.15"); err != nil {
		t.Fatal(err)
	}
	stored, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	if stored.Export.spacing() != 1.15 {
		t.Fatalf("the chosen spacing restored as %g", stored.Export.spacing())
	}
}
