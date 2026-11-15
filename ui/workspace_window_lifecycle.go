package ui

import (
	"errors"
	"slices"

	"cyberwriter/internal/core/aiworkspace"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type workspaceApplication struct {
	preferences  workspacePreferences
	aiHistory    *aiworkspace.History
	windows      []*workspaceWindow
	active       *workspaceWindow
	welcome      workspaceWelcome
	glow         workspaceGlowController
	featureGuide workspaceFeatureGuide
	stylesheet   workspaceStylesheet
	pdfWindows   map[string]*workspacePDFWindow
	imageWindows map[string]*workspaceImageWindow
	quitSession  *savedSession
	closeErr     error
}

func (a *workspaceApplication) newWindow(tab *workspaceTab, origin *workspaceWindow) *workspaceWindow {
	app := &workspaceWindow{
		application: a, preferences: &a.preferences, blackout: a.preferences.Appearance == appearanceBlackout,
		vaultVisible: true, previewZoom: normalizedPreviewZoom(a.preferences.Session.PreviewZoom),
		workspaceTab: tab, tabs: []*workspaceTab{tab},
	}
	if !a.preferences.AIStreamIntoEditor {
		app.quickActionRoute = quickActionChat
	}
	options := mygo.WindowOptions{
		Title: tab.title(), Width: 1294, Height: 831, MinWidth: 720, MinHeight: 420, Hidden: true,
		TitleBarStyle:        mygo.TitleBarHiddenInset,
		TitleBarHeight:       titleBarHeight,
		TrafficLightPosition: &mygo.Point{X: 18, Y: 18},
		Vibrancy:             workspaceVibrancy(),
		Content:              mygoui.View(app.view),
	}
	if origin != nil {
		bounds := origin.win.Bounds()
		options.X, options.Y = bounds.X+29, bounds.Y+29
		options.Width, options.Height = bounds.Width, bounds.Height
		app.vault, app.root, app.entries = origin.vault, origin.root, origin.entries
		app.vaultTreeEntries = origin.vaultTreeEntries
		app.vaultFileCount = origin.vaultFileCount
		app.index.data = origin.index.data
		app.folders = make(map[string]*bool)
		app.previewZoom = origin.previewZoom
	}
	if tab.code != nil {
		// Preserve the buffer, selection and undo history when the tab moves.
		tab.code.OnInput = app.sourceInput
		tab.code.ContextMenu = app.sourceContextMenu
	}
	a.windows = append(a.windows, app)
	app.win = mygo.NewWindow(options)
	app.previewPane.init(app)
	app.win.OnClose(app.closeRequested)
	app.win.OnClosed(app.closeGraphWindow)
	app.win.OnClosed(app.closePresentation)
	app.win.OnClosed(app.ai.stopResponse)
	app.win.OnClosed(func() { a.releaseWindow(app) })
	app.win.OnFocus(func() { a.activate(app) })
	app.refreshTemplateLibrary()
	if app.menu == nil {
		app.buildMenu()
	}
	app.installWorkspaceShortcutMonitor()
	app.activateTab(tab)
	if app.vault != nil && app.index.data == nil {
		app.requestIndex()
	}
	app.startVaultWatch()
	return app
}

func (a *workspaceApplication) activate(app *workspaceWindow) {
	if app.win.IsDestroyed() {
		return
	}
	app.refreshTemplateLibrary()
	if mygo.App.Menu() != app.menu {
		mygo.App.SetMenu(app.menu)
	}
	if app != a.active {
		a.active = app
		app.invalidateGlow()
	}
}

func (a *workspaceApplication) activeWorkspace() *workspaceWindow {
	if a.active != nil && !a.active.win.IsDestroyed() {
		return a.active
	}
	for _, app := range a.windows {
		if !app.win.IsDestroyed() {
			return app
		}
	}
	return nil
}

func (app *workspaceWindow) documentWindows() []*workspaceWindow {
	if app.application == nil {
		return []*workspaceWindow{app}
	}
	return app.application.windows
}

func (a *workspaceApplication) releaseWindow(app *workspaceWindow) {
	index := slices.Index(a.windows, app)
	if index < 0 {
		return
	}
	a.windows = slices.Delete(a.windows, index, index+1)
	if app == a.active {
		a.active = nil
	}
	if app.index.stopWatch != nil {
		app.index.stopWatch()
	}
	app.clearUnlinkedMentions()
	app.clearVaultSidebarSearch()
	storages := map[*workspace.Vault]bool{app.vault: true}
	for _, tab := range app.tabs {
		tab.cancelAPIRequests()
		storages[tab.storage] = true
	}
	app.vault, app.tabs = nil, nil
	for storage := range storages {
		a.closeErr = errors.Join(a.closeErr, app.closeUnusedVault(storage))
	}
	if current := a.activeWorkspace(); current != nil {
		if focused := mygo.FocusedWindow(); focused == nil || focused == current.win {
			a.activate(current)
		}
		current.persistSession()
	} else {
		a.closeGlowWindow()
	}
}

func (app *workspaceWindow) moveTabToNewWindow() {
	if app.modalOpen || len(app.tabs) < 2 {
		return
	}
	app.wys.sync(app)
	tab := app.workspaceTab
	index := slices.Index(app.tabs, tab)
	app.tabs = slices.Delete(app.tabs, index, index+1)
	app.activateTab(app.tabs[min(index, len(app.tabs)-1)])
	next := app.application.newWindow(tab, app)
	next.win.Show()
	app.application.activate(next)
	next.persistSession()
}

func (a *workspaceApplication) installSessionLifecycle() {
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) {
		a.quitSession = nil
		if current := a.activeWorkspace(); current != nil {
			session := current.sessionSnapshot()
			a.quitSession = &session
		}
	})
	mygo.App.OnWillQuit(func(_ *mygo.QuitEvent) {
		if a.quitSession != nil {
			a.preferences.Session = *a.quitSession
			a.closeErr = errors.Join(a.closeErr, a.preferences.write())
		}
	})
}

func (a *workspaceApplication) restoreSession(primary *workspaceWindow, session savedSession) {
	if session.Vault != primary.root {
		return
	}
	primary.restoreWindow(session.savedWindow)
	for _, window := range session.OtherWindows {
		app := a.newWindow(&workspaceTab{mode: a.preferences.DefaultView}, primary)
		app.restoreWindow(window)
		if app.document.Path != "" {
			app.win.Show()
		} else {
			app.win.Close()
		}
	}
}
