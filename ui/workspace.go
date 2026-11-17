package ui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"cyberwriter/internal/core/aiworkspace"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/editor"
)

const applicationName = "object-N/A"

const (
	sourceMode = iota
	splitMode
	previewMode
	wysiwygMode
)

type workspaceWindow struct {
	application         *workspaceApplication
	win                 *mygo.Window
	menu                *mygo.Menu
	moveTabMenu         *mygo.MenuItem
	settings            *mygo.Window
	settingsTab         string
	shortcutsSheet      *mygo.Window
	preferences         *workspacePreferences
	blackout            bool
	vault               *workspace.Vault
	root                string
	entries             []workspace.Entry
	vaultTreeEntries    []workspace.Entry // Display order stays separate from the indexed entries.
	vaultFileCount      int
	folders             map[string]*bool
	vaultVisible        bool
	sidebarTab          workspaceSidebarTab
	ai                  workspaceAI
	aiVaultContext      *workspaceAIVaultContext
	aiVaultBuildSheet   *workspaceAIVaultBuildSheet
	aiVaultMapBusy      bool
	aiVaultMapProblem   string
	aiConnectionForm    workspaceAIConnectionForm
	aiSettingsPrepared  bool
	aiServiceRequest    *aiSettingsRequest
	aiServiceResults    []aiworkspace.LocalServiceResult
	aiModelRequest      *aiSettingsRequest
	aiTestRequest       *aiSettingsRequest
	fileSearch          string
	focusVaultSearch    bool
	sidebarSearch       vaultSidebarSearch
	recentPaths         []string
	recentSlots         []*mygo.MenuItem
	sourceHighlightMenu *mygo.MenuItem
	recentsOpen         bool
	index               workspaceIndex
	graph               workspaceGraph
	mentions            workspaceMentions
	glow                workspaceGlow
	vaultMapBuilding    bool
	vaultHealthBuilding bool
	vaultStatsBuilding  bool
	vaultStatsMenuItem  *mygo.MenuItem
	vaultGuideOpen      bool
	vaultSetup          *vaultSetupState

	vaultEmbeddingOfferPath string
	deferredVaultOffers     map[string]bool

	*workspaceTab
	tabs            []*workspaceTab
	revealActiveTab bool

	documentEdited          bool
	focusSource             bool
	focusMode               bool
	outlineVisible          bool
	findMode                findMode
	split                   workspaceSplitState
	multiFile               workspaceMultiFile
	previewZoom             int
	previewPane             workspacePreview
	noteDropDownload        func(context.Context, string) ([]byte, error)
	pdfPreview              workspacePDFPreview
	snippets                workspaceSnippets
	markdownShortcutSession markdownShortcutSession
	sourceFence             sourceFenceCompletion
	templates               workspaceTemplates
	commandPalette          workspaceCommandPalette
	quickSwitcher           workspaceQuickSwitcher
	quickActions            workspaceQuickActions
	quickActionRoute        quickActionDestination
	presentation            *workspacePresentation
	apiHistory              *workspaceAPIHistory

	modalOpen      bool
	tableEditor    *workspaceTableEditor
	tableGrid      workspaceTableGrid
	closeConfirmed bool
	problem        string
}

type workspaceTab struct {
	document              workspace.Document
	templateTitle         string
	initialEmptyTab       bool
	startTitleFromSource  bool
	inspection            bool
	userGuide             bool
	largePreview          workspaceLargePreview
	storage               *workspace.Vault // An individually opened note outside the Vault.
	folderRoot            string
	draft                 *editor.Buffer
	code                  *editor.State
	text                  string
	textVersion           uint64
	metrics               workspace.TextMetrics
	sourceThemeFallback   *editor.Theme
	sourceHex             sourceHexColors
	sourceSpell           *sourceSpellCheck
	mode                  int
	autoSave              workspaceAutoSave
	maximizedPane         paneMaximization
	outlineCache          outlineCache
	wys                   workspaceWYSIWYG
	previewAnchor         string
	previewRevealLine     int
	pendingConceptExcerpt string
	previewThemeID        string
	quickPDFStylesheet    string
	quickPDFForm          exportPageForm
	quickPDFNumberFigures bool
	quickPDFFigureCounter int
	documentType          workspaceDocumentType
	drawing               *workspaceDrawing
	drawingVault          bool
	properties            workspaceProperties
	aiContext             *workspaceAIContext
	apiRequests           map[*apiRequestTarget]context.CancelFunc
}

// Run opens the workspace window and runs the MyGo event loop.
func Run(initialVault string) error {
	preferences, preferencesErr := loadWorkspacePreferences()
	application := &workspaceApplication{preferences: preferences}
	mygo.App.SetName(applicationName)
	mygo.App.SetWindowStateStore(application)
	mygo.App.WhenReady(func() {
		mygo.Theme.SetSource(preferences.Appearance.themeSource())
		application.installSessionLifecycle()
		app, opened := application.prepareWorkspace(initialVault, preferences.Session)
		if preferencesErr != nil && app.problem == "" {
			app.problem = fmt.Sprintf("Could not load workspace settings. %v", preferencesErr)
		}
		application.activate(app)
		if opened {
			app.persistSession()
		}
		welcome := func() {
			if application.preferences.ShowWelcomeOnLaunch && !app.win.IsDestroyed() {
				app.showWelcome()
			}
		}
		if preferences.ShowSplashOnLaunch {
			application.playSplash(app.win, true, welcome)
		} else {
			app.win.Show()
			welcome()
		}
	})
	err := mygo.App.Run()
	for _, app := range slices.Clone(application.windows) {
		application.releaseWindow(app)
	}
	return errors.Join(err, application.closeErr)
}

func (app *workspaceWindow) openVault(root string) bool {
	return app.loadVault(root, false)
}

func (app *workspaceWindow) loadVault(root string, restoredExample bool) bool {
	abs, err := filepath.Abs(root)
	if err != nil {
		app.problem = fmt.Sprintf("Could not resolve the Vault directory %q. %v", root, err)
		return false
	}
	if !restoredExample && app.vault != nil && app.root == abs {
		return true
	}
	var vault *workspace.Vault
	if restoredExample {
		vault, err = workspace.Open(abs)
	} else {
		vault, err = app.vaultForFolder(abs)
	}
	if err != nil {
		app.problem = fmt.Sprintf("Could not open Vault %q. %v", abs, err)
		return false
	}
	entries, count, err := readWorkspaceEntries(vault, app.preferences.VaultShowAllFiles)
	if err != nil {
		app.problem = fmt.Sprintf("Could not list Vault %q. %v", abs, errors.Join(err, app.closeUnusedVault(vault)))
		return false
	}
	previous := make(map[*workspace.Vault]bool)
	for _, window := range app.documentWindows() {
		previous[window.vault] = true
		window.applyVault(abs, vault, entries, count, restoredExample)
	}
	for old := range previous {
		if err := app.closeUnusedVault(old); err != nil {
			app.problem = fmt.Sprintf("The new Vault is open. Could not close the previous Vault. %v", err)
		}
	}
	app.rememberVault(abs)
	app.invalidateGlow()
	return true
}

func (app *workspaceWindow) applyVault(abs string, vault *workspace.Vault, entries []workspace.Entry, count int, restoredExample bool) {
	app.wys.sync(app)
	app.dismissQuickSwitcher()
	previous := app.vault
	previousContextRoot := app.aiVaultContextRoot()
	if restoredExample {
		app.rebindExampleTabs(abs, vault)
	}
	for _, tab := range app.tabs {
		tab.wys.queryResults = nil
		switch {
		case tab.storage == vault:
			tab.storage, tab.folderRoot = nil, ""
		case previous != nil && tab.storage == nil && tab.document.Path != "":
			tab.storage, tab.folderRoot = previous, app.root
		}
	}
	app.vault, app.root = vault, abs
	app.vaultEmbeddingOfferPath = normalizedVaultEmbeddingOfferPath(abs)
	if previousContextRoot != app.aiVaultContextRoot() {
		app.ai.vaultContextOverride = nil
		app.detachAIVaultContext(previousContextRoot)
	} else if previous != vault {
		state := app.aiVaultContextState()
		if state.request != nil && (state.request.kind == aiVaultBuild || state.request.kind == aiVaultLoad) {
			state.cancel()
		}
		if state.index == nil && state.request == nil {
			state.loaded = false
		}
	}
	app.vaultFileCount = count
	app.clearUnlinkedMentions()
	app.clearVaultSidebarSearch()
	app.recentPaths = nil
	app.recentsOpen = false
	if app.index.stopWatch != nil {
		app.index.stopWatch()
	}
	app.index = workspaceIndex{attachment: app.index.attachment + 1}
	app.vaultMapBuilding = false
	app.vaultHealthBuilding = false
	app.vaultStatsBuilding = false
	app.vaultGuideOpen = false
	app.closeGraphWindow()
	app.graph = workspaceGraph{}
	app.updateEntries(entries)
	app.folders = make(map[string]*bool)
	app.vaultVisible = true
	app.sidebarTab = workspaceFilesTab
	app.fileSearch = ""
	app.problem = ""
	if restoredExample {
		app.activateTab(app.workspaceTab)
	} else if app.storage == nil && app.document.Path != "" {
		app.recordRecent(app.document.Path)
	}
	app.requestIndex()
	app.startVaultWatch()
	if app.win != nil && !app.win.IsDestroyed() {
		app.win.Invalidate()
	}
}

func (app *workspaceWindow) refreshVault() {
	if app.vault == nil {
		return
	}
	entries, count, err := readWorkspaceEntries(app.vault, app.preferences.VaultShowAllFiles)
	if err != nil {
		app.problem = fmt.Sprintf("Could not refresh Vault %q. %v", app.root, err)
		return
	}
	app.vaultFileCount = count
	app.updateEntries(entries)
	app.requestIndex()
	app.problem = ""
}

func (app *workspaceWindow) workingText() string {
	if app.draft != nil {
		snapshot := app.draft.Snapshot()
		if version := snapshot.Version(); version != app.textVersion {
			app.text, app.textVersion = snapshot.String(), version
			app.metrics = workspace.MeasureText(app.text)
			if app.inspection || workspace.IsDelimitedPreviewPath(app.document.Path) {
				app.largePreview.measure(app.text)
			}
			if (app.inspection || app.startTitleFromSource) && app.document.Path == "" {
				title := workspace.FileInspectionTitle(app.text)
				if app.templateTitle != title {
					app.templateTitle = title
					if app.win != nil {
						app.win.SetTitle(title)
					}
				}
			}
		}
	}
	return app.text
}

func (tab *workspaceTab) dirty() bool {
	if tab.drawing != nil {
		return tab.drawing.dirty()
	}
	return tab.draft != nil && (tab.draft.Modified() || tab.document.NeedsRecreation() ||
		tab.inspection && tab.document.Path == "")
}

func (app *workspaceWindow) syncDocumentEdited() {
	app.wys.sync(app)
	edited := app.dirty()
	if edited != app.documentEdited {
		if app.win != nil {
			app.win.SetDocumentEdited(edited)
		}
		app.documentEdited = edited
	}
}

func (app *workspaceWindow) save() bool {
	if !app.permitsActiveDocument() {
		return false
	}
	app.wys.flush(app)
	if app.draft != nil && app.document.Path == "" {
		app.saveAs(app.workspaceTab, "", nil)
		return false
	}
	tab := app.workspaceTab
	return app.saveTab(tab, func() { app.offerMissingDocumentSave(tab, nil) })
}

func (app *workspaceWindow) saveTab(tab *workspaceTab, onMissing func()) bool {
	if !tab.dirty() {
		return true
	}
	if tab.drawing != nil {
		if err := tab.drawing.save(); err != nil {
			app.problem = fmt.Sprintf("Could not save %q. %v", tab.document.Path, err)
			return false
		}
		app.syncDocumentEdited()
		app.problem = ""
		return true
	}
	var output strings.Builder
	if tab.inspection {
		output.WriteString(tab.inspectionTextForSave())
	} else if _, err := tab.draft.WriteTo(&output); err != nil {
		app.problem = fmt.Sprintf("Could not read the edited text. %v", err)
		return false
	}
	vault := tab.storage
	if vault == nil {
		vault = app.vault
	}
	previous := tab.document
	doc, err := vault.SaveDocument(previous, output.String())
	if err != nil {
		if onMissing != nil && errors.Is(err, fs.ErrNotExist) {
			onMissing()
			return false
		}
		app.problem = fmt.Sprintf("Could not save %q. %v", tab.document.Path, err)
		return false
	}
	tab.document = doc
	tab.draft.MarkSaved()
	app.syncDocumentEdited()
	app.problem = ""
	if previous.NeedsRecreation() {
		if tab.storage == nil {
			app.refreshVault()
		}
		return true
	}
	if tab.storage == nil {
		if workspace.IsMarkdownPath(doc.Path) {
			app.requestIndex()
		} else {
			app.refreshVault()
		}
	}
	if workspace.CanBrowseVaultVersions(doc.Path) {
		app.recordVersion(tab, previous)
	}
	return true
}

func (app *workspaceWindow) beforeLeavingTabs(tabs []*workspaceTab, next func()) {
	if app.modalOpen {
		return
	}
	app.wys.flush(app)
	first := slices.IndexFunc(tabs, (*workspaceTab).dirty)
	if first < 0 {
		next()
		return
	}
	tab := tabs[first]
	const (
		saveChanges = iota
		discardChanges
		cancel
	)
	app.modalOpen = true
	options := mygo.MessageOptions{
		Parent:             app.win,
		Type:               mygo.MessageWarning,
		Title:              "Unsaved changes",
		Message:            fmt.Sprintf("Save changes to %q?", tab.title()),
		Buttons:            []string{"Save", "Discard Changes", "Cancel"},
		DefaultButton:      saveChanges,
		CancelButton:       cancel,
		DestructiveButtons: []int{discardChanges},
	}
	go func() {
		choice, err := mygo.Dialog.Message(options)
		app.win.Update(func() {
			app.modalOpen = false
			if err != nil {
				app.problem = fmt.Sprintf("Could not show the save prompt. %v", err)
				return
			}
			switch choice.Button {
			case saveChanges:
				continueLeaving := func() { app.beforeLeavingTabs(tabs[first+1:], next) }
				if tab.document.Path == "" {
					app.saveAs(tab, "", continueLeaving)
					return
				}
				if app.saveTab(tab, func() { app.offerMissingDocumentSave(tab, continueLeaving) }) {
					continueLeaving()
				}
			case discardChanges:
				app.beforeLeavingTabs(tabs[first+1:], next)
			}
		})
	}()
}

func (app *workspaceWindow) closeRequested(event *mygo.CloseEvent) {
	app.wys.flush(app)
	if app.closeConfirmed || (!app.modalOpen && !slices.ContainsFunc(app.tabs, (*workspaceTab).dirty)) {
		app.persistSession()
		return
	}
	event.PreventDefault()
	app.beforeLeavingTabs(app.tabs, func() {
		app.closeConfirmed = true
		app.win.Close()
		app.closeConfirmed = false
	})
}
