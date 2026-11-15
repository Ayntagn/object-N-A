// spec: FEAT-001 / EV-UI-186 / EV-UI-187
package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestFilesSearchPublishesToBothGraphHosts(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, main, file := snippetsWorkspace(t, "Active.md", "# Active\n", splitMode)
	root := filepath.Dir(file)
	for name, source := range map[string]string{
		"Phrase.md":     "kwfirst kwsecond",
		"Split.md":      "kwfirst aside kwsecond",
		"Tagged.md":     "#tagneedle",
		"Plain.txt":     "attachment-only",
		"Table.csv":     "value\nattachment-only\n",
		"Properties.md": "+++\nprobe = \"front-only\"\n+++\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	app.entries, app.vaultFileCount, err = readWorkspaceEntries(vault, false)
	if err != nil {
		t.Fatal(err)
	}
	app.index.data, err = vaultindex.Build(vault, app.entries)
	if err != nil {
		t.Fatal(err)
	}
	app.vault, app.root, app.vaultVisible = vault, root, true
	app.graph.open = true
	app.setGraphTag("tagneedle")
	floating := mygoui.NewTester(app.graphWindowView, 1100, 760)
	assertMatches := func(want ...string) {
		t.Helper()
		main.Frame()
		floating.Frame()
		for _, canvas := range []*graphCanvas{&app.graph.sidebar, &app.graph.floating} {
			var matched []string
			if len(canvas.projected) != len(canvas.graph.Notes) {
				t.Fatalf("graph projected %d of %d nodes", len(canvas.projected), len(canvas.graph.Notes))
			}
			for i, point := range canvas.projected {
				if point.matched {
					matched = append(matched, canvas.graph.Notes[i].Path)
				}
			}
			if !slices.Equal(matched, want) {
				t.Fatalf("query %q, matched %q, want %q", app.fileSearch, matched, want)
			}
		}
	}

	assertMatches("Tagged.md")
	if err := main.Click("Search Vault files"); err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		query string
		want  []string
	}{
		{"kwfirst kwsecond", []string{"Phrase.md"}},
		{"kwfirst  kwsecond", []string{"Tagged.md"}},
		{"attachment-only", nil},
		{"Table.csv", nil},
		{"front-only", []string{"Properties.md"}},
		{"  ", []string{"Tagged.md"}},
	} {
		main.Key(mygoui.Cmd, mygoui.KeyA)
		main.Type(sample.query)
		assertMatches(sample.want...)
	}
}
