// spec: FEAT-WYS-IMAGE / EV-MYGO-289
package ui

import (
	"encoding/base64"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestWikiImageEmbedsResolveVaultPaths(t *testing.T) {
	const note = "Notes/Draft.md"
	const source = "![[Pasted image 20261010225632.png]]\n\n![[Assets/笔记 图片.png]]\n\n![[duplicate.png]]\n\n![[ambiguous.png]]"
	root := t.TempDir()
	files := map[string]string{}
	for i, relative := range []string{
		"Screenshots/Pasted image 20261010225632.png",
		"Assets/笔记 图片.png", "Notes/Assets/笔记 图片.png",
		"Notes/duplicate.png", "Other/duplicate.png",
		"Assets/ambiguous.png", "Other/ambiguous.png",
	} {
		full := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		data := writeImageFixture(t, full, color.RGBA{R: uint8(20 + 30*i), A: 255})
		files[relative] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
	}
	for name, text := range map[string]string{
		note: source,
		"Notes/Pasted image 20261010225632.png.md": "Misleading note with a matching stem.",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(text), 0o600); err != nil {
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
	document, err := vault.ReadDocument(note)
	if err != nil {
		t.Fatal(err)
	}
	app := &workspaceWindow{vault: vault, workspaceTab: &workspaceTab{document: document}}
	app.updateEntries(entries)
	app.index.data, err = vaultindex.Build(vault, entries)
	if err != nil {
		t.Fatal(err)
	}
	origin := app.previewOrigin()
	preview, _ := renderPreviewResources(parser.New(webPreviewExtensions), document.Text, "Draft", origin.resources(), note, false)
	export := exportDocument{kind: documentMarkdown, source: document.Text, name: "Draft", origin: origin}
	html, err := renderExportHTML(export, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, pdfBody := renderPDFHTMLWithFigures(export, nil)
	for name, body := range map[string]string{"preview": preview, "HTML": html, "PDF markup": pdfBody} {
		if strings.Count(body, `<img src="data:image/png;base64,`) != 3 {
			t.Errorf("%s did not render exactly the three resolved images", name)
		}
		for _, relative := range []string{"Screenshots/Pasted image 20261010225632.png", "Assets/笔记 图片.png", "Notes/duplicate.png"} {
			if !strings.Contains(body, files[relative]) {
				t.Errorf("%s did not embed bytes from %s", name, relative)
			}
		}
		if !strings.Contains(body, "![[ambiguous.png]]") || strings.Contains(body, "Misleading note with a matching stem.") {
			t.Errorf("%s incorrectly resolved an ambiguous link or chose a note over the image", name)
		}
	}

	native := parser.New(parser.GFM)
	native.Set(projectNativeWikiAssets(document.Text+"\n\n`![[duplicate.png]]`", parser.GFM, origin))
	first := native.Inlines(1)[0]
	if first.Kind != parser.Image || !strings.Contains(native.Source(), "`![[duplicate.png]]`") {
		t.Fatalf("native Markdown failed to project the image or changed literal code")
	}
	if _, err := origin.loadMarkdownImage(first.Dest); err != nil {
		t.Fatalf("native bitmap did not load: %v", err)
	}
}
