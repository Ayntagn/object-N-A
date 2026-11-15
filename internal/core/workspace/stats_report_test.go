package workspace

// spec: EV-UI-086 / unmanaged report collision

import (
	"errors"
	"testing"
)

func TestVaultStatsCollisionRequiresAnObservedRevision(t *testing.T) {
	vault, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	const handwritten = "# Handwritten notes\n"
	original, err := vault.CreateDocument(VaultStatsFile, handwritten)
	if err != nil {
		t.Fatal(err)
	}
	const report = "---\ncyberwriter-generated: true\n---\n\n# Vault Stats"
	observed, err := vault.WriteVaultStats(report)
	if !errors.Is(err, ErrUnmanagedVaultStats) || observed.Text != handwritten {
		t.Fatalf("observed = %#v, error = %v", observed, err)
	}
	current, err := vault.ReadDocument(VaultStatsFile)
	if err != nil || current.Text != handwritten {
		t.Fatalf("current = %#v, error = %v", current, err)
	}
	if _, err := vault.SaveDocument(original, handwritten+"More notes.\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.SaveDocument(observed, report); !errors.Is(err, ErrChangedOnDisk) {
		t.Fatalf("confirmed replacement of an obsolete revision: %v", err)
	}
}
