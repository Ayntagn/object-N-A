// spec: EV-UI-285. System instructions can be edited, copied, cleared, and reset.
package ui

import (
	"runtime"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestAISystemPromptSettings(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, _, _ := snippetsWorkspace(t, "Prompt.md", "# Note\n", splitMode)
	settings := mygoui.NewTester(app.aiSettings, 760, 1400)
	const title = "System Prompt"
	if _, open := settings.AccessByRole(mygoui.RoleTextField, title); open {
		t.Fatal("the system prompt opened before disclosure")
	}
	if err := settings.Click("Go Down"); err != nil {
		t.Fatal(err)
	}
	field, found := settings.AccessByRole(mygoui.RoleTextField, title)
	if !found || !field.Multiline || field.Value != workspaceAIWritingInstructions {
		t.Fatalf("default system prompt editor = %+v", field)
	}
	command := mygoui.Ctrl
	if runtime.GOOS == "darwin" {
		command = mygoui.Cmd
	}
	settings.ClickAt(field.Bounds.X+15, field.Bounds.Y+15)
	settings.Key(command, mygoui.KeyA)
	const custom = "A\n👩🏽‍💻 "
	settings.Type(custom)
	if !settings.HasText("4 chars") {
		t.Fatal("the counter did not count the composed emoji as one character")
	}
	settings.Key(command, mygoui.KeyA)
	settings.Key(command, mygoui.KeyC)
	if got := settings.Clipboard(); got != custom {
		t.Fatalf("copied prompt = %q", got)
	}
	assertReopenedRequest := func(want string) {
		t.Helper()
		preferences, err := loadWorkspacePreferences()
		if err != nil {
			t.Fatal(err)
		}
		reopened := workspaceWindow{preferences: &preferences}
		messages := reopened.chatMessages(nil, "Current note context", "Current question")
		if len(messages) != 3 || messages[0].Role != "system" || messages[0].Content != want ||
			messages[1].Role != "system" || messages[1].Content != "Current note context" ||
			messages[2].Role != "user" || messages[2].Content != "Current question" {
			t.Fatalf("reopened request messages = %+v", messages)
		}
	}
	assertReopenedRequest(custom)
	settings.Key(0, mygoui.KeyBackspace)
	if !settings.HasText("0 chars") {
		t.Fatal("clearing the system prompt did not show an empty prompt")
	}
	assertReopenedRequest("")
	if err := settings.Click("Reset to Default"); err != nil {
		t.Fatal(err)
	}
	assertReopenedRequest(workspaceAIWritingInstructions)
	if err := settings.Click("Go Up"); err != nil {
		t.Fatal(err)
	}
	if _, open := settings.AccessByRole(mygoui.RoleTextField, title); open {
		t.Fatal("the system prompt stayed expanded")
	}
}
