// spec: FEAT-NOTE-DROP / EV-MYGO-279
package ui

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/transfer"
	mygoui "github.com/egoist/mygo/ui"
)

func TestVisualNoteDropUsesPointerAndOneUndoStep(t *testing.T) {
	const original = "开头😀\n\n正文结尾。\n"
	const address = "https://example.invalid/reference"
	app, tester, vault, filename := imageWorkspace(t, original)
	app.workspaceTab.storage = vault
	picked := filepath.Join(t.TempDir(), "Photo.png")
	imageBytes := writeImageFixture(t, picked, color.RGBA{200, 100, 60, 255})
	files, err := transfer.FileData(picked)
	if err != nil {
		t.Fatal(err)
	}
	drop := transfer.New(append(files.Items(), transfer.NewItem(transfer.Bytes(transfer.URIList, []byte(address))))...)

	tester.Frame()
	document := app.wys.session.Document
	editor, ok := tester.AccessByRole(mygoui.RoleTextField, "WYSIWYG editor")
	paragraphs := document.VisibleParagraphs()
	if !ok || len(paragraphs) != 2 {
		t.Fatalf("editor = %+v, paragraphs = %+v", editor, paragraphs)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	oldAnchor, oldCaret := document.Selection()
	if oldAnchor == oldCaret {
		t.Fatal("select all did not establish a prior selection")
	}
	second := paragraphs[1]
	if !tester.DropTransfer(editor.Bounds.X+12, editor.Bounds.Y+second.Top+second.Height/2, drop) {
		t.Fatal("WYSIWYG did not accept the image and link drop")
	}
	modified := app.draft.String()
	pictureAt := strings.Index(modified, "images/Photo.png")
	linkAt := strings.Index(modified, address)
	after := strings.Index(modified, "正文结尾。")
	if !strings.HasPrefix(modified, "开头😀\n\n") || pictureAt < 0 || linkAt <= pictureAt || after <= linkAt ||
		!strings.Contains(modified, "]("+address+")") {
		t.Fatalf("drop did not insert the image and link before the second paragraph: %q, problem %q", modified, app.problem)
	}
	attachment := filepath.Join(filepath.Dir(filename), "images", "Photo.png")
	if saved, err := os.ReadFile(attachment); err != nil || !bytes.Equal(saved, imageBytes) {
		t.Fatalf("attachment bytes differ: %v", err)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != original {
		t.Fatalf("one undo left %q", got)
	}
	if anchor, caret := document.Selection(); anchor != oldAnchor || caret != oldCaret {
		t.Fatalf("undo restored selection (%d, %d), want (%d, %d)", anchor, caret, oldAnchor, oldCaret)
	}
	tester.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ)
	if got := app.draft.String(); got != modified {
		t.Fatalf("one redo left %q", got)
	}
}
