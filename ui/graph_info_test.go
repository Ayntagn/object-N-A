// spec: EV-UI-066 / EV-UI-207 / EV-MYGO-097
package ui

import (
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestGraphInfoRendersInBothHostsAndDismisses(t *testing.T) {
	app := workspaceWindow{workspaceTab: &workspaceTab{}, preferences: &workspacePreferences{}}
	app.graph.open = true
	for _, host := range []struct {
		trigger string
		view    func(*mygoui.Context)
	}{
		{"Info", app.graphSection},
		{"About the graph", app.graphWindowView},
	} {
		t.Run(host.trigger, func(t *testing.T) {
			tester := mygoui.NewTester(host.view, 1100, 760)
			click := func() {
				t.Helper()
				if err := tester.Click(host.trigger); err != nil {
					t.Fatal(err)
				}
			}
			assertOpen := func(want bool) {
				t.Helper()
				if _, open := tester.AccessByRole(mygoui.RoleText, "Knowledge Graph"); open != want {
					t.Fatalf("Knowledge Graph help open = %v, want %v", open, want)
				}
			}
			click()
			assertOpen(true)
			for _, icon := range []string{
				"Circles Arranged In A Cross Pattern", "Search", "Tag", "Circle With Circles In A Hexagonal Grid",
				"Globe", "Play", "Three-Dimensional Rotation Arrows", "Hand Tapping",
			} {
				if _, ok := tester.AccessByRole(mygoui.RoleImage, icon); !ok {
					t.Errorf("Knowledge Graph help missing %q", icon)
				}
			}
			tester.Key(0, mygoui.KeyEscape)
			assertOpen(false)
			click()
			click()
			assertOpen(false)
		})
	}
}
