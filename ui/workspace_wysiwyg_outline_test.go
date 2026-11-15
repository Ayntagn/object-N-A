// spec: EV-UI-208 / EV-MYGO-098
package ui

import (
	"os"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

func TestVisualOutlineJumpsToHeading(t *testing.T) {
	const heading = "目标小节 Target 🧪"
	const repeated = "开场 prose with a compass 🧭 spans lines in the WYSIWYG editor.\n\n"
	source := "---\ntitle: Outline probe\n---\n\n# 起点 Outline 🧭\n\n" + strings.Repeat(repeated, 45) +
		"## **目标小节** Target 🧪\n\n" + strings.Repeat(repeated, 35) + "# Finish 中文 🧩\n"
	app, tester, note := snippetsWorkspace(t, "Outline Jump Probe.md", source, wysiwygMode)
	click := func(label string) {
		t.Helper()
		button, ok := tester.AccessByRole(mygoui.RoleButton, label)
		if !ok || button.Disabled || button.Offscreen {
			t.Fatalf("%q unavailable: %+v", label, button)
		}
		tester.ClickAt(button.Bounds.X+button.Bounds.W/2, button.Bounds.Y+button.Bounds.H/2)
	}
	doc := app.wys.session.Document
	before, _, found := strings.Cut(doc.Text().String(), heading)
	if !found {
		t.Fatal("target heading is absent from the projected document")
	}
	target := utf8.RuneCountInString(before)
	visible := func() bool {
		return slices.ContainsFunc(doc.VisibleParagraphs(), func(box richtext.ParagraphBox) bool { return box.Start == target })
	}
	doc.SetSelection(0, 4)
	version := doc.Version()
	tester.Frame()
	click("Toggle Outline")
	if visible() {
		t.Fatal("target heading was visible before jumping")
	}
	click(heading)
	if app.displayedMode() != wysiwygMode {
		t.Fatal("clicking a heading left WYSIWYG")
	}
	if a, z := doc.Selection(); a != target || z != target || !visible() {
		t.Fatalf("target %d was not revealed with a collapsed caret: %d/%d", target, a, z)
	}
	if editor, ok := tester.AccessByRole(mygoui.RoleTextField, "WYSIWYG editor"); !ok || !editor.Focused || !app.outlineVisible {
		t.Fatal("the editor lost focus or the outline closed after jumping")
	}
	if app.wys.session.Document != doc || doc.Version() != version || app.draft.Modified() || app.draft.String() != source {
		t.Fatal("jumping changed the document or its draft")
	}
	if got, err := os.ReadFile(note); err != nil || string(got) != source {
		t.Fatalf("jumping changed the file: %v", err)
	}
}
