// spec: EV-UI-166 / EV-UI-168
package vaultindex

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"cyberwriter/internal/core/workspace"
)

func TestHealthScanCountsRawWikilinksAndUsesVaultGraphForOrphans(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"Hub.md": "---\ntitle: Index\n---\n[[Wrong/Relay]] [[Missing|first]] [[Missing]] ![[Picture.png]]\n" +
			"[[Index/Archive.markdown#Section]] [[Relay#Absent]] [[#Here]] [other](bad.md)\n" +
			"![[Assets/extra.txt]] [[extra]] [[extra.txt]] ![[Picture.PNG]] [[Picture]]\n\n" +
			"Blue beacon, Blue  beacon, Rx, and [Blue beacon](https://example.invalid/).\n```text\n[[Code Ghost]] Blue beacon\n```\n",
		"North/Relay.md":         "---\naliases: [Blue beacon, Rx]\n---\n[[Hub]]\n",
		"South/relay.md":         "# Southern route\n",
		"Index/Archive.markdown": "[[Relay]] [[Ignore for scan]]\n",
		"Assets/extra.txt":       "Fixture attachment text.\n",
		"Assets/Picture.PNG":     "Fixture attachment bytes.\n",
		"Solo.MD":                "# Isolated\n",
		".hidden.md":             "[[Ignore hidden]]\n",
	}
	for relative, source := range files {
		file := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	result, err := scanVaultHealth(vault)
	if err != nil {
		t.Fatal(err)
	}
	if result.notesScanned != 4 || result.wikilinks != 14 || result.mentionCount != 1 {
		t.Fatalf("health counts = scanned %d, links %d, mentions %d %+v", result.notesScanned, result.wikilinks, result.mentionCount, result.mentions)
	}
	broken := []healthBrokenLink{{"Hub", "Missing|first", 4}, {"Hub", "Missing", 4}, {"Hub", "extra.txt", 6}, {"Hub", "Picture", 6}, {"Hub", "Code Ghost", 10}}
	if !slices.Equal(result.broken, broken) || !slices.Equal(result.mentions, []healthMention{{"Relay", 1, 1}}) {
		t.Fatalf("health broken = %+v, mentions = %+v", result.broken, result.mentions)
	}
	if !slices.Equal(result.orphans, []healthOrphan{{"Solo", 0}}) || len(result.duplicates) != 1 ||
		!slices.Equal(entryPaths(result.duplicates[0]), []string{"North/Relay.md", "South/relay.md"}) {
		t.Fatalf("health orphans = %+v, duplicates = %+v", result.orphans, result.duplicates)
	}
	if _, err := vault.CreateDocument(workspace.VaultHealthFile, "# Vault Health\n\n[[Solo]]\n"); err != nil {
		t.Fatal(err)
	}
	regenerated, err := scanVaultHealth(vault)
	if err != nil {
		t.Fatal(err)
	}
	if regenerated.notesScanned != result.notesScanned || regenerated.wikilinks != result.wikilinks ||
		!slices.Equal(regenerated.broken, broken) || len(regenerated.orphans) != 0 {
		t.Fatalf("regenerated report counts = %+v", regenerated)
	}
}
