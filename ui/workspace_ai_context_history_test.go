// spec: EV-UI-282. Limit the transmitted history and retain the current context.
package ui

import (
	"strconv"
	"strings"
	"testing"

	"cyberwriter/internal/core/aiworkspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestAIChatHistoryInContext(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const source = "Start of attached note 👩🏽‍💻\nEnd of attached note.\n"
	app, _, _ := snippetsWorkspace(t, "AI History Context.md", source, splitMode)
	settings := mygoui.NewTester(app.aiSettings, 760, 1400)
	const label = "Chat History in Context"
	slider, ok := settings.AccessByRole(mygoui.RoleSlider, label)
	if !ok || slider.Disabled || slider.Min != 10 || slider.Max != 200 || slider.Step != 10 || !settings.HasText("50 messages") {
		t.Fatalf("missing history limit at its default: %+v", slider)
	}
	settings.ClickAt(slider.Bounds.X+slider.Bounds.W/2, slider.Bounds.Y+slider.Bounds.H/2)
	settings.Key(0, mygoui.KeyHome)
	if !settings.HasText("10 messages") {
		t.Fatal("the slider did not display ten messages")
	}
	preferences, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	app.preferences = &preferences
	if !mygoui.NewTester(app.aiSettings, 760, 1400).HasText("10 messages") {
		t.Fatal("the slider did not restore after reloading preferences")
	}

	for index := range 4 {
		number := strconv.Itoa(index)
		app.ai.submitted = append(app.ai.submitted, workspaceAITurn{
			prompt: "question " + number, reply: "reply " + number, previewHint: "preview " + number,
			keepInHistory: true,
		})
	}
	app.attachAIContext()
	const referenceText = "Complete reference text α and ω."
	hint := app.aiChatContextHint("Current preview", workspaceAIReference{name: "Reference.txt", content: referenceText})
	got := app.chatMessages(app.activeAIContext(), hint, "current question")
	want := []aiworkspace.Message{
		{Role: "system", Content: workspaceAIWritingInstructions},
		{Role: "user", Content: "question 1"}, {Role: "assistant", Content: "reply 1"},
		{Role: "system", Content: "preview 2"}, {Role: "user", Content: "question 2"}, {Role: "assistant", Content: "reply 2"},
		{Role: "system", Content: "preview 3"}, {Role: "user", Content: "question 3"}, {Role: "assistant", Content: "reply 3"},
		{Role: "system", Content: hint}, {Role: "user", Content: "current question"},
	}
	if len(got) != len(want) {
		t.Fatalf("sent %d messages, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("message %d = %+v, want %+v", index, got[index], want[index])
		}
	}
	if !strings.Contains(hint, source) || !strings.Contains(hint, referenceText) || len(app.ai.submitted) != 4 {
		t.Fatal("message preparation changed the source context, reference, or visible conversation")
	}
}
