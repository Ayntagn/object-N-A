package ui

import (
	"fmt"
	"log"
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

// EV-UI-328/332: the syntax guide is a Help menu action even without a document.
func markdownSyntaxGuideMenuItem(language workspaceLanguage) *mygo.MenuItem {
	return &mygo.MenuItem{
		Label: language.text("Markdown Syntax Guide", "Markdown 语法指南"),
		Click: func(*mygo.MenuItem, *mygo.Window) {
			go func() {
				const destination = "https://www.markdownguide.org/basic-syntax/"
				if err := mygo.Shell.OpenExternal(destination); err != nil {
					log.Printf("Open Markdown Syntax Guide: %v", err)
				}
			}()
		},
	}
}

type workspaceFeatureGuide struct {
	win, returnTo *mygo.Window
	preferences   *workspacePreferences
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
	if app.application == nil || app.modalOpen {
		return
	}
	app.application.openFeatureGuide(topic, origin)
}

func (application *workspaceApplication) openFeatureGuide(topic string, origin *mygo.Window) {
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
	*guide = workspaceFeatureGuide{returnTo: origin, preferences: &application.preferences}
	guide.search("")
	if featureGuideCards[topic] != nil {
		guide.selectCard(topic)
		guide.reveal = false
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title: guide.preferences.InterfaceLanguage.text("Feature Guide", "功能指南"),
		Width: featureGuideWidth, Height: featureGuideHeight, Hidden: true,
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

func (app *workspaceWindow) closeWorkspaceCommand(_ *mygo.MenuItem, focused *mygo.Window) {
	app.win.Update(func() {
		if focused == nil || focused.IsDestroyed() {
			return
		}
		for _, workspace := range app.documentWindows() {
			if focused == workspace.settings {
				focused.Close()
				return
			}
		}
		if !app.modalOpen && !app.closeWelcomeIfFocused() && !app.closeFeatureGuideIfFocused() && focused == app.win {
			app.closeTab(app.workspaceTab)
		}
	})
}

func (a *workspaceApplication) refreshHelpLanguage() {
	guide := &a.featureGuide
	if guide.win != nil && !guide.win.IsDestroyed() {
		if runtime.GOOS != "darwin" {
			guide.win.SetTitle(a.preferences.InterfaceLanguage.text("Feature Guide", "功能指南"))
		}
		guide.win.Invalidate()
	}
	welcome := &a.welcome
	if welcome.win != nil && !welcome.win.IsDestroyed() {
		if runtime.GOOS != "darwin" {
			welcome.win.SetTitle(welcome.workspace.tr("Welcome Back", "欢迎回来"))
		}
		welcome.win.Invalidate()
	}
	for _, app := range a.windows {
		if flyout := app.ai.guideFlyout; flyout != nil && !flyout.IsDestroyed() {
			flyout.Invalidate()
		}
	}
}

func (guide *workspaceFeatureGuide) search(query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	words := strings.Fields(query)
	guide.matches = guide.matches[:0]
	for _, section := range featureGuideSections {
		wholeSection := strings.Contains(strings.ToLower(section.title), query) ||
			strings.Contains(strings.ToLower(section.titleChinese), query)
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
	language := guide.preferences.InterfaceLanguage
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
			mygoui.Text(c, language.text("Feature Guide", "功能指南")).FontSize(13).FontWeight(700).TextColor(colors.ink)
			mygoui.Text(c, fmt.Sprintf(language.text("· %d features", "· %d 项功能"), len(featureGuideCards))).
				FontSize(12).FontWeight(550).TextColor(colors.muted)
			mygoui.Spacer(c)
			chromeButtonIcon(c, "xmark", language.text("Close", "关闭"), closeGuide, 14, colors.muted).Size(16, 22)
		})
		mygoui.Row(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
			guide.drawTopics(c, colors, language)
			mygoui.Box(c).Width(1).Shrink(0).FillHeight().Background(colors.line)
			guide.drawDetail(c, colors, language)
		})
	})
}

func (guide *workspaceFeatureGuide) drawTopics(c *mygoui.Context, colors featureGuideColors, language workspaceLanguage) {
	mygoui.Column(c).Width(featureGuideSidebar).Shrink(0).FillHeight().Background(colors.sidebar).Children(func() {
		mygoui.Row(c).Height(34).Shrink(0).FillWidth().Padding(0, 12).Gap(7).AlignItems(mygoui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(colors.line).Children(func() {
			chromeIconColor(c, "magnifyingglass", 11, colors.muted)
			searchLabel := language.text("Search features", "搜索功能")
			mygoui.TextInputBase(c, &guide.query).Label(searchLabel).Placeholder(searchLabel).
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
				chromeButtonIcon(c, "xmark.circle.fill", language.text("Clear Search", "清除搜索"), func() {
					guide.query = ""
					guide.search("")
				}, 11, colors.muted).Size(12, 18).KeepFocus()
			}
		})
		mygoui.Scroll(c).TrackScroll(&guide.side).Grow(1).MinHeight(0).FillWidth().
			Label(language.text("Feature topics", "功能主题")).
			Padding(12, 8, 8).Children(func() {
			if len(guide.matches) == 0 {
				mygoui.Text(c, language.text("No matches", "没有匹配结果")).
					FontSize(12).TextColor(colors.muted).Padding(2, 8).Role(mygoui.RoleStatus)
			}
			for _, section := range featureGuideSections {
				guide.drawSection(c, section, colors, language)
			}
		})
	})
}

func (guide *workspaceFeatureGuide) drawSection(c *mygoui.Context, section featureGuideSection, colors featureGuideColors, language workspaceLanguage) {
	if !slices.ContainsFunc(section.cards, func(card featureGuideCard) bool { return slices.Contains(guide.matches, card.id) }) {
		return
	}
	mygoui.Column(c.Key(section.title)).FillWidth().Gap(2).Padding(0, 0, 14).Children(func() {
		mygoui.Text(c, language.text(strings.ToUpper(section.title), section.titleChinese)).Height(19).FontSize(10).FontWeight(800).
			LetterSpacing(1).TextColor(colors.muted).Padding(2, 8, 0).Role(mygoui.RoleHeading)
		for _, card := range section.cards {
			if !slices.Contains(guide.matches, card.id) {
				continue
			}
			active := card.id == guide.selected
			title := card.localizedCopy(language).title
			button := mygoui.ButtonBase(c.Key(card.id)).FillWidth().Height(featureGuideTopic).Radius(6).
				Label(title).AccessSelected(active).Padding(0, 10).AlignItems(mygoui.Center).
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
				mygoui.Text(c, title).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").
					FontSize(13).FontWeight(weight).TextColor(ink).Role(mygoui.RoleNone)
			})
		}
	})
}

func (guide *workspaceFeatureGuide) drawDetail(c *mygoui.Context, colors featureGuideColors, language workspaceLanguage) {
	card := featureGuideCards[guide.selected]
	if card == nil {
		return
	}
	copy := card.localizedCopy(language)
	mygoui.Scroll(c).TrackScroll(&guide.detail).Grow(1).MinWidth(0).FillHeight().
		Label(language.text("Feature details", "功能详情")).
		Padding(18, 26, 20).Children(func() {
		mygoui.Column(c.Key(card.id)).FillWidth().MinWidth(0).Gap(14).Children(func() {
			mygoui.Column(c).FillWidth().Gap(6).Children(func() {
				mygoui.Text(c, copy.title).FillWidth().MinWidth(0).Wrap().FontSize(24).FontWeight(700).
					TextColor(colors.ink).Role(mygoui.RoleHeading)
				mygoui.Text(c, copy.summary).FillWidth().MinWidth(0).Wrap().FontSize(13).
					TextColor(colors.muted)
			})
			if card.source != "" {
				label := language.text("SYNTAX", "语法")
				if card.preview > 0 {
					label = language.text("YOU TYPE", "输入示例")
				}
				guideDetailLabel(c, label, colors)
				mygoui.Scroll(c.Key("example")).FillWidth().MinHeight(69).MaxHeight(180).
					Label(language.text("Source example", "源文示例")).
					Radius(9).Background(colors.code).Border(1, colors.line).Padding(11, 12).Children(func() {
					mygoui.Text(c, card.source).FillWidth().MinWidth(0).Wrap().Selectable().
						Font("monospace").FontSize(13).TextColor(colors.ink)
				})
			}
			if card.preview > 0 {
				mygoui.Row(c).FillWidth().Height(14).Center().Gap(6).Children(func() {
					chromeIconColor(c, "arrow.down", 11, colors.muted)
					mygoui.Text(c, language.text("renders to", "渲染为")).FontSize(11).FontWeight(600).TextColor(colors.muted)
				})
				guideDetailLabel(c, language.text("YOU GET", "渲染结果"), colors)
				guide.preview.draw(c, card, language, guide.preferences.activeCustomStylesheet())
			}
			if card.inApp || card.illustration != "" {
				guideDetailLabel(c, language.text("IN THE APP", "在应用中"), colors)
			}
			guideIllustration(c, card, language)
			if len(copy.points) != 0 {
				mygoui.Column(c).FillWidth().Gap(7).Children(func() {
					for _, point := range copy.points {
						mygoui.Row(c).FillWidth().Gap(7).AlignItems(mygoui.Start).Children(func() {
							mygoui.Text(c, "•").FontSize(13).TextColor(colors.accent).Role(mygoui.RoleNone)
							mygoui.Text(c, point).Grow(1).MinWidth(0).FontSize(13).Wrap().TextColor(colors.muted)
						})
					}
				})
			}
			guide.drawHints(c, copy.hints, colors, language)
		})
	})
}

func guideDetailLabel(c *mygoui.Context, label string, colors featureGuideColors) {
	mygoui.Text(c, label).Height(13).FontSize(10).FontWeight(800).LetterSpacing(1.2).
		TextColor(colors.muted).Role(mygoui.RoleHeading)
}

func (guide *workspaceFeatureGuide) drawHints(c *mygoui.Context, hints []string, colors featureGuideColors, language workspaceLanguage) {
	mygoui.Column(c).FillWidth().Gap(7).Children(func() {
		guideDetailLabel(c, language.text("QUICK INSERT", "快速操作"), colors)
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
