// spec: EV-UI-280 / EV-UI-283. Local Use changes the active connection and preserves saved profiles.
package ui

import (
	"testing"

	"cyberwriter/internal/core/aiworkspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestAILocalServicesActivateWithoutRemovingSavedConnection(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app := &workspaceWindow{preferences: &workspacePreferences{}}
	view := mygoui.NewTester(app.aiSettings, 760, 900)
	if rescan, ok := view.AccessByRole(mygoui.RoleButton, "Rescan"); !ok || rescan.Disabled {
		t.Fatalf("missing enabled Rescan: %+v", rescan)
	}
	if _, hasUse := view.AccessByRole(mygoui.RoleButton, "Use"); !view.HasText("Offline") || hasUse {
		t.Fatal("unavailable services should show status without a Use button")
	}
	app.aiServiceRequest = &aiSettingsRequest{}
	view.Frame()
	if _, ok := view.AccessByRole(mygoui.RoleStatus, "Scanning local AI services"); !ok {
		t.Fatal("the active scan did not replace Rescan with a spinner")
	}
	app.aiServiceRequest = nil
	services := aiworkspace.LocalServices()
	ollama, studio, vllm := services[0], services[1], services[3]
	previous := workspaceAISavedConnection{BaseURL: studio.BaseURL, Model: "studio-model", Credential: "fixture-reference"}
	app.preferences.AIConnection = previous
	app.aiServiceResults = []aiworkspace.LocalServiceResult{
		{Service: ollama, Online: true, Models: []string{"first-local-model", "second-local-model"}},
		{Service: studio, Online: true, Models: []string{"studio-model"}},
		{Service: vllm, Online: true},
	}
	view.Frame()
	if used, ok := view.AccessByRole(mygoui.RoleButton, "In Use"); !ok || !used.Disabled ||
		!view.HasText("1 model") || !view.HasText("2 models") || !view.HasText("0 models") {
		t.Fatalf("service results or default In Use state are missing: %+v", used)
	}
	use := func(name string) {
		t.Helper()
		row, ok := view.AccessByRole(mygoui.RoleGroup, name)
		if !ok || row.Offscreen {
			t.Fatalf("missing service row %q: %+v", name, row)
		}
		view.ClickAt(row.Bounds.X+row.Bounds.W-10, row.Bounds.Y+row.Bounds.H/2)
	}
	use(vllm.Name)
	if draft := app.aiConnectionForm.Draft; draft.BaseURL != vllm.BaseURL || draft.Model != "" {
		t.Fatalf("using a service with no models produced %+v", draft)
	}
	use(ollama.Name)
	if draft := app.aiConnectionForm.Draft; draft.BaseURL != ollama.BaseURL || draft.Model != "first-local-model" {
		t.Fatalf("Use did not fill the connection draft: %+v", draft)
	}
	preferences, err := loadWorkspacePreferences()
	if err != nil || preferences.AIConnection.BaseURL != ollama.BaseURL || preferences.AIConnection.Model != "first-local-model" {
		t.Fatalf("Use did not activate the selected service: %+v, %v", preferences.AIConnection, err)
	}
	if len(preferences.AIProfiles) != 1 || preferences.AIProfiles[0].Connection != previous || preferences.AIActiveProfile != "" {
		t.Fatal("Use changed the prior saved connection or its credential reference")
	}
	row, _ := view.AccessByRole(mygoui.RoleGroup, ollama.Name)
	used, ok := view.AccessByRole(mygoui.RoleButton, "In Use")
	if !ok || !used.Disabled || used.Bounds.Y < row.Bounds.Y || used.Bounds.Y+used.Bounds.H > row.Bounds.Y+row.Bounds.H {
		t.Fatalf("saved service did not show In Use: %+v", used)
	}
}
