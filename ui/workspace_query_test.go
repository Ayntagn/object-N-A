// spec: EV-UI-158 / EV-UI-159 / EV-UI-160
package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestQueryBlockInsertionFromSourceAndSlash(t *testing.T) {
	const template = "```query\nTABLE role\nFROM #person\nSORT role ASC\n```"
	const source = "😀 Before.\n\nReplace this.\n\nAfter.\n"
	sourceApp, _, _ := snippetsWorkspace(t, "Query.md", source, sourceMode)
	start := strings.Index(source, "Replace this.")
	sourceApp.code.Select(start, start+len("Replace this."))
	action := sourceApp.commandPaletteActions()[commandPaletteKey{"BLOCKS", "Query Block"}]
	if action == nil {
		t.Fatal("Query Block is missing from the Source command palette")
	}
	action()
	want := strings.Replace(source, "Replace this.", template, 1)
	if sourceApp.draft.String() != want {
		t.Fatalf("Source inserted %q", sourceApp.draft.String())
	}
	sel := sourceApp.code.Selection()
	if sel.Start() != sel.End() || len(utf16.Encode([]rune(want[:sel.Start()]))) != len(utf16.Encode([]rune(source[:start])))+9 {
		t.Fatalf("Source selection = %+v, want caret at the first query character", sel)
	}
	sourceApp.code.Run(editor.Undo)
	if sourceApp.draft.String() != source || sourceApp.code.Selection().Start() != start {
		t.Fatalf("Source undo = %q at %+v", sourceApp.draft.String(), sourceApp.code.Selection())
	}

	app, tester, _ := snippetsWorkspace(t, "Query.md", "", wysiwygMode)
	tester.Type("/query")
	item, ok := tester.AccessByRole(mygoui.RoleMenuItem, "Query Block")
	if !ok || item.Disabled {
		t.Fatalf("slash menu item = %+v", item)
	}
	tester.Key(0, mygoui.KeyEnter)
	if app.draft.String() != template+"\n\n" {
		t.Fatalf("WYSIWYG inserted %q", app.draft.String())
	}
	if a, z := app.wys.session.Document.Selection(); a != 2 || z != 2 {
		t.Fatalf("WYSIWYG selection = %d…%d, want caret after the query", a, z)
	}
}

func TestQueryCardEditsAndOpensVaultResult(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const initial = "# Query Block\n\n~~~~QuErY experiment\nLIST FROM #person\n~~~~\n\n后续 __段落__。\n"
	app, tester, name := snippetsWorkspace(t, "Query.md", initial, wysiwygMode)
	ada := filepath.Join(filepath.Dir(name), "Ada.md")
	const adaSource = "---\ntags: [person]\nrole: Engineer\n---\n"
	if err := os.WriteFile(ada, []byte(adaSource), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := workspace.Open(filepath.Dir(name))
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	entries, err := vault.Entries()
	if err != nil {
		t.Fatal(err)
	}
	index, err := vaultindex.Build(vault, entries)
	if err != nil {
		t.Fatal(err)
	}
	app.vault, app.index.data = vault, index
	app.document, err = vault.ReadDocument("Query.md")
	if err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	click := func(role mygoui.Role, label string) {
		t.Helper()
		item, ok := tester.AccessByRole(role, label)
		if !ok || item.Disabled || item.Offscreen {
			t.Fatalf("%q is unavailable: %+v", label, item)
		}
		tester.ClickAt(item.Bounds.X+item.Bounds.W/2, item.Bounds.Y+item.Bounds.H/2)
	}
	edit := func(value string) {
		t.Helper()
		click(mygoui.RoleButton, "Edit source")
		field, ok := tester.AccessByRole(mygoui.RoleTextField, "Query source")
		if !ok || !field.Focused {
			t.Fatalf("query source focus = %+v", field)
		}
		tester.Key(mygoui.Cmd, mygoui.KeyA)
		tester.Type(value)
	}
	edit("WHERE status")
	tester.Key(0, mygoui.KeyEscape)
	if app.draft.String() != initial {
		t.Fatalf("cancel saved %q", app.draft.String())
	}
	const query = "TABLE role\nFROM #person"
	edit(query)
	if app.draft.String() != initial {
		t.Fatalf("uncommitted edit changed the note to %q", app.draft.String())
	}
	tester.Key(mygoui.Cmd, mygoui.KeyEnter)
	want := strings.Replace(initial, "LIST FROM #person", query, 1)
	if app.draft.String() != want {
		t.Fatalf("commit changed Markdown outside the source: %q", app.draft.String())
	}
	if _, ok := tester.AccessByRole(mygoui.RoleTable, "Query results"); !ok {
		t.Fatal("committed TABLE result is absent")
	}
	// The native card retains the old result on a background index refresh.
	if err := os.WriteFile(ada, []byte(strings.Replace(adaSource, "Engineer", "Architect", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	app.index.data, err = vaultindex.Build(vault, nil)
	if err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if _, ok := tester.AccessByRole(mygoui.RoleText, "Engineer"); !ok {
		t.Fatal("background index publication replaced the displayed snapshot")
	}
	click(mygoui.RoleButton, "Edit source")
	tester.Key(0, mygoui.KeyEscape)
	if _, ok := tester.AccessByRole(mygoui.RoleText, "Architect"); !ok || app.draft.String() != want {
		t.Fatal("leaving source edit did not refresh the result without changing the note")
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if app.draft.String() != initial {
		t.Fatalf("undo left %q", app.draft.String())
	}
	tester.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ)
	if app.draft.String() != want {
		t.Fatalf("redo left %q", app.draft.String())
	}
	// Shift the cursor to a sibling paragraph while the query input is active.
	// The edited source contains a fence-like line, so the old delimiter grows.
	const blurSource = query + "\n~~~~\nLIMIT 1"
	edit(blurSource)
	body, _ := tester.AccessByRole(mygoui.RoleGroup, "WYSIWYG editor")
	boxes := app.wys.session.Document.VisibleParagraphs()
	following := boxes[len(boxes)-1]
	tester.ClickAt(body.Bounds.X+180, body.Bounds.Y+following.Top+following.Height/2)
	want = strings.Replace(initial, "~~~~QuErY experiment\nLIST FROM #person\n~~~~", "~~~~~QuErY experiment\n"+blurSource+"\n~~~~~", 1)
	if app.draft.String() != want {
		t.Fatalf("blur saved %q, want %q", app.draft.String(), want)
	}
	// A tester has no native window for the post-save Vault index request.
	app.storage = vault
	app.documentEdited = false
	if !app.save() {
		t.Fatalf("save failed: %s", app.problem)
	}
	saved, err := vault.ReadDocument("Query.md")
	if err != nil || saved.Text != want {
		t.Fatalf("saved Markdown = %q, error %v", saved.Text, err)
	}
	app.storage = nil
	app.setTabDocument(app.workspaceTab, saved)
	tester.Frame()
	click(mygoui.RoleLink, "Ada")
	if app.document.Path != "Ada.md" || len(app.tabs) != 2 {
		t.Fatalf("result link opened %q with %d tabs", app.document.Path, len(app.tabs))
	}
}

func TestQueryBlockPreviewUsesSnapshot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Note & One.md"), []byte("---\ntag: fixture\nrole: '<b>Research</b>'\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	entries, err := vault.Entries()
	if err != nil {
		t.Fatal(err)
	}
	index, err := vaultindex.Build(vault, entries)
	if err != nil {
		t.Fatal(err)
	}
	source := "```query\nTABLE role\n```\n\n```query\nLIST FROM #absent\n```\n\n```query\nWHERE status\n```"
	markup := previewMarkup{doc: parser.New(webPreviewExtensions), relative: "Query.md", liveQueries: true, queryIndex: index}
	body, _ := markup.render(source, "Query", true)
	for _, fragment := range []string{`<th>File</th><th>role</th>`, `href="cyberwriter-note://open?path=Note+%26+One.md"`, "Note &amp; One", "&lt;b&gt;Research&lt;/b&gt;", "No matching notes", "Invalid query"} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("Preview missing %q in %s", fragment, body)
		}
	}
}
