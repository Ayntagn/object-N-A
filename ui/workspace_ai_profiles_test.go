// spec: EV-UI-283. Connection saves, selects and removes saved profiles.
package ui

import (
	"os"
	"path/filepath"
	"testing"

	"cyberwriter/internal/core/aiworkspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestAIConnectionSavedProfiles(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CYBERWRITER_CONFIG_DIR", configDir)
	const legacy = `{"aiConnection":{"baseURL":"http://localhost:1234/v1","model":"first"},"aiSystemPrompt":"first prompt"}`
	if err := os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	prefs, err := loadWorkspacePreferences()
	if err != nil || len(prefs.AIProfiles) != 1 || prefs.AIActiveProfile != prefs.AIProfiles[0].ID {
		t.Fatalf("could not load legacy connection as a saved profile: %v", err)
	}
	first := prefs.AIConnection
	config, err := first.config()
	if err != nil {
		t.Fatal(err)
	}
	app := &workspaceWindow{preferences: &prefs, aiConnectionForm: workspaceAIConnectionForm{Draft: config, ProfileID: prefs.AIActiveProfile}}
	view := mygoui.NewTester(app.aiSettings, 760, 1240)
	if err := view.Click("New"); err != nil {
		t.Fatal(err)
	}
	if app.aiConnectionForm.Draft != (aiworkspace.Config{}) || prefs.AIConnection != first || prefs.aiSystemPrompt() != "first prompt" {
		t.Fatal("New did not clear only the connection form")
	}
	const secondBase, secondModel, secondPrompt = "http://localhost:8000/v1", "second", "second prompt"
	for _, field := range []struct{ title, contents string }{{"Base URL", secondBase}, {"Model", secondModel}} {
		input, ok := view.AccessByRole(mygoui.RoleTextField, field.title)
		if !ok || input.Offscreen {
			t.Fatalf("missing input %q", field.title)
		}
		view.ClickAt(input.Bounds.X+input.Bounds.W/2, input.Bounds.Y+input.Bounds.H/2)
		view.Type(field.contents)
	}
	prefs.AISystemPrompt = new(secondPrompt)
	for range 2 {
		if err := view.Click("Save"); err != nil {
			t.Fatal(err)
		}
	}
	if len(prefs.AIProfiles) != 2 || prefs.AIConnection.BaseURL != secondBase || prefs.AIConnection.Model != secondModel {
		t.Fatal("Save failed to retain the previous connection or duplicated an unchanged save")
	}
	view.Frame()
	picker, ok := view.AccessByRole(mygoui.RolePopUpButton, "second@localhost:8000")
	if !ok || picker.Offscreen {
		t.Fatal("missing saved profiles picker")
	}
	view.ClickAt(picker.Bounds.X+picker.Bounds.W/2, picker.Bounds.Y+picker.Bounds.H/2)
	view.Key(0, mygoui.KeyHome)
	view.Key(0, mygoui.KeyDown)
	view.Key(0, mygoui.KeyEnter)
	if prefs.AIConnection != first || prefs.aiSystemPrompt() != "first prompt" || app.aiConnectionForm.Draft != config {
		t.Fatal("choosing the first profile did not activate its connection and prompt")
	}
	if err := view.Click("Trash"); err != nil {
		t.Fatal(err)
	}
	reopened, err := loadWorkspacePreferences()
	if err != nil || len(reopened.AIProfiles) != 1 || reopened.AIConnection != first || reopened.AIActiveProfile != "" ||
		reopened.AIProfiles[0].Connection.Model != secondModel || reopened.aiSystemPrompt() != "first prompt" {
		t.Fatalf("Trash did not remove only the saved entry after reload: %v", err)
	}
}
