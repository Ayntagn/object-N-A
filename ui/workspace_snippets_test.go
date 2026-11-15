// spec: FEAT-001 / EV-UI-054 / EV-UI-235 / EV-UI-238
package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"cyberwriter/internal/core/snippets"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestSnippetsCaptureSearchAndInsert(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CYBERWRITER_CONFIG_DIR", config)
	const content = "  func greet() {\n  print(\"你好👩‍💻\")\n  }  "
	app, tester, original := snippetsWorkspace(t, "Capture.md", content, sourceMode)
	source, found := tester.Access("Source editor")
	if !found {
		t.Fatal("source editor is missing")
	}
	tester.SetClipboard("clipboard fixture")
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.RightClickAt(source.Bounds.X+22, source.Bounds.Y+11)
	if err := tester.ChooseMenuItem("Save as Snippet"); err != nil {
		t.Fatalf("%v; menu %v", err, tester.Menu())
	}
	file := filepath.Join(config, "snippets.json")
	items, err := snippets.Load(file)
	if err != nil || len(items) != 1 {
		t.Fatalf("load capture: %v, items %d, problem %q", err, len(items), app.problem)
	}
	item := items[0]
	if item.Content != content || item.Title != "Capture" || item.SourceFileName != "Capture.md" ||
		item.SourcePageTitle != "Capture" || item.Language != "swift" || !slices.Equal(item.Tags, []string{"swift", "md"}) {
		t.Fatalf("unexpected saved selection: %+v", item)
	}
	if app.draft.String() != content || app.draft.Modified() || tester.Clipboard() != "clipboard fixture" {
		t.Fatal("saving a snippet changed the active note or clipboard")
	}

	const target = "Keep the prefixX"
	app, tester, targetFile := snippetsWorkspace(t, "Target.md", target, splitMode)
	source, _ = tester.Access("Source editor")
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.Key(0, mygoui.KeyRight)
	tester.Key(mygoui.Shift, mygoui.KeyLeft)
	if err := tester.Click("Snippets"); err != nil {
		t.Fatal(err)
	}
	// The shade starts above its hit area. Advance the tester's clock before using it.
	tester.Advance(time.Second)
	shade, found := tester.Access("Snippets Shade")
	if !found || shade.Bounds.W != 640 || shade.Bounds.H != 380 {
		t.Fatalf("shade geometry: %+v, found %v", shade.Bounds, found)
	}
	if current, _ := tester.Access("Source editor"); current.Bounds != source.Bounds {
		t.Fatalf("opening shade moved source from %+v to %+v", source.Bounds, current.Bounds)
	}
	tester.Type("not present")
	if !tester.HasText("No snippets") {
		t.Fatal("query should hide the stored item")
	}
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.Type("SWIFT")
	if err := tester.Click("Capture"); err != nil {
		t.Fatal(err)
	}
	if app.draft.String() != target {
		t.Fatal("selecting a row changed the document")
	}
	if err := tester.Click("Insert"); err != nil {
		t.Fatal(err)
	}
	const callout = "> [!note] Capture\n> ```swift\n>   func greet() {\n>   print(\"你好👩‍💻\")\n>   }  \n> ```\n"
	if got := app.draft.String(); got != target[:len(target)-1]+callout || !app.snippets.open {
		t.Fatalf("header insertion should keep the shade open and replace the selection with a callout: %q", got)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != target {
		t.Fatalf("one undo should remove the inserted callout: %q", got)
	}
	for _, action := range []struct{ label, inserted string }{
		{"Insert Embed (live)", "![[snippet:Capture]]\n"},
		{"Insert Link (hover)", "[[snippet:Capture]]\n"},
	} {
		if err := tester.Click("Snippet insertion options"); err != nil {
			t.Fatal(err)
		}
		if err := tester.ChooseMenuItem(action.label); err != nil {
			t.Fatal(err)
		}
		if got := app.draft.String(); got != target[:len(target)-1]+action.inserted {
			t.Fatalf("%s inserted %q", action.label, got)
		}
		tester.Key(mygoui.Cmd, mygoui.KeyZ)
	}
	if err := tester.RightClick("Capture"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Insert at Cursor"); err != nil {
		t.Fatal(err)
	}
	if got := app.draft.String(); got != target[:len(target)-1]+content {
		t.Fatalf("insert did not replace the current selection: %q", got)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != target {
		t.Fatalf("undo changed surrounding text: %q", got)
	}
	tester.Key(0, mygoui.KeyEscape)
	if _, found := tester.Access("Snippets Shade"); found {
		t.Fatal("Escape did not close the shade")
	}
	tester.Key(mygoui.Cmd|mygoui.Alt, mygoui.KeyS)
	tester.Advance(time.Second)
	tester.Type("SWIFT")
	tester.Key(0, mygoui.KeyEscape)
	if _, found := tester.Access("Snippets Shade"); found {
		t.Fatal("Escape did not close the shade from search")
	}
	if saved, err := os.ReadFile(original); err != nil || string(saved) != content {
		t.Fatalf("note on disk changed: %q %v", saved, err)
	}
	if saved, err := os.ReadFile(targetFile); err != nil || string(saved) != target {
		t.Fatalf("target note on disk changed: %q %v", saved, err)
	}
}

func TestSnippetsInsertMarkdownInWYSIWYG(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CYBERWRITER_CONFIG_DIR", config)
	item := snippets.Capture("**片段**", "Example.md", "Example", time.Now())
	if _, err := snippets.Append(filepath.Join(config, "snippets.json"), item); err != nil {
		t.Fatal(err)
	}
	app, tester, _ := snippetsWorkspace(t, "Visual.md", "target", wysiwygMode)
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.Key(mygoui.Cmd|mygoui.Alt, mygoui.KeyS)
	tester.Advance(time.Second)
	if err := tester.RightClick("Example"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Insert at Cursor"); err != nil {
		t.Fatalf("%v; menu %v", err, tester.Menu())
	}
	if got := app.draft.String(); got != "**片段**" {
		t.Fatalf("visual selection did not receive Markdown: %q", got)
	}
}

func TestSnippetsShadeSwitchesLibraryAndCopiesSelection(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CYBERWRITER_CONFIG_DIR", config)
	for _, fixture := range []struct{ file, title, content string }{
		{"snippets.json", "Reusable Greeting", "Hello, draft."},
		{"notes.json", "Meeting Notes", "First line\n第二行👋"},
	} {
		item := snippets.Capture(fixture.content, "", fixture.title, time.Now())
		if _, err := snippets.Append(filepath.Join(config, fixture.file), item); err != nil {
			t.Fatal(err)
		}
	}
	app, tester, _ := snippetsWorkspace(t, "Target.md", "Editor text", sourceMode)
	tester.SetClipboard("previous clipboard")
	tester.Key(mygoui.Cmd|mygoui.Alt, mygoui.KeyS)
	tester.Advance(time.Second)
	for _, label := range []string{"Notes", "Meeting Notes", "Copy"} {
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if got := tester.Clipboard(); got != "First line\n第二行👋" {
		t.Fatalf("copied note = %q", got)
	}
	if _, found := tester.Access("Reusable Greeting"); found {
		t.Fatal("the Snippets collection is visible in Notes")
	}
	if got := app.draft.String(); got != "Editor text" {
		t.Fatalf("selecting and copying the note changed the editor: %q", got)
	}
	if err := tester.Click("Pin"); err != nil {
		t.Fatal(err)
	}
	if _, found := tester.Access("Snippet list"); found {
		t.Fatal("pinned reading mode still shows the library list")
	}
	if _, found := tester.Access("Snippet title"); found {
		t.Fatal("pinned reading mode still shows an editable title")
	}
	if _, found := tester.Access("Pinned snippet"); !found {
		t.Fatal("pinned reading mode is missing its source")
	}
	tester.Key(0, mygoui.KeyEscape)
	if _, found := tester.Access("Snippets Shade"); found {
		t.Fatal("Escape did not close the pinned shade")
	}
	tester.Key(mygoui.Cmd|mygoui.Alt, mygoui.KeyS)
	tester.Advance(time.Second)
	if _, found := tester.Access("Pinned snippet"); !found {
		t.Fatal("opening the shade lost its pinned snippet")
	}
	if err := tester.Click("Insert as Callout"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(app.draft.String(), "> [!note] Meeting Notes\n") {
		t.Fatalf("the reader did not insert the selected note as a callout: %q", app.draft.String())
	}
	if _, found := tester.Access("Pinned snippet"); !found {
		t.Fatal("inserting closed the reader")
	}
	if err := tester.Click("Back to Snippets"); err != nil {
		t.Fatal(err)
	}
	if _, found := tester.Access("Meeting Notes"); !found {
		t.Fatal("the reader did not return to the selected collection")
	}
}

func TestSnippetsShadeEditsNestedRecordInPlace(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CYBERWRITER_CONFIG_DIR", config)
	file := filepath.Join(config, "snippets.json")
	fixture := []byte(`[
		{"id":"folder","title":"Examples","isFolder":true,"isExpanded":true,"tags":[],
		 "extraFolder":42,"children":[
			{"id":"example","title":"Old title","content":"one","tags":["swift"],
			 "rtfData":"Zg==","sourceURL":"file:///example.swift","extraItem":{"owned":true}}
		 ]}
	]`)
	if err := os.WriteFile(file, fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	app, tester, _ := snippetsWorkspace(t, "Target.md", "Editor text", splitMode)
	tester.Key(mygoui.Cmd|mygoui.Alt, mygoui.KeyS)
	tester.Advance(time.Second)
	for _, label := range []string{"Old title", "Snippet title"} {
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.Type("New title")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var folders []struct {
		Extra    int `json:"extraFolder"`
		Children []struct {
			snippets.Snippet
			Extra map[string]bool `json:"extraItem"`
		}
	}
	if err := json.Unmarshal(data, &folders); err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0].Extra != 42 || len(folders[0].Children) != 1 {
		t.Fatalf("editing changed folder structure or metadata: %s", data)
	}
	got := folders[0].Children[0]
	if got.Title != "New title" || got.Content != "one" || got.SourceURL != "file:///example.swift" ||
		string(got.RTFData) != "f" || !got.Extra["owned"] || !slices.Equal(got.Tags, []string{"swift"}) {
		t.Fatalf("editing changed other snippet fields: %+v", got)
	}
	if got := app.draft.String(); got != "Editor text" {
		t.Fatalf("editing the snippet changed the document: %q", got)
	}
}

func snippetsWorkspace(t *testing.T, name, content string, mode int) (*workspaceWindow, *mygoui.Tester, string) {
	t.Helper()
	root := t.TempDir()
	original := filepath.Join(root, name)
	if err := os.WriteFile(original, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vault.Close() })
	document, err := vault.ReadDocument(name)
	if err != nil {
		t.Fatal(err)
	}
	tab := &workspaceTab{mode: mode}
	app := &workspaceWindow{workspaceTab: tab, tabs: []*workspaceTab{tab}, previewZoom: 100, focusSource: true, blackout: true,
		preferences: &workspacePreferences{}}
	app.setTabDocument(tab, document)
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		// This workspace has the normal view and no platform window.
		app.wys.sync(app)
		app.documentEdited = app.dirty()
		app.view(c)
	}, 1294, 831)
	return app, tester, original
}
