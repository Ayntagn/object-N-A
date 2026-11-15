package ui

import (
	"fmt"
	"runtime"
	"strings"

	"cyberwriter/internal/core/commandpalette"
	mygoui "github.com/egoist/mygo/ui"
)

type workspaceCommandPalette struct {
	open, focus, reveal bool
	query               string
	selected            int
	scroll              mygoui.ScrollState
	actions             map[commandPaletteKey]func()
}

func (app *workspaceWindow) toggleCommandPalette() {
	if app.modalOpen {
		return
	}
	if app.commandPalette.open {
		app.dismissCommandPalette()
		return
	}
	app.dismissQuickSwitcher()
	focusWorkspaceSurface(app.win)
	app.commandPalette = workspaceCommandPalette{
		open: true, focus: true, actions: app.commandPaletteActions(),
	}
}

func (app *workspaceWindow) dismissCommandPalette() {
	app.commandPalette = workspaceCommandPalette{}
	switch app.displayedMode() {
	case sourceMode, splitMode:
		app.focusSource = app.draft != nil
	case wysiwygMode:
		app.wys.focus = app.draft != nil
	case previewMode:
		if view := app.previewPane.view; view != nil && !view.IsDestroyed() {
			view.Focus()
		}
	}
}

func (app *workspaceWindow) executePaletteCommand(match commandpalette.Match) {
	action := app.commandPalette.actions[commandPaletteKey{match.Section, match.Name}]
	if action != nil {
		app.dismissCommandPalette()
		action()
	}
}

func (app *workspaceWindow) submitPaletteCommand() {
	palette := &app.commandPalette
	matches := commandpalette.Search(palette.query)
	if palette.selected < len(matches) {
		app.executePaletteCommand(matches[palette.selected])
	}
}

func (palette *workspaceCommandPalette) moveSelection(key mygoui.TextKey) bool {
	if key.Mods != 0 {
		return false
	}
	var delta int
	switch key.Key {
	case mygoui.KeyUp:
		delta = -1
	case mygoui.KeyDown:
		delta = 1
	default:
		return false
	}
	if matches := commandpalette.Search(key.Text()); len(matches) != 0 {
		palette.selected = max(0, min(palette.selected+delta, len(matches)-1))
		palette.reveal = true
	}
	return true
}

const (
	commandPaletteWidth  = 380
	commandPaletteTop    = 172.5
	commandPaletteHeader = 42.5
	commandPaletteList   = 320
	commandPaletteFooter = 30
)

type commandPaletteColors struct {
	surface, selected, accent, description, caption, separator mygoui.Color
}

func (app *workspaceWindow) drawCommandPalette(c *mygoui.Context) {
	palette := &app.commandPalette
	if !palette.open {
		return
	}
	windowWidth, windowHeight := c.Size()
	width := min(commandPaletteWidth, windowWidth-16)
	top := min(commandPaletteTop, max(titleBarHeight, windowHeight-commandPaletteHeader-commandPaletteList-commandPaletteFooter-8))
	listHeight := min(commandPaletteList, windowHeight-top-commandPaletteHeader-commandPaletteFooter-8)
	if width <= 0 || listHeight <= 0 {
		return
	}
	theme := c.Theme()
	colors := commandPaletteColors{
		surface: theme.Surface, selected: theme.SurfacePressed, accent: theme.Accent,
		description: theme.TextMuted, caption: theme.TextMuted, separator: theme.Border.Alpha(0.45),
	}
	if app.blackout && !c.Preferences().HighContrast {
		// EV-UI-075: sRGB samples from the Blackout command palette.
		colors.surface, colors.selected = mygoui.RGB(24, 24, 28), mygoui.RGB(41, 45, 56)
		colors.accent, colors.description = mygoui.RGB(168, 199, 255), mygoui.RGB(139, 141, 151)
		colors.caption, colors.separator = mygoui.RGB(100, 101, 107), mygoui.RGB(29, 29, 33)
	}
	mygoui.Overlay(c, func() {
		mygoui.Box(c.Key("command palette backdrop")).Absolute().Top(0).Left(0).Size(windowWidth, windowHeight).
			Background(mygoui.RGB(0, 0, 0).Alpha(0.35)).OnClick(app.dismissCommandPalette).Role(mygoui.RoleNone)
		panel := mygoui.Column(c.Key("command palette")).Absolute().Top(top).Left((windowWidth-width)/2).
			Width(width).Height(commandPaletteHeader+listHeight+commandPaletteFooter).
			Role(mygoui.RoleDialog).Label("Command Palette").Radius(12).Clip().
			Background(colors.surface).Shadow(0, 8, 24, 0, mygoui.RGB(0, 0, 0).Alpha(0.3))
		if panel.OverlayShortcut(0, mygoui.KeyEscape) {
			app.dismissCommandPalette()
			c.Invalidate()
			return
		}
		panel.Children(func() {
			app.commandPaletteSearch(c, colors)
			matches := commandpalette.Search(palette.query)
			app.commandPaletteResults(c, matches, listHeight, colors)
			commandPaletteStatus(c, len(matches), colors)
		})
	})
}

func (app *workspaceWindow) commandPaletteSearch(c *mygoui.Context, colors commandPaletteColors) {
	palette := &app.commandPalette
	theme := c.Theme()
	mygoui.Row(c).FillWidth().Height(commandPaletteHeader).Shrink(0).Padding(0, 16).Gap(8).
		AlignItems(mygoui.Center).BorderWidth(0, 0, 1, 0).BorderColor(colors.separator).Children(func() {
		mygoui.Box(c).Size(15, 20).Center().Children(func() {
			chromeIconColor(c, "slash.circle", 19, colors.accent)
		})
		input := mygoui.TextInputBase(c.Key("command palette query"), &palette.query).
			Grow(1).MinWidth(0).Height(20).FontSize(15).Label("Search commands...").
			Placeholder("Search commands...").OnTextKey(palette.moveSelection).OnSubmit(app.submitPaletteCommand).
			OnChange(func() {
				palette.selected, palette.scroll.Y = 0, 0
			})
		if palette.focus {
			input.Focus()
			palette.focus = false
		}
		mygoui.Box(c).Size(28, 15).Radius(3).Background(theme.Text.Alpha(0.05)).Center().Children(func() {
			mygoui.Text(c, "ESC").FontSize(9).FontWeight(600).TextColor(colors.description)
		})
	})
}

func (app *workspaceWindow) commandPaletteResults(c *mygoui.Context, matches []commandpalette.Match, height float32, colors commandPaletteColors) {
	palette := &app.commandPalette
	mygoui.Scroll(c.Key("command palette results")).FillWidth().Height(height).Shrink(0).
		Label("Command results").TrackScroll(&palette.scroll).Children(func() {
		mygoui.Column(c).FillWidth().Padding(16, 0, 12).Children(func() {
			if len(matches) == 0 {
				mygoui.Text(c, "No commands found").Padding(6, 16).FontSize(12).TextColor(colors.description)
				return
			}
			var section string
			for i, match := range matches {
				if match.Section != section {
					heading := mygoui.Row(c.Key(match.Section)).FillWidth().Height(17).Shrink(0).Padding(0, 16).
						AlignItems(mygoui.Start)
					if section != "" {
						heading.Margin(12, 0, 0)
					}
					heading.Children(func() {
						mygoui.Text(c, match.Section).Role(mygoui.RoleHeading).FontSize(10).FontWeight(700).
							LetterSpacing(1).TextColor(colors.caption)
					})
					section = match.Section
				}
				row := app.commandPaletteRow(c, match, i == palette.selected, colors)
				if i == palette.selected && palette.reveal {
					row.ScrollIntoView()
					palette.reveal = false
				}
			}
		})
	})
}

func (app *workspaceWindow) commandPaletteRow(c *mygoui.Context, match commandpalette.Match, selected bool, colors commandPaletteColors) mygoui.Element {
	key := commandPaletteKey{match.Section, match.Name}
	enabled := app.commandPalette.actions[key] != nil
	label := match.Name + ", " + match.Description
	if match.Shortcut != "" {
		label += ", " + commandPaletteShortcut(match.Shortcut)
	}
	height := float32(39)
	switch match.Name {
	case "Insert Drawing", "Convert to Table", "Paste as Markdown", "Custom Stylesheet":
		height = 52
	}
	theme := c.Theme()
	row := mygoui.ButtonBase(c.Key(key)).FillWidth().Height(height).Shrink(0).Padding(0, 16).Gap(12).
		Radius(0).Justify(mygoui.Start).AlignItems(mygoui.Center).KeepFocus().
		Label(label).AccessSelected(selected).Disabled(!enabled).OnClick(func() { app.executePaletteCommand(match) })
	ink := colors.description
	if selected {
		row.Background(colors.selected)
		ink = colors.accent
	} else if row.Hovered() {
		row.Background(theme.SurfaceHover)
	}
	return row.Children(func() {
		chromeIconColor(c, commandPaletteSymbol(match), 16, ink)
		mygoui.Column(c).Grow(1).MinWidth(0).Children(func() {
			mygoui.Text(c, match.Name).FontSize(12).FontWeight(600).SingleLine().Ellipsis("…").Role(mygoui.RoleNone)
			mygoui.Text(c, match.Description).FontSize(10.5).MaxLines(2).TextColor(colors.description).Role(mygoui.RoleNone)
		})
		if match.Shortcut != "" {
			mygoui.Text(c, commandPaletteShortcut(match.Shortcut)).FontSize(10).SingleLine().
				TextColor(colors.caption).Role(mygoui.RoleNone)
		}
	})
}

func commandPaletteStatus(c *mygoui.Context, count int, colors commandPaletteColors) {
	mygoui.Row(c).FillWidth().Height(commandPaletteFooter).Shrink(0).Padding(0, 16).
		AlignItems(mygoui.Center).Gap(6).BorderWidth(1, 0, 0, 0).BorderColor(colors.separator).Children(func() {
		mygoui.Text(c, fmt.Sprintf("%d commands", count)).FontSize(11).TextColor(colors.caption)
		mygoui.Spacer(c)
		chromeIconColor(c, "arrow.up.arrow.down", 12, colors.caption)
		mygoui.Text(c, "navigate").FontSize(10).TextColor(colors.caption)
		mygoui.Text(c, "↵").FontSize(11).TextColor(colors.caption).Role(mygoui.RoleNone)
		mygoui.Text(c, "select").FontSize(10).TextColor(colors.caption)
	})
}

var portablePaletteShortcut = strings.NewReplacer("⌘", "Ctrl+", "⌃", "Ctrl+", "⇧", "Shift+", "⌥", "Alt+")

func commandPaletteShortcut(hint string) string {
	if runtime.GOOS != "darwin" {
		return portablePaletteShortcut.Replace(hint)
	}
	return hint
}

func commandPaletteSymbol(match commandpalette.Match) string {
	switch match.Name {
	case "Bold":
		return "bold"
	case "Italic":
		return "italic"
	case "Strikethrough":
		return "strikethrough"
	case "Inline Code", "Code Block":
		return "chevron.left.forwardslash.chevron.right"
	case "Math (inline)", "Math (display)", "Colored Math":
		return "function"
	case "Link":
		return "link"
	case "Query Block":
		return queryMenuSymbol
	case "API Request":
		return "network"
	case "API History":
		return "clock.arrow.circlepath"
	case "Editor Only":
		return "doc.text"
	case "Split View":
		return "rectangle.split.2x1"
	case "Preview Only":
		return "eye"
	case "WYSIWYG":
		return "wand.and.stars"
	case "Toggle GLOW Terminal":
		return "terminal"
	case "Keyboard Shortcuts":
		return "keyboard"
	case "Meeting Notes":
		return "person.3"
	case "Blog Post":
		return "newspaper"
	case "Project Plan":
		return "list.clipboard"
	case "Changelog":
		return "clock.arrow.circlepath"
	case "Save as Template":
		return "square.and.arrow.down.on.square"
	}
	switch match.Section {
	case "HEADINGS", "FORMATTING":
		return "textformat.size"
	case "LISTS":
		return "list.number"
	case "TABLES":
		return "tablecells"
	case "BLOCKS":
		if symbol := chartSymbol(match.Name); symbol != "" {
			return symbol
		}
		return "doc.text"
	case "DIAGRAMS":
		return diagramSymbol(match.Name)
	case "AI":
		return "sparkles"
	case "SETTINGS":
		return "gearshape"
	case "HELP":
		return "questionmark.circle"
	default:
		return "doc.text"
	}
}
