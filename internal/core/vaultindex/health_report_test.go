// spec: EV-UI-166 / EV-UI-167 / EV-UI-168
package vaultindex

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"cyberwriter/internal/core/workspace"
)

func TestHealthReportMarkdownMatchesEmptySections(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Alpha.md"), []byte("[[Beta]]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Beta.md"), []byte("# Beta\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	report, err := HealthReport(vault, "Fixture", time.Date(2026, 11, 14, 13, 56, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	const want = "# Vault Health\n\n_Generated 2026-11-14 13:56 · Fixture_\n\n" +
		"## Summary\n\n| | |\n|---|---|\n| Notes scanned | 2 |\n| Wikilinks | 1 |\n| Broken links | 0 |\n" +
		"| Orphan notes | 0 |\n| Unlinked mentions | 0 |\n| Duplicate titles | 0 |\n\n" +
		"## Broken Links\n\nNone - every wikilink resolves. ✓\n\n" +
		"## Orphan Notes\n\nNone - every note is connected. ✓\n\n" +
		"## Unlinked Mentions\n\nNone found - mentions and links agree. ✓\n\n" +
		"## Duplicate Titles\n\nNone - every note name is unique. ✓\n\n" +
		"---\n\n_Regenerate any time: `⌘/` → Vault Health Report. Unlinked mentions can be linked one by one from the vault sidebar's Unlinked Mentions section._\n"
	if report != want {
		t.Fatalf("report differs from the frozen Markdown format\ngot:\n%s\nwant:\n%s", report, want)
	}
}
