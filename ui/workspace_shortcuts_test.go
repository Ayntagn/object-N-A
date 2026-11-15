// spec: FEAT-001 / EV-UI-029, EV-UI-065
package ui

import (
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestShortcutSheetScrollsWithFixedDismissal(t *testing.T) {
	app := &workspaceWindow{blackout: true}
	dismissals := 0
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		app.shortcutSheetView(c, func() { dismissals++ })
	}, 340, 700)
	access := func(role mygoui.Role, name string) mygoui.AccessInfo {
		t.Helper()
		node, ok := tester.AccessByRole(role, name)
		if !ok {
			t.Fatalf("missing %q", name)
		}
		return node
	}
	scroller := access(mygoui.RoleScroll, "Shortcut categories")
	closeButton := access(mygoui.RoleButton, "Close")
	if scroller.Bounds != (mygoui.Rect{X: 0, Y: 53, W: 340, H: 647}) || closeButton.Bounds.H != 17 || closeButton.Bounds.Y >= scroller.Bounds.Y {
		t.Fatalf("sheet viewport=%+v, Close=%+v", scroller.Bounds, closeButton.Bounds)
	}
	if access(mygoui.RoleHeading, "FILE").Offscreen || !access(mygoui.RoleHeading, "GENERAL").Offscreen {
		t.Fatal("unexpected initial scroll position")
	}
	for range 2 {
		if access(mygoui.RoleScroll, "Shortcut categories").Focused {
			break
		}
		tester.Key(0, mygoui.KeyTab)
	}
	tester.Key(0, mygoui.KeyPageDown)
	if !access(mygoui.RoleHeading, "FILE").Offscreen {
		t.Fatal("Page Down did not scroll the keyboard-focused body")
	}
	tester.Scroll(scroller.Bounds.X+30, scroller.Bounds.Y+20, 0, 1200)
	if access(mygoui.RoleText, "Show shortcuts").Offscreen || access(mygoui.RoleButton, "Close").Bounds != closeButton.Bounds {
		t.Fatal("the last row did not become visible with a fixed Close control")
	}
	tester.Key(0, mygoui.KeyEscape)
	if dismissals != 1 {
		t.Fatalf("Escape ran %d dismissal actions", dismissals)
	}
	if err := tester.Click("Close"); err != nil {
		t.Fatal(err)
	}
	if dismissals != 2 {
		t.Fatalf("Close did not use the same dismissal action, count=%d", dismissals)
	}
}
