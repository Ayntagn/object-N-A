// spec: EV-UI-199 / EV-UI-200
package ui

import (
	"os"
	"strings"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestVisualFindReplacePreservesModeAndUndo(t *testing.T) {
	const source = "# Find\n\nGo😀 goat [GO](./go.md) and **go**.\n\nAgain Go.\n"
	app, tester, note := snippetsWorkspace(t, "Find.md", source, wysiwygMode)
	control := func(role mygoui.Role, name string) mygoui.AccessInfo {
		t.Helper()
		info, ok := tester.AccessByRole(role, name)
		if !ok {
			t.Fatalf("%q is absent", name)
		}
		return info
	}
	click := func(info mygoui.AccessInfo) {
		t.Helper()
		if info.Disabled || info.Offscreen {
			t.Fatalf("%q is unavailable: %+v", info.Name, info)
		}
		tester.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	}
	selected := func() string {
		doc := app.wys.session.Document
		a, z := doc.Selection()
		return doc.Text().Slice(min(a, z), max(a, z)).String()
	}

	before := control(mygoui.RoleTextField, "WYSIWYG editor").Bounds.H
	click(control(mygoui.RoleButton, "Search"))
	if query := control(mygoui.RoleTextField, "Find in document…"); !query.Focused {
		t.Fatal("opening Find did not focus its query")
	}
	if app.displayedMode() != wysiwygMode || before-control(mygoui.RoleTextField, "WYSIWYG editor").Bounds.H != 33 {
		t.Fatalf("Find changed mode or left an unexpected viewport")
	}
	tester.Type("go")
	tester.Key(0, mygoui.KeyEnter)
	control(mygoui.RoleStatus, "1 of 5")
	if got := selected(); got != "Go" {
		t.Fatalf("first match is %q", got)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyG)
	tester.Key(mygoui.Cmd, mygoui.KeyG)
	control(mygoui.RoleStatus, "3 of 5")
	if got := selected(); got != "GO" {
		t.Fatalf("link text match is %q", got)
	}
	click(control(mygoui.RoleButton, "Two Arrows Following A Square Path"))
	if newHeight := control(mygoui.RoleTextField, "WYSIWYG editor").Bounds.H; before-newHeight != 63 {
		t.Fatalf("Find and Replace used %v points", before-newHeight)
	}
	click(control(mygoui.RoleTextField, "Replace with…"))
	tester.Type("visit")
	tester.Key(0, mygoui.KeyEnter)
	afterReturn := app.draft.String()
	if !strings.Contains(afterReturn, "[visit](./go.md)") || !strings.Contains(afterReturn, "**go**") {
		t.Fatalf("Return did not replace the current match and retain formatting: %q", afterReturn)
	}
	control(mygoui.RoleStatus, "3 of 4")
	if field := control(mygoui.RoleTextField, "Replace with…"); !field.Focused {
		t.Fatal("Return moved focus away from the replacement field")
	}
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.Type("stay")
	click(control(mygoui.RoleButton, "Replace"))
	afterReplace := app.draft.String()
	if !strings.Contains(afterReplace, "**stay**") || !strings.Contains(afterReplace, "Go😀 goat") {
		t.Fatalf("Replace did not apply the current field value to one match: %q", afterReplace)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.Type("walk")
	click(control(mygoui.RoleButton, "All"))
	control(mygoui.RoleStatus, "not found")
	if got := app.draft.String(); !strings.Contains(got, "**stay**") || !strings.Contains(got, "walk😀 walkat") {
		t.Fatalf("All did not apply the current field value or retain formatting and Unicode: %q", got)
	}
	click(control(mygoui.RoleButton, "Close"))
	if _, ok := tester.AccessByRole(mygoui.RoleTextField, "Find in document…"); ok {
		t.Fatal("Close did not hide Find")
	}
	if current := control(mygoui.RoleTextField, "WYSIWYG editor"); current.Bounds.H != before {
		t.Fatalf("closing Find did not restore the editor: %+v", current)
	}
	click(control(mygoui.RoleTextField, "WYSIWYG editor"))
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != afterReplace {
		t.Fatalf("undo did not restore the entire Replace All: %q", got)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != afterReturn {
		t.Fatalf("undo did not restore the button replacement: %q", got)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyZ)
	if got := app.draft.String(); got != source {
		t.Fatalf("undo did not restore the original Markdown: %q", got)
	}
	if got, err := os.ReadFile(note); err != nil || string(got) != source {
		t.Fatalf("editing changed the file before save: %q, %v", got, err)
	}
}
