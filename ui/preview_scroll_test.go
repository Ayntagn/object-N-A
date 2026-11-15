// spec: FEAT-001 / EV-UI-009, EV-UI-022, EV-UI-059
package ui

import (
	"os"
	"path/filepath"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestScrollSyncControlsSharePreferences(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CYBERWRITER_CONFIG_DIR", config)
	defaults, err := loadWorkspacePreferences()
	if err != nil || !defaults.ScrollSyncEnabled || defaults.UseLineBasedScrollSync || !defaults.ScrollPastEnd {
		t.Fatalf("default scroll settings = %+v, err=%v", defaults, err)
	}
	// A workspace saved before scroll sync settings were introduced.
	if err := os.WriteFile(filepath.Join(config, "settings.json"), []byte(`{"scrollPastEnd":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	app, window, _ := snippetsWorkspace(t, "ScrollStudy.md", "# Opening\n\nA note.\n", splitMode)
	*app.preferences, err = loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	window.Frame()
	// Use the native General content viewport with the new Editor preferences.
	settings := mygoui.NewTester(app.generalSettings, 712, 860)
	button, found := window.Access("Scroll Sync Mode")
	if !found || button.Disabled || button.Bounds.W != 30 || button.Bounds.H != 28 || button.Description != "Percentage sync (smooth)" {
		t.Fatalf("initial mode button = %+v, found=%t", button, found)
	}
	if err := window.Click("Scroll Sync Mode"); err != nil {
		t.Fatal(err)
	}
	lineMode, _ := window.Access("Scroll Sync Mode")
	if lineMode.Description != "Line-based sync (accurate)" || lineMode.Bounds != button.Bounds {
		t.Fatalf("line-based mode control = %+v", lineMode)
	}
	if err := window.Click("Scroll Sync"); err != nil {
		t.Fatal(err)
	}
	if _, found := window.Access("Scroll Sync Mode"); found {
		t.Fatal("mode button remained visible with sync off")
	}
	settings.Frame()
	if err := settings.Click("Scroll Sync"); err != nil {
		t.Fatal(err)
	}
	window.Frame()
	if _, found := window.Access("Scroll Sync Mode"); !found {
		t.Fatal("General did not reenable the toolbar mode control")
	}
	loaded, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.ScrollSyncEnabled || !loaded.UseLineBasedScrollSync || loaded.ScrollPastEnd {
		t.Fatalf("saved settings = %+v", loaded)
	}
}
