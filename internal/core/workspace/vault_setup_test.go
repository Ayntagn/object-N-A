// spec: EV-UI-088 / EV-UI-090
package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestVaultSetupPlanAndCollision(t *testing.T) {
	invalid, err := PrepareVaultSetup("Gear Probe", []string{"Projects", "Reading/Inbox"})
	if err == nil || !slices.Equal(invalid.Folders, []string{"Projects"}) {
		t.Fatalf("invalid folder plan = %#v, %v", invalid, err)
	}
	plan, err := PrepareVaultSetup(" Gear Probe ", []string{"Projects", " Areas ", "projects", ""})
	if err != nil || plan.Name != "Gear Probe" || !slices.Equal(plan.Folders, []string{"Projects", "Areas"}) {
		t.Fatalf("valid folder plan = %#v, %v", plan, err)
	}

	parent := t.TempDir()
	existing := filepath.Join(parent, plan.Name)
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(existing, "Welcome.md")
	if err := os.WriteFile(original, []byte("keep this note"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := CreateVault(parent, plan)
	if err != nil || root != filepath.Join(parent, "Gear Probe 2") {
		t.Fatalf("new Vault root = %q, %v", root, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !slices.Equal(names, []string{"Areas", "Projects", "Welcome.md"}) {
		t.Fatalf("new Vault entries = %v", names)
	}
	if contents, err := os.ReadFile(original); err != nil || string(contents) != "keep this note" {
		t.Fatalf("existing Welcome.md = %q, %v", contents, err)
	}
}
