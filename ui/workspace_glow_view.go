package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"cyberwriter/internal/core/glow"
	mygoui "github.com/egoist/mygo/ui"
)

type glowPalette struct {
	id, name                         string
	background, input, text, prompt  mygoui.Color
	accent, muted, errorText, swatch mygoui.Color
	slash                            glowSlashColors
}

type glowSlashColors struct {
	surface, selected, divider mygoui.Color
	slug, title, muted         mygoui.Color
}

const glowFloatTitleHeight = 24

// EV-UI-072: sampled surfaces converted from the screenshot's display profile to sRGB.
var glowPalettes = [...]glowPalette{
	{
		id: "plato", name: "Plato",
		background: mygoui.RGB(13, 6, 1), input: mygoui.RGB(22, 17, 9),
		text: mygoui.RGB(230, 173, 88), prompt: mygoui.RGB(253, 130, 34),
		accent: mygoui.RGB(248, 189, 101), muted: mygoui.RGB(133, 102, 59),
		errorText: mygoui.RGB(255, 115, 101), swatch: mygoui.RGB(253, 130, 34),
		slash: glowSlashColors{
			surface: mygoui.RGB(28, 21, 15), selected: mygoui.RGB(71, 43, 19), divider: mygoui.RGB(72, 55, 33),
			slug: mygoui.RGB(255, 140, 31), title: mygoui.RGB(255, 196, 107), muted: mygoui.RGB(143, 109, 64),
		},
	},
	{
		id: "mono", name: "Mono",
		background: mygoui.RGB(13, 13, 13), input: mygoui.RGB(24, 24, 24),
		text: mygoui.RGB(213, 213, 213), prompt: mygoui.RGB(232, 232, 232),
		accent: mygoui.RGB(246, 246, 246), muted: mygoui.RGB(122, 122, 122),
		errorText: mygoui.RGB(247, 162, 149), swatch: mygoui.RGB(255, 255, 255),
		slash: glowSlashColors{
			surface: mygoui.RGB(29, 29, 29), selected: mygoui.RGB(73, 73, 73), divider: mygoui.RGB(66, 66, 66),
			slug: mygoui.RGB(255, 255, 255), title: mygoui.RGB(255, 255, 255), muted: mygoui.RGB(126, 126, 126),
		},
	},
	{
		id: "matrix", name: "Matrix",
		background: mygoui.RGB(0, 1, 0), input: mygoui.RGB(5, 10, 6),
		text: mygoui.RGB(108, 239, 153), prompt: mygoui.RGB(115, 255, 151),
		accent: mygoui.RGB(134, 255, 167), muted: mygoui.RGB(48, 127, 75),
		errorText: mygoui.RGB(255, 132, 111), swatch: mygoui.RGB(115, 255, 151),
		slash: glowSlashColors{
			surface: mygoui.RGB(10, 14, 11), selected: mygoui.RGB(31, 62, 40), divider: mygoui.RGB(23, 56, 33),
			slug: mygoui.RGB(124, 255, 158), title: mygoui.RGB(156, 255, 176), muted: mygoui.RGB(46, 122, 69),
		},
	},
}

func (p glowPalette) spanColor(style glow.Style) mygoui.Color {
	switch style {
	case glow.ScriptText:
		if p.id == "plato" {
			return mygoui.RGB(255, 206, 126)
		}
		return p.accent
	case glow.ScriptMarker:
		if p.id == "plato" {
			return mygoui.RGB(255, 158, 39)
		}
		return p.prompt
	case glow.ScriptMuted:
		if p.id == "plato" {
			return mygoui.RGB(161, 128, 81)
		}
		return p.muted
	case glow.ScriptError:
		if p.id == "plato" {
			return mygoui.RGB(255, 114, 69)
		}
		return p.errorText
	case glow.Prompt:
		return p.prompt
	case glow.Accent:
		return p.accent
	case glow.Muted:
		return p.muted
	case glow.Error:
		return p.errorText
	default:
		return p.text
	}
}

func (app *workspaceWindow) glowPane(c *mygoui.Context, host glowHost) {
	if pane := app.glow.pane; pane != nil {
		pane.view(c, host)
	}
}

func (p *workspaceGlowPane) view(c *mygoui.Context, host glowHost) {
	prefs := p.application.preferences.Terminal.normalized()
	palette := glowPalettes[0]
	for _, candidate := range glowPalettes {
		if prefs.Theme == candidate.id {
			palette = candidate
			break
		}
	}

	headerHeight := float32(39)
	if host == glowFloat {
		headerHeight = 34
	}
	var name string
	if app := p.workspace(); app != nil {
		name = app.glowEnvironment().Name
	}
	session := &p.application.glow.session
	prompt := session.Prompt(name)
	mygoui.Column(c).FillWidth().FillHeight().MinHeight(0).Background(palette.background).Children(func() {
		if host == glowFloat {
			mygoui.Row(c).FillWidth().Height(glowFloatTitleHeight).Shrink(0).Center().DragWindow().Children(func() {
				mygoui.Text(c, "Terminal").FontSize(11).FontWeight(600).TextColor(c.Theme().TextMuted).
					Role(mygoui.RoleNone)
			})
		}
		p.header(c, host, palette, strings.TrimSuffix(prompt, ">")+"/", headerHeight)
		p.output(c, palette, prefs.FontSize)
		if !session.DemoRunning() {
			if menu := p.slashMenu(p.input); menu.Open {
				p.slashView(c, menu, palette.slash)
			}
		}
		p.inputView(c, palette, prompt, prefs.FontSize)
		if p.document != nil && !p.document.glow.maximized {
			// The grip overlays the first five points of the panel.
			mygoui.Box(c.Key("glow resize")).Absolute().Top(0).Left(0).Right(0).Height(5).
				Role(mygoui.RoleSplitter).Label("Resize GLOW Terminal").Cursor(mygoui.CursorResizeNS).
				Bind(&p.heightGrip).HandleInput(func(event mygoui.InputEvent) bool {
				return p.resizeHeight(event, p.heightGrip.Bounds(c).Y+event.Y)
			})
		}
	})
}

func (p *workspaceGlowPane) header(c *mygoui.Context, host glowHost, palette glowPalette, directory string, height float32) {
	mygoui.Row(c).FillWidth().Height(height).Shrink(0).Padding(0, 12).Gap(18).AlignItems(mygoui.Center).
		Role(mygoui.RoleToolbar).Label("GLOW Terminal controls").Children(func() {
		mygoui.Row(c).Grow(1).MinWidth(0).Gap(10).AlignItems(mygoui.Center).Clip().Children(func() {
			mygoui.Text(c, "GLOW TERMINAL").Font("monospace").FontSize(10).FontWeight(700).
				LetterSpacing(2.2).SingleLine().Shrink(0).TextColor(palette.muted)
			mygoui.Text(c, directory).Font("monospace").FontSize(11).SingleLine().Ellipsis("…").
				MinWidth(0).TextColor(palette.prompt)
		})
		mygoui.Row(c).Gap(10).Shrink(0).Children(func() {
			glowButton(c, "textformat.size.smaller", "Decrease Text Size", "Decrease font size", palette.muted,
				func() { p.changeFont(-1) })
			glowButton(c, "textformat.size.larger", "Increase Text Size", "Increase font size", palette.muted,
				func() { p.changeFont(1) })
		})
		mygoui.Row(c).Gap(10).Shrink(0).Children(func() {
			for _, theme := range glowPalettes {
				selected := palette.id == theme.id
				chromeControl(c.Key(theme.id), theme.name, false).Size(22, 22).KeepFocus().
					AccessSelected(selected).OnClick(func() { p.setTheme(theme.id) }).Children(func() {
					ring := mygoui.Box(c).Size(18, 18).Radius(9).Center().Role(mygoui.RoleNone)
					if selected {
						ring.Border(1, theme.swatch)
					}
					ring.Children(func() {
						mygoui.Box(c).Size(12, 12).Radius(6).Background(theme.swatch).Role(mygoui.RoleNone)
					})
				})
			}
		})
		mygoui.Row(c).Gap(10).Shrink(0).Children(func() {
			ink := func(mode glowHost) mygoui.Color {
				if mode == host {
					return palette.prompt
				}
				return palette.muted
			}
			glowButton(c, "arrow.down.to.line", "End", "Dock above the status bar", ink(glowDock),
				func() { p.setHost(glowDock) }).AccessSelected(host == glowDock)
			glowButton(c, "rectangle.on.rectangle", "Screen Sharing", "Shade over the editor", ink(glowShade),
				func() { p.setHost(glowShade) }).AccessSelected(host == glowShade)
			glowButton(c, "arrow.up.forward.app", "Up-Forward Arrow On An App Rectangle", "Pop out to a floating window",
				ink(glowFloat), func() { p.setHost(glowFloat) }).AccessSelected(host == glowFloat)
			if host == glowDock {
				symbol, label, help := "arrow.up.left.and.arrow.down.right", "Enter Full Screen", "Maximize Terminal"
				if p.document.glow.maximized {
					symbol, label, help = "arrow.down.right.and.arrow.up.left", "Exit Full Screen", "Restore Terminal"
				}
				glowButton(c, symbol, label, help, palette.muted, p.toggleMaximize)
			}
			glowButton(c, "xmark", "Close", "Close Terminal", palette.muted, p.close)
		})
	})
}

func glowButton(c *mygoui.Context, symbol, label, help string, color mygoui.Color, action func()) mygoui.Element {
	return chromeButtonIcon(c, symbol, label, action, 20, color, help).Size(22, 22).Shrink(0).KeepFocus()
}

func (p *workspaceGlowPane) output(c *mygoui.Context, palette glowPalette, fontSize float32) {
	mygoui.Scroll(c.Key("glow-output")).FillWidth().Grow(1).MinHeight(0).Label("GLOW Terminal output").
		TrackScroll(&p.scroll).Children(func() {
		mygoui.Column(c).FillWidth().MinWidth(0).Padding(4, 12).Selectable().Children(func() {
			session := &p.application.glow.session
			for i, line := range session.Lines() {
				p.outputLine(c.Key(i), palette, fontSize, line)
			}
			if preview := session.DemoPreview(); len(preview) != 0 {
				p.outputLine(c.Key("demo preview"), palette, fontSize, preview)
			}
		})
	})
}

func (p *workspaceGlowPane) outputLine(c *mygoui.Context, palette glowPalette, fontSize float32, line glow.Line) {
	mygoui.RichText(c).FillWidth().Font("monospace").FontSize(fontSize).
		LineHeight(1.2).MinHeight(fontSize * 1.32).Children(func() {
		for index, span := range line {
			color := palette.spanColor(span.Style)
			if span.Path != "" {
				mygoui.Link(c, span.Text, "").TextColor(color).Underline().OnClick(func() { p.openLink(span.Path) })
				continue
			}
			label := mygoui.Text(c, span.Text).TextColor(color)
			if index > 0 && line[0].Style == glow.Prompt && strings.HasSuffix(line[0].Text, "> ") {
				label.FontWeight(600)
			}
		}
	})
}

// EV-UI-078: the menu takes space from the scrollback above the fixed input.
func (p *workspaceGlowPane) slashView(c *mygoui.Context, menu glow.SlashMenu, colors glowSlashColors) {
	const rowHeight, footerHeight = 22, 18
	contentHeight := rowHeight * len(menu.Rows)
	if contentHeight == 0 {
		contentHeight = 24
	}
	mygoui.Column(c.Key("glow app commands")).FillWidth().Height(float32(contentHeight+footerHeight+6)).
		MinHeight(0).Shrink(0).Padding(2, 0, 3).Clip().BorderWidth(1, 0, 0, 0).BorderColor(colors.divider).
		Background(colors.surface).Role(mygoui.RoleListBox).Label("GLOW app commands").Children(func() {
		if len(menu.Rows) == 0 {
			mygoui.Row(c).FillWidth().Height(24).Shrink(0).Padding(0, 12).AlignItems(mygoui.Center).Children(func() {
				mygoui.Text(c, "no matching command - esc clears").Font("monospace").FontSize(11.5).
					SingleLine().Ellipsis("…").TextColor(colors.title)
			})
		}
		for i, command := range menu.Rows {
			slug := "/" + command.Slug
			selected := i == p.slashSelection
			row := mygoui.ButtonBase(c.Key(command.Slug)).FillWidth().Height(rowHeight).Shrink(0).
				Padding(0, 12).Gap(10).Radius(0).KeepFocus().AlignItems(mygoui.Center).Justify(mygoui.Start).
				Role(mygoui.RoleOption).Label(slug + ", " + command.Title + ", " + command.Category).AccessSelected(selected).
				OnClick(func() { p.clickAppCommand(slug) })
			if selected {
				row.Background(colors.selected)
			}
			row.Children(func() {
				mygoui.Text(c, slug).Font("monospace").FontSize(12.5).FontWeight(600).
					Shrink(0).SingleLine().TextColor(colors.slug).Role(mygoui.RoleNone)
				mygoui.Text(c, command.Title).Font("monospace").FontSize(11.5).FontWeight(600).
					Grow(1).MinWidth(0).SingleLine().Ellipsis("…").TextColor(colors.title).Role(mygoui.RoleNone)
				mygoui.Text(c, command.Category).Font("monospace").FontSize(10).Shrink(0).
					SingleLine().TextColor(colors.muted).Role(mygoui.RoleNone)
			})
		}
		mygoui.Row(c).FillWidth().Height(footerHeight).Shrink(0).Padding(0, 12).Gap(10).
			AlignItems(mygoui.Center).Children(func() {
			mygoui.Text(c, "↑↓ choose   ·   ⏎ run   ·   ⇥ complete   ·   esc clear").Font("monospace").FontSize(10).
				Grow(1).MinWidth(0).SingleLine().Ellipsis("…").TextColor(colors.muted)
			if menu.More > 0 {
				mygoui.Text(c, fmt.Sprintf("+%d more - keep typing", menu.More)).Font("monospace").FontSize(10).
					Shrink(0).SingleLine().TextColor(colors.muted)
			}
		})
	})
}

func (p *workspaceGlowPane) inputView(c *mygoui.Context, palette glowPalette, prompt string, fontSize float32) {
	const height = 29
	mygoui.Row(c.Key("glow-input-row")).FillWidth().Height(height).Shrink(0).Padding(0, 12).Gap(5).AlignItems(mygoui.Center).
		Background(palette.input).Children(func() {
		mygoui.Text(c, prompt).Font("monospace").FontSize(fontSize).SingleLine().Ellipsis("…").
			MinWidth(0).TextColor(palette.prompt)
		input := mygoui.TextInputBase(c.Key("glow-input"), &p.input).Grow(1).MinWidth(48).
			Height(min(height-4, max(17, fontSize+4.5))).Font("monospace").FontSize(fontSize).
			TextColor(palette.text).CaretColor(palette.prompt).Label("GLOW Terminal input").
			OnTextKey(p.key).OnSubmit(p.submit)
		if p.focus {
			input.Focus()
			p.focus = false
		}
		if p.moveCaret {
			end := utf8.RuneCountInString(p.input)
			input.SetTextSelection(end, end)
			p.moveCaret = false
		}
	})
}
