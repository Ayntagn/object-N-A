package vaultindex

// spec: EV-UI-086 / EV-UI-087

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cyberwriter/internal/core/workspace"
)

func TestStatsReportUsesSeparateFileAndLinkSets(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"A.md":                      "# A\n\n[[B]] [[B|again]] [[Missing]] #demo\n",
		"Folder/B.markdown":         "# B\nalpha—beta #demo\n",
		"Folder/Generated Probe.md": "---\ncyberwriter-generated: true\n---\n\nbody\n",
		"Folder/Plain.txt":          "Attachment text is outside the Markdown word count.\n",
		"Folder/Pixel.png":          "Image type probe\n",
		"Folder/Data.json":          "{}\n",
		"Folder/.hidden.md":         "# Hidden\n\n[[Missing]] #hidden\n",
		"Vault Stats.md":            "---\ncyberwriter-generated: true\n---\n\n[[A]] #demo\n",
	}
	for relative, source := range files {
		name := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	when := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	report, err := StatsReport(vault, "Probe", when)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"cyberwriter-generated: true", "# Probe - Vault Stats", "2026-11-12T08:00:00Z",
		"| Files | 5 |", "| Markdown files | 3 |", "| Total words | 15 |", "| Tags | 1 |", "| Wikilinks | 3 |",
		"| #demo | 3 |", "| [[A]] | 1 |", "| [[B]] | 1 |", "- [[Generated Probe]]",
		"| Folder | 4 |", "| (root) | 1 |", "| .png | 1 |", "| .txt | 1 |",
	} {
		if !strings.Contains(report, expected) {
			t.Errorf("report missing %q\n%s", expected, report)
		}
	}
	if strings.Contains(report, "[[Missing]]") || strings.Contains(report, ".json") || strings.Contains(report, "#hidden") {
		t.Fatalf("report contains unlisted files or link targets\n%s", report)
	}
}
