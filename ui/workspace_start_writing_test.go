// spec: FEAT-001 / EV-UI-315 / EV-UI-116. The empty editor lays out cards by pane
// width and binds custom cards to the template library.
package ui

import (
	"math"
	"testing"

	"cyberwriter/internal/core/templates"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestStartWritingGallery(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app := &workspaceWindow{preferences: &workspacePreferences{}}
	app.workspaceTab = &workspaceTab{mode: splitMode}
	app.tabs = []*workspaceTab{app.workspaceTab}
	app.setTabDocument(app.workspaceTab, workspace.Document{})

	paneWidth := float32(453.5)
	test := mygoui.NewTester(func(c *mygoui.Context) {
		mygoui.Column(c).Width(paneWidth).FillHeight().Children(func() { app.drawStartWriting(c, paneWidth) })
	}, 454, 640)
	assertCard := func(label string, x, y, width float32) {
		t.Helper()
		card, ok := test.AccessByRole(mygoui.RoleButton, label)
		if !ok || card.Offscreen || !nearStartWritingFrame(card.Bounds, mygoui.Rect{X: x, Y: y, W: width, H: 89}) {
			t.Fatalf("%s = %+v, want button at (%g, %g) with size %g × 89", label, card, x, y, width)
		}
	}
	assertCard("Blank, Empty document", 43.5, 111.5, 179)
	assertCard("README, Project docs", 231.5, 111.5, 178.5)
	assertCard("Meeting Notes, Agenda & notes", 43.5, 209.5, 179)
	assertCard("Changelog, Version history", 231.5, 307.5, 178.5)
	if _, ok := test.AccessByRole(mygoui.RoleButton, "Note, Wikilinks & tags"); ok {
		t.Fatal("the Vault Note card appeared without a Vault")
	}

	vault, err := workspace.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	app.vault = vault
	dir, err := templateDirectory()
	if err != nil {
		t.Fatal(err)
	}
	const content = "# Fixture\n\n[[Linked Note]] and a template body.\n"
	if _, err := templates.Save(dir, content); err != nil {
		t.Fatal(err)
	}
	app.templates.entries, err = templates.List(dir)
	if err != nil {
		t.Fatal(err)
	}
	paneWidth = 706.5
	test.SetSize(707, 640)
	assertCard("Blank, Empty document", 147, 111.5, 131.5)
	assertCard("Meeting Notes, Agenda & notes", 428, 111.5, 131.5)
	assertCard("Note, Wikilinks & tags", 147, 307.5, 131.5)
	assertCard("Meeting, Attendees & actions", 287.5, 307.5, 131.5)
	assertCard("Daily, Today's journal", 428, 307.5, 131.5)
	assertCard("Fixture, Custom", 147, 405.5, 131.5)
	if err := test.Click("Fixture, Custom"); err != nil {
		t.Fatal(err)
	}
	if got := app.draft.String(); got != content {
		t.Fatalf("custom card applied %q; want %q", got, content)
	}
}

func nearStartWritingFrame(got, want mygoui.Rect) bool {
	return math.Abs(float64(got.X-want.X)) <= .5 && math.Abs(float64(got.Y-want.Y)) <= .5 &&
		math.Abs(float64(got.W-want.W)) <= .5 && math.Abs(float64(got.H-want.H)) <= .5
}
