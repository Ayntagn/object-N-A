// spec: FEAT-NOTE-DROP / EV-MYGO-283.
package ui

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/transfer"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestBrowserImageDropPublishesAttachmentBeforeReference(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const original = "First marker.\n\nKeep this paragraph.\n"
	const address = "https://example.test/render?size=medium"
	app, view, note := snippetsWorkspace(t, "Drop.md", original, splitMode)
	root := filepath.Dir(note)
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	app.vault, app.root = vault, root
	view.Frame()
	zone, ok := view.AccessByRole(mygoui.RoleTextField, "Source editor")
	if !ok {
		t.Fatal("Source editor is missing")
	}
	x, y := zone.Bounds.X+zone.Bounds.W/2, zone.Bounds.Y+8
	png := writeImageFixture(t, filepath.Join(t.TempDir(), "fixture.png"), color.RGBA{G: 90, A: 255})
	requests := 0
	app.noteDropDownload = func(_ context.Context, url string) ([]byte, error) {
		requests++
		if url != address || app.draft.String() != original {
			t.Fatalf("fetch used the wrong source or changed the draft before receiving the file: %q", url)
		}
		if requests == 1 {
			return nil, errors.New("image fetch failed")
		}
		return png, nil
	}
	payload := transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.URIList, []byte("https://example.test/article")),
		transfer.Bytes(transfer.HTML, []byte(`<a href="https://example.test/article"><img src="`+address+`" alt="Example image"></a>`)),
	))
	if !view.DropTransfer(x, y, payload) || requests != 1 || app.draft.String() != original || app.problem == "" {
		t.Fatalf("failed download inserted a reference or missed feedback: requests=%d, draft=%q, problem=%q", requests, app.draft.String(), app.problem)
	}
	if _, err := os.Stat(filepath.Join(root, "images")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed download created an attachment: %v", err)
	}
	zone, ok = view.AccessByRole(mygoui.RoleTextField, "Source editor")
	if !ok {
		t.Fatal("Source editor is missing after the error")
	}
	x, y = zone.Bounds.X+zone.Bounds.W/2, zone.Bounds.Y+8
	if !view.DropTransfer(x, y, payload) || requests != 2 {
		t.Fatalf("drop did not retry the image: requests=%d", requests)
	}
	applied := app.draft.String()
	if !strings.Contains(applied, "![[images/render.png|Example image]]") || strings.Contains(applied, "https://example.test") {
		t.Fatalf("drop did not insert a local image: %q", applied)
	}
	if data, err := os.ReadFile(filepath.Join(root, "images", "render.png")); err != nil || !bytes.Equal(data, png) {
		t.Fatalf("downloaded attachment missing or altered: %v", err)
	}
	html, _ := renderPreviewResources(parser.New(webPreviewExtensions), applied, "Drop",
		app.previewOrigin().resources(), app.document.Path, false)
	if !strings.Contains(html, `src="data:image/png;base64,`) {
		t.Fatalf("preview did not resolve the local image: %s", html)
	}
	view.Frame()
	view.Key(mygoui.Cmd, mygoui.KeyZ)
	if app.draft.String() != original {
		focus, _ := view.AccessByRole(mygoui.RoleTextField, "Source editor")
		t.Fatalf("one undo did not restore the original note: %q, target %+v, undo available %t",
			app.draft.String(), focus, app.draft.CanUndo())
	}
}
