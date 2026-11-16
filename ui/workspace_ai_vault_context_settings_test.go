// spec: EV-UI-226 / EV-UI-289 / EV-UI-291 / EV-MYGO-196
package ui

import (
	"context"
	"fmt"
	"testing"

	"cyberwriter/internal/core/vaultcontext"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestAIVaultContextSettingsControlsAndIndexDepth(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	root := t.TempDir()
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	if _, err := vault.CreateDocument("Fixture.md", "# Fixture\n\nSynthetic note.\n"); err != nil {
		t.Fatal(err)
	}
	prefs := &workspacePreferences{}
	app := &workspaceWindow{preferences: prefs, root: root, vault: vault, vaultFileCount: 1}
	view := mygoui.NewTester(app.aiSettings, 664, 1600)
	if !view.HasText("No index built yet for this vault.") {
		t.Fatal("empty Vault Context status is missing")
	}
	for _, item := range []struct {
		label   string
		checked bool
	}{
		{"Use vault embeddings as AI context", false},
		{"Auto-update when files change", true},
		{"Send vault structure with each request", false},
	} {
		toggle, ok := view.AccessByRole(mygoui.RoleSwitch, item.label)
		if !ok || toggle.Disabled || toggle.Checked != item.checked {
			t.Fatalf("unexpected initial %q switch: %+v", item.label, toggle)
		}
		view.ClickAt(toggle.Bounds.X+toggle.Bounds.W/2, toggle.Bounds.Y+toggle.Bounds.H/2)
		toggle, _ = view.AccessByRole(mygoui.RoleSwitch, item.label)
		if toggle.Checked == item.checked {
			t.Fatalf("%q did not change", item.label)
		}
	}
	const quick = "Quick, Files only - fastest, retrieves whole notes"
	if err := view.Click(quick); err != nil {
		t.Fatal(err)
	}
	if selected, ok := view.AccessByRole(mygoui.RoleButton, quick); !ok || !selected.Checked || app.aiVaultContextDepth() != vaultcontext.Quick {
		t.Fatalf("Quick did not update immediately without an index: %+v", selected)
	}
	for _, item := range []struct {
		label    string
		disabled bool
	}{{"Build Index", false}, {"Rebuild", true}, {"Clear", true}} {
		button, ok := view.AccessByRole(mygoui.RoleButton, item.label)
		if !ok || button.Disabled != item.disabled {
			t.Fatalf("unexpected empty-index %q action: %+v", item.label, button)
		}
	}
	saved, err := loadWorkspacePreferences()
	if err != nil || !saved.AIVaultContextEnabled || saved.aiVaultContextAutoUpdate() ||
		!saved.AIVaultContextSendStructure || saved.AIVaultContextDepth != vaultcontext.Quick {
		t.Fatalf("Vault Context preferences did not persist: %v", err)
	}
	if !view.HasText("Privacy: this sends vault filenames to whichever AI provider is selected.") {
		t.Fatal("sending the structure did not disclose the filenames it shares")
	}

	index, err := vaultcontext.Build(t.Context(), []vaultcontext.Document{
		{Path: "Fixture.md", Text: "Synthetic notes describe a reusable telescope mount and its calibration."},
	}, vaultcontext.Quick)
	if err != nil {
		t.Fatal(err)
	}
	state := app.aiVaultContextState()
	state.index = index
	view.Frame()
	ready := fmt.Sprintf("Indexed %d chunks from %d files", index.ChunkCount(), index.FileCount())
	if _, ok := view.AccessByRole(mygoui.RoleStatus, ready); !ok {
		t.Fatal("the ready state did not show actual index counts")
	}
	const balanced = "Balanced, Sections - sweet spot for find-related-notes"
	if err := view.Click(balanced); err != nil {
		t.Fatal(err)
	}
	if _, ok := view.AccessByRole(mygoui.RoleAlertDialog, "Rebuild AI index?"); !ok ||
		app.aiVaultContextDepth() != vaultcontext.Quick {
		t.Fatal("changing a ready index's depth should ask before committing")
	}
	if err := view.Click("Change Later"); err != nil {
		t.Fatal(err)
	}
	if app.aiVaultContextDepth() != vaultcontext.Balanced || state.index != index {
		t.Fatal("Change Later should commit the preference and keep the current index")
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	state.request = &workspaceAIVaultBuild{cancel: cancel}
	view.Frame()
	if control, ok := view.AccessByRole(mygoui.RoleButton, "Rebuild"); !ok || !control.Disabled {
		t.Fatal("an active rebuild should disable a second Rebuild")
	}
	if err := view.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() != context.Canceled || state.index != index {
		t.Fatal("Cancel should stop the current task and keep the usable index")
	}
	if err := view.Click("Clear"); err != nil {
		t.Fatal(err)
	}
	if state.index != nil || !view.HasText("No index built yet for this vault.") {
		t.Fatal("Clear did not return the Vault Context section to its empty state")
	}
}
