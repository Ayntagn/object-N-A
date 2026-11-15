package ui

import (
	"time"

	mygoui "github.com/egoist/mygo/ui"
)

var blackoutSettingsAccent = mygoui.Hex("#ffaaf7")

// EV-UI-022: Settings separates the heading from the section's surface.
func (app *workspaceWindow) settingsSection(c *mygoui.Context, title, symbol string, content func()) {
	mygoui.Column(c.Key(title)).FillWidth().Gap(12).Children(func() {
		mygoui.Row(c).Height(26).Shrink(0).Gap(10).AlignItems(mygoui.Center).Children(func() {
			chip := mygoui.Box(c).Size(26, 26).Shrink(0).Center().Radius(7).
				Background(c.Theme().Surface).Border(1, c.Theme().Border).Role(mygoui.RoleNone)
			if app.blackout && !c.Preferences().HighContrast {
				chip.Background(mygoui.Hex("#1d1d20")).Border(1, mygoui.Hex("#2d2d31"))
			}
			chip.Children(func() { chromeIconColor(c, symbol, 15, c.Theme().TextMuted) })
			mygoui.Text(c, title).FontSize(14).FontWeight(650).Role(mygoui.RoleHeading).Level(2)
		})
		content()
	})
}

func (app *workspaceWindow) settingsCard(c *mygoui.Context) mygoui.Element {
	card := mygoui.Column(c).FillWidth().MinWidth(0).Padding(16).
		Radius(8).Background(c.Theme().Surface).Border(1, c.Theme().Border)
	if app.blackout && !c.Preferences().HighContrast {
		card.LinearGradient(mygoui.LinearGradient{
			From: mygoui.Hex("#252527"), To: mygoui.Hex("#17171a"), Angle: 180, End: 0.8,
		}).Border(1, mygoui.Hex("#2d2d31"))
	}
	return card
}

func settingsRow(c *mygoui.Context, title, detail, symbol string, height float32, control func()) {
	mygoui.Row(c.Key(title)).Reverse().FillWidth().MinWidth(0).MinHeight(height).
		Gap(16).AlignItems(mygoui.Center).Children(func() {
		// Give the named control priority in traversal while keeping it at the trailing edge.
		control()
		mygoui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			mygoui.Text(c, title).FontSize(13).FontWeight(600).Role(mygoui.RoleNone)
			if detail != "" {
				mygoui.Text(c, detail).FontSize(11).TextColor(c.Theme().TextMuted)
			}
		})
		if symbol != "" {
			mygoui.Box(c).Size(20, 20).Shrink(0).Center().Role(mygoui.RoleNone).Children(func() {
				chromeIconColor(c, symbol, 17, c.Theme().TextMuted)
			})
		}
	})
}

func (app *workspaceWindow) settingsSwitch(c *mygoui.Context, title, detail, symbol string, height float32, value *bool) {
	app.settingsSwitchOnChange(c, title, detail, symbol, height, value, app.preferencesChanged)
}

func (app *workspaceWindow) settingsSwitchOnChange(c *mygoui.Context, title, detail, symbol string, height float32, value *bool, changed func()) {
	settingsRow(c, title, detail, symbol, height, func() {
		if !app.blackout || c.Preferences().HighContrast {
			mygoui.Switch(c.Key(title), value).Label(title).Size(36, 20).Shrink(0).OnChange(changed)
			return
		}
		switcher := mygoui.SwitchBase(c.Key(title), value).Label(title).Size(36, 20).Shrink(0).
			FocusRing(true).OnChange(changed)
		var target float32
		if *value {
			target = 1
		}
		position := switcher.Animate("settings switch thumb", target, 140*time.Millisecond)
		switcher.Radius(10).Background(mygoui.Hex("#292a3f").Mix(blackoutSettingsAccent, position)).
			Draw(func(p *mygoui.Painter, rect mygoui.Rect) {
				thumb := mygoui.Rect{X: rect.X + 2 + (rect.W-20)*position, Y: rect.Y + 2, W: 16, H: 16}
				p.Shadow(thumb, 8, 0, 1, 3, 0, mygoui.RGBA(0, 0, 0, .23))
				p.Fill(thumb, mygoui.RGB(255, 255, 255), 8)
			})
	})
}

func (app *workspaceWindow) settingsSegments(c *mygoui.Context, label string, options []string, width float32, selected int, onChange func(int)) {
	theme := c.Theme()
	accent, ink := theme.Accent, theme.AccentText
	surface := theme.SurfaceHover
	if app.blackout && !c.Preferences().HighContrast {
		accent, ink, surface = mygoui.Hex("#428cf7"), mygoui.Hex("#ffffff"), mygoui.Hex("#303032")
	}
	mygoui.RadioGroup(c.Key(label), func() {
		for index, value := range options {
			segment := mygoui.RadioBase(c.Key(value), &selected, index).Basis(0).Grow(1).MinWidth(0).
				Height(24).Padding(0, 4).Center().Radius(5).Label(value).
				OnChange(func() { onChange(selected) })
			textColor := theme.Text
			if index == selected {
				segment.Background(accent)
				textColor = ink
			}
			segment.Children(func() {
				mygoui.Text(c, value).FontSize(12).SingleLine().Ellipsis("…").TextColor(textColor).
					Role(mygoui.RoleNone)
			})
		}
	}).Row().Gap(0).Width(width).Shrink(1).MinWidth(0).Padding(1).Radius(6).
		Background(surface).Label(label)
}
