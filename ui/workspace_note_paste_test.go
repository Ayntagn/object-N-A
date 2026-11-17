// spec: FEAT-NOTE-DROP / EV-MYGO-291
package ui

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/transfer"
	mygoui "github.com/egoist/mygo/ui"
)

func TestPasteImageWritesWikiEmbedAndUndoesOnce(t *testing.T) {
	for _, mode := range []struct {
		name, editor string
		mode         int
	}{
		{"Split", "Source editor", splitMode},
		{"WYSIWYG", "WYSIWYG editor", wysiwygMode},
	} {
		t.Run(mode.name, func(t *testing.T) {
			t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
			const original = "Replace this text.\n"
			app, view, note := snippetsWorkspace(t, "Fixture.md", original, mode.mode)
			root := filepath.Dir(note)
			vault, err := workspace.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = vault.Close() })
			app.vault, app.root = vault, root
			view.Frame()
			if err := view.Click(mode.editor); err != nil {
				t.Fatal(err)
			}
			view.Key(mygoui.Cmd, mygoui.KeyA)
			png := writeImageFixture(t, filepath.Join(t.TempDir(), "clipboard.png"), color.RGBA{G: 180, A: 255})
			view.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, png))))
			view.Key(mygoui.Cmd, mygoui.KeyV)
			applied := strings.TrimSpace(app.draft.String())
			pattern := regexp.MustCompile("^!\\[\\[(images/Pasted image [0-9]{14}\\.png)\\]\\]$")
			match := pattern.FindStringSubmatch(applied)
			if match == nil {
				t.Fatalf("pasted note = %q, problem = %q", applied, app.problem)
			}
			if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(match[1]))); err != nil || !bytes.Equal(data, png) {
				t.Fatalf("clipboard PNG missing or altered: %v", err)
			}
			if saved, err := os.ReadFile(note); err != nil || string(saved) != original {
				t.Fatalf("paste changed the saved note before save: %q, %v", saved, err)
			}
			view.Key(mygoui.Cmd, mygoui.KeyZ)
			if app.draft.String() != original {
				t.Fatalf("one undo did not restore the note: %q", app.draft.String())
			}
		})
	}
}
