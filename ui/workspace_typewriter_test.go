// spec: user Typewriter Mode request and deathau/cm-typewriter-scroll-obsidian keyboard scrolling
package ui

import (
	"math"
	"strings"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestTypewriterFollowsEditorKeysAndLeavesPointerScrollAlone(t *testing.T) {
	app, test, _ := snippetsWorkspace(t, "Typewriter.md", strings.Repeat("Type a line of prose here.\n", 110), splitMode)
	test.SetSize(1294, 612)
	source, ok := test.AccessByRole(mygoui.RoleTextField, "Source editor")
	if !ok || !source.Focused {
		t.Fatalf("source editor did not receive focus: %+v", source)
	}

	p := app.preferences
	p.TypewriterMode, p.TypewriterPosition = true, 0.25
	app.code.GoTo(editor.Pos{Line: 50})
	test.Frame()
	start := app.code.CursorPos().Line
	test.Key(0, mygoui.KeyDown)
	if app.code.CursorPos().Line != start+1 {
		t.Fatal("Down did not move the active line")
	}
	assertAnchor := func(position float64) {
		t.Helper()
		line := app.code.CursorPos().Line
		top, bottom := app.code.LineTop(line), app.code.LineTop(line+1)
		want := position * (app.code.ViewportHeight() - (bottom - top))
		if got := top - app.code.ScrollY(); math.Abs(got-want) > 1 {
			t.Fatalf("active line top in viewport = %.1f, want %.1f", got, want)
		}
	}
	assertAnchor(0.25)

	x, y := source.Bounds.X+45, source.Bounds.Y+220
	scrollY := app.code.ScrollY()
	test.Scroll(x, y, 0, 140)
	if app.code.ScrollY() <= scrollY {
		t.Fatal("pointer scroll did not move the source viewport")
	}
	scrollY = app.code.ScrollY()
	test.Press(x, y)
	test.Move(x+65, y)
	test.Release(x+65, y)
	if got := app.code.ScrollY(); math.Abs(got-scrollY) > 1 || app.code.Selection().Start() == app.code.Selection().End() {
		t.Fatalf("pointer selection changed scroll from %.1f to %.1f or did not select text", scrollY, got)
	}

	p.TypewriterPosition = 0.75
	test.Type("🧪")
	assertAnchor(0.75)
}

func TestTypewriterSettingsPickAndRememberPosition(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	prefs, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatal(err)
	}
	app := workspaceWindow{preferences: &prefs}
	test := mygoui.NewTester(func(c *mygoui.Context) {
		mygoui.Column(c).Padding(20).Gap(12).Children(func() {
			app.typewriterSettings(c)
		})
	}, 394, 450)
	if _, ok := test.AccessByRole(mygoui.RoleSlider, "Typewriter Position"); ok {
		t.Fatal("the position preview is open with Typewriter Mode disabled")
	}
	if err := test.Click("Typewriter Mode"); err != nil {
		t.Fatal(err)
	}
	page, ok := test.AccessByRole(mygoui.RoleSlider, "Typewriter Position")
	if !ok {
		t.Fatal("enabling Typewriter Mode did not show its position preview")
	}
	test.ClickAt(page.Bounds.X+page.Bounds.W/2, page.Bounds.Y+page.Bounds.H*0.7)
	clicked := prefs.TypewriterPosition
	if clicked < 0.6 || clicked > 0.8 {
		t.Fatalf("click picked position %g, want around 70%%", clicked)
	}
	test.Key(0, mygoui.KeyUp)
	if !(prefs.TypewriterPosition < clicked) {
		t.Fatalf("Up should move the marker higher: click %g, key %g", clicked, prefs.TypewriterPosition)
	}
	if err := test.Click("Typewriter Mode"); err != nil {
		t.Fatal(err)
	}
	if _, ok := test.AccessByRole(mygoui.RoleSlider, "Typewriter Position"); ok {
		t.Fatal("the position preview remained open after disabling Typewriter Mode")
	}
	if err := test.Click("Auto-Indent Paragraphs"); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadWorkspacePreferences()
	if err != nil || loaded.TypewriterMode || loaded.TypewriterPosition != prefs.TypewriterPosition || !loaded.AutoIndentParagraphs {
		t.Fatalf("reloaded mode %t, position %g, paragraph indent %t (%v)", loaded.TypewriterMode, loaded.TypewriterPosition, loaded.AutoIndentParagraphs, err)
	}
}
