// spec: FEAT-NOTE-DROP / EV-MYGO-278
package ui

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/transfer"
	mygoui "github.com/egoist/mygo/ui"
)

func TestSourceNoteDropInsertsMixedItemsAtPointerAndUndoesOnce(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const first = "起始🙂 Start"
	const rest = "\n\nKeep this paragraph.\n"
	const source = first + rest
	const address = "https://example.test/page"
	app, view, note := snippetsWorkspace(t, "Drop.md", source, splitMode)
	root := filepath.Dir(note)
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	app.vault, app.root = vault, root
	view.Frame()

	imageFile := filepath.Join(t.TempDir(), "sample image.png")
	imageData := writeImageFixture(t, imageFile, color.RGBA{R: 180, A: 255})
	file, err := transfer.FileData(imageFile)
	if err != nil {
		t.Fatal(err)
	}
	link := transfer.NewItem(transfer.Bytes(transfer.URIList, []byte(address)))
	payload := transfer.New(append(file.Items(), link)...)

	zone, ok := view.AccessByRole(mygoui.RoleTextField, "Source editor")
	if !ok || zone.Offscreen {
		t.Fatalf("Source editor unavailable: %+v", zone)
	}
	x, y := zone.Bounds.X+zone.Bounds.W/2, zone.Bounds.Y+8
	view.ClickAt(x, y)
	view.Key(mygoui.Cmd, mygoui.KeyA)
	selected := app.code.Selection()
	if selected.Start() != 0 || selected.End() != len(source) {
		t.Fatalf("fixture did not establish a selection: %+v", selected)
	}
	if !view.DropTransfer(x, y, payload) {
		t.Fatal("Markdown source editor did not take the native drop")
	}
	got := app.draft.String()
	middle, positioned := strings.CutPrefix(got, first+"\n\n")
	middle, separated := strings.CutSuffix(middle, rest)
	image, linkMarkup, split := strings.Cut(middle, "\n\n")
	if !positioned || !separated || !split || image != "![[images/sample image.png]]" || !strings.Contains(linkMarkup, address) {
		t.Fatalf("image and link did not appear together at the end of the first line: %q", got)
	}
	if imported, err := os.ReadFile(filepath.Join(root, "images", filepath.Base(imageFile))); err != nil || !bytes.Equal(imported, imageData) {
		t.Fatalf("drop did not import the attachment bytes: %v", err)
	}
	view.Key(mygoui.Cmd, mygoui.KeyZ)
	if app.draft.String() != source || app.code.Selection() != selected || app.draft.CanUndo() {
		t.Fatalf("undo did not restore the original text and selection: %q, %+v", app.draft.String(), app.code.Selection())
	}
	if saved, err := os.ReadFile(note); err != nil || string(saved) != source {
		t.Fatalf("drop changed the saved note before save: %q, %v", saved, err)
	}
}

func TestSourceNoteDropTargetsNonactiveMultiFileTile(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const first = "First note.\n"
	const second = "Second line.\n\nKeep this.\n"
	const address = "https://example.test/multi-file"
	app, view, note := snippetsWorkspace(t, "First.md", first, sourceMode)
	original := app.workspaceTab
	root := filepath.Dir(note)
	if err := os.WriteFile(filepath.Join(root, "Second.md"), []byte(second), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	app.vault, app.root = vault, root
	app.openNote("Second.md", true)
	target := app.workspaceTab
	app.toggleMultiFile()
	view.Frame()
	firstEditor, firstOK := view.AccessByRole(mygoui.RoleTextField, "Source editor — First.md")
	secondEditor, secondOK := view.AccessByRole(mygoui.RoleTextField, "Source editor — Second.md")
	if !firstOK || !secondOK || firstEditor.Offscreen || secondEditor.Offscreen {
		t.Fatal("fixture did not mount both source editors")
	}
	view.ClickAt(firstEditor.Bounds.X+20, firstEditor.Bounds.Y+8)
	if app.workspaceTab != original {
		t.Fatal("fixture did not activate the first tile")
	}
	payload := transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte(address))))
	x, y := secondEditor.Bounds.X+secondEditor.Bounds.W/2, secondEditor.Bounds.Y+8
	if !view.DropTransfer(x, y, payload) || app.workspaceTab != target || original.draft.String() != first {
		t.Fatalf("drop did not target the second tile: %q", app.document.Path)
	}
	want := "Second line.[" + address + "](" + address + ")\n\nKeep this.\n"
	if target.draft.String() != want {
		t.Fatalf("second tile changed at the wrong location: %q", target.draft.String())
	}
	view.Key(mygoui.Cmd, mygoui.KeyZ)
	if target.draft.String() != second {
		t.Fatalf("second editor did not receive keyboard undo: %q", target.draft.String())
	}
}
