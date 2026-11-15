// spec: FEAT-001 / EV-UI-196
package ui

import (
	"path/filepath"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestSourceFolderButtonReopensVaultWithUntitledDraft(t *testing.T) {
	app, tester, filename := snippetsWorkspace(t, "Example.md", "# Example\n", splitMode)
	root := filepath.Dir(filename)
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	app.vault, app.root = vault, root
	const draft = "# Untitled draft\n"
	app.setTabDocument(app.workspaceTab, workspace.Document{Text: draft})
	tester.Frame()

	button, found := tester.AccessByRole(mygoui.RoleButton, "Move")
	if !found || button.Disabled {
		t.Fatalf("source folder button is unavailable: found=%v disabled=%v", found, button.Disabled)
	}
	if button.Bounds.W != 26 || button.Bounds.H != 22 || button.Description != "Show Vault" {
		t.Fatalf("source folder button geometry/help = %+v, %q", button.Bounds, button.Description)
	}
	if err := tester.Click("Move"); err != nil {
		t.Fatal(err)
	}
	if _, found := tester.Access("Vault sidebar"); !found || app.modalOpen {
		t.Fatal("the source folder button did not reopen the Vault sidebar directly")
	}
	if app.document.Path != "" || app.draft.String() != draft {
		t.Fatal("reopening the Vault sidebar changed the untitled draft")
	}
}
