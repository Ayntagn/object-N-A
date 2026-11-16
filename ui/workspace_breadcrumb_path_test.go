// spec: FEAT-073 / EV-UI-095 / EV-UI-242 / EV-UI-274. Document path labels and targets.
package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"cyberwriter/internal/core/workspace"
)

func TestDocumentBreadcrumbsFollowActivePath(t *testing.T) {
	base := t.TempDir()
	open := func(directory string) *workspace.Vault {
		t.Helper()
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		storage, err := workspace.Open(directory)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = storage.Close() })
		return storage
	}
	vault := filepath.Join(base, "Project")
	tab := &workspaceTab{document: workspace.Document{Path: "A/Terms Phrase.md"}}
	app := &workspaceWindow{workspaceTab: tab, root: vault, vault: open(vault)}
	parts, collapsed := app.documentBreadcrumbs()
	want := []workspaceBreadcrumb{
		{label: "Project", target: vault, home: true},
		{label: "A", target: filepath.Join(vault, "A")},
		{label: "Terms Phrase", target: filepath.Join(vault, "A", "Terms Phrase.md"), file: true},
	}
	if !slices.Equal(parts, want) || collapsed != "" {
		t.Fatalf("Vault breadcrumbs = %+v, collapsed = %q", parts, collapsed)
	}

	tab.document.Path = "A/B/C/Terms Phrase.md"
	parts, collapsed = app.documentBreadcrumbs()
	want = []workspaceBreadcrumb{
		{label: "B", target: filepath.Join(vault, "A", "B")},
		{label: "C", target: filepath.Join(vault, "A", "B", "C")},
		{label: "Terms Phrase", target: filepath.Join(vault, "A", "B", "C", "Terms Phrase.md"), file: true},
	}
	if !slices.Equal(parts, want) || collapsed != filepath.Join(vault, "A", "B", "C", "Terms Phrase.md") {
		t.Fatalf("deep Vault breadcrumbs = %+v, collapsed = %q", parts, collapsed)
	}

	external := filepath.Join(base, "External", "Vault")
	tab.folderRoot = filepath.Join(external, "Content")
	tab.storage = open(tab.folderRoot)
	tab.document.Path = "Terms Phrase.md"
	parts, collapsed = app.documentBreadcrumbs()
	want = []workspaceBreadcrumb{
		{label: "Vault", target: external},
		{label: "Content", target: tab.folderRoot},
		{label: "Terms Phrase", target: filepath.Join(tab.folderRoot, "Terms Phrase.md"), file: true},
	}
	if !slices.Equal(parts, want) || collapsed != filepath.Join(tab.folderRoot, "Terms Phrase.md") {
		t.Fatalf("detached breadcrumbs = %+v, collapsed = %q", parts, collapsed)
	}
	for _, name := range []string{"Terms Phrase.markdown", "Terms Phrase.MD", ".md"} {
		tab.document.Path = name
		parts, _ = app.documentBreadcrumbs()
		if len(parts) != 3 || parts[2].label != name || parts[2].target != filepath.Join(tab.folderRoot, name) {
			t.Fatalf("breadcrumbs for %q = %+v", name, parts)
		}
	}
	tab.document.Path = "Terms Phrase.md"
	if home, err := os.UserHomeDir(); err == nil && filepath.IsAbs(home) {
		// The model only reads path values, so no files in the Home directory are opened.
		tab.folderRoot = filepath.Join(home, "Content")
		parts, collapsed = app.documentBreadcrumbs()
		if len(parts) != 3 || parts[0].label != "~" || !parts[0].home ||
			parts[0].target != filepath.Clean(home) || collapsed != "" {
			t.Fatal("a shallow home-relative file did not stop at ~")
		}
	}
	tab.document.Path = ""
	if parts, collapsed := app.documentBreadcrumbs(); len(parts) != 0 || collapsed != "" {
		t.Fatalf("unsaved draft breadcrumbs = %+v, collapsed = %q", parts, collapsed)
	}
}
