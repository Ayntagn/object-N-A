package ui

import (
	"fmt"
	"path"
	"slices"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func (tab *workspaceTab) title() string {
	if tab.document.Path == "" {
		if tab.templateTitle != "" {
			return tab.templateTitle
		}
		return "Untitled"
	}
	return path.Base(tab.document.Path)
}

func (app *workspaceWindow) blankTab() *workspaceTab {
	return &workspaceTab{
		mode: app.mode,
		wys:  workspaceWYSIWYG{size: app.wys.size, pageWidth: app.wys.pageWidth},
	}
}

func (app *workspaceWindow) setTabDocument(tab *workspaceTab, doc workspace.Document) {
	if app.workspaceTab == tab {
		app.tableGrid = workspaceTableGrid{}
	}
	tab.cancelAPIRequests()
	tab.wys.reset()
	tab.drawing = nil
	tab.document = doc
	app.reconcileAIContextDocument(tab)
	tab.templateTitle = ""
	tab.initialEmptyTab = false
	tab.startTitleFromSource = false
	tab.inspection = false
	tab.userGuide = false
	tab.largePreview = workspaceLargePreview{}
	tab.draft = editor.NewBuffer(doc.Text)
	tab.code = app.newSourceEditor(tab)
	tab.sourceThemeFallback = nil
	tab.text, tab.textVersion = tab.draft.String(), tab.draft.Version()
	if workspace.IsDelimitedPreviewPath(doc.Path) {
		tab.largePreview.measure(doc.Text)
	}
	tab.metrics = workspace.MeasureText(tab.text)
	tab.maximizedPane = paneNotMaximized
	tab.outlineCache = outlineCache{}
	tab.previewAnchor = ""
	tab.previewRevealLine = 0
	tab.pendingConceptExcerpt = ""
	tab.previewThemeID = normalizedPreviewThemeID(app.preferences.PreviewThemeID)
	tab.quickPDFStylesheet = app.activeCustomStylesheet()
	tab.quickPDFNumberFigures, tab.quickPDFFigureCounter = app.preferences.NumberFigures, 0
	tab.quickPDFForm = exportPageForm{}
	if app.preferences.Export.RememberPDFSettings {
		tab.quickPDFForm = app.preferences.Export.rememberedPDFForm()
	}
	tab.documentType = workspaceDocumentType{}
}

func (app *workspaceWindow) activateTab(tab *workspaceTab) {
	app.dismissQuickSwitcher()
	if app.multiFile.active && !multiFileTabEligible(tab) {
		if index := slices.IndexFunc(app.tabs, multiFileTabEligible); index >= 0 {
			tab = app.tabs[index]
		}
	}
	if app.workspaceTab != tab || tab.draft == nil {
		app.closeAIContextReader()
	}
	if app.workspaceTab != tab {
		if app.multiFile.editing != nil {
			app.stopMultiFileRenderedEdit()
		} else {
			app.wys.flush(app)
		}
		app.wys.closeTableControls()
		app.wys.closeBlockControls()
		app.workspaceTab = tab
		if app.pdfPreview.form.paper != "" {
			app.pdfPreview.form = app.documentPDFForm()
		}
	}
	app.revealActiveTab = true
	app.focusSource = false
	app.focusVaultSearch = false
	app.findMode = findClosed
	app.problem = ""
	app.templates.pendingNotice = ""
	if tab.storage == nil && tab.document.Path != "" {
		app.recordRecent(tab.document.Path)
	}
	app.recordWelcomeDocument(tab)
	if tab.draft != nil {
		app.modeChanged()
	} else {
		app.focusMode = false
	}
	if win := app.win; win != nil {
		win.SetTitle(tab.title())
		win.SetRepresentedFile(app.absoluteDocument(tab))
		app.syncDocumentEdited()
	}
	app.updateMoveTabMenu()
	if item := app.templates.saveMenu; item != nil && item.IsEnabled() != (tab.draft != nil) {
		item.SetEnabled(tab.draft != nil)
	}
	app.invalidateGraphWindow()
}

func (app *workspaceWindow) updateMoveTabMenu() {
	if item := app.moveTabMenu; item != nil {
		item.SetEnabled(len(app.tabs) > 1)
	}
}

func (app *workspaceWindow) newTab() {
	if app.modalOpen {
		return
	}
	if app.multiFile.active {
		app.newDocument()
		return
	}
	tab := app.blankTab()
	app.setTabDocument(tab, workspace.Document{})
	app.tabs = append(app.tabs, tab)
	app.activateTab(tab)
}

func (app *workspaceWindow) openDocument(relativePath string) {
	app.openNote(relativePath, false)
}

func (app *workspaceWindow) openNote(relativePath string, newTab bool) {
	if app.multiFile.active {
		app.openNoteAt(relativePath, true, "")
		return
	}
	if isDrawingPath(relativePath) {
		app.openDrawing(relativePath, newTab)
		return
	}
	app.openNoteAt(relativePath, newTab, "")
}

func (app *workspaceWindow) openVaultRow(relativePath string, newTab bool) {
	app.openVaultRowFollowing(relativePath, newTab, nil)
}

func (app *workspaceWindow) openVaultRowFollowing(relativePath string, newTab bool, opened func()) {
	if app.modalOpen || app.vault == nil {
		return
	}
	if app.multiFile.active {
		app.openNoteWithFocus(relativePath, true, "", false, opened)
		return
	}
	if workspace.IsPDFPath(relativePath) {
		app.openVaultPDF(relativePath)
		return
	}
	if workspace.IsRasterImagePath(relativePath) {
		app.openVaultImage(relativePath)
		return
	}
	if isDrawingPath(relativePath) {
		app.openNote(relativePath, newTab)
		return
	}
	if workspace.IsMarkdownPath(relativePath) || workspace.IsSourceTextPath(relativePath) {
		keepSearchFocus := !newTab && vaultindex.TrimSidebarSearchQuery(app.fileSearch) != ""
		app.openNoteWithFocus(relativePath, newTab, "", keepSearchFocus, opened)
		return
	}
	app.openVaultFileInDefaultApp(relativePath)
}

func (app *workspaceWindow) openNoteAt(relativePath string, newTab bool, anchor string) {
	app.openNoteWithFocus(relativePath, newTab, anchor, false, nil)
}

func (app *workspaceWindow) openNoteWithFocus(relativePath string, newTab bool, anchor string, keepSearchFocus bool, opened func()) {
	if app.modalOpen || app.vault == nil {
		return
	}
	if app.multiFile.active {
		if !workspace.IsMarkdownPath(relativePath) {
			app.problem = multiFileMarkdownOnly
			return
		}
		newTab = app.draft != nil || app.drawing != nil
	}
	reveal := func() {
		if keepSearchFocus && app.vaultVisible && !app.showingProperties() && vaultindex.TrimSidebarSearchQuery(app.fileSearch) != "" {
			app.focusSource = false
			app.wys.focus = false
			app.focusVaultSearch = true
		}
		app.followPreviewAnchor(anchor)
		if opened != nil {
			opened()
		}
	}
	if tab := app.findVaultTab(relativePath); tab != nil {
		if tab != app.workspaceTab {
			app.activateTab(tab)
		}
		reveal()
		return
	}
	load := func() {
		doc, err := app.vault.ReadTextDocument(relativePath)
		if err != nil {
			app.problem = fmt.Sprintf("Could not open %q. %v", relativePath, err)
			return
		}
		tab := app.workspaceTab
		if newTab {
			tab = app.blankTab()
			app.tabs = append(app.tabs, tab)
		}
		app.releaseTabStorage(tab)
		app.setTabDocument(tab, doc)
		if workspace.IsSourceTextPath(relativePath) {
			tab.mode = splitMode
		}
		app.activateTab(tab)
		reveal()
	}
	if newTab {
		load()
		return
	}
	app.beforeLeavingTabs([]*workspaceTab{app.workspaceTab}, load)
}

func (app *workspaceWindow) closeTab(tab *workspaceTab) {
	if app.modalOpen {
		return
	}
	if !slices.Contains(app.tabs, tab) {
		return
	}
	if len(app.tabs) == 1 {
		app.win.Close()
		return
	}
	app.beforeLeavingTabs([]*workspaceTab{tab}, func() {
		app.removeTab(tab)
	})
}

func (app *workspaceWindow) removeTab(tab *workspaceTab) {
	index := slices.Index(app.tabs, tab)
	if index < 0 {
		return
	}
	app.tabs = slices.Delete(app.tabs, index, index+1)
	tab.cancelAPIRequests()
	app.releaseTabStorage(tab)
	if len(app.tabs) == 0 {
		app.tabs = []*workspaceTab{app.blankTab()}
	}
	if tab == app.workspaceTab {
		app.activateTab(app.tabs[min(index, len(app.tabs)-1)])
	} else {
		app.updateMoveTabMenu()
	}
}

func (app *workspaceWindow) moveTab(direction int) {
	if app.modalOpen || len(app.tabs) < 2 {
		return
	}
	index := slices.Index(app.tabs, app.workspaceTab)
	for range app.tabs {
		index = (index + direction + len(app.tabs)) % len(app.tabs)
		if !app.multiFile.active || multiFileTabEligible(app.tabs[index]) {
			app.activateTab(app.tabs[index])
			return
		}
	}
}

const (
	tabBarHeight = 28
	tabBarGap    = 8
)

func (app *workspaceWindow) tabBar(c *mygoui.Context) {
	if len(app.tabs) < 2 || app.focusMode {
		return
	}
	current := slices.Index(app.tabs, app.workspaceTab)
	selected := current
	theme := c.Theme()
	track, active, add, edge, inactiveText := theme.Surface, theme.SurfacePressed, theme.SurfaceHover, theme.Border, theme.TextMuted
	if app.blackout && !c.Preferences().HighContrast {
		// The dark archive supplies the tab colors. The regular, legible
		// navigation surface also works when native transparency is unavailable.
		track = mygoui.RGB(59, 59, 62)
		active = mygoui.RGB(48, 48, 51)
		add = mygoui.RGB(33, 33, 33)
		edge = mygoui.RGB(58, 58, 62)
		inactiveText = mygoui.RGB(174, 174, 178)
	}
	windowWidth, _ := c.Size()
	mygoui.Column(c.Key("tab strip")).Height(tabBarHeight + tabBarGap).FillWidth().Shrink(0).Children(func() {
		tabs := mygoui.TabsBase(c, &selected, len(app.tabs))
		tabs.List.Label("tab bar").Height(tabBarHeight).FillWidth().Gap(3).
			Padding(0, 7, 0, 8).AlignItems(mygoui.Center).Children(func() {
			mygoui.ScrollHorizontal(c).Role(mygoui.RoleNone).ScrollbarInsets(0).Scrollbars(false).
				Height(tabBarHeight).Grow(1).MinWidth(0).Padding(2).Gap(1).
				AlignItems(mygoui.Center).Background(track).Radius(14).Clip().Children(func() {
				for index, documentTab := range app.tabs {
					width := tabWidth(windowWidth, len(app.tabs), index)
					slot := mygoui.Row(c.Key(documentTab)).Size(width, 24).Shrink(0)
					if index == current && app.revealActiveTab {
						slot.ScrollIntoView()
					}
					slot.Children(func() {
						item := tabs.Tab(index).Height(24).FillWidth().Label(documentTab.title()).
							Disabled(app.modalOpen || app.multiFile.active && !multiFileTabEligible(documentTab)).
							AlignItems(mygoui.Center).Justify(mygoui.Center).
							Padding(0, 24).Radius(12).ClipX()
						ink := inactiveText
						if index == current {
							ink = theme.Text
							item.Background(active).Border(1, edge)
						} else if item.Hovered() {
							item.Background(active)
						}
						item.Children(func() {
							mygoui.Text(c, documentTab.title()).FontSize(11).TextColor(ink).
								SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
						})
						close := chromeControl(c, "Close tab", app.modalOpen).
							Tooltip("Close "+documentTab.title()).Absolute().Left(4).Top(3).
							Size(18, 18).KeepFocus().OnClick(func() { app.closeTab(documentTab) })
						close.Children(func() {
							switch {
							case item.Hovered() || close.Hovered() || close.FocusVisible() || c.Preferences().HighContrast:
								chromeIcon(c, "xmark", 11)
							case documentTab.dirty():
								mygoui.Box(c).Size(6, 6).Radius(3).Background(ink)
							}
						})
					})
				}
			})
			chromeButtonIcon(c, "plus", "new tab", paneAction(!app.modalOpen, app.newTab), 17, theme.Text).
				Size(30, 30).Radius(15).Background(add).Tooltip("New Tab")
		}).OnChange(func() {
			if !app.modalOpen && selected != current && selected >= 0 && selected < len(app.tabs) {
				app.activateTab(app.tabs[selected])
			}
		})
	})
	app.revealActiveTab = false
}

func tabWidth(windowWidth float32, count, index int) float32 {
	// EV-UI-037: the second tab starts at the midpoint, with space for the
	// add button deducted at the end. Extra tabs share the available rail.
	if count == 2 {
		if index == 0 {
			return windowWidth/2 - 11
		}
		return windowWidth/2 - 42
	}
	return max(104, (windowWidth-52-float32(count-1))/float32(count))
}
