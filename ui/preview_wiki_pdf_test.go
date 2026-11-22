// spec: FEAT-001 / EV-MYGO-292
package ui

import (
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestPreviewResolvesWikiPDFLinksToPages(t *testing.T) {
	const note = "Notes/Draft.md"
	const lecture = "02 数制和编码.pdf"
	const textbook = "逻辑设计基础：第7版 = Fundamentals of Logic Design (7th Edition) (z-library.sk, 1lib.sk, z-lib.sk).pdf"
	const missing = "[[缺少的附件.pdf#page=4|缺少的附件]]"
	const literal = "[[" + lecture + "#page=22|示例]]"
	source := "查 ASCII 表。[[" + lecture + "#page=22|课件 ASCII 表]] · [[" + textbook + "#page=33|教材表 1-3]]\n\n" + missing + " `" + literal + "`\n"
	root := t.TempDir()
	for name, body := range map[string]string{
		note: source, "Slides/" + lecture: "%PDF-1.4\n%%EOF\n", "Books/" + textbook: "%PDF-1.4\n%%EOF\n",
	} {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
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
	doc, err := vault.ReadDocument(note)
	if err != nil {
		t.Fatal(err)
	}
	app := &workspaceWindow{vault: vault, workspaceTab: &workspaceTab{document: doc}}
	app.updateEntries(entries)
	origin := app.previewOrigin()
	preview, _ := renderPreviewResources(parser.New(webPreviewExtensions), source, "Draft", origin.resources(), note, false)
	document := exportDocument{name: "Draft", kind: documentMarkdown, source: source, origin: origin}
	exported, err := renderExportHTML(document, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, printed := renderPDFHTML(document)
	for _, link := range []struct {
		path, label string
		page        int
	}{
		{"Slides/" + lecture, "课件 ASCII 表", 22},
		{"Books/" + textbook, "教材表 1-3", 33},
	} {
		wantURL := fmt.Sprintf(`cyberwriter-pdf://open?page=%d&amp;path=%s`, link.page, url.QueryEscape(link.path))
		if !strings.Contains(preview, `href="`+wantURL+`">`+link.label+`</a>`) {
			t.Errorf("preview omitted %q for %q in %s", wantURL, link.label, preview)
		}
		path, page, ok := previewPDFNavigation(html.UnescapeString(wantURL))
		if !ok || path != link.path || page != link.page {
			t.Errorf("navigation returned %q, %d, %t", path, page, ok)
		}
		portableURL := url.URL{Path: link.path, Fragment: fmt.Sprintf("page=%d", link.page)}
		if !strings.Contains(exported, `href="`+html.EscapeString(portableURL.String())+`">`+link.label+`</a>`) ||
			!strings.Contains(printed, link.label) {
			t.Errorf("export omitted PDF reference %q", link.label)
		}
	}
	if !strings.Contains(preview, missing) || !strings.Contains(preview, "<code>"+literal+"</code>") {
		t.Errorf("preview did not preserve missing and literal PDF links in %s", preview)
	}
	if strings.Contains(exported, "cyberwriter-pdf:") || strings.Contains(printed, "cyberwriter-pdf:") {
		t.Error("export included an app-only PDF URL")
	}
}
