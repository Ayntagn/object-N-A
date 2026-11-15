package ui

import (
	_ "embed"
	"encoding/json"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type workspaceAppearance string

const (
	appearanceCyber    workspaceAppearance = "cyber"
	appearanceBlackout workspaceAppearance = "blackout"
	appearancePLATO    workspaceAppearance = "plato"
	appearanceLight    workspaceAppearance = "systemLight"
	appearanceDark     workspaceAppearance = "systemDark"
	appearanceSystem   workspaceAppearance = "system" // Existing replica preference.
)

type appearanceOption struct {
	mode          workspaceAppearance
	label, symbol string
}

// EV-UI-022, EV-UI-063: the toolbar menu and Settings share these five modes.
var appearanceOptions = [...]appearanceOption{
	{appearanceCyber, "Cyber", "circle.lefthalf.filled"},
	{appearanceBlackout, "Blackout", "circle"},
	{appearancePLATO, "PLATO", "smallcircle.filled.circle"},
	{appearanceLight, "System Light", "sun.max"},
	{appearanceDark, "System Dark", "moon"},
}

func (mode workspaceAppearance) themeSource() mygo.ThemeSource {
	switch mode {
	case appearanceLight:
		return mygo.ThemeLight
	case appearanceCyber, appearanceBlackout, appearancePLATO, appearanceDark:
		return mygo.ThemeDark
	default:
		return mygo.ThemeSystem
	}
}

func (app *workspaceWindow) appAppearance() workspaceAppearance {
	if app.blackout {
		return appearanceBlackout
	}
	return app.preferences.Appearance
}

func (app *workspaceWindow) setAppAppearance(mode workspaceAppearance) {
	app.preferences.Appearance = mode
	for _, window := range app.documentWindows() {
		window.blackout = mode == appearanceBlackout
	}
	if app.win != nil && !app.win.IsDestroyed() {
		mygo.Theme.SetSource(mode.themeSource())
	}
	app.preferencesChanged()
}

func (app *workspaceWindow) appearanceButton(c *mygoui.Context) mygoui.Element {
	mode := app.appAppearance()
	symbol, help := "circle", "App Appearance"
	for _, option := range appearanceOptions {
		if option.mode == mode {
			symbol, help = option.symbol, "App Appearance · "+option.label
			break
		}
	}
	ink := c.Theme().Text
	if app.blackout {
		ink = blackoutTitleInk
	}
	if style, ok := appearancePalettes[mode]; ok {
		ink = style.muted
	}
	return chromeMenuControl(c, "Appearance", app.appearanceMenu).Tooltip(help).Children(func() {
		chromeIconColor(c, symbol, 20, ink)
	})
}

func (app *workspaceWindow) appearanceMenu(m *mygoui.Menu) {
	for _, option := range appearanceOptions {
		if m.Item(option.label).Checked(option.mode == app.appAppearance()).Chosen() {
			app.setAppAppearance(option.mode)
		}
	}
}

func (app *workspaceWindow) appearanceChoices(c *mygoui.Context) {
	selected := -1
	labels := make([]string, len(appearanceOptions))
	for i, option := range appearanceOptions {
		labels[i] = option.label
		switch option.mode {
		case appearanceLight:
			labels[i] = app.tr(option.label, "系统浅色")
		case appearanceDark:
			labels[i] = app.tr(option.label, "系统深色")
		}
		if option.mode == app.appAppearance() {
			selected = i
		}
	}
	mygoui.Row(c).FillWidth().Children(func() {
		app.settingsSegments(c, app.tr("App Appearance", "应用外观"), labels, 517, selected, func(value int) {
			app.setAppAppearance(appearanceOptions[value].mode)
		})
	})
}

type appearancePalette struct {
	background, sidebar, text, muted, border, accent mygoui.Color
}

// EV-UI-063: normalized colors from the MainWindowView and Vault call sites.
//
//go:embed themes/appearance.json
var appearancePaletteData []byte

var appearancePalettes = func() map[workspaceAppearance]appearancePalette {
	var data map[workspaceAppearance]struct {
		Background, Sidebar, Text, Muted, Border, Accent string
		BorderOpacity                                    float32
	}
	if err := json.Unmarshal(appearancePaletteData, &data); err != nil {
		panic(err)
	}
	palettes := make(map[workspaceAppearance]appearancePalette, len(data))
	for mode, palette := range data {
		palettes[mode] = appearancePalette{
			background: mygoui.Hex(palette.Background), sidebar: mygoui.Hex(palette.Sidebar),
			text: mygoui.Hex(palette.Text), muted: mygoui.Hex(palette.Muted),
			border: mygoui.Hex(palette.Border).Alpha(palette.BorderOpacity), accent: mygoui.Hex(palette.Accent),
		}
	}
	return palettes
}()

func (app *workspaceWindow) applyAppearance(c *mygoui.Context) {
	applyWorkspaceTheme(c)
	theme := *c.Theme()
	if app.blackout {
		// EV-UI-036 uses the archived dark sample's color-managed surfaces.
		theme.Background = mygoui.RGB(0, 0, 0)
		theme.Surface = blackoutSidebarFill
		theme.SurfaceHover = mygoui.RGB(34, 36, 40)
		theme.SurfacePressed = mygoui.RGB(36, 36, 36)
		theme.Border = blackoutSidebarEdge
		theme.Text = mygoui.Hex(blackoutTextCSS)
	} else if palette, ok := appearancePalettes[app.appAppearance()]; ok {
		theme.Dark = true
		theme.Background, theme.Surface = palette.background, palette.sidebar
		theme.SurfaceHover = palette.sidebar.Mix(palette.text, 0.08)
		theme.SurfacePressed = palette.sidebar.Mix(palette.text, 0.14)
		theme.Text, theme.TextMuted, theme.Border = palette.text, palette.muted, palette.border
		theme.Accent, theme.Focus = palette.accent, palette.accent
		theme.AccentText = palette.background
		theme.AccentHover = palette.accent.Mix(theme.Text, 0.2)
		theme.AccentPressed = palette.accent.Mix(palette.background, 0.15)
		theme.Selection = palette.accent.Alpha(0.4)
	} else {
		return
	}
	c.SetTheme(&theme)
	c.Root().Background(theme.Background)
}
