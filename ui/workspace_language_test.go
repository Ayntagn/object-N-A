// spec: FEAT-072 / Settings and Help language selection
package ui

import (
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestSettingsLanguageSelectionPersists(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	prefs, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	prefs.LineNumbers = true
	app := &workspaceWindow{preferences: &prefs, settingsTab: "General"}
	view := mygoui.NewTester(app.appearanceSettings, 880, 892)
	if !view.HasText("Settings & Help Language") {
		t.Fatal("English settings language choice is missing")
	}
	if err := view.Click("简体中文"); err != nil {
		t.Fatal(err)
	}
	if !view.HasText("设置与帮助语言") || !view.HasText("通用") {
		t.Fatalf("Settings did not switch language: %v", view.Texts())
	}
	reloaded, err := loadWorkspacePreferences()
	if err != nil || reloaded.InterfaceLanguage != languageChinese || !reloaded.LineNumbers {
		t.Fatalf("language or editor settings did not survive reload: %q, %t, %v", reloaded.InterfaceLanguage, reloaded.LineNumbers, err)
	}
	app = &workspaceWindow{preferences: &reloaded, settingsTab: "General"}
	view = mygoui.NewTester(app.appearanceSettings, 880, 892)
	if err := view.Click("English"); err != nil {
		t.Fatal(err)
	}
	if !view.HasText("Settings & Help Language") || !view.HasText("General") {
		t.Fatalf("Settings did not switch back: %v", view.Texts())
	}
	reloaded, err = loadWorkspacePreferences()
	if err != nil || reloaded.InterfaceLanguage != languageEnglish {
		t.Fatalf("English preference after reload: %q, %v", reloaded.InterfaceLanguage, err)
	}
}
