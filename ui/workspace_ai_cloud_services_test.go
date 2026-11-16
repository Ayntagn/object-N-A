// spec: EV-UI-284 / EV-MYGO-190. Cloud presets populate the connection draft.
package ui

import (
	"testing"

	"cyberwriter/internal/core/aiworkspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestAICloudServicesDiscloseAndUseConnectionDraft(t *testing.T) {
	saved := workspaceAISavedConnection{BaseURL: "https://api.openai.com/v1", Model: "saved-model"}
	app := &workspaceWindow{preferences: &workspacePreferences{AIConnection: saved}}
	app.aiConnectionForm = workspaceAIConnectionForm{
		Draft:     aiworkspace.Config{BaseURL: saved.BaseURL, Model: saved.Model, APIKey: "synthetic-draft-key"},
		Models:    []string{"saved-model"},
		ProfileID: "synthetic-profile",
		Status:    "Old test result",
	}
	view := mygoui.NewTester(app.aiSettings, 712, 1150)
	if _, visible := view.AccessByRole(mygoui.RoleGroup, "Claude (Anthropic)"); visible {
		t.Fatal("cloud providers should begin collapsed")
	}
	if err := view.Click("Show 7"); err != nil {
		t.Fatal(err)
	}
	var previousY float32
	for _, name := range []string{"OpenAI", "Claude (Anthropic)", "OpenRouter", "DeepSeek", "Groq", "Mistral", "GLM (Zhipu)"} {
		row, ok := view.AccessByRole(mygoui.RoleGroup, name)
		if !ok || row.Offscreen || row.Bounds.Y <= previousY {
			t.Fatalf("missing or out-of-order cloud provider %q: %+v", name, row)
		}
		previousY = row.Bounds.Y
	}
	if used, ok := view.AccessByRole(mygoui.RoleButton, "In Use"); !ok || !used.Disabled {
		t.Fatalf("saved OpenAI URL should produce disabled In Use: %+v", used)
	}
	claude, _ := view.AccessByRole(mygoui.RoleGroup, "Claude (Anthropic)")
	view.ClickAt(claude.Bounds.X+claude.Bounds.W-12, claude.Bounds.Y+16)
	form := app.aiConnectionForm
	if form.Draft.BaseURL != "https://api.anthropic.com" || form.Draft.Model != "" || form.Draft.APIKey != "" ||
		len(form.Models) != 0 || form.ProfileID != "" {
		t.Fatal("Use should select Claude's URL and clear the stale draft fields")
	}
	if form.Status != "Add your Claude (Anthropic) API key and model below, then Save." {
		t.Fatalf("unexpected connection hint %q", form.Status)
	}
	if app.preferences.AIConnection != saved || app.aiModelRequest != nil || app.aiTestRequest != nil {
		t.Fatal("Use changed the saved connection or started a connection request")
	}
	if err := view.Click("Hide"); err != nil {
		t.Fatal(err)
	}
	if _, visible := view.AccessByRole(mygoui.RoleGroup, "Claude (Anthropic)"); visible || app.aiConnectionForm.Draft != form.Draft {
		t.Fatal("Hide should conceal the providers and retain the connection draft")
	}
}
