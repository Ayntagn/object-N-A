// spec: FEAT-NOTE-DROP / EV-MYGO-278, EV-MYGO-280, EV-MYGO-285.
package ui

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestNoteDropKeepsBrowserLinkDestinationAndLabel(t *testing.T) {
	const address = "https://example.test/chapter(2)?a=1&copy;=2"
	const label = "Reference [A] *phase*"
	payload := transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.URIList, []byte(address)),
		transfer.Bytes(transfer.HTML, []byte(`<a href="https://example.test/chapter(2)?a=1&amp;copy;=2">Reference [A] *phase*</a>`)),
		transfer.Bytes(transfer.Text, []byte(address)),
	))
	tab := &workspaceTab{}
	app := &workspaceWindow{workspaceTab: tab}
	source, image := app.noteDropItemsMarkdown(tab, classifyNoteDrop(payload), nil)
	if image || source == "" {
		t.Fatalf("browser link was not inserted: %q, %q", source, app.problem)
	}
	doc := parser.New(parser.GFM)
	doc.Set(source)
	inline := doc.Inlines(1)
	if len(inline) < 2 || inline[0].Kind != parser.Link || inline[0].Dest != address ||
		parser.PlainText(inline[1:inline[0].Next]) != label || inline[0].Next != len(inline) {
		t.Fatalf("link did not keep one original address and its literal label: %q / %+v", source, inline)
	}
}

func TestNoteDropPersistsTransferredPNGWithoutOverwriting(t *testing.T) {
	root := t.TempDir()
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vault.Close() })
	note := filepath.Join(root, "Note.md")
	if err := os.WriteFile(note, []byte("Original note\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	images := filepath.Join(root, "images")
	if err := os.Mkdir(images, 0o700); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(images, "image.png")
	previous := writeImageFixture(t, original, color.RGBA{20, 45, 120, 255})
	incoming := writeImageFixture(t, filepath.Join(t.TempDir(), "source.png"), color.RGBA{200, 92, 68, 255})
	tab := &workspaceTab{document: workspace.Document{Path: "Note.md"}, storage: vault}
	app := &workspaceWindow{workspaceTab: tab}

	bad := transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, []byte("not a PNG"))))
	if source, _ := app.noteDropItemsMarkdown(tab, classifyNoteDrop(bad), nil); source != "" || app.problem == "" {
		t.Fatalf("invalid bytes produced %q, problem %q", source, app.problem)
	}
	if entries, err := os.ReadDir(images); err != nil || len(entries) != 1 {
		t.Fatalf("invalid image left an attachment: %v, %v", entries, err)
	}
	expired, err := transfer.FileData(filepath.Join(t.TempDir(), "Screenshot.png"))
	if err != nil {
		t.Fatal(err)
	}
	fileURL, err := expired.Read(transfer.FileList)
	if err != nil {
		t.Fatal(err)
	}
	payload := transfer.New(transfer.NewItem(transfer.Bytes(transfer.FileList, fileURL), transfer.Bytes(transfer.PNG, incoming),
		transfer.Bytes(transfer.URIList, []byte("https://example.test/source.png"))))
	source, image := app.noteDropItemsMarkdown(tab, classifyNoteDrop(payload), nil)
	if !image || source != "![[images/image-1.png]]" || app.problem != "" {
		t.Fatalf("browser image produced %q, problem %q", source, app.problem)
	}
	for path, want := range map[string][]byte{
		note: []byte("Original note\n"), original: previous, filepath.Join(images, "image-1.png"): incoming,
	} {
		if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
			t.Fatalf("read %q: bytes match %t, error %v", path, bytes.Equal(got, want), err)
		}
	}
}
