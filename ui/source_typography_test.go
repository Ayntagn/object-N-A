// spec: FEAT-001 / EV-UI-055 / EV-MYGO-258 / EV-MYGO-261 / EV-MYGO-265
package ui

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestSourceTypographyMenusUpdateSource(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const note = "# Source Typography\n\nMMMMMMMM 88888888 00000000\nconst café = \"字型\"\n"
	app, tester, filename := snippetsWorkspace(t, "SourceTypography.md", note, splitMode)
	prefs, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	app.preferences = &prefs
	tester.Frame()
	expectedMaple := "Maple Mono"
	if slices.ContainsFunc(mygoui.FontFamilies(), func(family mygoui.FontFamily) bool {
		return family.Name == "Maple Mono NF CN"
	}) {
		expectedMaple = "Maple Mono NF CN"
	} else if !slices.ContainsFunc(mygoui.FontFamilies(), func(family mygoui.FontFamily) bool {
		return family.Name == expectedMaple && family.Registered && family.Monospaced
	}) {
		t.Fatal("the bundled Maple Mono fallback was not loaded")
	}
	if app.editorFontLabel() != expectedMaple || app.code.Font.Family != expectedMaple+", monospace" ||
		app.code.Font.Features != "kern=0, "+mapleMonoOpenTypeFeatures {
		t.Fatalf("fresh source font label = %q, font = %+v", app.editorFontLabel(), app.code.Font)
	}
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
	families := []string{"System Mono", expectedMaple, "JetBrains Mono", "Fira Code", "Cascadia Code", "Space Grotesk", "Nunito Sans", "Noto Sans", "-"}
	if menu := tester.Menu(); len(menu) < len(families) || !slices.Equal(menu[:len(families)], families) {
		t.Fatalf("Aa menu is missing the recommended font group, items = %d", len(menu))
	} else if slices.Contains(menu[len(families):], expectedMaple) {
		t.Fatalf("Aa menu duplicates the active Maple Mono family %q", expectedMaple)
	}
	pick("Space Grotesk")
	proportional, _ := tester.TextCaret()
	if app.code.Font.Family != "Space Grotesk, monospace" || proportional.X == mono.X {
		t.Fatalf("family and caret: %q, %v to %v", app.code.Font.Family, mono, proportional)
	}
	if current, _ := tester.Access("Source editor"); current.Bounds != viewport.Bounds || app.code.Selection() != selection {
		t.Fatalf("changed viewport or selection: %+v %+v", current.Bounds, app.code.Selection())
	}
	savedPrefs, err := loadWorkspacePreferences()
	if err != nil || savedPrefs.FontSize != 18 || savedPrefs.EditorFont != "Space Grotesk" {
		t.Fatalf("reopened preferences = %+v, %v", savedPrefs, err)
	}
	app.mode = wysiwygMode
	tester.Frame()
	if _, present := tester.Access("WYSIWYG editor"); !present || app.wys.fontSize() != 15 {
		t.Fatalf("WYSIWYG font size = %d, editor present %t", app.wys.fontSize(), present)
	}
	app.mode = splitMode
	tester.Frame()
	open("Text Formatting")
	pick("System Mono")
	if saved, err := loadWorkspacePreferences(); err != nil || saved.EditorFont != "" {
		t.Fatalf("reopened System Mono selection = %q, %v", saved.EditorFont, err)
	}

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

func TestSourceMapleMonoFollowsLocalObsidian(t *testing.T) {
	const family = "Maple Mono NF CN"
	if !slices.ContainsFunc(mygoui.FontFamilies(), func(font mygoui.FontFamily) bool {
		return font.Name == family
	}) {
		t.Skip("requires the user's locally installed Maple Mono NF CN")
	}
	config := t.TempDir()
	t.Setenv("CYBERWRITER_CONFIG_DIR", config)
	// Build 162 saved this fallback name even when the user kept its default.
	if err := os.WriteFile(filepath.Join(config, "settings.json"), []byte(`{"editorFontFamily":"Maple Mono","fontSize":15}`), 0o600); err != nil {
		t.Fatal(err)
	}
	prefs, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	const operators = "!= == >= -> =>"
	const note = "M字0　" + operators
	app, tester, filename := snippetsWorkspace(t, "Obsidian Maple Mono.md", note, sourceMode)
	app.preferences = &prefs
	tester.Frame()
	const features = "kern=0, calt, cv02, cv05, cv09, cv10, cv61, ss07, ss08, ss09, ss10, ss11"
	if app.editorFontLabel() != family || app.code.Font.Family != family+", monospace" ||
		app.code.Font.Size != 15 || app.code.Font.Features != features {
		t.Fatalf("resolved source label %q and font %+v", app.editorFontLabel(), app.code.Font)
	}
	caret, ok := tester.TextCaret()
	if !ok {
		t.Fatal("source caret is missing")
	}
	var widths []float32
	for _, r := range note {
		want := float32(9)
		if r == '字' || r == '　' {
			want = 18
		}
		tester.Key(0, mygoui.KeyRight)
		next, ok := tester.TextCaret()
		width := next.X - caret.X
		if !ok || next.Y != caret.Y || math.Abs(float64(width-want)) > 0.06 {
			t.Fatalf("caret advance = %.3f pt, want %.3f pt on the same line", width, want)
		}
		widths = append(widths, width)
		caret = next
	}
	for range operators {
		tester.Key(mygoui.Shift, mygoui.KeyLeft)
	}
	command := mygoui.Ctrl
	if runtime.GOOS == "darwin" {
		command = mygoui.Cmd
	}
	tester.Key(command, mygoui.KeyC)
	if got := tester.Clipboard(); got != operators {
		t.Fatalf("copied Maple Mono ligatures = %q, want %q", got, operators)
	}
	if disk, err := os.ReadFile(filename); err != nil || string(disk) != note || app.draft.String() != note {
		t.Fatalf("source text changed: %q %v", disk, err)
	}
	t.Logf("family=%q features=%q size=%.0f pt caret advances for %q=%v", family, features, app.code.Font.Size, note, widths)
}

func TestSourceMixedTextKeepsMonospacedAdvances(t *testing.T) {
	// EV-MYGO-259: punctuation and Han fallback must not change the advance of ASCII cells.
	const note = "A中B字i　W（M）0"
	app, tester, _ := snippetsWorkspace(t, "MixedWidths.md", note, sourceMode)
	app.preferences.EditorFont = ""
	tester.Frame()
	previous, ok := tester.TextCaret()
	if !ok {
		t.Fatal("source caret is missing")
	}
	var cell float32
	for _, r := range note {
		tester.Key(0, mygoui.KeyRight)
		next, ok := tester.TextCaret()
		if !ok || next.Y != previous.Y {
			t.Fatalf("lost caret on fixture row after %q", r)
		}
		if r < 128 {
			advance := next.X - previous.X
			if cell == 0 {
				cell = advance
			}
			if advance <= 0 || math.Abs(float64(advance-cell)) > 0.05 {
				t.Fatalf("advance after %q = %.2f, expected %.2f", r, advance, cell)
			}
		}
		previous = next
	}
	t.Logf("mixed-text ASCII advance %.2f pt", cell)
}
