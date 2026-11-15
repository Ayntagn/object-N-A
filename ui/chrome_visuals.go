package ui

import (
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

const chromeIconSize = 14

var blackoutToolbarHoverInk = mygoui.RGB(232, 233, 243)

func toolbarPressInside(button mygoui.Element) bool {
	if _, held := button.PressedButton(); held {
		// Request frames for the drag so its press fill clears at the edge.
		button.PointerPosition()
	}
	return button.Pressed()
}

// EV-UI-220. Unselected pane buttons show a fill only while pressed.
func paneToolbarFeedback(button mygoui.Element) bool {
	fill := mygoui.Transparent
	if toolbarPressInside(button) {
		fill = mygoui.RGB(227, 226, 230).Alpha(.1)
	}
	button.Radius(4).Background(fill)
	return button.Hovered()
}

// Keep the Settings press surface within the title capsule.
func drawTitleSettingsPress(p *mygoui.Painter, frame mygoui.Rect) {
	const width, height = float32(26), float32(28)
	plate := mygoui.Rect{X: frame.X + (frame.W-width)/2, Y: frame.Y + (frame.H-height)/2, W: width, H: height}
	p.Fill(plate, mygoui.RGB(227, 226, 230).Alpha(.07), 5)
}

func chromeIcon(c *mygoui.Context, symbol string, size float32) mygoui.Element {
	return chromeIconColor(c, symbol, size, c.Theme().Text)
}

func chromeIconColor(c *mygoui.Context, symbol string, size float32, color mygoui.Color) mygoui.Element {
	return sfsymbols.Symbol(c, symbol).Size(size).Natural().Color(color).
		Fallback(chromeIconFallbacks[symbol]).Role(mygoui.RoleNone)
}

func chromeButton(c *mygoui.Context, symbol, label string, action func()) mygoui.Element {
	return chromeButtonIcon(c, symbol, label, action, chromeIconSize, c.Theme().Text)
}

func chromeButtonIcon(c *mygoui.Context, symbol, label string, action func(), size float32, color mygoui.Color, help ...string) mygoui.Element {
	button := chromeControl(c, label, action == nil, help...)
	if action != nil {
		button.OnClick(action)
	}
	return button.Children(func() { chromeIconColor(c, symbol, size, color) })
}

func chromeMenu(c *mygoui.Context, symbol, label string, build func(*mygoui.Menu)) mygoui.Element {
	return chromeMenuControl(c, label, build).Gap(2).Children(func() {
		chromeIcon(c, symbol, chromeIconSize)
		chromeIcon(c, "chevron.down", 6)
	})
}

func chromeMenuControl(c *mygoui.Context, label string, build func(*mygoui.Menu)) mygoui.Element {
	button := chromeControl(c, label, build == nil).Role(mygoui.RoleMenuButton)
	if build != nil {
		button.Menu(build)
	}
	return button
}

func chromeControl(c *mygoui.Context, label string, disabled bool, help ...string) mygoui.Element {
	theme := c.Theme()
	tip := label
	if len(help) != 0 {
		tip = help[0]
	}
	// The source toolbar's ordinary buttons measure 26 × 22 pt (EV-UI-009).
	button := mygoui.ButtonBase(c).Size(26, 22).Radius(theme.Radius).
		Label(label).Tooltip(tip).Disabled(disabled)
	switch {
	case button.Pressed():
		button.Background(theme.SurfacePressed)
	case button.Hovered():
		button.Background(theme.SurfaceHover)
	}
	return button
}
