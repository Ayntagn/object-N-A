// spec: EV-UI-278. The context limit marks an attachment without replacing its text.
package ui

import (
	"strings"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestAIContextLimitKeepsFullAttachment(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	source := strings.Repeat("x", 2999) + "👩🏽‍💻β"
	app, _, _ := snippetsWorkspace(t, "Context Limit.md", source, splitMode)
	settings := mygoui.NewTester(app.aiSettings, 760, 1400)
	const label = "Document Context Limit"
	picker, ok := settings.AccessByRole(mygoui.RolePopUpButton, label)
	if !ok || picker.Disabled || !settings.HasText("~5,000 tokens") {
		t.Fatalf("missing the default document context limit: %+v", picker)
	}
	settings.ClickAt(picker.Bounds.X+picker.Bounds.W/2, picker.Bounds.Y+picker.Bounds.H/2)
	settings.Key(0, mygoui.KeyHome)
	settings.Key(0, mygoui.KeyEnter)
	if !settings.HasText("~750 tokens") {
		t.Fatal("selecting Small did not change the limit estimate")
	}
	preferences, err := loadWorkspacePreferences()
	if err != nil || preferences.AIContextLimitPreset != "small" {
		t.Fatalf("the picker did not persist Small: %q, %v", preferences.AIContextLimitPreset, err)
	}
	app.attachAIContext()
	context := app.activeAIContext()
	if context.characters != 3001 || context.snapshot != source {
		t.Fatalf("unexpected attachment: %d graphemes, %d bytes", context.characters, len(context.snapshot))
	}
	reader := mygoui.NewTester(func(c *mygoui.Context) {
		drawAIContextReader(c, true, context.snapshot, context.characters, context.limitWarning(languageEnglish), &context.notes, nil, nil)
	}, aiContextWidth, aiContextHeight)
	const warning = "Document context exceeds the selected limit. This request uses the full text."
	if !reader.HasText(warning) {
		t.Fatal("the 3001-grapheme attachment did not show the limit warning")
	}
	if err := reader.Click("Copy"); err != nil || reader.Clipboard() != source {
		t.Fatalf("copy lost part of the attachment: %d bytes, %v", len(reader.Clipboard()), err)
	}
	if err := settings.Click(label); err != nil {
		t.Fatal(err)
	}
	settings.Key(0, mygoui.KeyEnd)
	settings.Key(0, mygoui.KeyEnter)
	input, ok := settings.AccessByRole(mygoui.RoleTextField, "Custom limit")
	if !ok || input.Offscreen {
		t.Fatal("selecting Custom did not show an editable limit")
	}
	settings.ClickAt(input.Bounds.X+input.Bounds.W/2, input.Bounds.Y+input.Bounds.H/2)
	settings.Key(mygoui.Cmd, mygoui.KeyA)
	settings.Type("3001")
	preferences, err = loadWorkspacePreferences()
	if err != nil || preferences.AIContextLimitPreset != "custom" || preferences.AICustomContextLimit != 3001 {
		t.Fatalf("the custom field did not persist 3001 characters: %q, %d, %v", preferences.AIContextLimitPreset, preferences.AICustomContextLimit, err)
	}
	app.attachAIContext()
	reader.Frame()
	if reader.HasText(warning) {
		t.Fatal("an attachment equal to the custom limit showed a warning")
	}
}
