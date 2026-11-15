// spec: FEAT-001 / EV-UI-055
package ui

import (
	"os"
	"runtime"
	"slices"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestSourceTypographyMenusUpdateSource(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const note = "# Source Typography\n\nMMMMMMMM 88888888 00000000\nconst café = \"字型\"\n"
	app, tester, filename := snippetsWorkspace(t, "SourceTypography.md", note, splitMode)
	viewport, _ := tester.Access("Source editor")
	if app.code.Font.Size != 14 {
		t.Fatalf("initial source font size = %g", app.code.Font.Size)
	}
	tester.Key(0, mygoui.KeyDown)
	tester.Key(0, mygoui.KeyDown)
	for range 8 {
		tester.Key(mygoui.Shift, mygoui.KeyRight)
	}
	selection := app.code.Selection()
	if app.code.SelectedText() != "MMMMMMMM" {
		t.Fatalf("fixture selection = %q", app.code.SelectedText())
	}
	caretBefore, _ := tester.TextCaret()
	open := func(name string) {
		t.Helper()
		if err := tester.Click(name); err != nil {
			t.Fatal(err)
		}
	}
	pick := func(name string) {
		t.Helper()
		if err := tester.ChooseMenuItem(name); err != nil {
			t.Fatal(err)
		}
		// Measure the text with focus in the editor after choosing a menu item.
		app.focusSource = true
		tester.Frame()
	}
	modifier := "Ctrl+"
	if runtime.GOOS == "darwin" {
		modifier = "⌘"
	}
	less := "Decrease (" + modifier + "-)"
	open("AA")
	want := []string{"Increase (" + modifier + "+)", less, "-", "Reset to Default", "-", "12 pt", "14 pt", "16 pt", "18 pt", "20 pt", "24 pt"}
	if menu := tester.Menu(); !slices.Equal(menu, want) {
		t.Fatalf("AA menu = %q", menu)
	}
	pick("18 pt")
	mono, _ := tester.TextCaret()
	if app.code.Font.Size != 18 || mono.H <= caretBefore.H {
		t.Fatalf("size and caret: %g, %v to %v", app.code.Font.Size, caretBefore, mono)
	}

	open("Text Formatting")
	families := []string{"System Mono", "JetBrains Mono", "Fira Code", "Cascadia Code", "Space Grotesk", "Nunito Sans", "Noto Sans", "-"}
	if menu := tester.Menu(); len(menu) < len(families) || !slices.Equal(menu[:len(families)], families) {
		t.Fatalf("Aa menu is missing the recommended font group, items = %d", len(menu))
	}
	pick("Space Grotesk")
	proportional, _ := tester.TextCaret()
	if app.code.Font.Family != "Space Grotesk, monospace" || proportional.X == mono.X {
		t.Fatalf("family and caret: %q, %v to %v", app.code.Font.Family, mono, proportional)
	}
	if current, _ := tester.Access("Source editor"); current.Bounds != viewport.Bounds || app.code.Selection() != selection {
		t.Fatalf("changed viewport or selection: %+v %+v", current.Bounds, app.code.Selection())
	}
	prefs, err := loadWorkspacePreferences()
	if err != nil || prefs.FontSize != 18 || prefs.EditorFont != "Space Grotesk" {
		t.Fatalf("reopened preferences = %+v, %v", prefs, err)
	}
	app.mode = wysiwygMode
	tester.Frame()
	if _, present := tester.Access("WYSIWYG editor"); !present || app.wys.fontSize() != 15 {
		t.Fatalf("WYSIWYG font size = %d, editor present %t", app.wys.fontSize(), present)
	}
	app.mode = splitMode
	tester.Frame()

	open("AA")
	pick("Reset to Default")
	if app.code.Font.Size != 14 {
		t.Fatalf("reset to %g", app.code.Font.Size)
	}
	for range 6 {
		app.changeFontSize(-1) // The global Format command has a lower limit of eight.
	}
	tester.Frame()
	open("AA")
	pick(less)
	if app.code.Font.Size != 10 {
		t.Fatalf("AA decrease at the Format lower bound yielded %g", app.code.Font.Size)
	}
	if disk, err := os.ReadFile(filename); err != nil || string(disk) != note || app.draft.String() != note {
		t.Fatalf("source text changed: %q %v", disk, err)
	}
	t.Logf("source viewport=%+v caret 14=%+v mono 18=%+v Space Grotesk 18=%+v", viewport.Bounds, caretBefore, mono, proportional)
}
