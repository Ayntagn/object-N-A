package ui

import (
	"fmt"
	"slices"
	"unicode/utf8"

	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

type workspaceWYSFind struct {
	open, replacing, focus bool
	query, replacement     string
	activeQuery            string
	doc                    *richtext.Document
	version                uint64
	matches                []wysiwyg.FindRange
	index                  int // One-based. Zero means there is no current match.
}

func (f *workspaceWYSFind) update() {
	f.version = f.doc.Version()
	f.matches = wysiwyg.FindRanges(f.doc.Text(), f.activeQuery)
	f.index = 0
}

func (f *workspaceWYSFind) selectMatch(index int) {
	if len(f.matches) == 0 {
		return
	}
	f.index = index + 1
	match := f.matches[index]
	f.doc.SetSelection(match.Start, match.End)
}

// EV-UI-200. A new query uses the current start, while Next starts after it.
func (f *workspaceWYSFind) submit() {
	if f.doc == nil {
		return
	}
	if f.query == f.activeQuery {
		f.step(false)
		return
	}
	f.activeQuery = f.query
	f.update()
	a, z := f.doc.Selection()
	position := min(a, z)
	i := slices.IndexFunc(f.matches, func(m wysiwyg.FindRange) bool { return m.Start >= position })
	f.selectMatch(max(0, i))
}

func (f *workspaceWYSFind) step(backward bool) {
	if f.doc == nil || f.activeQuery == "" {
		return
	}
	if f.version != f.doc.Version() {
		f.update()
	}
	a, z := f.doc.Selection()
	start, end := min(a, z), max(a, z)
	if backward {
		i := len(f.matches) - 1
		for j := i; j >= 0; j-- {
			if f.matches[j].Start < start {
				i = j
				break
			}
		}
		f.selectMatch(i)
		return
	}
	i := slices.IndexFunc(f.matches, func(m wysiwyg.FindRange) bool {
		return m.Start > start || m.Start == start && m.End > end
	})
	f.selectMatch(max(0, i))
}

func (w *workspaceWYSIWYG) replaceFind(app *workspaceWindow, all bool) {
	if !w.editable(app) {
		return
	}
	f := &w.find
	if f.version != f.doc.Version() {
		f.update()
	}
	if len(f.matches) == 0 {
		return
	}
	if all {
		wysiwyg.ReplaceFindRanges(f.doc, f.matches, f.replacement)
	} else {
		a, z := f.doc.Selection()
		selected := wysiwyg.FindRange{Start: min(a, z), End: max(a, z)}
		if slices.Contains(f.matches, selected) {
			wysiwyg.ReplaceFindRanges(f.doc, []wysiwyg.FindRange{selected}, f.replacement)
		}
	}
	w.sync(app)
	f.update()
	if !all {
		f.step(false)
	}
}

func (f *workspaceWYSFind) status() string {
	switch {
	case f.activeQuery == "":
		return ""
	case len(f.matches) == 0:
		return "not found"
	default:
		return fmt.Sprintf("%d of %d", f.index, len(f.matches))
	}
}

func (w *workspaceWYSIWYG) closeFind() {
	w.find = workspaceWYSFind{}
}

// EV-UI-199. Keep the native-size find rows above the visual editor.
func (w *workspaceWYSIWYG) drawFind(c *mygoui.Context, app *workspaceWindow) {
	f := &w.find
	if f.doc != w.session.Document {
		f.doc = w.session.Document
	}
	if f.version != f.doc.Version() {
		f.update()
	}
	theme := c.Theme()
	blackout := app.blackout && !c.Preferences().HighContrast
	rowFill := theme.Surface
	actionFill, actionInk := theme.Surface, theme.Text
	arrowFill, arrowInk := mygoui.Transparent, theme.TextMuted
	if blackout {
		// EV-UI-199, sRGB samples from the original find and replace rows.
		rowFill = mygoui.Hex("#18181c")
		actionFill, actionInk = mygoui.Hex("#2e2e32"), mygoui.Hex("#e2e2e3")
		arrowFill, arrowInk = actionFill, actionInk
	}
	field := func(value *string, label string) mygoui.Element {
		return mygoui.TextInputBase(c.Key(value), value).Height(16).Grow(1).MinWidth(24).
			Padding(0).FontSize(13).FocusRing(false).Label(label).Placeholder(label)
	}
	button := func(label string, width, height float32, fill mygoui.Color, disabled bool, action func()) mygoui.Element {
		control := mygoui.ButtonBase(c.Key(label)).Label(label).Size(width, height).Radius(theme.Radius).
			KeepFocus().Disabled(disabled).OnClick(action)
		switch {
		case !disabled && control.Pressed():
			fill = theme.SurfacePressed
		case !disabled && control.Hovered():
			fill = theme.SurfaceHover
		}
		return control.Background(fill)
	}
	iconButton := func(icon, label string, width, height, size float32, fill, ink mygoui.Color, action func()) mygoui.Element {
		return button(label, width, height, fill, false, action).Tooltip(label).Children(func() {
			chromeIconColor(c, icon, size, ink)
		})
	}
	mygoui.Column(c.Key("WYSIWYG find")).FillWidth().Shrink(0).Role(mygoui.RoleToolbar).
		Label("Find in document").Children(func() {
		mygoui.Row(c.Key("find row")).Height(33).Gap(8).AlignItems(mygoui.Center).Padding(0, 10, 0, 11).
			Background(rowFill).Children(func() {
			mygoui.Box(c).Size(13.5, 16).Shrink(0).AlignItems(mygoui.Center).Justify(mygoui.Center).Children(func() {
				chromeIconColor(c, "magnifyingglass", 15, theme.TextMuted)
			})
			query := field(&f.query, "Find in document…").OnSubmit(f.submit)
			if f.focus {
				query.Focus().SetTextSelection(0, utf8.RuneCountInString(f.query))
				f.focus = false
			}
			if status := f.status(); status != "" {
				mygoui.Text(c, status).Role(mygoui.RoleStatus).FontSize(11).TextColor(theme.TextMuted).SingleLine()
			}
			iconButton("chevron.up", "Go Up", 37, 15.5, 12, arrowFill, arrowInk, func() { f.step(true) })
			iconButton("chevron.down", "Go Down", 37, 15.5, 12, arrowFill, arrowInk, func() { f.step(false) })
			toggleInk := theme.TextMuted
			if blackout && f.replacing {
				toggleInk = theme.Accent
			}
			toggle := iconButton("arrow.triangle.2.circlepath", "Two Arrows Following A Square Path",
				41, 20.5, 16, mygoui.Transparent, toggleInk, func() { f.replacing = !f.replacing }).AccessSelected(f.replacing)
			if blackout && f.replacing {
				toggle.Background(mygoui.Hex("#293952"))
			}
			iconButton("xmark", "Close", 9.5, 9, 12, mygoui.Transparent, theme.TextMuted, w.closeFind)
		})
		if f.replacing {
			mygoui.Row(c.Key("replace row")).Height(30).Gap(8).AlignItems(mygoui.Center).Padding(0, 9.5, 0, 11).
				Background(rowFill).Children(func() {
				mygoui.Box(c).Size(13, 16).Shrink(0).AlignItems(mygoui.Center).Justify(mygoui.Center).Children(func() {
					chromeIconColor(c, "arrow.turn.down.right", 14, theme.TextMuted)
				})
				field(&f.replacement, "Replace with…").OnSubmit(func() { w.replaceFind(app, false) })
				captionButton := func(label string, width float32, all bool) {
					control := button(label, width, 24, actionFill, len(f.matches) == 0, func() { w.replaceFind(app, all) }).Padding(0, 6)
					if !blackout {
						control.Border(1, theme.Border)
					}
					control.Children(func() {
						mygoui.Text(c, label).FontSize(13).TextColor(actionInk).SingleLine().Role(mygoui.RoleNone)
					})
				}
				captionButton("Replace", 72.5, false)
				captionButton("All", 39.5, true)
			})
		}
	})
}
