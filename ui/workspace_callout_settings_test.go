// spec: FEAT-WYS-CALLOUT / user-editable Callout styles
package ui

import (
	"slices"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestCalloutStyleSettingsPersistAndRestore(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	prefs, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	app := &workspaceWindow{preferences: &prefs, settingsTab: "Themes"}
	view := mygoui.NewTester(app.appearanceSettings, 650, 1600)
	click := func(label string) {
		t.Helper()
		if err := view.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	edit := func(label, value string) {
		t.Helper()
		field, ok := view.AccessByRole(mygoui.RoleTextField, label)
		if !ok || field.Offscreen {
			t.Fatalf("missing visible field %q", label)
		}
		view.ClickAt(field.Bounds.X+field.Bounds.W/2, field.Bounds.Y+field.Bounds.H/2)
		view.Key(mygoui.Cmd, mygoui.KeyA)
		view.Type(value)
	}
	choose := func(label string) {
		t.Helper()
		click(app.tr("Callout Type", "提示块类型"))
		if err := view.ChooseMenuItem(label); err != nil {
			t.Fatal(err)
		}
	}
	if !view.HasText("Callout Styles") {
		t.Fatal("Themes has no Callout Styles section")
	}
	choose("Tip [!tip]")
	edit("Display Name", "Reminder")
	edit("Accent Color", "#456789")
	click("Save Style")
	click("Add Type…")
	edit("Type Identifier", "seminar")
	edit("Display Name", "Seminar")
	edit("Icon", "📌")
	edit("Accent Color", "#246")
	click("Save Style")
	if _, ok := view.AccessByRole(mygoui.RoleAlert, "Enter a six-digit hex color, such as #A9C7FF."); !ok {
		t.Fatal("invalid hex color did not show accessible feedback")
	}
	edit("Accent Color", "#2468ac")
	click("Save Style")

	reloaded, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	reloaded.InterfaceLanguage = languageChinese
	app = &workspaceWindow{preferences: &reloaded, settingsTab: "Themes"}
	view = mygoui.NewTester(app.appearanceSettings, 500, 1600)
	choose("Seminar [!seminar]")
	for label, want := range map[string]string{"显示名称": "Seminar", "图标": "📌", "主题色": "#2468ac"} {
		if field, ok := view.AccessByRole(mygoui.RoleTextField, label); !ok || field.Value != want {
			t.Errorf("%s after reload = %q, want %q", label, field.Value, want)
		}
	}
	click("恢复默认样式")
	choose("Reminder [!tip]")
	click("恢复默认样式")
	click("提示块类型")
	if menu := view.Menu(); !slices.Contains(menu, "Tip [!tip]") || slices.Contains(menu, "Seminar [!seminar]") {
		t.Fatalf("Callout types after restoring defaults: %q", menu)
	}
}
