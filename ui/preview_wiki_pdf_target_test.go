// spec: EV-MYGO-292
package ui

import (
	"os"
	"path/filepath"
	"testing"

	"cyberwriter/internal/core/workspace"
)

func TestWikiPDFTargetResolvesPageAndVaultPath(t *testing.T) {
	const source = "Notes/课堂笔记.md"
	const course = "02 数制和编码.pdf"
	const textbook = "逻辑设计基础：第7版 (Fundamentals of Logic Design).pdf"
	root := t.TempDir()
	for _, relative := range []string{"Notes/" + course, "Course/" + course, "Course/" + textbook} {
		full := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	entries, err := vault.SidebarEntries(false)
	if err != nil {
		t.Fatal(err)
	}
	app := &workspaceWindow{vault: vault, workspaceTab: &workspaceTab{document: workspace.Document{Path: source}}}
	app.updateEntries(entries)
	for _, tt := range []struct {
		source, target, want string
		page                 int
	}{
		{source, course + "#page=22", "Notes/" + course, 22},
		{source, textbook + "#page=33", "Course/" + textbook, 33},
		{source, "Course/" + course, "Course/" + course, 0},
		{source, "../Course/" + course + "#page=2", "Course/" + course, 2},
		{"Other/Note.md", course, "", 0},
		{source, course + "#page=0", "", 0},
		{source, "../../" + course + "#page=22", "", 0},
	} {
		res := app.previewOrigin().resources()
		got, page, ok := res.resolveWikiPDF(tt.source, tt.target)
		if ok != (tt.want != "") || got != tt.want || page != tt.page {
			t.Errorf("resolve from %q to %q = (%q, %d, %t), want (%q, %d, %t)",
				tt.source, tt.target, got, page, ok, tt.want, tt.page, tt.want != "")
		}
	}
}
