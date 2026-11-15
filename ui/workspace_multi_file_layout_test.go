// spec: FEAT-071 / EV-MYGO-168. Splitter changes belong to the current
// arrangement and follow the same tabs through resizing and removal.
package ui

import (
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestWorkspaceMultiFileLayout(t *testing.T) {
	var layout workspaceMultiLayout
	tabs := []*workspaceTab{{}, {}, {}, {}, {}}
	names := map[*workspaceTab]string{
		tabs[0]: "One", tabs[1]: "Two", tabs[2]: "Three", tabs[3]: "Four", tabs[4]: "Five",
	}
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		mygoui.Row(c).Fill().AlignItems(mygoui.Stretch).Children(func() {
			mygoui.Box(c).Width(140).Shrink(0)
			layout.draw(c, tabs, func(c *mygoui.Context, tab *workspaceTab) {
				mygoui.Box(c).Fill().Role(mygoui.RoleGroup).Label(names[tab])
			})
		})
	}, 1040, 600)
	frame := func(role mygoui.Role, label string) mygoui.Rect {
		t.Helper()
		info, ok := tester.AccessByRole(role, label)
		if !ok || info.Offscreen {
			t.Fatalf("missing visible %q: %+v", label, info)
		}
		return info.Bounds
	}
	pane := func(name string) mygoui.Rect {
		return frame(mygoui.RoleGroup, name)
	}
	assertFrame := func(name string, x, y, width, height float32) {
		t.Helper()
		got := pane(name)
		want := mygoui.Rect{X: x, Y: y, W: width, H: height}
		equal := func(a, b float32) bool { return a-b > -.5 && a-b < .5 }
		if !equal(got.X, want.X) || !equal(got.Y, want.Y) || !equal(got.W, want.W) || !equal(got.H, want.H) {
			t.Fatalf("%s = %+v, want %+v", name, got, want)
		}
	}
	moveDivider := func(label string, cursor mygoui.Cursor, dx, dy float32) {
		t.Helper()
		b := frame(mygoui.RoleSplitter, label)
		x, y := b.X+b.W/2, b.Y+b.H/2
		if cursor == mygoui.CursorResizeEW {
			y = b.Y + b.H/4
		}
		tester.Move(x, y)
		if got := tester.Cursor(); got != cursor {
			t.Fatalf("%s cursor = %v, want %v", label, got, cursor)
		}
		tester.Press(x, y)
		tester.Move(x+dx, y+dy)
		tester.Release(x+dx, y+dy)
	}

	// Three columns, two rows, with the last tile covering the open cell.
	assertFrame("One", 140, 0, 300, 300)
	assertFrame("Three", 740, 0, 300, 300)
	assertFrame("Four", 140, 300, 300, 300)
	assertFrame("Five", 440, 300, 600, 300)
	moveDivider("Resize columns 1 and 2", mygoui.CursorResizeEW, 60, 0)
	moveDivider("Resize rows 1 and 2", mygoui.CursorResizeNS, 0, 48)
	assertFrame("One", 140, 0, 360, 348)
	assertFrame("Two", 500, 0, 240, 348)
	assertFrame("Three", 740, 0, 300, 348)
	assertFrame("Four", 140, 348, 360, 252)
	assertFrame("Five", 500, 348, 540, 252)

	layout.arrangement = multiFileColumns
	tester.Frame()
	assertFrame("One", 140, 0, 180, 600)
	assertFrame("Five", 860, 0, 180, 600)
	moveDivider("Resize columns 2 and 3", mygoui.CursorResizeEW, 45, 0)
	assertFrame("Two", 320, 0, 225, 600)
	assertFrame("Three", 545, 0, 135, 600)
	assertFrame("Four", 680, 0, 180, 600)

	layout.arrangement = multiFileRows
	tester.Frame()
	assertFrame("Five", 140, 480, 900, 120)
	layout.arrangement = multiFileGrid
	tester.Frame()
	assertFrame("Two", 500, 0, 240, 348)
	assertFrame("Five", 500, 348, 540, 252)

	layout.arrangement = multiFileColumns
	tabs = append(tabs[:3:3], tabs[4])
	tester.Frame()
	assertFrame("Two", 365, 0, 281.25, 600)
	assertFrame("Three", 646.25, 0, 168.75, 600)
	if _, ok := tester.AccessByRole(mygoui.RoleGroup, "Four"); ok {
		t.Fatal("closed tile is still present")
	}
	tester.SetSize(320, 600)
	tester.Frame()
	assertFrame("Two", 185, 0, 56.25, 600)
	assertFrame("Three", 241.25, 0, 33.75, 600)
	for _, name := range []string{"One", "Two", "Three", "Five"} {
		if b := pane(name); b.W <= 0 || b.X+b.W > 320.5 {
			t.Fatalf("narrow workspace cropped %q: %+v", name, b)
		}
	}
	opened := &workspaceTab{}
	names[opened] = "Six"
	tabs = append(tabs, opened)
	tester.SetSize(1040, 600)
	tester.Frame()
	assertFrame("Two", 320, 0, 225, 600)
	assertFrame("Six", 860, 0, 180, 600)

	tester.Key(0, mygoui.KeyTab)
	if info, ok := tester.AccessByRole(mygoui.RoleSplitter, "Resize columns 1 and 2"); !ok || !info.Focused {
		t.Fatalf("keyboard did not reach the first divider: %+v", info)
	}
	tester.Key(0, mygoui.KeyRight)
	assertFrame("One", 140, 0, 190, 600)
	assertFrame("Two", 330, 0, 215, 600)
}
