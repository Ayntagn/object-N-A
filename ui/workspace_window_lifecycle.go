package ui

import (
	"errors"
	"log"
	"runtime"
	"slices"

	"cyberwriter/internal/core/aiworkspace"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type workspaceApplication struct {
	preferences     workspacePreferences
	aiHistory       *aiworkspace.History
	aiVaultContexts map[string]*workspaceAIVaultContext
	windows         []*workspaceWindow
	active          *workspaceWindow
	welcome         workspaceWelcome
	splash          *workspaceSplash
	glow            workspaceGlowController
	featureGuide    workspaceFeatureGuide
	stylesheet      workspaceStylesheet
	pdfWindows      map[string]*workspacePDFWindow
	imageWindows    map[string]*workspaceImageWindow
	quitSession     *savedSession
	quitting        bool
	closeErr        error
	idlePicker      bool

	deferredVaultOffers map[string]bool
}

func (a *workspaceApplication) prepareWorkspace(root string, session savedSession) (*workspaceWindow, bool) {
	if root == "" {
		root = session.Vault
	}
	app := a.newWindow(&workspaceTab{mode: a.preferences.DefaultView}, nil)
	opened := root != "" && app.openVault(root)
	if opened {
		a.restoreSession(app, session)
	}
	if app.draft == nil && app.drawing == nil {
		app.setTabDocument(app.workspaceTab, workspace.Document{})
		app.initialEmptyTab = true
		app.activateTab(app.workspaceTab)
	}
	return app, opened
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
		app.vaultEmbeddingOfferPath = origin.vaultEmbeddingOfferPath
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
	app.win.OnClosed(app.ai.clearAIReferences)
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
	} else {
		app.syncAIVaultContext()
	}
	app.startVaultWatch()
	return app
}

func (a *workspaceApplication) activate(app *workspaceWindow) {
	if app.win.IsDestroyed() {
		return
	}
	if app.sidebarSearch.relatedRefreshDeferred {
		app.sidebarSearch.relatedRefreshDeferred = false
		app.win.Invalidate()
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
	contextRoot := app.aiVaultContextRoot()
	app.vault, app.tabs = nil, nil
	app.detachAIVaultContext(contextRoot)
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
	if runtime.GOOS == "darwin" {
		mygo.App.OnWindowAllClosed(a.installDocumentlessMenu)
		mygo.App.OnActivate(func(hasVisibleWindows bool) {
			if !hasVisibleWindows {
				// Defer creation until AppKit's reopen handler has returned.
				mygo.App.WhenReady(a.reopenWorkspace)
			}
		})
	}
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) {
		a.quitSession = nil
		if current := a.activeWorkspace(); current != nil {
			session := current.sessionSnapshot()
			a.quitSession = &session
		}
	})
	mygo.App.OnWillQuit(func(_ *mygo.QuitEvent) {
		a.quitting = true
		if a.quitSession != nil {
			a.preferences.Session = *a.quitSession
			a.closeErr = errors.Join(a.closeErr, a.preferences.write())
		}
	})
}

func (a *workspaceApplication) reopenWorkspace() {
	if a.quitting || len(a.windows) != 0 || a.idlePicker || a.splash != nil {
		return
	}
	previous := a.preferences.Session
	// A warm reopen starts a blank document in the current Vault. Keep the
	// browsing context without reopening the closed document tabs.
	session := savedSession{
		Vault: previous.Vault,
		savedWindow: savedWindow{
			Recent:      previous.Recent,
			PreviewZoom: previous.PreviewZoom,
		},
	}
	app, opened := a.prepareWorkspace("", session)
	a.activate(app)
	if opened {
		app.persistSession()
	}
	app.win.Show()
}

func (a *workspaceApplication) installDocumentlessMenu() {
	language := a.preferences.InterfaceLanguage
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{{Role: mygo.RoleAbout}, {Role: mygo.RoleQuit}}},
		{Label: "File", Submenu: []*mygo.MenuItem{
			{Label: "New Document", Accelerator: "CmdOrCtrl+N", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.openDocumentlessWindow(func(app *workspaceWindow) {
					app.newDocument()
					app.initialEmptyTab = app.draft != nil && app.document.Path == ""
				})
			}},
			{Label: "Open...", Accelerator: "CmdOrCtrl+O", Click: func(*mygo.MenuItem, *mygo.Window) { a.openDocumentlessPicker() }},
		}},
		{Label: language.text("Help", "帮助"), Submenu: []*mygo.MenuItem{
			{Label: applicationName + " " + language.text("Help", "帮助"), Click: func(*mygo.MenuItem, *mygo.Window) {
				a.openFeatureGuide("", nil)
			}},
			{Label: applicationName + " " + language.text("User Guide", "用户指南"), Click: func(*mygo.MenuItem, *mygo.Window) {
				a.openDocumentlessWindow(func(app *workspaceWindow) {
					app.setUserGuide(app.workspaceTab)
					app.activateTab(app.workspaceTab)
				})
			}},
			markdownSyntaxGuideMenuItem(language),
			mygo.Separator(),
			{Label: language.text("Keyboard Shortcuts", "键盘快捷键"), Disabled: true},
			{Label: language.text("Show Welcome", "显示欢迎页"), Disabled: true},
			mygo.Separator(),
			a.replaySplashMenuItem(language),
		}},
	}))
}

func (a *workspaceApplication) openDocumentlessWindow(prepare func(*workspaceWindow)) {
	mygo.RunOnMain(func() {
		tab := &workspaceTab{mode: a.preferences.DefaultView}
		app := a.newWindow(tab, nil)
		if root := a.preferences.Session.Vault; root != "" {
			app.openVault(root)
		}
		prepare(app)
		app.win.Show()
		a.activate(app)
		app.persistSession()
	})
}

func (a *workspaceApplication) openDocumentlessPicker() {
	if app := a.activeWorkspace(); app != nil {
		app.filePicker()
		return
	}
	if a.idlePicker {
		return
	}
	a.idlePicker = true
	options := mygo.OpenDialogOptions{
		Title: documentPickerTitle, DefaultPath: a.preferences.Session.Vault, Filters: []mygo.FileFilter{documentPickerFilter},
	}
	go func() {
		files, err := mygo.Dialog.Open(options)
		mygo.RunOnMain(func() {
			a.idlePicker = false
			if err != nil {
				log.Printf("Open Document: %v", err)
				return
			}
			if len(files) == 0 {
				return
			}
			if app := a.activeWorkspace(); app != nil {
				app.openPickedDocument(files[0], false)
				return
			}
			a.openDocumentlessWindow(func(app *workspaceWindow) {
				app.openPickedFile(files[0])
				if app.draft == nil && app.drawing == nil {
					problem := app.problem
					app.setTabDocument(app.workspaceTab, workspace.Document{})
					app.initialEmptyTab = true
					app.activateTab(app.workspaceTab)
					app.problem = problem
				}
			})
		})
	}()
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
