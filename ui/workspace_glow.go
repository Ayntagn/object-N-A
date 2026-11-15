package ui

import (
	"math"
	"path/filepath"
	"strings"
	"time"

	"cyberwriter/internal/core/glow"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type glowHost string

const (
	glowDock  glowHost = "docked"
	glowShade glowHost = "shade"
	glowFloat glowHost = "window"

	glowDefaultHeight = 280
	glowMinHeight     = 140
	glowMaxHeight     = 640
	glowWindowKey     = "glowTerminal"
)

type glowPreferences struct {
	Mode     glowHost `json:"mode"`
	Theme    string   `json:"theme"`
	FontSize float32  `json:"fontSize"`
	Height   float32  `json:"height"`
}

func (p glowPreferences) normalized() glowPreferences {
	switch p.Mode {
	case glowDock, glowShade, glowFloat:
	default:
		p.Mode = glowDock
	}
	switch p.Theme {
	case "plato", "mono", "matrix":
	default:
		p.Theme = "plato"
	}
	if p.FontSize == 0 {
		p.FontSize = 12.5
	}
	p.FontSize = min(20, max(10, p.FontSize))
	if p.Height == 0 {
		p.Height = glowDefaultHeight
	}
	p.Height = min(glowMaxHeight, max(glowMinHeight, p.Height))
	return p
}

type workspaceGlowController struct {
	session   glow.Session
	window    *mygo.Window
	pane      *workspaceGlowPane
	demoTimer *time.Timer
}

type workspaceGlow struct {
	pane      *workspaceGlowPane
	host      glowHost
	shown     bool
	maximized bool
}

type workspaceGlowPane struct {
	application *workspaceApplication
	document    *workspaceWindow // The floating pane follows the active document.
	input       string
	history     glow.HistoryCursor
	scroll      mygoui.ScrollState

	slashInput     string
	slashSelection int

	focus      bool
	moveCaret  bool
	heightGrip mygoui.Handle
	heightDrag glowHeightDrag
}

type glowHeightDrag struct {
	startY, startHeight float32
	active, moved       bool
}

func (app *workspaceWindow) glowEnvironment() glow.Environment {
	return glow.Environment{
		Vault: app.vault, Name: filepath.Base(app.root), Entries: app.entries, Index: app.index.data,
		Active: app.currentVaultPath(), Recent: app.recentPaths, TrashItem: mygo.Shell.TrashItem,
	}
}

func (app *workspaceWindow) toggleGlow() {
	if app.vault == nil || app.modalOpen || app.application == nil {
		return
	}
	host := glowDock
	if app.preferences.Terminal.Mode == glowShade {
		host = glowShade
	}
	controller := &app.application.glow
	if controller.window != nil && !controller.window.IsDestroyed() {
		app.application.closeGlowWindow()
	} else if app.glow.shown && app.glow.host == host {
		app.hideGlow()
		return
	}
	app.showEmbeddedGlow(host)
}

func (app *workspaceWindow) showEmbeddedGlow(host glowHost) {
	app.application.glow.session.Start(app.glowEnvironment())
	app.glow = workspaceGlow{
		host: host, shown: true,
		pane: &workspaceGlowPane{
			application: app.application, document: app,
			scroll: mygoui.ScrollState{Y: math.MaxFloat32}, focus: true,
		},
	}
	focusWorkspaceSurface(app.win)
	app.invalidateGlow()
}

func (app *workspaceWindow) hideGlow() {
	if !app.glow.shown {
		return
	}
	app.glow = workspaceGlow{}
	app.invalidateGlow()
}

func (p *workspaceGlowPane) workspace() *workspaceWindow {
	if p.document != nil {
		return p.document
	}
	return p.application.activeWorkspace()
}

func (p *workspaceGlowPane) window() *mygo.Window {
	if p.document != nil {
		return p.document.win
	}
	return p.application.glow.window
}

func (p *workspaceGlowPane) isOpen() bool {
	if p.document != nil {
		return p.document.glow.pane == p
	}
	return p.application.glow.pane == p
}

func (p *workspaceGlowPane) close() {
	if !p.isOpen() {
		return
	}
	if p.document != nil {
		p.document.hideGlow()
	} else {
		p.application.closeGlowWindow()
	}
}

func (p *workspaceGlowPane) setHost(host glowHost) {
	app := p.workspace()
	if app == nil || !p.isOpen() {
		return
	}
	current := glowFloat
	if p.document != nil {
		current = app.glow.host
	}
	if host == current {
		return
	}
	if host == glowFloat {
		p.application.showGlowWindow(app)
		return
	}
	app.preferences.Terminal.Mode = host
	app.writePreferences()
	if p.document != nil {
		app.showEmbeddedGlow(host)
		return
	}
	app.invalidateGlow()
	a := p.application
	a.closeGlowWindow()
	// EV-UI-099: the document that is key after the panel closes receives the toggle.
	time.AfterFunc(150*time.Millisecond, func() {
		mygo.RunOnMain(func() {
			if target := a.activeWorkspace(); target != nil && target.win.IsFocused() {
				target.toggleGlow()
			}
		})
	})
}

func (p *workspaceGlowPane) setTheme(theme string) {
	app := p.workspace()
	if app != nil && p.isOpen() {
		app.setGlowTheme(theme)
	}
}

func (app *workspaceWindow) setGlowTheme(theme string) {
	if theme == app.preferences.Terminal.Theme {
		return
	}
	app.preferences.Terminal.Theme = theme
	app.writePreferences()
	app.invalidateGlow()
	app.application.refreshGlow(nil)
}

func (p *workspaceGlowPane) changeFont(delta float32) {
	app := p.workspace()
	if app == nil || !p.isOpen() {
		return
	}
	prefs := &app.preferences.Terminal
	size := min(20, max(10, prefs.FontSize+delta))
	if size == prefs.FontSize {
		return
	}
	prefs.FontSize = size
	app.writePreferences()
	app.invalidateGlow()
	p.application.refreshGlow(nil)
}

func (p *workspaceGlowPane) toggleMaximize() {
	app := p.document
	if app == nil || !p.isOpen() {
		return
	}
	app.glow.maximized = !app.glow.maximized
	p.focus = true
	focusWorkspaceSurface(app.win)
	app.invalidateGlow()
}

// EV-UI-099 and EV-UI-101: both embedded hosts adjust the same height.
func (p *workspaceGlowPane) resizeHeight(event mygoui.InputEvent, y float32) bool {
	app := p.document
	if app == nil || !p.isOpen() || app.glow.maximized {
		return false
	}
	height := &app.preferences.Terminal.Height
	drag := &p.heightDrag
	switch event.Kind {
	case mygoui.InputPointerDown:
		if event.Button != 0 || drag.active {
			return false
		}
		*drag = glowHeightDrag{active: true, startY: y, startHeight: *height}
	case mygoui.InputPointerMove, mygoui.InputPointerUp:
		if !drag.active {
			return false
		}
		delta := y - drag.startY
		if !event.Canceled && (drag.moved || delta <= -1 || delta >= 1) {
			drag.moved = true
			next := min(glowMaxHeight, max(glowMinHeight, drag.startHeight-delta))
			if next != *height {
				*height = next
				p.application.refreshGlow(nil)
			}
		}
		if event.Kind == mygoui.InputPointerUp {
			if *height != drag.startHeight {
				app.writePreferences()
			}
			*drag = glowHeightDrag{}
		}
	default:
		return false
	}
	return true
}

func (a *workspaceApplication) showGlowWindow(origin *workspaceWindow) {
	for _, app := range a.windows {
		app.hideGlow()
	}
	if win := a.glow.window; win != nil && !win.IsDestroyed() {
		win.Focus()
		return
	}
	a.glow.session.Start(origin.glowEnvironment())
	pane := &workspaceGlowPane{application: a, scroll: mygoui.ScrollState{Y: math.MaxFloat32}}
	a.glow.pane = pane
	win := mygo.NewWindow(mygo.WindowOptions{
		// EV-UI-101: the resized native panel can reach the header and input alone.
		Title: "Terminal", Width: 720, Height: 444, MinWidth: 354, MinHeight: 87,
		Panel: true, Hidden: true, StateKey: glowWindowKey,
		TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: glowFloatTitleHeight,
		TrafficLightPosition: &mygo.Point{X: 8, Y: 6},
		Content: mygoui.View(func(c *mygoui.Context) {
			if app := a.activeWorkspace(); app != nil {
				app.applyAppearance(c)
			}
			c.OnWindowShortcut(mygoui.Ctrl, mygoui.KeyBackquote, pane.close)
			pane.view(c, glowFloat)
		}),
	})
	a.glow.window = win
	configureGlowPanel(win)
	win.OnClosed(func() {
		if a.glow.window == win {
			a.glow.window, a.glow.pane = nil, nil
		}
	})
	win.Show()
}

func (a *workspaceApplication) closeGlowWindow() {
	win := a.glow.window
	a.glow.window, a.glow.pane = nil, nil
	if win != nil && !win.IsDestroyed() {
		win.Close()
	}
}

func (app *workspaceWindow) invalidateGlow() {
	if app.win != nil && !app.win.IsDestroyed() {
		app.win.Invalidate()
	}
	if app.application != nil && app.application.glow.pane != nil {
		app.application.glow.pane.refresh(nil)
	}
}

func (a *workspaceApplication) refreshGlow(source *workspaceGlowPane) {
	for _, app := range a.windows {
		if pane := app.glow.pane; pane != nil {
			pane.refresh(source)
		}
	}
	if pane := a.glow.pane; pane != nil {
		pane.refresh(source)
	}
}

func (p *workspaceGlowPane) refresh(source *workspaceGlowPane) {
	if source != nil && (p == source || p.scroll.Y >= p.scroll.MaxY) {
		p.scroll.Y = math.MaxFloat32
	}
	if win := p.window(); win != nil && !win.IsDestroyed() {
		win.Invalidate()
	}
}

func (p *workspaceGlowPane) submit() {
	app := p.workspace()
	if app == nil || !p.isOpen() {
		return
	}
	input := p.input
	if app.modalOpen || app.vault == nil {
		p.clearInput()
		return
	}
	session := &p.application.glow.session
	if !session.DemoRunning() {
		if strings.TrimSpace(input) == "" {
			p.clearInput()
			return
		}
		if menu := p.slashMenu(input); len(menu.Rows) > 0 {
			p.submitAppCommand(app, "/"+menu.Rows[p.slashSelection].Slug)
			return
		}
		if strings.HasPrefix(strings.TrimSpace(input), "/") {
			p.submitAppCommand(app, input)
			return
		}
	}
	p.clearInput()
	env := app.glowEnvironment()
	env.InsertMarkdown = app.insertMarkdownAtSelection
	env.AppCommandAvailable = func(command glow.AppCommand) bool { return app.glowAppAction(command) != nil }
	result := session.Submit(input, env)
	app.applyGlowResult(result)
	if result.DemoDelay > 0 || !session.DemoRunning() {
		p.application.scheduleGlowDemo(result.DemoDelay)
	}
	p.focus = true
	for _, effect := range result.ScriptEffects {
		env = app.glowEnvironment()
		env.InsertMarkdown = app.insertMarkdownAtSelection
		action := session.ApplyScriptEffect(effect, env)
		if action.OpenPath != "" {
			// Script navigation retains earlier edits in their own tab.
			app.wys.sync(app)
			if app.workspaceTab.dirty() {
				app.openNote(action.OpenPath, true)
				app.focusSource, app.wys.focus = false, false
				action.OpenPath = ""
			}
		}
		app.applyGlowResult(action)
		if action.AppCommand.Slug != "" {
			p.runAppCommand(app, action.AppCommand)
		}
		if action.DemoDelay > 0 {
			p.application.scheduleGlowDemo(action.DemoDelay)
		}
	}
	app.invalidateGlow()
	p.application.refreshGlow(p)
}

func (app *workspaceWindow) applyGlowResult(result glow.Result) {
	if result.Theme != "" {
		app.setGlowTheme(result.Theme)
	}
	if result.VaultChanged {
		app.refreshVault()
	}
	if result.OpenPath != "" {
		app.openDocument(result.OpenPath)
		app.focusSource, app.wys.focus = false, false
	}
}

func (a *workspaceApplication) scheduleGlowDemo(delay time.Duration) {
	controller := &a.glow
	if controller.demoTimer != nil {
		controller.demoTimer.Stop()
		controller.demoTimer = nil
	}
	if delay <= 0 {
		return
	}
	var timer *time.Timer
	timer = time.AfterFunc(delay, func() {
		mygo.RunOnMain(func() {
			if controller.demoTimer != timer {
				return
			}
			controller.demoTimer = nil
			app := a.activeWorkspace()
			if app == nil {
				controller.session.StopDemo()
				return
			}
			env := app.glowEnvironment()
			result, next := controller.session.TickDemo(env)
			app.applyGlowResult(result)
			pane := controller.pane
			if pane == nil {
				pane = app.glow.pane
			}
			a.refreshGlow(pane)
			a.scheduleGlowDemo(next)
		})
	})
	controller.demoTimer = timer
}

func (p *workspaceGlowPane) clearInput() {
	p.input, p.slashInput, p.slashSelection = "", "", 0
	p.history = glow.HistoryCursor{}
}

func (p *workspaceGlowPane) openLink(path string) {
	if app := p.workspace(); app != nil && p.isOpen() {
		if path == glow.ExampleVaultLink {
			app.openExampleVault(false)
		} else {
			app.openDocument(path)
		}
		app.invalidateGlow()
	}
}

func (p *workspaceGlowPane) slashMenu(input string) glow.SlashMenu {
	if p.slashInput != input {
		p.slashInput, p.slashSelection = input, 0
	}
	return glow.SlashSuggestions(input)
}

func (p *workspaceGlowPane) clickAppCommand(slug string) {
	win := p.window()
	if win == nil || win.IsDestroyed() || !p.isOpen() {
		return
	}
	input := p.input
	// Keep the hit target in place through the release before shrinking the menu.
	win.Update(func() {
		if app := p.workspace(); app != nil && p.isOpen() && p.input == input {
			p.submitAppCommand(app, slug)
		}
	})
}

func (p *workspaceGlowPane) submitAppCommand(app *workspaceWindow, input string) {
	if app.modalOpen || app.vault == nil {
		return
	}
	p.clearInput()
	command := p.application.glow.session.SubmitApp(input, app.glowEnvironment(), func(command glow.AppCommand) bool {
		return app.glowAppAction(command) != nil
	})
	if command.Slug == "" {
		p.focus = true
	} else {
		p.runAppCommand(app, command)
		app.invalidateGlow()
	}
	p.application.refreshGlow(p)
}

func (app *workspaceWindow) glowAppAction(command glow.AppCommand) func() {
	switch command.Slug {
	case "features":
		return app.showFeatureGuide
	case "page-break":
		return app.insertPageBreak
	case "table-of-contents":
		return app.insertContents
	case "editor", "split", "preview", "wys", "focus", "outline", "pdf-preview", "snippets", "terminal",
		"find", "replace", "line-numbers", "sync", "font-up", "font-down", "map-vault", "health", "new-folder", "slides", "flashcards", "custom-stylesheet",
		"readme", "meeting-notes", "blog-post", "project-plan", "changelog", "save-template":
		return app.commandPaletteActions()[commandPaletteKey{strings.ToUpper(command.Category), command.Title}]
	default:
		return nil
	}
}

func (p *workspaceGlowPane) runAppCommand(app *workspaceWindow, command glow.AppCommand) {
	action := app.glowAppAction(command)
	if action == nil {
		return
	}
	wasWYSIWYG := app.displayedMode() == wysiwygMode
	editorInsert := command.Category == "Templates" || command.Slug == "page-break" || command.Slug == "table-of-contents"
	var richStart bool
	if editorInsert && wasWYSIWYG && app.wys.editable(app) {
		anchor, caret := app.wys.session.Document.Selection()
		richStart = anchor == 0 && caret == 0
	}
	switch command.Slug {
	case "features":
		app.openFeatureGuide("", p.window())
	case "custom-stylesheet":
		app.openFeatureGuide("custom-css", p.window())
	default:
		action()
	}
	switch {
	case command.Slug == "replace":
		app.findMode = findReplaceQuery
	case command.Slug == "find" || command.Slug == "terminal":
		// The find field takes focus, or GLOW closes.
	case editorInsert:
		app.focusSource = false
		app.wys.focus = richStart && p.document != nil
		if app.wys.focus && command.Slug == "readme" {
			// The native visual editor returns to the new opening heading.
			app.wys.session.Document.SetSelection(0, 0)
		}
		p.focus = !app.wys.focus
	case command.Slug == "snippets":
		app.snippets.focusSearch = false
		p.focus = true
	case command.Slug == "split":
		if !wasWYSIWYG || p.document == nil {
			app.focusSource = false
			p.focus = true
		}
	default:
		app.focusSource, app.wys.focus = false, false
		p.focus = true
	}
}

func (p *workspaceGlowPane) key(key mygoui.TextKey) bool {
	app := p.workspace()
	if app == nil || app.modalOpen || !p.isOpen() {
		return false
	}
	session := &p.application.glow.session
	if session.DemoRunning() {
		if key.Mods == mygoui.Ctrl && key.Key == mygoui.KeyC {
			key.SetText("")
			p.clearInput()
			session.StopDemo()
			p.application.scheduleGlowDemo(0)
			p.application.refreshGlow(p)
			return true
		}
		return false
	}
	if key.Mods != 0 {
		return false
	}
	text := key.Text()
	if menu := p.slashMenu(text); menu.Open {
		return p.slashKey(key, menu.Rows)
	}
	var replacement string
	switch key.Key {
	case mygoui.KeyUp:
		replacement = session.RecallFrom(&p.history, text, -1)
	case mygoui.KeyDown:
		replacement = session.RecallFrom(&p.history, text, 1)
	case mygoui.KeyTab:
		previous := len(session.Lines())
		replacement = session.Complete(text, app.glowEnvironment())
		if len(session.Lines()) != previous {
			p.application.refreshGlow(p)
		}
	default:
		return false
	}
	if replacement != text {
		key.SetText(replacement)
		p.moveCaret = true
	}
	return true
}

func (p *workspaceGlowPane) slashKey(key mygoui.TextKey, rows []glow.AppCommand) bool {
	switch key.Key {
	case mygoui.KeyUp, mygoui.KeyDown:
		if len(rows) > 1 {
			delta := 1
			if key.Key == mygoui.KeyUp {
				delta = -1
			}
			p.slashSelection = (p.slashSelection + delta + len(rows)) % len(rows)
		}
		p.focus = true
		p.refresh(nil)
	case mygoui.KeyTab:
		if len(rows) > 0 {
			key.SetText("/" + rows[p.slashSelection].Slug)
			p.moveCaret = true
		}
	case mygoui.KeyEscape:
		key.SetText("")
	default:
		return false
	}
	return true
}

func (app *workspaceWindow) glowShadeView(c *mygoui.Context) {
	if !app.glow.shown || app.glow.host != glowShade || app.drawing != nil {
		return
	}
	left, bottom := float32(6), float32(0)
	if app.vaultPanelVisible() {
		left += vaultPanelWidth
	}
	if app.draft != nil && !app.focusMode {
		bottom = statusBarHeight - 2
	}
	mygoui.Overlay(c, func() {
		mygoui.Column(c.Key("GLOW shade")).Absolute().Left(left).Right(0).Bottom(bottom).
			Height(app.preferences.Terminal.Height).Children(func() { app.glowPane(c, glowShade) })
	})
}
