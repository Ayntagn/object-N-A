// spec: EV-MYGO-293 / Vault PDF Wikilinks in the native multi-file reader.
package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestNativePDFWikilinkRetainsPageAndLabel(t *testing.T) {
	const relative = "资料/02 二进制练习.pdf"
	const alias = "示例字符表"
	const token = "[[02 二进制练习.pdf#page=22|" + alias + "]]"
	const source = "参见 " + token + "。示例 `" + token + "`。"
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "资料"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), []byte("%PDF-1.4\n"), 0o600); err != nil {
		t.Fatal(err)
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
	app := &workspaceWindow{vault: vault, workspaceTab: &workspaceTab{document: workspace.Document{Path: "笔记/习题.md"}}}
	app.updateEntries(entries)
	rendered := projectNativeWikiAssets(source, parser.GFM, app.previewOrigin())
	doc := parser.New(parser.GFM)
	doc.Set(rendered)
	inlines := doc.Inlines(1)
	for i, inline := range inlines {
		if inline.Kind != parser.Link {
			continue
		}
		file, page, ok := previewPDFNavigation(inline.Dest)
		if !ok || file != relative || page != 22 || parser.PlainText(inlines[i+1:inline.Next]) != alias {
			t.Fatalf("native link destination or label is wrong: %s", rendered)
		}
		if !strings.Contains(rendered, "示例 `"+token+"`。") {
			t.Fatalf("inline code changed: %s", rendered)
		}
		return
	}
	t.Fatalf("native projection did not produce a PDF link: %s", rendered)
}
