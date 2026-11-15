// spec: EV-UI-255, document auto-save.
package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestAutoSaveDebouncesAndRefusesExternalRevision(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	root := t.TempDir()
	store, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	const original = "# Draft\n"
	doc, err := store.CreateDocument("Draft.md", original)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, doc.Path)
	tab := &workspaceTab{document: doc, draft: editor.NewBuffer(doc.Text), storage: store, folderRoot: root}
	app := &workspaceWindow{workspaceTab: tab, tabs: []*workspaceTab{tab}}
	tester := mygoui.NewTester(app.observeAutoSave, 320, 240)
	appendText := func(text string) {
		end := tab.draft.Len()
		tab.draft.Edit(editor.Edit{Start: end, End: end, Text: text})
		tester.Frame()
	}
	checkDisk := func(want string) {
		t.Helper()
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("disk contains %q; want %q (%v)", got, want, err)
		}
	}

	appendText("First\n")
	tester.Advance(7 * time.Second)
	appendText("Second\n")
	tester.Advance(8 * time.Second)
	checkDisk(original)
	tester.Advance(7 * time.Second)
	const saved = original + "First\nSecond\n"
	checkDisk(saved)
	if tab.draft.Modified() {
		t.Fatal("successful auto-save left the draft marked edited")
	}

	appendText("Still in the editor\n")
	const external = "# Changed outside the editor\n"
	if err := os.WriteFile(path, []byte(external), 0o600); err != nil {
		t.Fatal(err)
	}
	tester.Advance(15 * time.Second)
	checkDisk(external)
	if !strings.Contains(app.problem, workspace.ErrChangedOnDisk.Error()) ||
		tab.draft.String() != saved+"Still in the editor\n" || !tab.draft.Modified() {
		t.Fatalf("conflicting auto-save changed the draft or hid the error: %q", app.problem)
	}

	if err := os.WriteFile(path, []byte(saved), 0o600); err != nil {
		t.Fatal(err)
	}
	tester.Advance(30 * time.Second)
	checkDisk(saved)
}
