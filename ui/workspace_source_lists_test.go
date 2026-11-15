// spec: FEAT-001 / EV-MYGO-094 / EV-UI-216 / EV-UI-232
package ui

import (
	"testing"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestSourceToolbarViewToggles(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const prefix = "前言 😀\n\n"
	const body = "梨 🍐\n苹果"
	const original = prefix + body + "\n"
	for _, tc := range []struct {
		name, button string
		mode         int
		outline      bool
	}{
		{"source outline", "Toggle Outline", sourceMode, true},
		{"split outline", "Toggle Outline", splitMode, true},
		{"source line numbers", "Toggle Line Numbers", sourceMode, false},
		{"split line numbers", "Toggle Line Numbers", splitMode, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, tester, _ := snippetsWorkspace(t, "View Toggles.md", original, tc.mode)
			selection := editor.Selection{Anchor: len(prefix) + len(body), Head: len(prefix)}
			app.code.Select(selection.Anchor, selection.Head)
			tester.Frame()
			before, _ := tester.AccessByRole(mygoui.RoleTextField, "Source editor")
			beforeCaret, beforeCaretOK := tester.TextCaret()
			for _, on := range []bool{true, false} {
				button, ok := tester.AccessByRole(mygoui.RoleButton, tc.button)
				if !ok || button.Offscreen || button.Disabled || button.Checked == on {
					t.Fatalf("%q unavailable or wrong state before toggle to %v: %+v", tc.button, on, button)
				}
				tester.ClickAt(button.Bounds.X+button.Bounds.W/2, button.Bounds.Y+button.Bounds.H/2)
				if got := app.draft.String(); got != original || app.draft.Modified() {
					t.Fatalf("toggling %q edited the draft to %q", tc.button, got)
				}
				if got := app.code.Selection(); got != selection {
					t.Fatalf("selection after toggling %q = %+v, want %+v", tc.button, got, selection)
				}
				source, ok := tester.AccessByRole(mygoui.RoleTextField, "Source editor")
				if !ok || !source.Focused {
					t.Fatal("source editor lost focus")
				}
				if tc.outline {
					_, visible := tester.AccessByRole(mygoui.RoleHeading, "Outline")
					if visible != on || tester.HasText("No headings") != on {
						t.Fatalf("empty Outline visibility = %v, want %v", visible, on)
					}
					if on && source.Bounds.W >= before.Bounds.W || !on && source.Bounds != before.Bounds {
						t.Fatalf("source bounds with Outline %v = %+v, initially %+v", on, source.Bounds, before.Bounds)
					}
					if on && tc.mode == splitMode {
						card, cardOK := tester.AccessByRole(mygoui.RoleGroup, "Outline")
						close, closeOK := tester.AccessByRole(mygoui.RoleButton, "Close Outline")
						if !cardOK || !closeOK || card.Bounds.X != 4 || card.Bounds.W != 200 || close.Bounds.X != 166 {
							t.Fatalf("Outline without Vault: card %+v, close %+v", card.Bounds, close.Bounds)
						}
						if source.Bounds.X != 214 || source.Bounds.W != 526.5 {
							t.Fatalf("Outline changed the split source allocation: %+v", source.Bounds)
						}
					}
				} else {
					ruler, visible := tester.AccessByRole(mygoui.RoleGroup, "Source line numbers")
					if visible != on || app.preferences.LineNumbers != on {
						t.Fatalf("line-number ruler visible=%t, preference=%t, want %t", visible, app.preferences.LineNumbers, on)
					}
					want := before.Bounds
					if on {
						if ruler.Bounds.X != before.Bounds.X || ruler.Bounds.W != 37 {
							t.Fatalf("unexpected ruler geometry: %+v", ruler.Bounds)
						}
						want.X += 37
						want.W -= 37
					}
					if source.Bounds != want {
						t.Fatalf("source bounds with ruler %t = %+v, want %+v", on, source.Bounds, want)
					}
					caret, ok := tester.TextCaret()
					if !beforeCaretOK || !ok || caret.X != beforeCaret.X+(want.X-before.Bounds.X) {
						t.Fatalf("caret with gutter %v = %+v, initially %+v", on, caret, beforeCaret)
					}
				}
				if button, ok := tester.AccessByRole(mygoui.RoleButton, tc.button); !ok || button.Checked != on {
					t.Fatalf("%q checked = %v, want %v", tc.button, button.Checked, on)
				}
			}
		})
	}
}
