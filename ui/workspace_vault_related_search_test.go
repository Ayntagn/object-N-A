// spec: EV-UI-187 / EV-UI-294 / EV-UI-296 / EV-UI-318 / EV-UI-321 / EV-UI-325.
package ui

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/vaultcontext"
	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestVaultSidebarRelatedSearchOpensExcerptAndHighlightsBothGraphs(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, view, filename := snippetsWorkspace(t, "Active.md", "# Active\n", splitMode)
	root := filepath.Dir(filename)
	related := strings.Repeat("orchid with humidity\n", 5)
	var docs []vaultcontext.Document
	for name, text := range map[string]string{
		"Phrase.md":     strings.Repeat("orchid humidity ", 9) + "trimming beds",
		"Greenhouse.md": related,
		"Bread.md":      strings.Repeat("bread dough flour cuisine ", 4),
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		docs = append(docs, vaultcontext.Document{Path: name, Text: text})
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	app.vault, app.root, app.vaultVisible, app.graph.open = vault, root, true, true
	app.entries, app.vaultFileCount, err = readWorkspaceEntries(vault, false)
	if err != nil {
		t.Fatal(err)
	}
	app.index.data, err = vaultindex.Build(vault, app.entries)
	if err != nil {
		t.Fatal(err)
	}
	index, err := vaultcontext.Build(t.Context(), docs, vaultcontext.Quick)
	if err != nil {
		t.Fatal(err)
	}
	app.aiVaultContextState().index = index
	view.Frame()
	if err := view.Click("Search Vault files"); err != nil {
		t.Fatal(err)
	}
	view.Type("or")
	if view.HasText("RELATED IDEAS") {
		t.Fatal("two characters displayed concept results")
	}
	view.Key(mygoui.Cmd, mygoui.KeyA)
	view.Type("orchid humidity")
	hits := app.sidebarSearch.related.matches
	if len(hits) != 2 || hits[0].Path != "Phrase.md" || hits[1].Path != "Greenhouse.md" ||
		len(app.sidebarSearch.results) != 1 || !view.HasText("RELATED IDEAS") {
		t.Fatalf("expected one lexical hit and two ranked passages, got %v / %v", app.sidebarSearch.results, hits)
	}
	floating := mygoui.NewTester(app.graphWindowView, 1100, 760)
	floating.Frame()
	for _, graph := range []*graphCanvas{&app.graph.sidebar, &app.graph.floating} {
		var checked bool
		for i, point := range graph.projected {
			if graph.graph.Notes[i].Path != "Greenhouse.md" {
				continue
			}
			want := float32(hits[1].Score) / float32(hits[0].Score)
			got := point.radius / point.baseRadius
			if !point.matched || math.Abs(float64(point.searchWeight-want)) > 1e-6 ||
				math.Abs(float64(got-(1+1.4*want))) > 1e-6 {
				t.Fatalf("Related Graph weight = %g, scale = %g, want %g", point.searchWeight, got, want)
			}
			checked = true
		}
		if !checked {
			t.Fatal("Graph did not project the concept-only file")
		}
	}
	label := "Greenhouse, " + hits[1].Text
	app.setMode(sourceMode)
	view.Frame()
	if err := view.Click("Search Vault files"); err != nil {
		t.Fatal(err)
	}
	for _, from := range []string{"Active.md", "Greenhouse.md"} {
		if app.document.Path != from {
			t.Fatalf("expected to start from %q, got %q", from, app.document.Path)
		}
		if err := view.Click(label); err != nil {
			t.Fatal(err)
		}
		selection := app.code.Selection()
		if app.document.Path != "Greenhouse.md" || selection.Start() != 0 || selection.End() != 0 ||
			!view.Focused("Search Vault files") {
			t.Fatalf("click from %q opened %q with selection %+v and Files focus %t",
				from, app.document.Path, selection, view.Focused("Search Vault files"))
		}
	}
	app.setMode(splitMode)
	view.Frame()
	if err := view.Click("Search Vault files"); err != nil {
		t.Fatal(err)
	}
	if err := view.Click(label); err != nil {
		t.Fatal(err)
	}
	selection := app.code.Selection()
	if app.document.Path != "Greenhouse.md" || selection.Start() != 0 || selection.End() != 0 ||
		app.draft.String() != related || !view.Focused("Search Vault files") {
		t.Fatalf("open matched %q at %+v", app.document.Path, selection)
	}
	app.setMode(wysiwygMode)
	view.Frame()
	if err := view.Click("Search Vault files"); err != nil {
		t.Fatal(err)
	}
	if err := view.Click(label); err != nil {
		t.Fatal(err)
	}
	a, z := app.wys.session.Document.Selection()
	if app.displayedMode() != wysiwygMode || a != 0 || z != 0 || !view.Focused("Search Vault files") {
		t.Errorf("same note changed visual selection to %d:%d, mode to %d, or Files focus", a, z, app.displayedMode())
	}
	if err := view.Click("Search Vault files"); err != nil {
		t.Fatal(err)
	}
	if err := view.Click("Phrase, " + hits[0].Text); err != nil {
		t.Fatal(err)
	}
	a, z = app.wys.session.Document.Selection()
	if app.document.Path != "Phrase.md" || app.displayedMode() != wysiwygMode || a != 0 || z != 0 ||
		!view.Focused("Search Vault files") {
		t.Fatalf("visual document switched to %q with selection %d:%d and Files focus %t",
			app.document.Path, a, z, view.Focused("Search Vault files"))
	}
	app.setMode(previewMode)
	view.Frame()
	if err := view.Click("Phrase, " + hits[0].Text); err != nil {
		t.Fatal(err)
	}
	if app.previewRevealLine != 0 || !view.Focused("Search Vault files") {
		t.Fatalf("same note in Preview requested line %d or moved Files focus", app.previewRevealLine)
	}
	app.setMode(splitMode)
	view.Frame()
	want, _, _ := vaultContextPrefix(hits[0].Text, 80)
	if got := app.code.SelectedText(); got != want || !view.Focused("Source editor") {
		t.Errorf("Source remount selected %q, want %q, with Source focus", got, want)
	}
	app.setMode(sourceMode)
	view.Frame()
	view.Key(mygoui.Cmd, mygoui.KeyDown)
	view.Key(mygoui.Shift, mygoui.KeyLeft)
	if app.code.SelectedText() == "" {
		t.Fatal("Source control did not select its last character")
	}
	app.setMode(previewMode)
	view.Frame()
	app.setMode(sourceMode)
	view.Frame()
	caret := app.code.Selection()
	if caret.Start() != 0 || caret.End() != 0 || app.code.ScrollY() != 0 || !view.Focused("Source editor") {
		t.Errorf("ordinary Source return kept selection %+v, scroll %g, or lost Source focus", caret, app.code.ScrollY())
	}
	app.setMode(previewMode)
	app.openVaultConceptMatch(hits[1], 0)
	if app.displayedMode() != previewMode || app.previewRevealLine != 1 {
		t.Fatalf("preview did not keep its mode and receive the excerpt's line")
	}
	conceptWeight := app.graph.highlight["Greenhouse.md"]
	app.clearAIVaultContext()
	view.Frame()
	if view.HasText("RELATED IDEAS") || app.graph.highlight["Greenhouse.md"] != conceptWeight ||
		app.graph.highlight["Phrase.md"] != 1 ||
		!view.HasText("Build vault embeddings in AI Settings to search by concept") {
		t.Fatal("clearing the local index should remove Related Ideas and retain the published graph emphasis")
	}
	app.fileSearch = "zz"
	app.vaultSearchChanged()
	if len(app.graph.highlight) != 0 {
		t.Fatal("editing the query should replace the previously published graph emphasis")
	}
}
