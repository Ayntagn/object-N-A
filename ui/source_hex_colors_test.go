// spec: EV-UI-260 / EV-UI-262, Collapse Hex Colors.
package ui

import (
	"os"
	"slices"
	"strings"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestSourceHexColorsKeepTextAndSelection(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const source = "# Colors\n\n中文🙂 #31ad70, #ab2 and #ADE018. Alpha #1234, #12345678 and #fff0. Invalid #12345, #1234567.\n"
	app, sourceView, filename := snippetsWorkspace(t, "Colors.md", source, splitMode)
	tokens := []struct {
		text  string
		color mygoui.Color
	}{
		{"#31ad70", mygoui.RGB(49, 173, 112)},
		{"#ab2", mygoui.RGB(170, 187, 34)},
		{"#ADE018", mygoui.RGB(173, 224, 24)},
		{"#1234", mygoui.Color{R: 17, G: 34, B: 51, A: 68}},
		{"#12345678", mygoui.Color{R: 18, G: 52, B: 86, A: 120}},
		{"#fff0", mygoui.Color{R: 255, G: 255, B: 255}},
	}
	checkSpans := func(collapsed bool) {
		t.Helper()
		var want []editor.TextBackground
		for _, token := range tokens {
			start := strings.Index(source, token.text)
			end := start + 1
			if collapsed {
				end = start + len(token.text)
			}
			want = append(want, editor.TextBackground{Start: start, End: end, Color: token.color})
		}
		if !slices.Equal(app.code.TextBackgrounds, want) {
			t.Fatalf("collapsed=%v, source swatches = %+v; want %+v", collapsed, app.code.TextBackgrounds, want)
		}
	}
	checkSpans(false)
	start := strings.Index(source, "#31ad70")
	app.code.Select(start+1, start+5)
	selection, version := app.code.Selection(), app.draft.Version()
	settings := mygoui.NewTester(app.generalSettings, 800, 560)
	if err := settings.Click("Collapse Hex Colors"); err != nil {
		t.Fatal(err)
	}
	sourceView.Frame()
	checkSpans(true)
	if app.code.Selection() != selection || app.draft.Version() != version || app.draft.String() != source || app.draft.Modified() {
		t.Fatal("switching swatches edited the source, moved the selection, or marked the note dirty")
	}
	if saved, err := os.ReadFile(filename); err != nil || string(saved) != source {
		t.Fatalf("switching swatches changed the source file: %v", err)
	}
	if prefs, err := loadWorkspacePreferences(); err != nil || !prefs.CollapseHexColors {
		t.Fatalf("the Collapse Hex Colors preference did not reload: %v", err)
	}
	if err := settings.Click("Collapse Hex Colors"); err != nil {
		t.Fatal(err)
	}
	sourceView.Frame()
	checkSpans(false)
}
