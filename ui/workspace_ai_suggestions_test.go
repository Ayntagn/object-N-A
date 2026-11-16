// spec: EV-UI-225 / EV-UI-245 / EV-UI-277. Empty-chat suggestions prefill the composer.
package ui

import (
	"os"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestAIConversationSuggestionsPrefillPrompt(t *testing.T) {
	const note = "# Fixture\n\nThis stays in the note.\n"
	for _, scenario := range []struct{ name, model string }{
		{"unconfigured", ""}, {"configured", "fixture"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			app, view, filename := snippetsWorkspace(t, "Fixture.md", note, splitMode)
			app.preferences.AIConnection = workspaceAISavedConnection{Model: scenario.model, BaseURL: "invalid:"}
			app.openAIWorkspace()
			view.Frame()
			context := *app.activeAIContext()
			for index, want := range []string{
				"Summarize this note", "Improve this paragraph", "Diagram this as a flowchart", "Turn this into a table",
			} {
				if index == 1 {
					if err := view.Click("AI prompt"); err != nil {
						t.Fatal(err)
					}
					view.Type(" previous draft")
				}
				if index == 2 {
					if err := view.Click("Attach current note"); err != nil {
						t.Fatal(err)
					}
					context = *app.activeAIContext()
					if context.snapshot != note {
						t.Fatalf("test attachment = %q", context.snapshot)
					}
				}
				focused := view.Focused("AI prompt")
				if button, ok := view.AccessByRole(mygoui.RoleButton, want); !ok || button.Disabled {
					t.Fatalf("suggestion %q is unavailable: %+v", want, button)
				}
				if err := view.Click(want); err != nil {
					t.Fatal(err)
				}
				field, ok := view.AccessByRole(mygoui.RoleTextField, "AI prompt")
				if !ok || field.Value != want || field.Focused != focused || app.ai.prompt != want {
					t.Fatalf("suggestion %q produced prompt %q, field %q, focus=%t (was %t)", want, app.ai.prompt, field.Value, field.Focused, focused)
				}
				if len(app.ai.submitted) != 0 || app.ai.request != nil || *app.activeAIContext() != context {
					t.Fatalf("suggestion %q submitted a turn or changed its context", want)
				}
				if send, ok := view.AccessByRole(mygoui.RoleButton, "Send"); !ok || send.Disabled != (scenario.model == "") {
					t.Fatalf("suggestion %q changed model gating: %+v", want, send)
				}
			}
			if saved, err := os.ReadFile(filename); err != nil || string(saved) != note || app.draft.String() != note {
				t.Fatalf("suggestions changed the note: %q, %v", saved, err)
			}
		})
	}
}
