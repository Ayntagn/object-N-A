package ui

import (
	"fmt"
	"runtime"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

const (
	featureGuideWidth   = 840
	featureGuideHeight  = 852
	featureGuideSidebar = 200
	featureGuideTopic   = 27
)

type workspaceFeatureGuide struct {
	win, returnTo *mygo.Window
	workspace     *workspaceWindow
	query         string
	selected      string
	matches       []string
	reveal        bool
	side, detail  mygoui.ScrollState
	preview       featureGuidePreview
}

func (app *workspaceWindow) showFeatureGuide() {
	app.openFeatureGuide("", app.win)
}

func (app *workspaceWindow) openFeatureGuide(topic string, origin *mygo.Window) {
	application := app.application
	if application == nil || app.modalOpen {
		return
	}
	guide := &application.featureGuide
	if guide.win != nil && !guide.win.IsDestroyed() {
		if featureGuideCards[topic] != nil {
			guide.query = ""
			guide.search("")
			guide.detail.Y = 0
			guide.selectCard(topic)
			guide.reveal = false
		}
		if mygo.FocusedWindow() != guide.win {
			guide.returnTo = origin
		}
		guide.win.Focus()
		guide.win.Invalidate()
		return
	}
	*guide = workspaceFeatureGuide{returnTo: origin, workspace: app}
	guide.search("")
	if featureGuideCards[topic] != nil {
		guide.selectCard(topic)
		guide.reveal = false
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title: "Feature Guide", Width: featureGuideWidth, Height: featureGuideHeight, Hidden: true,
		TitleBarStyle: mygo.TitleBarHiddenInset, TitleBarHeight: 32,
		TrafficLightPosition: &mygo.Point{X: 8, Y: 8},
		DisableMinimize:      true, DisableMaximize: true, DisableFullScreen: true,
		DisableResize: true, Content: mygoui.View(guide.draw),
	})
	guide.win = win
	if runtime.GOOS == "darwin" {
		win.SetTitle("")
	}
	guide.preview.init(win)
	win.OnClosed(func() {
		if guide.win != win {
			return
		}
		next := guide.returnTo
		*guide = workspaceFeatureGuide{}
		if next == nil || next.IsDestroyed() {
			if active := application.activeWorkspace(); active != nil {
				next = active.win
			}
		}
		if next != nil && !next.IsDestroyed() {
			next.Focus()
		}
	})
	win.Show()
}

func (app *workspaceWindow) closeFeatureGuideIfFocused() bool {
	if app.application != nil {
		win := app.application.featureGuide.win
		if win != nil && mygo.FocusedWindow() == win {
			win.Close()
			return true
		}
	}
	return false
}

func (app *workspaceWindow) closeWorkspaceCommand() {
	if !app.closeWelcomeIfFocused() && !app.closeFeatureGuideIfFocused() {
		app.closeTab(app.workspaceTab)
	}
}

func (guide *workspaceFeatureGuide) search(query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	words := strings.Fields(query)
	guide.matches = guide.matches[:0]
	for _, section := range featureGuideSections {
		wholeSection := strings.Contains(strings.ToLower(section.title), query)
		for i := range section.cards {
			card := &section.cards[i]
			if wholeSection || matchesGuideWords(card.searchableText(), words) {
				guide.matches = append(guide.matches, card.id)
			}
		}
	}
	guide.side.Y = 0
	if len(guide.matches) > 0 && !slices.Contains(guide.matches, guide.selected) {
		guide.selectCard(guide.matches[0])
	}
}

func matchesGuideWords(text string, words []string) bool {
	for _, word := range words {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

func (guide *workspaceFeatureGuide) selectCard(id string) {
	if guide.selected != id {
		guide.selected = id
		guide.detail.Y = 0
	}
	guide.reveal = true
}

func (guide *workspaceFeatureGuide) move(delta int) {
	if i := slices.Index(guide.matches, guide.selected); i >= 0 {
		guide.selectCard(guide.matches[min(len(guide.matches)-1, max(0, i+delta))])
	}
}

type featureGuideColors struct {
	background, sidebar, line, code, selected, ink, muted, accent mygoui.Color
}

func guideColors(c *mygoui.Context) featureGuideColors {
	theme := c.Theme()
	if theme.Dark && !c.Preferences().HighContrast {
		// EV-UI-122, sampled in sRGB in the independent window.
		return featureGuideColors{
			mygoui.Hex("#28292b"), mygoui.Hex("#2c2d2f"), mygoui.Hex("#3d3e40"), mygoui.Hex("#303133"),
			mygoui.Hex("#303b4b"), mygoui.Hex("#dededf"), mygoui.Hex("#9e9f9f"), mygoui.Hex("#4095fa"),
		}
	}
	return featureGuideColors{theme.Background, theme.Surface, theme.Border, theme.Surface,
		theme.Accent.Alpha(.13), theme.Text, theme.TextMuted, theme.Accent}
}

func (guide *workspaceFeatureGuide) draw(c *mygoui.Context) {
	applyWorkspaceTheme(c)
	colors := guideColors(c)
	closeGuide := func() { guide.win.Close() }
	c.OnWindowShortcut(0, mygoui.KeyEscape, closeGuide)
	c.OnWindowShortcut(mygoui.Cmd, mygoui.KeyW, closeGuide)
	c.OnWindowShortcut(0, mygoui.KeyUp, func() { guide.move(-1) })
	c.OnWindowShortcut(0, mygoui.KeyDown, func() { guide.move(1) })
	c.OnWindowShortcut(0, mygoui.KeyLeft, func() { guide.move(-1) })
	c.OnWindowShortcut(0, mygoui.KeyRight, func() { guide.move(1) })
	mygoui.Column(c).Fill().Background(colors.background).Children(func() {
		mygoui.Box(c).Height(32).Shrink(0).FillWidth().DragWindow()
		mygoui.Row(c).Height(41).Shrink(0).FillWidth().Padding(0, 17, 0, 18).Gap(8).AlignItems(mygoui.Center).
			DragWindow().BorderWidth(0, 0, 1, 0).BorderColor(colors.line).Children(func() {
			mygoui.Text(c, "Feature Guide").FontSize(13).FontWeight(700).TextColor(colors.ink)
			mygoui.Text(c, fmt.Sprintf("· %d features", len(featureGuideCards))).FontSize(12).FontWeight(550).TextColor(colors.muted)
			mygoui.Spacer(c)
			chromeButtonIcon(c, "xmark", "Close", closeGuide, 14, colors.muted).Size(16, 22)
		})
		mygoui.Row(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
			guide.drawTopics(c, colors)
			mygoui.Box(c).Width(1).Shrink(0).FillHeight().Background(colors.line)
			guide.drawDetail(c, colors)
		})
	})
}

func (guide *workspaceFeatureGuide) drawTopics(c *mygoui.Context, colors featureGuideColors) {
	mygoui.Column(c).Width(featureGuideSidebar).Shrink(0).FillHeight().Background(colors.sidebar).Children(func() {
		mygoui.Row(c).Height(34).Shrink(0).FillWidth().Padding(0, 12).Gap(7).AlignItems(mygoui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(colors.line).Children(func() {
			chromeIconColor(c, "magnifyingglass", 11, colors.muted)
			mygoui.TextInputBase(c, &guide.query).Label("Search features").Placeholder("Search features").
				Height(15).Grow(1).MinWidth(0).FontSize(13).TextColor(colors.ink).
				OnChange(func() { guide.search(guide.query) }).OnTextKey(func(key mygoui.TextKey) bool {
				if key.Mods != 0 {
					return false
				}
				switch key.Key {
				case mygoui.KeyUp, mygoui.KeyLeft:
					guide.move(-1)
				case mygoui.KeyDown, mygoui.KeyRight:
					guide.move(1)
				case mygoui.KeyEscape:
					guide.win.Close()
				default:
					return false
				}
				return true
			})
			if guide.query != "" {
				chromeButtonIcon(c, "xmark.circle.fill", "Clear Search", func() {
					guide.query = ""
					guide.search("")
				}, 11, colors.muted).Size(12, 18).KeepFocus()
			}
		})
		mygoui.Scroll(c).TrackScroll(&guide.side).Grow(1).MinHeight(0).FillWidth().Label("Feature topics").
			Padding(12, 8, 8).Children(func() {
			if len(guide.matches) == 0 {
				mygoui.Text(c, "No matches").FontSize(12).TextColor(colors.muted).Padding(2, 8).Role(mygoui.RoleStatus)
			}
			for _, section := range featureGuideSections {
				guide.drawSection(c, section, colors)
			}
		})
	})
}

func (guide *workspaceFeatureGuide) drawSection(c *mygoui.Context, section featureGuideSection, colors featureGuideColors) {
	if !slices.ContainsFunc(section.cards, func(card featureGuideCard) bool { return slices.Contains(guide.matches, card.id) }) {
		return
	}
	mygoui.Column(c.Key(section.title)).FillWidth().Gap(2).Padding(0, 0, 14).Children(func() {
		mygoui.Text(c, strings.ToUpper(section.title)).Height(19).FontSize(10).FontWeight(800).
			LetterSpacing(1).TextColor(colors.muted).Padding(2, 8, 0).Role(mygoui.RoleHeading)
		for _, card := range section.cards {
			if !slices.Contains(guide.matches, card.id) {
				continue
			}
			active := card.id == guide.selected
			button := mygoui.ButtonBase(c.Key(card.id)).FillWidth().Height(featureGuideTopic).Radius(6).
				Label(card.title).AccessSelected(active).Padding(0, 10).AlignItems(mygoui.Center).
				OnClick(func() { guide.selectCard(card.id) })
			ink, weight := colors.ink.Mix(colors.muted, .32), 400
			if active {
				button.Background(colors.selected)
				ink, weight = colors.accent, 650
				if guide.reveal {
					button.ScrollIntoView()
					guide.reveal = false
				}
			} else if button.Hovered() {
				button.Background(colors.line)
			}
			button.Children(func() {
				mygoui.Text(c, card.title).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").
					FontSize(13).FontWeight(weight).TextColor(ink).Role(mygoui.RoleNone)
			})
		}
	})
}

func (guide *workspaceFeatureGuide) drawDetail(c *mygoui.Context, colors featureGuideColors) {
	card := featureGuideCards[guide.selected]
	if card == nil {
		return
	}
	mygoui.Scroll(c).TrackScroll(&guide.detail).Grow(1).MinWidth(0).FillHeight().Label("Feature details").
		Padding(18, 26, 20).Children(func() {
		mygoui.Column(c.Key(card.id)).FillWidth().MinWidth(0).Gap(14).Children(func() {
			mygoui.Column(c).FillWidth().Gap(6).Children(func() {
				mygoui.Text(c, card.title).FillWidth().MinWidth(0).Wrap().FontSize(24).FontWeight(700).
					TextColor(colors.ink).Role(mygoui.RoleHeading)
				mygoui.Text(c, card.summary).FillWidth().MinWidth(0).Wrap().FontSize(13).
					TextColor(colors.muted)
			})
			if card.source != "" {
				label := "SYNTAX"
				if card.preview > 0 {
					label = "YOU TYPE"
				}
				guideDetailLabel(c, label, colors)
				mygoui.Scroll(c.Key("example")).FillWidth().MinHeight(69).MaxHeight(180).Label("Source example").
					Radius(9).Background(colors.code).Border(1, colors.line).Padding(11, 12).Children(func() {
					mygoui.Text(c, card.source).FillWidth().MinWidth(0).Wrap().Selectable().
						Font("monospace").FontSize(13).TextColor(colors.ink)
				})
			}
			if card.preview > 0 {
				mygoui.Row(c).FillWidth().Height(14).Center().Gap(6).Children(func() {
					chromeIconColor(c, "arrow.down", 11, colors.muted)
					mygoui.Text(c, "renders to").FontSize(11).FontWeight(600).TextColor(colors.muted)
				})
				guideDetailLabel(c, "YOU GET", colors)
				guide.preview.draw(c, card, guide.workspace.activeCustomStylesheet())
			}
			if card.inApp || card.illustration != "" {
				guideDetailLabel(c, "IN THE APP", colors)
			}
			guideIllustration(c, card)
			if len(card.points) != 0 {
				mygoui.Column(c).FillWidth().Gap(7).Children(func() {
					for _, point := range card.points {
						mygoui.Row(c).FillWidth().Gap(7).AlignItems(mygoui.Start).Children(func() {
							mygoui.Text(c, "•").FontSize(13).TextColor(colors.accent).Role(mygoui.RoleNone)
							mygoui.Text(c, point).Grow(1).MinWidth(0).FontSize(13).Wrap().TextColor(colors.muted)
						})
					}
				})
			}
			guide.drawHints(c, card.hints, colors)
		})
	})
}

func guideDetailLabel(c *mygoui.Context, label string, colors featureGuideColors) {
	mygoui.Text(c, label).Height(13).FontSize(10).FontWeight(800).LetterSpacing(1.2).
		TextColor(colors.muted).Role(mygoui.RoleHeading)
}

func (guide *workspaceFeatureGuide) drawHints(c *mygoui.Context, hints []string, colors featureGuideColors) {
	mygoui.Column(c).FillWidth().Gap(7).Children(func() {
		guideDetailLabel(c, "QUICK INSERT", colors)
		for _, hint := range hints {
			mygoui.Row(c).FillWidth().Gap(10).AlignItems(mygoui.Start).Children(func() {
				symbol := "textformat"
				if strings.Contains(hint, "⌘") || strings.Contains(hint, "⌃") {
					symbol = "command"
				} else if strings.Contains(hint, "→") {
					symbol = "list.bullet.rectangle"
				}
				chromeIconColor(c, symbol, 13, colors.accent)
				mygoui.Text(c, shortcutLabel(hint)).Grow(1).MinWidth(0).Wrap().
					Font("monospace").FontSize(13).TextColor(colors.muted)
			})
		}
	})
}
