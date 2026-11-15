// spec: FEAT-WYS-CALLOUT / EV-EDITOR-004 / EV-UI-154 / EV-UI-155
package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestVisualCalloutTitleBodySaveAndReopen(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const other = "\n\n> [! NOTE] 普通引用\n\n外部 __粗体__。\n"
	const initial = "# Callout\n\n> [!NOTE] 标题甲\n> 正文甲\n\n> [!HINT]- 自定义\n> 另一张\n\n> [!info]\n>\n> 空标题正文" + other
	const changed = "# Callout\n\n> [!NOTE] 标题乙\n> 正文甲补充\n\n> [!HINT]- 自定义\n> 另一张\n\n> [!info]新标题\n>\n> 空标题正文" + other
	app, tester, path := snippetsWorkspace(t, "Callout.md", initial, wysiwygMode)
	doc := app.wys.session.Document
	if doc == nil {
		t.Fatal("a titled callout made the note read-only")
	}
	if strings.Contains(doc.Text().String(), "￼a") {
		t.Fatal("empty title contains text")
	}
	selectText := func(label string, collapse bool) {
		t.Helper()
		text := doc.Text().String()
		at := strings.Index(text, label)
		if at < 0 {
			t.Fatalf("can't select %q in %q", label, text)
		}
		first := utf8.RuneCountInString(text[:at])
		end := first + utf8.RuneCountInString(label)
		if collapse {
			first = end
		}
		doc.SetSelection(first, end)
		tester.Frame()
	}
	selectText("标题甲", false)
	tester.Type("标题乙")
	selectText("正文甲", true)
	tester.Type("补充")
	title := strings.Index(doc.Text().String(), "\n空标题正文")
	if title < 0 {
		t.Fatal("missing empty title body")
	}
	at := utf8.RuneCountInString(doc.Text().String()[:title])
	doc.SetSelection(at, at)
	tester.Frame()
	tester.Type("新标题")
	if got := app.draft.String(); got != changed {
		t.Fatalf("editing title and body left %q", got)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != strings.Replace(changed, "[!info]新标题", "[!info]", 1) {
		t.Fatalf("undo empty title changed %q", got)
	}
	tester.Key(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ)
	if got := app.draft.String(); got != changed {
		t.Fatalf("redo left %q", got)
	}
	vault, err := workspace.Open(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	app.document, err = vault.ReadDocument("Callout.md")
	if err != nil {
		t.Fatal(err)
	}
	app.storage, app.folderRoot = vault, filepath.Dir(path)
	app.documentEdited = false
	if !app.save() {
		t.Fatalf("save: %s", app.problem)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != changed {
		t.Fatalf("saved %q, error %v", data, err)
	}
	saved, err := vault.ReadDocument("Callout.md")
	if err != nil {
		t.Fatal(err)
	}
	app.setTabDocument(app.workspaceTab, saved)
	tester.Frame()
	if note, ok := tester.AccessByRole(mygoui.RoleGroup, "Note callout"); !ok || note.Offscreen {
		t.Fatalf("reopened Note group = %+v", note)
	}
	if tip, ok := tester.AccessByRole(mygoui.RoleGroup, "HINT callout"); !ok || tip.Offscreen {
		t.Fatalf("reopened Hint group = %+v", tip)
	}
}

func TestVisualCalloutInsertionAndTitleReturn(t *testing.T) {
	app, tester, _ := snippetsWorkspace(t, "Insert.md", "Before.\n\nAfter.\n", wysiwygMode)
	doc := app.wys.session.Document
	doc.SetSelection(len("Before."), len("Before."))
	tester.Frame()
	tester.Key(0, mygoui.KeyEnter)
	tester.Type("/callout")
	tester.Key(0, mygoui.KeyEnter)
	const inserted = "Before.\n\n> [!note] Note\n>\n> Content\n\nAfter.\n"
	if got := app.draft.String(); got != inserted {
		t.Fatalf("slash insertion left %q", got)
	}
	a, z := doc.Selection()
	if a != z || doc.Text().Slice(z-len("Content"), z).String() != "Content" {
		t.Fatalf("caret %d…%d should follow Content in the callout", a, z)
	}
	txt := doc.Text().String()
	title := utf8.RuneCountInString(txt[:strings.Index(txt, "Note")+len("Note")])
	doc.SetSelection(title, title)
	tester.Frame()
	tester.Key(0, mygoui.KeyEnter)
	tester.Type("新增")
	if got := app.draft.String(); got != strings.Replace(inserted, "> Content", "> 新增\n>\n> Content", 1) {
		t.Fatalf("Return from title left %q", got)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	doc.SetSelection(title-len("Note"), title-len("Note"))
	tester.Frame()
	tester.Key(0, mygoui.KeyBackspace)
	if got := app.draft.String(); got != inserted {
		t.Fatalf("Backspace at title start changed %q", got)
	}
	txt = doc.Text().String()
	body := utf8.RuneCountInString(txt[:strings.Index(txt, "Content")])
	doc.SetSelection(body, body)
	tester.Frame()
	tester.Key(0, mygoui.KeyBackspace)
	if got := app.draft.String(); got != strings.Replace(inserted, "Note\n>\n> Content", "NoteContent", 1) {
		t.Fatalf("Backspace at body start changed %q", got)
	}
	if a, z := doc.Selection(); a != title || z != title {
		t.Fatalf("merge caret = %d…%d, want %d", a, z, title)
	}
	doc.SetSelection(title-2, title-2)
	tester.Frame()
	tester.Key(0, mygoui.KeyEnter)
	if got := app.draft.String(); got != strings.Replace(inserted, "Note\n>\n> Content", "No\n>\n> teContent", 1) {
		t.Fatalf("Return inside title changed %q", got)
	}
	doc.SetSelection(doc.Text().Len(), doc.Text().Len())
	tester.Frame()
	tester.Key(0, mygoui.KeyEnter)
	tester.Type("/quote")
	tester.Key(0, mygoui.KeyEnter)
	tester.Type("[!Warning]- 自制标题")
	tester.Key(0, mygoui.KeyEnter)
	tester.Type("刚输入的正文")
	if got := app.draft.String(); !strings.HasSuffix(got, "After.\n\n> [!Warning] 自制标题\n>\n> 刚输入的正文\n") {
		t.Fatalf("Return from quote header left %q", got)
	}
}

func TestAlertMenuUsesOriginalTemplates(t *testing.T) {
	t.Run("source", func(t *testing.T) {
		app, tester, _ := snippetsWorkspace(t, "Source.md", "😀文本", sourceMode)
		app.code.Select(0, len("😀文本"))
		tester.Frame()
		if err := tester.Click("Alert"); err != nil {
			t.Fatal(err)
		}
		want := []string{"📝 Note", "💡 Tip", "ℹ️ Info", "⚠️ Warning", "🔴 Danger", "✅ Success", "❓ Question", "📋 Example", "💬 Quote", "🐛 Bug", "📄 Abstract"}
		if got := tester.Menu(); !slices.Equal(got, want) {
			t.Fatalf("Alert menu = %q, want %q", got, want)
		}
		if err := tester.ChooseMenuItem("⚠️ Warning"); err != nil {
			t.Fatal(err)
		}
		const expected = "> [!warning] Warning\n> 😀文本\n"
		if got, caret := app.draft.String(), app.code.Selection().Head; got != expected || caret != len(expected) {
			t.Fatalf("Alert inserted %q with byte caret %d", got, caret)
		}
		app.code.Select(0, 0)
		tester.Frame()
		if err := tester.Click("Alert"); err != nil {
			t.Fatal(err)
		}
		if err := tester.ChooseMenuItem("📝 Note"); err != nil {
			t.Fatal(err)
		}
		if got, caret := app.draft.String(), app.code.Selection().Head; got != "> [!note] Note\n> Content\n"+expected || caret != len("> [!note] Note\n> C") {
			t.Fatalf("empty Alert inserted %q with byte caret %d", got, caret)
		}
	})
	t.Run("visual", func(t *testing.T) {
		const original = "Before 😀 REPLACE after.\n\nNext paragraph.\n"
		app, tester, _ := snippetsWorkspace(t, "Visual.md", original, wysiwygMode)
		doc := app.wys.session.Document
		insert := func(start, end int, want string) {
			t.Helper()
			doc.SetSelection(start, end)
			tester.Frame()
			if err := tester.Click("Alert"); err != nil {
				t.Fatal(err)
			}
			if err := tester.ChooseMenuItem("📝 Note"); err != nil {
				t.Fatal(err)
			}
			if got := app.draft.String(); got != want {
				t.Fatalf("Alert [%d, %d] inserted %q, want %q", start, end, got, want)
			}
		}
		insert(7, 16, "Before Note\n\n😀 REPLACE after.\n\nNext paragraph.\n")
		tester.Key(mygoui.Cmd, mygoui.KeyZ)
		insert(0, 23, "> [!note] Note\n>\n> Before 😀 REPLACE after.\n\nNext paragraph.\n")
		tester.Key(mygoui.Cmd, mygoui.KeyZ)
		insert(7, 28, "Before \n\n> [!note] Note\n>\n> 😀 REPLACE after.\n\nNext paragraph.\n")
		if a, z := doc.Selection(); a != z || doc.Text().Slice(z-4, z).String() != "Next" {
			t.Fatalf("Alert caret = %d…%d", a, z)
		}
	})
}
