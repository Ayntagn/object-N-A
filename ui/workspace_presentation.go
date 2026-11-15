package ui

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"

	"cyberwriter/internal/core/presentation"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

type workspacePresentation struct {
	tab           *workspaceTab
	origin        previewResourceOrigin
	slides        presentation.SlideDeck
	cards         []presentation.Card
	flash         bool
	progress      presentation.Progress
	due           int
	problem       string
	position      int
	order         []int
	revealed      bool
	help          bool
	layout        int
	zoom          int
	wasFullScreen bool
	interval      time.Duration
	timer         *time.Timer
	timerEpoch    uint64
	web           presentationWeb
}

func (app *workspaceWindow) canPresent() bool {
	return !app.modalOpen && !app.multiFile.active && app.draft != nil && app.drawing == nil &&
		(app.document.Path == "" || workspace.IsMarkdownPath(app.document.Path))
}

func (app *workspaceWindow) presentationMenu(menu *mygoui.Menu) {
	if menu.Item("Play as Slides").Chosen() {
		app.openPresentation(false)
	}
	if menu.Item("Flash Cards").Chosen() {
		app.openPresentation(true)
	}
}

func (app *workspaceWindow) openPresentation(flash bool) {
	if app.presentation != nil || app.win == nil || !app.canPresent() {
		return
	}
	app.wys.sync(app)
	origin := app.previewOrigin()
	p := &workspacePresentation{
		tab: app.workspaceTab, origin: origin, flash: flash, zoom: 100,
		interval: 5 * time.Second, wasFullScreen: app.win.IsFullScreen(),
	}
	prefix, properties, body := workspace.SplitFrontmatter(app.workingText())
	if flash {
		p.cards = presentation.Flashcards(body)
		p.progress = presentation.Progress{}
		if origin.vault != nil && origin.note != "" {
			progress, err := origin.vault.FlashcardProgress(origin.note)
			if err != nil {
				p.problem = err.Error()
			} else {
				p.progress = progress
			}
		}
		p.due = p.progress.Due(p.cards, time.Now())
	} else {
		if !strings.HasPrefix(strings.TrimPrefix(prefix, "\ufeff"), "---") {
			properties = ""
		}
		var date time.Time
		if origin.vault != nil && origin.note != "" {
			if info, err := origin.vault.DocumentInfo(origin.note); err == nil {
				date = info.Created
				if date.IsZero() {
					date = info.Modified
				}
			}
		}
		p.slides = presentation.Slides(body, properties, app.title(), date)
	}
	app.presentation = p
	p.web.init(app, p)
	app.win.Invalidate()
}

func (app *workspaceWindow) closePresentation() {
	p := app.presentation
	if p == nil {
		return
	}
	app.presentation = nil
	p.stopTimer()
	if p.web.view != nil && !p.web.view.IsDestroyed() {
		p.web.view.Destroy()
	}
	if app.win.IsDestroyed() {
		return
	}
	if app.win.IsFullScreen() != p.wasFullScreen {
		app.win.SetFullScreen(p.wasFullScreen)
	}
	if app.workspaceTab == p.tab {
		switch app.displayedMode() {
		case sourceMode, splitMode:
			app.focusSource = true
		case wysiwygMode:
			app.wys.focus = true
		}
	}
	app.win.Invalidate()
}

func (p *workspacePresentation) count() int {
	if p.flash {
		return len(p.cards)
	}
	return len(p.slides.Pages) + 1
}

func (p *workspacePresentation) index() int {
	if p.order != nil {
		return p.order[p.position]
	}
	return p.position
}

func (p *workspacePresentation) turn(step int) {
	if n := p.count(); n > 0 {
		p.position = (p.position + step + n) % n
		p.revealed = false
	}
}

func (p *workspacePresentation) toggleShuffle() {
	if p.count() <= 1 {
		return
	}
	index := p.index()
	if p.order != nil {
		p.order = nil
		p.position = index
	} else {
		p.order = rand.Perm(p.count())
		at := slices.Index(p.order, index)
		p.order[p.position], p.order[at] = p.order[at], p.order[p.position]
	}
}

func (p *workspacePresentation) stopTimer() {
	p.timerEpoch++
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
}

func (p *workspacePresentation) startTimer(app *workspaceWindow) {
	p.stopTimer()
	epoch := p.timerEpoch
	win := app.win
	p.timer = time.AfterFunc(p.interval, func() {
		win.Update(func() {
			if app.presentation == p && p.timerEpoch == epoch {
				p.turn(1)
				p.startTimer(app)
			}
		})
	})
}

func (p *workspacePresentation) rate(rating presentation.Rating) {
	if !p.flash || !p.revealed || p.count() == 0 {
		return
	}
	card, now := p.cards[p.index()], time.Now()
	if p.origin.vault != nil && p.origin.note != "" {
		progress, err := p.origin.vault.RateFlashcard(p.origin.note, card, rating, now)
		if err != nil {
			p.problem = err.Error()
			return
		}
		p.progress = progress
	} else {
		p.progress.Rate(card, rating, now)
	}
	p.problem = ""
	p.due = p.progress.Due(p.cards, now)
	p.turn(1)
}

func (p *workspacePresentation) resetProgress() {
	if p.origin.vault != nil && p.origin.note != "" {
		if err := p.origin.vault.ResetFlashcardProgress(p.origin.note); err != nil {
			p.problem = err.Error()
			return
		}
	}
	p.problem = ""
	p.progress = presentation.Progress{}
	p.due = len(p.cards)
}

type presentationAction string

const (
	presentationPrevious   presentationAction = "previous"
	presentationNext       presentationAction = "next"
	presentationPrimary    presentationAction = "primary"
	presentationShuffle    presentationAction = "shuffle"
	presentationLayout     presentationAction = "layout"
	presentationZoomIn     presentationAction = "zoom-in"
	presentationZoomOut    presentationAction = "zoom-out"
	presentationFullScreen presentationAction = "fullscreen"
	presentationHelp       presentationAction = "help"
	presentationClose      presentationAction = "close"
)

var presentationKeys = []struct {
	key     mygoui.Key
	mods    mygoui.Modifiers
	browser string
	action  presentationAction
}{
	{mygoui.KeyLeft, 0, "ArrowLeft", presentationPrevious},
	{mygoui.KeyRight, 0, "ArrowRight", presentationNext},
	{mygoui.KeySpace, 0, " ", presentationPrimary},
	{mygoui.KeyR, 0, "r", presentationShuffle},
	{mygoui.KeyW, 0, "w", presentationLayout},
	{mygoui.KeyEqual, mygoui.Shift, "+", presentationZoomIn},
	{mygoui.KeyMinus, 0, "-", presentationZoomOut},
	{mygoui.KeyF, 0, "f", presentationFullScreen},
	{mygoui.KeySlash, mygoui.Shift, "?", presentationHelp},
	{mygoui.KeyEscape, 0, "Escape", presentationClose},
	{mygoui.Key1, 0, "1", "1"},
	{mygoui.Key2, 0, "2", "2"},
	{mygoui.Key3, 0, "3", "3"},
	{mygoui.Key4, 0, "4", "4"},
}

func (p *workspacePresentation) act(app *workspaceWindow, action presentationAction) {
	if app.presentation != p {
		return
	}
	switch action {
	case presentationPrevious:
		p.turn(-1)
	case presentationNext:
		p.turn(1)
	case presentationPrimary:
		if p.flash {
			p.revealed = p.count() > 0 && !p.revealed
		} else if p.timer != nil {
			p.stopTimer()
		} else if p.count() > 1 {
			p.startTimer(app)
		}
	case presentationShuffle:
		p.toggleShuffle()
	case presentationLayout:
		p.layout = (p.layout + 1) % len(presentationLayouts)
	case presentationZoomIn:
		p.zoom = min(200, p.zoom+10)
	case presentationZoomOut:
		p.zoom = max(50, p.zoom-10)
	case presentationFullScreen:
		app.win.SetFullScreen(!app.win.IsFullScreen())
	case presentationHelp:
		p.help = !p.help
	case presentationClose:
		app.closePresentation()
	case "1", "2", "3", "4":
		p.rate(presentation.Rating(action[0] - '0'))
	}
}

var presentationLayouts = [...]struct{ css, label, icon string }{
	{"standard", "Standard", "rectangle"},
	{"wide", "Wide", "rectangle.landscape"},
	{"card", "Card", "rectangle.portrait"},
}

func (p *workspacePresentation) draw(c *mygoui.Context, app *workspaceWindow) {
	for _, key := range presentationKeys {
		c.OnWindowShortcut(key.mods, key.key, func() { p.act(app, key.action) })
	}
	mygoui.Column(c.Key("presentation")).Fill().Label("Presentation Mode").Background(mygoui.RGB(0, 0, 0)).Children(func() {
		// EV-UI-084: the content starts at y=112 and extends 12 points
		// into the footer's frame. Ratings overlay the content.
		mygoui.Box(c).Height(112).Shrink(0).FillWidth().DragWindow()
		p.web.draw(c, p)
		mygoui.Box(c).Height(60).Shrink(0)
	})
	mygoui.Overlay(c, func() {
		mygoui.Row(c).Absolute().Right(max(16, c.TitleBar().Right)).Top(120).Height(32).Gap(8).
			AlignItems(mygoui.Center).Children(func() {
			p.button(c, app, "questionmark.circle", "Question Mark In A Circle", presentationHelp).Tooltip("Presentation help")
			p.button(c, app, "xmark", "Close", presentationClose)
		})
		if p.problem != "" {
			mygoui.Text(c, p.problem).Absolute().Left(40).Right(40).Bottom(132).
				Role(mygoui.RoleAlert).Padding(6, 12).Background(mygoui.Hex("#231213")).
				TextColor(mygoui.Hex("#ff9a90"))
		}
		if p.flash {
			p.drawReview(c)
		}
		p.drawControls(c, app)
	})
	if p.help {
		p.drawHelp(c)
	}
}

var presentationText = mygoui.Hex("#c4c4c4")
var presentationMuted = mygoui.Hex("#929292")

func (p *workspacePresentation) button(c *mygoui.Context, app *workspaceWindow, symbol, label string, action presentationAction) mygoui.Element {
	return chromeButtonIcon(c, symbol, label, func() { p.act(app, action) }, 14, presentationText).Size(32, 32).Shrink(0)
}

func (p *workspacePresentation) drawControls(c *mygoui.Context, app *workspaceWindow) {
	width, _ := c.Size()
	inset, gap := float32(40), float32(20)
	if width < 780 {
		inset, gap = 12, 10
	}
	mygoui.Row(c).Absolute().Left(inset).Right(inset).Bottom(16).Height(56).
		Padding(0, 16, 0, 34).AlignItems(mygoui.Center).Gap(gap).
		Radius(12).Background(mygoui.Hex("#0a0a0a")).Label("Presentation Controls").Children(func() {
		mygoui.Column(c).Width(55).Shrink(0).Children(func() {
			position := 0
			if p.count() > 0 {
				position = p.position + 1
			}
			mygoui.Text(c, fmt.Sprintf("%d / %d", position, p.count())).FontSize(14).Bold().TextColor(presentationMuted)
			if p.flash {
				mygoui.Text(c, fmt.Sprintf("%d due", p.due)).FontSize(11).Bold().TextColor(mygoui.Hex("#cb7c27"))
			}
		})
		mygoui.Spacer(c)
		mygoui.Row(c).Gap(gap).AlignItems(mygoui.Center).Shrink(0).Children(func() {
			p.button(c, app, "backward.end.fill", "Go To End", presentationPrevious).Tooltip("Previous")
			label, icon := "Play", "play.fill"
			color := presentationText
			if p.flash {
				label, icon = "Show", "eye"
				if p.revealed {
					label, icon = "Hide", "eye.slash"
					color = mygoui.Hex("#459bfa")
				}
			} else if p.timer != nil {
				label, icon = "Pause", "pause.fill"
			}
			chromeButtonIcon(c, icon, label, func() { p.act(app, presentationPrimary) }, 14, color).
				Size(32, 32).Shrink(0).Disabled(p.count() == 0)
			p.button(c, app, "forward.end.fill", "Go To Start", presentationNext).Tooltip("Next")
		})
		mygoui.Spacer(c)
		mygoui.Row(c).Gap(gap).AlignItems(mygoui.Center).Shrink(0).Children(func() {
			p.button(c, app, "shuffle", "Shuffle", presentationShuffle).AccessSelected(p.order != nil)
			chromeMenuControl(c, "Timer", func(m *mygoui.Menu) {
				for _, seconds := range []int{3, 5, 8, 12, 20} {
					interval := time.Duration(seconds) * time.Second
					if m.Item(fmt.Sprintf("%d seconds", seconds)).Radio(interval == p.interval).Chosen() {
						p.interval = interval
						if p.timer != nil {
							p.startTimer(app)
						}
					}
				}
			}).Size(34, 32).Gap(3).Children(func() {
				chromeIconColor(c, "timer", 14, presentationText)
				chromeIconColor(c, "chevron.down", 6, presentationText)
			})
			p.button(c, app, "minus.magnifyingglass", "Zoom Out", presentationZoomOut).Disabled(p.zoom == 50)
			mygoui.Text(c, strconv.Itoa(p.zoom)+"%").Width(44).TextAlign(mygoui.Center).
				FontSize(12).Bold().TextColor(presentationMuted)
			p.button(c, app, "plus.magnifyingglass", "Zoom In", presentationZoomIn).Disabled(p.zoom == 200)
			layout := presentationLayouts[p.layout]
			p.button(c, app, layout.icon, "Rectangle", presentationLayout).Tooltip(layout.label + " Layout")
			if p.flash {
				chromeButtonIcon(c, "arrow.counterclockwise", "Counterclockwise Curved Arrow", p.resetProgress,
					14, presentationText, "Reset review progress").Size(32, 32).Shrink(0)
			}
			fullLabel, fullIcon := "Enter Full Screen", "arrow.up.left.and.arrow.down.right"
			if app.win.IsFullScreen() {
				fullLabel, fullIcon = "Exit Full Screen", "arrow.down.right.and.arrow.up.left"
			}
			p.button(c, app, fullIcon, fullLabel, presentationFullScreen)
		})
	})
}

func (p *workspacePresentation) drawReview(c *mygoui.Context) {
	mygoui.Row(c).Absolute().Left(0).Right(0).Bottom(75).Height(53).Center().Gap(11).Children(func() {
		if !p.revealed {
			if p.count() > 0 {
				mygoui.Text(c, "Click or press Space to reveal answer").FontSize(12).TextColor(presentationMuted)
			}
			return
		}
		for index, item := range []struct{ label, fill, border string }{
			{"Again", "#450d0d", "#751a1a"},
			{"Hard", "#442606", "#6a380b"},
			{"Good", "#052341", "#0b467b"},
			{"Easy", "#093616", "#125e28"},
		} {
			key := strconv.Itoa(index + 1)
			rating := presentation.Rating(index + 1)
			mygoui.ButtonBase(c.Key(item.label)).Label(item.label+", "+key).Size(71, 45).Radius(8).
				Background(mygoui.Hex(item.fill)).Border(1, mygoui.Hex(item.border)).
				OnClick(func() { p.rate(rating) }).Children(func() {
				mygoui.Column(c).AlignItems(mygoui.Center).Gap(1).Children(func() {
					mygoui.Text(c, item.label).FontSize(13).Bold().TextColor(mygoui.RGB(255, 255, 255))
					mygoui.Text(c, key).FontSize(10).Bold().TextColor(presentationMuted)
				})
			})
		}
	})
}

func (p *workspacePresentation) drawHelp(c *mygoui.Context) {
	w, h := c.Size()
	width, height := min(500, w-32), min(470, h-32)
	mygoui.Overlay(c, func() {
		mygoui.Column(c.Key("presentation help")).Absolute().Left((w-width)/2).Top((h-height)/2).Size(width, height).
			Background(mygoui.Hex("#171717")).Border(1, mygoui.Hex("#333333")).Radius(14).Padding(18, 22).
			Label("Presentation Help").Gap(8).Children(func() {
			mygoui.Row(c).FillWidth().AlignItems(mygoui.Center).Children(func() {
				mygoui.Text(c, "Presentation Mode").Role(mygoui.RoleHeading).FontSize(20).Bold().TextColor(presentationText)
				mygoui.Spacer(c)
				chromeButtonIcon(c, "xmark", "Close help", func() { p.help = false }, 14, presentationText).Size(28, 28)
			})
			mygoui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Gap(9).Children(func() {
				lines := []string{
					"How to format your document",
					"Use --- dividers to separate slides or cards.",
					"Without dividers, ## or ### headings start slides.",
					"YAML title, author, date and tags enrich the cover.",
				}
				if p.flash {
					lines = append(lines, "Use ??? between the question and the answer.", "Space reveals the answer. 1–4 rate recall.")
				} else {
					lines = append(lines, "Space starts or pauses automatic advance.")
				}
				lines = append(lines, "Keyboard shortcuts", "← / →    Previous / Next", "R    Toggle shuffle",
					"W    Cycle layout", "+ / −    Zoom", "F    Fullscreen", "?    Help", "Esc    Exit")
				for _, line := range lines {
					mygoui.Text(c, line).FontSize(13).TextColor(presentationText)
				}
			})
		})
	})
}
