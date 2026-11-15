// spec: FEAT-WYS-MATH / EV-UI-152 / EV-UI-153
package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
)

func TestVisualInlineMathCommitsThroughItsPopover(t *testing.T) {
	const initial = "# LaTeX\n\n行内 $x^2+1$，尾句。\n\n正文包含 __粗体__。\n"
	app, tester, _ := snippetsWorkspace(t, "Inline.md", initial, wysiwygMode)
	if app.wys.session.Document == nil {
		t.Fatal("an inline formula made its document read-only")
	}
	clickMathControl(t, tester, mygoui.RoleButton, "Inline formula x^2+1")
	field, ok := tester.AccessByRole(mygoui.RoleTextField, "Inline formula source")
	if !ok || field.Focused || field.Value != "x^2+1" {
		t.Fatalf("popover after selecting formula: %+v", field)
	}
	clickMathControl(t, tester, mygoui.RoleTextField, "Inline formula source")
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.Type("x^3+2")
	tester.Key(0, mygoui.KeyEscape)
	if got, _ := tester.AccessByRole(mygoui.RoleTextField, "Inline formula source"); got.Value != "x^3+2" || app.draft.String() != initial {
		t.Fatalf("Escape changed the draft or lost the field: %+v", got)
	}
	tester.Key(0, mygoui.KeyEnter)
	want := strings.Replace(initial, "$x^2+1$", "$x^3+2$", 1)
	if app.draft.String() != want {
		t.Fatalf("Return committed %q", app.draft.String())
	}
	if field, ok := tester.AccessByRole(mygoui.RoleTextField, "Inline formula source"); !ok || field.Focused {
		t.Fatalf("commit should retain the popover with body focus: %+v", field)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if app.draft.String() != initial {
		t.Fatalf("undo restored %q", app.draft.String())
	}
	if a, z := app.wys.session.Document.Selection(); z-a != 1 {
		t.Fatalf("undo selection is %d…%d", a, z)
	}
	tester.Key(0, mygoui.KeyRight)
	tester.Key(0, mygoui.KeyLeft)
	if field, ok := tester.AccessByRole(mygoui.RoleTextField, "Inline formula source"); !ok || field.Value != "x^2+1" {
		t.Fatalf("Left should select the formula after leaving it: %+v", field)
	}
	tester.Key(0, mygoui.KeyBackspace)
	if strings.Contains(app.draft.String(), "$x^2+1$") {
		t.Fatalf("Backspace left the selected atom in %q", app.draft.String())
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if app.draft.String() != initial {
		t.Fatalf("undo deletion restored %q", app.draft.String())
	}
}

func TestVisualMathBlockInsertsUpdatesAndReopens(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const initial = "Before.\n\nAfter.\n"
	app, tester, file := snippetsWorkspace(t, "Display.md", initial, wysiwygMode)
	doc := app.wys.session.Document
	end := len("Before.")
	doc.SetSelection(end, end)
	tester.Frame()
	tester.Key(0, mygoui.KeyEnter)
	tester.Type("/math")
	tester.Key(0, mygoui.KeyEnter)
	const empty = "Before.\n\n$$\n$$\n\nAfter.\n"
	if app.draft.String() != empty {
		t.Fatalf("slash insertion left %q", app.draft.String())
	}
	source, ok := tester.AccessByRole(mygoui.RoleTextField, "LaTeX source")
	if !ok || source.Focused {
		t.Fatalf("middle insertion should keep the following paragraph focused: %+v", source)
	}
	clickMathControl(t, tester, mygoui.RoleTextField, "LaTeX source")
	tester.Type("y = x^2 + 1")
	const want = "Before.\n\n$$\ny = x^2 + 1\n$$\n\nAfter.\n"
	if app.draft.String() != want {
		t.Fatalf("editing the formula did not write through: %q", app.draft.String())
	}
	clickMathControl(t, tester, mygoui.RoleButton, "Copy")
	if got := tester.Clipboard(); got != "y = x^2 + 1" {
		t.Fatalf("Copy wrote %q", got)
	}
	clickMathControl(t, tester, mygoui.RoleButton, "Hide")
	if _, ok := tester.AccessByRole(mygoui.RoleTextField, "LaTeX source"); ok || app.draft.String() != want {
		t.Fatal("Hide should leave just the preview and preserve source")
	}
	clickMathControl(t, tester, mygoui.RoleButton, "Edit")
	vault, err := workspace.Open(filepath.Dir(file))
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	app.document, err = vault.ReadDocument("Display.md")
	if err != nil {
		t.Fatal(err)
	}
	app.storage, app.folderRoot = vault, filepath.Dir(file)
	app.documentEdited = false
	if !app.save() {
		t.Fatalf("save: %s", app.problem)
	}
	if got, err := os.ReadFile(file); err != nil || string(got) != want {
		t.Fatalf("saved %q, error %v", got, err)
	}
	saved, err := vault.ReadDocument("Display.md")
	if err != nil {
		t.Fatal(err)
	}
	app.setTabDocument(app.workspaceTab, saved)
	tester.Frame()
	if field, ok := tester.AccessByRole(mygoui.RoleTextField, "LaTeX source"); !ok || field.Value != "y = x^2 + 1" {
		t.Fatalf("reopened LaTeX source: %+v", field)
	}
	chooseLanguage := func(current, next string) {
		t.Helper()
		clickMathControl(t, tester, mygoui.RoleButton, current)
		tester.Type(next)
		clickMathControl(t, tester, mygoui.RoleOption, next)
	}
	chooseLanguage("LaTeX", "JavaScript")
	if got := app.draft.String(); got != "Before.\n\n```JavaScript\ny = x^2 + 1\n```\n\nAfter.\n" {
		t.Fatalf("changing language saved %q", got)
	}
	chooseLanguage("JavaScript", "LaTeX")
	if got := app.draft.String(); got != want {
		t.Fatalf("changing back to LaTeX saved %q", got)
	}
	clickMathControl(t, tester, mygoui.RoleTextField, "LaTeX source")
	tester.Key(mygoui.Cmd, mygoui.KeyEnter)
	if got := app.draft.String(); got != "Before.\n\n$$\ny = x^2 + 1\n$$\n\n<br />\n\nAfter.\n" {
		t.Fatalf("Cmd+Return did not make the caret paragraph: %q", got)
	}
}

func TestMathTypedAndMenuInsertions(t *testing.T) {
	app, tester, _ := snippetsWorkspace(t, "Typed.md", "", wysiwygMode)
	for _, letter := range []string{"$", "x", "$"} {
		tester.Type(letter)
	}
	doc := app.wys.session.Document
	if f, ok := wysiwyg.FormulaAt(doc.Text(), 0); !ok || f.Source != "x" || f.Block {
		t.Fatalf("closing $ did not form an inline atom: %q", doc.Text().String())
	}
	if _, ok := tester.AccessByRole(mygoui.RoleTextField, "Inline formula source"); ok {
		t.Fatal("typing a formula should leave the caret after the atom")
	}
	tester.Key(0, mygoui.KeyEnter)
	for _, letter := range []string{"$", "$", " "} {
		tester.Type(letter)
	}
	if got := app.draft.String(); got != "$x$\n\n$$\n$$\n\n" {
		t.Fatalf("typing $$ at EOF left %q", got)
	}
	if field, ok := tester.AccessByRole(mygoui.RoleTextField, "LaTeX source"); !ok || !field.Focused {
		t.Fatalf("EOF insertion should focus LaTeX source: %+v", field)
	}
	tester.Key(0, mygoui.KeyBackspace)
	if _, ok := tester.AccessByRole(mygoui.RoleTextField, "LaTeX source"); ok || strings.Contains(app.draft.String(), "$$") {
		t.Fatal("Backspace in an empty LaTeX block should restore a body paragraph")
	}

	source, sourceTester, _ := snippetsWorkspace(t, "Menu.md", "😀α", sourceMode)
	source.code.Select(0, len("😀α"))
	sourceTester.Frame()
	source.toggleCommandPalette()
	sourceTester.Frame()
	sourceTester.Type("Colored Math")
	sourceTester.Key(0, mygoui.KeyEnter)
	if got, caret := source.draft.String(), source.code.Selection().Head; got != "$😀α$:lightgreen" || caret != len("$😀α$") {
		t.Fatalf("colored command wrote %q with byte caret %d", got, caret)
	}
}

func clickMathControl(t *testing.T, tester *mygoui.Tester, role mygoui.Role, label string) {
	t.Helper()
	info, ok := tester.AccessByRole(role, label)
	if !ok || info.Offscreen || info.Disabled {
		t.Fatalf("%q unavailable: %+v", label, info)
	}
	tester.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
}
