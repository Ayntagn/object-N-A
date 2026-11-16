// spec: EV-UI-190 / EV-MYGO-260. Vault tags stay within the sidebar and remain selectable.
package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestVaultTagOverflowPreservesFilesAndFilter(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, view, active := snippetsWorkspace(t, "Active.md", "# Active\n", sourceMode)
	root := filepath.Dir(active)
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	longTag := "a-" + strings.Repeat("long-category-", 16) + "first"
	lastTag := "z-" + strings.Repeat("很长的分类-", 18) + "last"
	var tags strings.Builder
	fmt.Fprintln(&tags, "#"+longTag)
	for i := range 320 {
		fmt.Fprintf(&tags, "#category-%03d-synthetic-test\n", i)
	}
	fmt.Fprintln(&tags, "#"+lastTag)
	if _, err := vault.CreateDocument("Tag Deck.md", tags.String()); err != nil {
		t.Fatal(err)
	}
	entries, count, err := readWorkspaceEntries(vault, false)
	if err != nil {
		t.Fatal(err)
	}
	app.index.data, err = vaultindex.Build(vault, entries)
	if err != nil {
		t.Fatal(err)
	}
	app.vault, app.root, app.vaultVisible, app.vaultFileCount = vault, root, true, count
	app.updateEntries(entries)
	app.preferences.VaultTagsExpanded = true
	view.SetSize(720, 560)

	access := func(label string) mygoui.AccessInfo {
		t.Helper()
		item, ok := view.Access(label)
		if !ok || item.Offscreen || item.Bounds.H <= 0 {
			t.Fatalf("%q is not visible: %+v", label, item)
		}
		return item
	}
	pill := func(label string) mygoui.AccessInfo {
		t.Helper()
		item, ok := view.AccessByRole(mygoui.RoleButton, label)
		if !ok || item.Offscreen || item.Bounds.H <= 0 {
			t.Fatalf("tag %q is not visible: %+v", label, item)
		}
		return item
	}
	within := func(outer, inner mygoui.Rect) bool {
		return inner.X >= outer.X && inner.X+inner.W <= outer.X+outer.W &&
			inner.Y >= outer.Y && inner.Y+inner.H <= outer.Y+outer.H
	}
	sidebar := access("Vault sidebar").Bounds
	firstLabel, lastLabel := "#"+longTag+", 1", "#"+lastTag+", 1"
	first := pill(firstLabel).Bounds
	files := access("Vault file list").Bounds
	if !within(sidebar, first) || files.H < 48 {
		t.Fatalf("oversized tag or collapsed file viewport: sidebar=%+v tag=%+v files=%+v", sidebar, first, files)
	}
	if !within(sidebar, access("2 files").Bounds) {
		t.Fatal("tag pane pushed the Files footer outside the sidebar")
	}
	viewport := access("Vault tags").Bounds
	view.Scroll(viewport.X+viewport.W/2, viewport.Y+viewport.H/2, 0, 10000)
	last := pill(lastLabel).Bounds
	if !within(viewport, last) {
		t.Fatalf("last tag could not scroll into view: viewport=%+v tag=%+v", viewport, last)
	}
	view.ClickAt(last.X+last.W/2, last.Y+last.H/2)
	selected, filteredFiles, result := pill(lastLabel), access("Vault file list").Bounds, access("Tag Deck").Bounds
	if app.graph.tag != lastTag || !selected.Checked || !within(filteredFiles, result) {
		t.Fatalf("could not filter the Vault by the full tag name: %q, tag=%+v files=%+v result=%+v", app.graph.tag, selected, filteredFiles, result)
	}
	if err := view.Click("Clear tag filter"); err != nil || app.graph.tag != "" {
		t.Fatalf("clearing the filter failed: %v", err)
	}
	t.Logf("sidebar=%+v files=%+v tags=%+v first=%+v last=%+v", sidebar, files, viewport, first, last)

	app.graph.open, app.preferences.GraphShowStats = true, true
	app.recentPaths = []string{"Active.md"}
	app.vaultEmbeddingOfferPath, app.aiVaultContext = root, &workspaceAIVaultContext{root: root, loaded: true}
	view.SetSize(720, 420)
	sidebar = access("Vault sidebar").Bounds
	for _, label := range []string{vaultEmbeddingOfferTitle, "Recents, 1", "Vault file list", "Vault tags", "Graph", "Vault Stats panel", "2 files"} {
		if item := access(label); !within(sidebar, item.Bounds) {
			t.Fatalf("%q escapes the short sidebar: sidebar=%+v control=%+v", label, sidebar, item.Bounds)
		}
	}
	card := access("Vault embedding offer").Bounds
	view.Scroll(card.X+20, card.Y+card.H/2, 0, 200)
	if !within(card, access("Not now").Bounds) {
		t.Fatal("the compact embedding offer cannot scroll to its dismiss action")
	}
	t.Logf("short sidebar=%+v offer=%+v files=%+v tags=%+v stats=%+v footer=%+v", sidebar, card,
		access("Vault file list").Bounds, access("Vault tags").Bounds, access("Vault Stats panel").Bounds, access("2 files").Bounds)
}
