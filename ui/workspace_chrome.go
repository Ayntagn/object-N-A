package ui

import (
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func (app *workspaceWindow) view(c *mygoui.Context) {
	app.applyAppearance(c)
	app.syncDocumentEdited()
	app.observeAutoSave(c)
	if p := app.presentation; p != nil {
		if p.tab == app.workspaceTab && p.origin.note == app.document.Path {
			p.draw(c, app)
			return
		}
		app.closePresentation()
	}
	if app.draft != nil && !app.modalOpen && app.permitsActiveDocument() {
		app.observeDocumentType(c)
		if !app.commandPalette.open && !app.quickSwitcher.open && !app.quickActions.open {
			app.registerEditorShortcuts(c)
		}
	}
	if !app.modalOpen && !app.quickActions.open {
		c.OnWindowShortcut(mygoui.Cmd, mygoui.KeySlash, app.toggleCommandPalette)
		if app.drawing == nil {
			c.OnWindowShortcut(mygoui.Cmd, mygoui.KeyP, app.toggleQuickSwitcher)
		}
		if !app.commandPalette.open && !app.quickSwitcher.open {
			app.registerMultiFileShortcuts(c)
			c.OnWindowShortcut(mygoui.Cmd|mygoui.Shift, mygoui.KeySlash, app.showShortcuts)
			c.OnWindowShortcut(mygoui.Cmd|mygoui.Alt, mygoui.KeyS, app.toggleSnippets)
			if app.canOpenQuickActions() {
				c.OnWindowShortcut(mygoui.Cmd, mygoui.KeyJ, app.openQuickActions)
			}
			if app.vault != nil {
				c.OnWindowShortcut(mygoui.Ctrl, mygoui.KeyBackquote, app.toggleGlow)
			}
		}
		app.drawSnippets(c)
	}
	mygoui.Column(c).Fill().AlignItems(mygoui.Stretch).Children(func() {
		app.toolbar(c)
		app.tabBar(c)
		if app.problem != "" {
			mygoui.Text(c, app.problem).Role(mygoui.RoleAlert).Padding(6, 12).
				TextColor(c.Theme().Danger)
		}
		if app.vault == nil && app.draft == nil && app.drawing == nil {
			mygoui.Column(c).Grow(1).Gap(12).Padding(18, 16).Background(c.Theme().Background).Children(func() {
				mygoui.Text(c, "No Vault Open").Role(mygoui.RoleHeading).Bold()
				mygoui.Button(c, "Open Vault").OnClick(app.chooseVault).Disabled(app.modalOpen)
				mygoui.Button(c, "Create Vault").OnClick(app.createVault).Disabled(app.modalOpen)
				mygoui.Button(c, "New Document").OnClick(app.newDocument).Disabled(app.modalOpen)
			})
			return
		}
		body := mygoui.Row(c).Grow(1).MinHeight(0).AlignItems(mygoui.Stretch)
		if app.focusMode {
			body.Padding(0, 0, 2)
		}
		body.Children(func() {
			vault, outline := app.visibleSidebars()
			aiOnRight := vault && app.showingAI() && app.preferences.AIPanelRight
			if vault && !aiOnRight {
				app.sidebar(c)
			}
			app.workspaceDocumentArea(c, outline)
			if aiOnRight {
				app.sidebar(c)
			}
		})
		if app.draft != nil && !app.focusMode && app.permitsActiveDocument() {
			app.statusBar(c)
		}
	})
	app.glowShadeView(c)
	app.drawVaultSetup(c)
	app.drawTemplateNotice(c)
	app.drawAIVaultBuildSheet(c)
	if !app.modalOpen {
		app.drawCommandPalette(c)
		app.drawQuickSwitcher(c)
		app.drawQuickActions(c)
	}
	width, _ := c.Size()
	app.previewPane.syncSource(app, width)
}

func (app *workspaceWindow) workspaceDocumentArea(c *mygoui.Context, outline bool) {
	const dockInset = 4
	docked := app.glow.shown && app.glow.host == glowDock && !app.focusMode && app.drawing == nil
	mygoui.Column(c.Key("workspace document area")).Grow(1).MinWidth(0).MinHeight(0).Children(func() {
		panes := mygoui.Row(c.Key("workspace panes")).Grow(1).MinWidth(0).MinHeight(0).AlignItems(mygoui.Stretch)
		if docked {
			panes.MinHeight(44)
		}
		if docked && app.glow.maximized {
			panes.Grow(0).Basis(44).Shrink(0).Clip()
		}
		panes.Children(func() {
			if app.multiFile.active {
				app.drawMultiFileWorkspace(c)
				return
			}
			if app.drawing != nil {
				app.drawingPane(c)
				return
			}
			if app.draft == nil {
				mygoui.Column(c).Grow(1).Background(c.Theme().Background).Children(func() {
					mygoui.Text(c, "Choose a Markdown file from the Vault.").Padding(18, 16)
				})
				return
			}
			if outline {
				app.outline(c)
			}
			app.documentPanes(c)
		})
		if docked {
			panel := mygoui.Column(c.Key("GLOW dock")).FillWidth().MinHeight(0).Padding(dockInset, 4, 0, 6)
			if app.glow.maximized {
				panel.Grow(1)
			} else {
				panel.Height(app.preferences.Terminal.Height + dockInset).Shrink(0)
			}
			panel.Children(func() { app.glowPane(c, glowDock) })
		}
	})
}

func (app *workspaceWindow) vaultPanelVisible() bool {
	if app.drawing != nil {
		return app.vault != nil && app.drawingVault
	}
	return app.vaultVisible && (app.vault != nil || app.showingAI())
}

func (app *workspaceWindow) setVaultPanelVisible(visible bool) {
	if !visible {
		app.vaultGuideOpen = false
		app.graph.sidebar.cancelTour()
		app.leaveVaultFiles()
	}
	if app.drawing != nil {
		app.drawingVault = visible
	} else {
		app.vaultVisible = visible
	}
}

type workspaceSidebarTab uint8

const (
	workspaceFilesTab workspaceSidebarTab = iota
	workspaceAITab
	workspacePropertiesTab
)

func (app *workspaceWindow) showingAI() bool {
	return app.sidebarTab == workspaceAITab && app.drawing == nil
}

func (app *workspaceWindow) openFilesSidebar() {
	if app.vault == nil {
		app.chooseVault()
		return
	}
	app.sidebarTab = workspaceFilesTab
	app.setVaultPanelVisible(true)
}

func (app *workspaceWindow) openAIWorkspace() {
	if app.modalOpen || app.drawing != nil || (app.vault == nil && app.draft == nil) {
		return
	}
	app.vaultGuideOpen = false
	app.graph.sidebar.cancelTour()
	app.leaveVaultFiles()
	app.sidebarTab = workspaceAITab
	app.setVaultPanelVisible(true)
}

// EV-UI-225: reactivating the AI title control closes the entire sidebar.
func (app *workspaceWindow) toggleAIWorkspace() {
	if app.showingAI() && app.vaultPanelVisible() {
		app.setVaultPanelVisible(false)
		return
	}
	app.openAIWorkspace()
}

func (app *workspaceWindow) documentPanes(c *mygoui.Context) {
	mode := app.displayedMode()
	if mode != splitMode {
		app.split.drag = workspaceSplitDrag{}
	}
	if mode == wysiwygMode {
		mygoui.Column(c.Key("wysiwyg-pane")).Grow(1).MinWidth(0).MinHeight(0).AlignItems(mygoui.Stretch).
			Padding(0, 4, 0, 6).Background(c.Theme().Background).
			Children(func() {
				mygoui.Column(c).Grow(1).MinHeight(0).DrawOver(app.drawDocumentBorder(c)).
					Children(func() { app.wys.draw(c, app) })
			})
		return
	}
	width, _ := c.Size()
	vault, outline := app.visibleSidebars()
	panes := app.documentPaneWidths(width, vault, outline, mode == splitMode)
	document := mygoui.Row(c.Key("document")).Grow(1).MinWidth(0).MinHeight(0).AlignItems(mygoui.Stretch).
		Padding(0, panes.trailing, 0, panes.leading).Gap(panes.gap).Background(c.Theme().Background)
	document.Children(func() {
		if mode != previewMode {
			source := mygoui.Column(c.Key("source-pane")).MinHeight(0).MinWidth(0).AlignItems(mygoui.Stretch)
			if mode == splitMode {
				source.Width(panes.source).Shrink(0)
			} else {
				source.Grow(1)
			}
			source.DrawOver(app.drawDocumentBorder(c)).Children(func() { app.source(c) })
		}
		if mode != sourceMode {
			mygoui.Column(c.Key("preview-pane")).Grow(1).MinWidth(0).MinHeight(0).
				AlignItems(mygoui.Stretch).DrawOver(app.drawDocumentBorder(c)).Children(func() { app.preview(c) })
		}
		if mode == splitMode {
			app.drawSplitDivider(c, document, panes.divider, width)
		}
	})
}

func (app *workspaceWindow) sidebar(c *mygoui.Context) {
	label, page := "Vault sidebar", workspaceFilesTab
	if app.showingAI() {
		label = "AI sidebar"
		page = workspaceAITab
	} else if app.showingProperties() {
		page = workspacePropertiesTab
	}
	sidebar := mygoui.Column(c.Key("vault")).Width(vaultPanelWidth).Shrink(0).MinHeight(0).Label(label).Clip().
		Background(workspaceChromeSurface(c)).BorderWidth(0, 1, 0, 0).BorderColor(c.Theme().Border)
	if app.showingAI() && app.preferences.AIPanelRight {
		sidebar.BorderWidth(0, 0, 0, 1)
	}
	if app.blackout {
		fill := blackoutSidebarFill
		if app.showingProperties() {
			fill = c.Theme().Background
		}
		sidebar.Background(c.Theme().Background).BorderColor(mygoui.Transparent).Draw(func(p *mygoui.Painter, r mygoui.Rect) {
			frame := mygoui.Rect{X: r.X + 4, Y: r.Y + 4, W: r.W - 8, H: r.H - 4}
			p.Fill(frame, fill, 8)
			p.Stroke(frame, blackoutSidebarEdge, 8, 1)
		})
	} else if _, ok := appearancePalettes[app.appAppearance()]; ok {
		sidebar.Background(c.Theme().Surface)
	}
	revealWorkspacePage(c, sidebar, sidebar, page, c.Theme().Surface)
	sidebar.Children(func() {
		if app.showingAI() {
			composer := app.aiComposerOptions()
			aiReferenceDrop(sidebar, composer.dropReferences)
			app.ai.draw(c, app.blackout, func() { app.vaultTabs(c, false) }, app.openAISettings, func() {
				app.setVaultPanelVisible(false)
			}, composer)
			return
		}
		app.vaultToolbar(c)
		if app.showingProperties() {
			app.propertiesPanel(c)
			return
		}
		mygoui.Column(c).Padding(0, 16, 8).Gap(8).Children(func() {
			mygoui.Row(c).Gap(8).AlignItems(mygoui.Center).Children(func() {
				chromeIcon(c, "building.columns", 16)
				mygoui.Text(c, filepath.Base(app.root)).Role(mygoui.RoleHeading).Level(2).Bold().
					FontSize(15).SingleLine().Ellipsis("…")
			})
			app.vaultSearchField(c)
			app.vaultTagFilter(c)
		})
		tagged := app.index.data.GraphTagFiles(app.graph.tag)
		filtering := app.fileSearch != "" || app.graph.tag != ""
		app.drawVaultEmbeddingOffer(c)
		showRecents := !filtering && len(app.recentPaths) > 0
		if showRecents {
			app.recentHeader(c)
		}
		mygoui.Scroll(c).Grow(1).MinHeight(0).Margin(0, 3, 0, 4).Padding(4, 0, 0).Label("Vault file list").Children(func() {
			mygoui.Column(c).FillWidth().Shrink(0).Children(func() {
				if showRecents && app.recentsOpen {
					app.recentRows(c)
				}
				if filtering {
					app.searchVault(c, tagged)
				} else {
					app.drawVaultFileTree(c)
				}
			})
		})
		app.vaultTags(c)
		app.graphSection(c)
		app.drawBacklinks(c)
		app.unlinkedMentions(c)
		app.vaultFileCountFooter(c)
	})
}

func (app *workspaceWindow) updateEntries(entries []workspace.Entry) {
	app.entries = entries
	app.vaultTreeEntries = orderedVaultTreeEntries(entries)
	if app.quickSwitcher.open {
		app.refreshQuickSwitcherFiles()
	}
	present := make(map[string]bool, len(app.recentPaths))
	var scan func([]workspace.Entry)
	scan = func(entries []workspace.Entry) {
		for _, entry := range entries {
			if entry.IsDir {
				scan(entry.Children)
				continue
			}
			if slices.Contains(app.recentPaths, entry.Path) {
				present[entry.Path] = true
			}
		}
	}
	scan(entries)
	app.recentPaths = slices.DeleteFunc(app.recentPaths, func(relative string) bool { return !present[relative] })
	app.updateRecentMenu()
	app.updateVaultStatsMenu()
}

func (app *workspaceWindow) vaultFileCountFooter(c *mygoui.Context) {
	colors := app.vaultSectionColors(c)
	mygoui.Row(c.Key("Vault file count")).Margin(0, 3, 0, 4).Height(22).Shrink(0).
		Padding(4, 12).AlignItems(mygoui.Center).Background(colors.headerFill).Children(func() {
		mygoui.Text(c, strconv.Itoa(app.vaultFileCount)+" files").Font("monospace").FontSize(11).
			TextColor(colors.secondary.Alpha(.5)).FixedLineHeight(14).SingleLine()
	})
}

func (app *workspaceWindow) recordRecent(relative string) {
	app.recentPaths = slices.DeleteFunc(app.recentPaths, func(existing string) bool { return existing == relative })
	app.recentPaths = slices.Insert(app.recentPaths, 0, relative)
	if len(app.recentPaths) > maxRecent {
		app.recentPaths = slices.Delete(app.recentPaths, maxRecent, len(app.recentPaths))
	}
	app.updateRecentMenu()
}

func (app *workspaceWindow) recentHeader(c *mygoui.Context) {
	count := strconv.Itoa(len(app.recentPaths))
	disclosure := "▸"
	if app.recentsOpen {
		disclosure = "▾"
	}
	button := mygoui.ButtonBase(c.Key("recents")).Height(27).Margin(0, 3, 0, 4).Padding(0, 12).
		Gap(6).Justify(mygoui.Start).Label("Recents, " + count).Expanded(app.recentsOpen).
		Disabled(app.modalOpen).OnClick(func() { app.recentsOpen = !app.recentsOpen })
	if app.blackout {
		button.Background(mygoui.RGB(17, 17, 19))
	}
	if button.Hovered() {
		button.Background(c.Theme().SurfaceHover).Radius(5)
	}
	button.Children(func() {
		mygoui.Text(c, disclosure).FontSize(12).TextColor(c.Theme().TextMuted)
		if app.blackout {
			chromeIconColor(c, "clock", 13, blackoutTitleInk)
		}
		mygoui.Text(c, "Recents").FontSize(12).Bold()
		mygoui.Spacer(c)
		mygoui.Text(c, count).FontSize(11).TextColor(c.Theme().TextMuted)
	})
}

func (app *workspaceWindow) recentRows(c *mygoui.Context) {
	for _, relative := range app.recentPaths {
		parent := path.Base(path.Dir(relative))
		if parent == "." {
			parent = filepath.Base(app.root)
		}
		title := app.noteLabel(path.Base(relative))
		button := mygoui.ButtonBase(c.Key("recent:"+relative)).Height(39).MarginStart(2).
			Padding(0, 10).Justify(mygoui.Start).Label(title + ", " + parent).Tooltip(relative).
			Disabled(app.modalOpen).OnClick(func() { app.openVaultRow(relative, false) }).
			ContextMenu(func(menu *mygoui.Menu) { app.vaultFileMenu(menu, relative) })
		if button.Hovered() {
			button.Background(c.Theme().SurfaceHover).Radius(5)
		}
		button.Children(func() {
			mygoui.Column(c).Grow(1).MinWidth(0).Children(func() {
				mygoui.Text(c, title).FontSize(12).SingleLine().Ellipsis("…")
				mygoui.Text(c, parent).FontSize(11).SingleLine().Ellipsis("…").TextColor(c.Theme().TextMuted)
			})
		})
	}
}

func noteName(name string) string { return strings.TrimSuffix(name, path.Ext(name)) }

func (app *workspaceWindow) noteLabel(name string) string {
	if app.preferences.ShowExtension || !workspace.IsMarkdownPath(name) {
		return name
	}
	return noteName(name)
}

func (app *workspaceWindow) folder(relative string) *bool {
	if app.folders[relative] == nil {
		app.folders[relative] = new(bool)
	}
	return app.folders[relative]
}

func (app *workspaceWindow) revealFolder(relative string) {
	app.setVaultPanelVisible(true)
	app.fileSearch = ""
	app.setGraphTag("")
	app.vaultSearchChanged()
	for relative != "." && relative != "" {
		*app.folder(relative) = true
		relative = path.Dir(relative)
	}
}

func (app *workspaceWindow) statusBar(c *mygoui.Context) {
	app.workingText()
	mygoui.Toolbar(c, func() {
		mygoui.ScrollHorizontal(c.Key("breadcrumbs")).Grow(1).MinWidth(0).Height(18).Padding(0, 4).Gap(7.5).
			AlignItems(mygoui.Center).Label("Document path").Children(func() {
			parts, collapsed := app.documentBreadcrumbs()
			if len(parts) == 0 && collapsed == "" {
				mygoui.Text(c, app.title()).FontSize(11).SingleLine()
				return
			}
			if collapsed != "" {
				mygoui.Text(c, "…").FontSize(11).FixedLineHeight(14).Size(9, 14).Shrink(0).
					TextColor(c.Theme().TextMuted).Tooltip(collapsed)
			}
			for index, part := range parts {
				if index > 0 || collapsed != "" {
					breadcrumbSeparator(c)
				}
				app.breadcrumb(c, part)
			}
		})
		if app.dirty() {
			mygoui.Text(c, "Unsaved changes").FontSize(11).Role(mygoui.RoleStatus)
		}
		app.terminalButton(c)
		statusCount(c, strconv.Itoa(app.metrics.Words), "words")
		statusCount(c, strconv.Itoa(app.metrics.Characters), "chars")
		statusCount(c, strconv.Itoa(app.metrics.ReadingMinutes)+" min", "read")
	}).Height(statusBarHeight).Shrink(0).AlignItems(mygoui.Center).Padding(0, 12).
		Background(app.titleSurface(c)).BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border).
		Gap(12).Label("Document status")
}

func breadcrumbSeparator(c *mygoui.Context) {
	mygoui.Box(c).Size(4.5, 7.5).Center().Shrink(0).Role(mygoui.RoleImage).Label("Forward").
		Children(func() { chromeIconColor(c, "chevron.right", 9, c.Theme().TextMuted) })
}

func (app *workspaceWindow) breadcrumb(c *mygoui.Context, part workspaceBreadcrumb) {
	theme := c.Theme()
	symbol, ink := "folder", theme.TextMuted
	if part.file {
		symbol, ink = "doc.text", theme.Accent
	}
	if app.blackout && !c.Preferences().HighContrast {
		ink = blackoutTitleInk
		if part.file {
			ink = blackoutSourceHead
		}
	}
	mygoui.ButtonBase(c.Key(part.target)).Height(14).Shrink(0).Gap(4).KeepFocus().
		Label(part.label).Tooltip(part.target).Disabled(app.modalOpen || part.target == "").
		OnClick(func() { mygo.Shell.ShowItemInFolder(part.target) }).Children(func() {
		if part.home {
			sfsymbols.Symbol(c, "house").Size(10).Natural().Color(ink).
				Fallback(chromeIconFallbacks["folder"]).Role(mygoui.RoleNone).Margin(0, 1)
		} else {
			icon := chromeIconColor(c, symbol, 10, ink)
			if !part.file {
				icon.Margin(0, 1)
			}
		}
		mygoui.Text(c, part.label).FontSize(11).FixedLineHeight(14).SingleLine().Role(mygoui.RoleNone)
	})
}

func (app *workspaceWindow) terminalButton(c *mygoui.Context) {
	disabled := app.vault == nil || app.modalOpen
	theme := c.Theme()
	fill, edge := theme.Surface, theme.Border
	if app.glow.shown {
		fill, edge = theme.SurfaceHover, theme.Accent
	}
	if app.blackout && !c.Preferences().HighContrast {
		fill, edge = mygoui.RGB(21, 26, 35), mygoui.RGB(60, 75, 97)
		if app.glow.shown {
			fill, edge = mygoui.RGB(39, 48, 64), mygoui.RGB(88, 108, 138)
		}
	}
	mygoui.ButtonBase(c).Label("GLOW Terminal").Size(86, 19).Gap(4).Radius(4).
		Background(fill).Border(1, edge).Disabled(disabled).AccessSelected(app.glow.shown).
		OnClick(app.toggleGlow).Children(func() {
		mygoui.Box(c).Size(17, 12).Shrink(0).Role(mygoui.RoleNone).
			Draw(func(p *mygoui.Painter, frame mygoui.Rect) {
				x, y := frame.X+1.5, frame.Y+3
				var prompt mygoui.Path
				prompt.MoveTo(x, y).LineTo(x+5.5, y+2.5).LineTo(x+5.5, y+4).
					LineTo(x, y+6.5).LineTo(x, y+4.65).LineTo(x+3.25, y+3.25).
					LineTo(x, y+1.85).Close()
				p.FillPath(&prompt, theme.Text)
				p.Fill(mygoui.Rect{X: x + 8, Y: y + 7, W: 6, H: 1.5}, theme.Text, 0.25)
			})
		mygoui.Text(c, "Terminal").FontSize(11).FontWeight(600)
	})
}

func statusCount(c *mygoui.Context, count, unit string) {
	mygoui.Row(c).Gap(3).Shrink(0).AlignItems(mygoui.Center).Children(func() {
		mygoui.Text(c, count).FontSize(11).FontFeatures("tnum").SingleLine()
		mygoui.Text(c, unit).FontSize(11).SingleLine().TextColor(c.Theme().TextMuted)
	})
}

func (app *workspaceWindow) preview(c *mygoui.Context) {
	if !app.focusMode {
		app.previewToolbar(c)
	}
	if app.largePreviewPaused(c) {
		return
	}
	if app.pdfPreview.enabled && app.previewDocumentType() != documentHTML {
		app.pdfPreview.draw(c, app)
	} else if app.showsStartPage() {
		_, width := app.paneToolWidths(c)
		app.drawStartPreview(c, width)
	} else {
		app.previewPane.draw(c, app)
	}
}

func (app *workspaceWindow) modeChanged() {
	app.tableGrid = workspaceTableGrid{}
	app.wys.closeTableControls()
	app.wys.closeBlockControls()
	mode := app.displayedMode()
	app.focusSource = mode == sourceMode || mode == splitMode
	if mode == wysiwygMode {
		app.wys.queryResults = nil
		app.wys.focus = true
	}
}

func (app *workspaceWindow) setMode(mode int) {
	if app.draft == nil || app.modalOpen {
		return
	}
	previous := app.displayedMode()
	remountSource := !app.multiFile.active && (previous == previewMode || previous == wysiwygMode) && (mode == sourceMode || mode == splitMode)
	returnToSource := app.mode == previewMode && mode == sourceMode && app.maximizedPane == paneNotMaximized
	app.wys.flush(app)
	app.multiFile.active = false
	app.multiFile.editing = nil
	app.mode = mode
	app.maximizedPane = paneNotMaximized
	app.modeChanged()
	if remountSource {
		// EV-UI-325. Restore the initial Source position in the existing editor.
		if returnToSource && app.pendingConceptExcerpt == "" && app.code != nil {
			app.code.Select(0, 0)
			app.code.SetScrollY(0)
			app.focusVaultSearch = false
		}
		app.revealPendingVaultConceptSource()
	}
}

func (app *workspaceWindow) toggleFocus() {
	if app.draft != nil && !app.multiFile.active {
		app.tableGrid = workspaceTableGrid{}
		app.focusMode = !app.focusMode
	}
}

func (app *workspaceWindow) toggleOutline() {
	if app.draft != nil {
		app.outlineVisible = !app.outlineVisible
	}
}

func (app *workspaceWindow) togglePreview() {
	if app.multiFile.active {
		app.setMultiFileRendered(!app.multiFile.rendered)
		return
	}
	mode := app.displayedMode()
	if mode == sourceMode {
		app.setMode(splitMode)
	} else if mode != wysiwygMode {
		app.setMode(sourceMode)
	}
}

func (app *workspaceWindow) buildMenu() {
	command := app.menuCommand
	recent := app.recentMenuItems()
	app.sourceHighlightMenu = &mygo.MenuItem{
		Label: app.sourceHighlightMenuLabel(), Accelerator: "CmdOrCtrl+Shift+H", Click: command(app.toggleSourceHighlighting),
	}
	app.moveTabMenu = &mygo.MenuItem{
		Label: "Move Tab to New Window", Disabled: len(app.tabs) < 2, Click: command(app.moveTabToNewWindow),
	}
	app.templates.saveMenu = &mygo.MenuItem{
		Label: "Save as Template...", Disabled: app.draft == nil, Click: command(app.saveAsTemplate),
	}
	app.menu = mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout},
			{Label: app.tr("Settings…", "设置…"), Accelerator: "CmdOrCtrl+,", Click: command(app.showSettings)},
			mygo.Separator(), {Role: mygo.RoleServices},
			mygo.Separator(), {Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide},
			mygo.Separator(), {Role: mygo.RoleQuit},
		}},
		{Label: "File", Submenu: []*mygo.MenuItem{
			{Label: "New Document", Accelerator: "CmdOrCtrl+N", Click: command(app.newDocument)},
			{Label: "New Tab", Accelerator: "CmdOrCtrl+T", Click: command(app.newTab)},
			{Label: "New Drawing", Click: command(app.newDrawing)},
			{Label: "New from Template", Submenu: app.templateMenuItems()},
			app.templates.saveMenu,
			mygo.Separator(),
			{Label: "Open…", Accelerator: "CmdOrCtrl+O", Click: command(app.filePicker)},
			{Label: "Open Recent", Submenu: recent},
			{Label: "Open Vault…", Click: command(app.chooseVault)},
			{Label: "Create Vault…", Click: command(app.createVault)},
			mygo.Separator(),
			{Label: "Save", Accelerator: "CmdOrCtrl+S", Click: app.saveFromMenu},
			{Label: "Save As…", Accelerator: "CmdOrCtrl+Alt+Shift+S", Click: command(func() {
				if app.permitsActiveDocument() {
					app.saveAs(app.workspaceTab, "", nil)
				}
			})},
			{Label: "Duplicate", Accelerator: "CmdOrCtrl+Shift+S", Click: command(func() {
				if relative := app.currentVaultPath(); relative != "" {
					app.duplicateNote(relative)
				}
			})},
			{Label: "Rename…", Click: command(func() {
				if relative := app.currentVaultPath(); relative != "" {
					app.renameNote(relative)
				}
			})},
			{Label: "Move To…", Click: command(func() {
				if relative := app.currentVaultPath(); relative != "" {
					app.chooseMoveFolder(relative)
				}
			})},
			{Label: "Revert to Saved…", Click: command(app.reloadNote)},
			{Label: "Browse Versions…", Click: command(app.showVersionHistory)},
			mygo.Separator(),
			{Label: "Quick Export PDF", Accelerator: "CmdOrCtrl+E", Click: command(app.quickExportPDF)},
			{Label: "Export…", Accelerator: "CmdOrCtrl+Shift+E", Click: command(app.showExport)},
			mygo.Separator(),
			{Label: "Close", Accelerator: "CmdOrCtrl+W", Click: command(app.closeWorkspaceCommand)},
			{Label: "Close All", Accelerator: "CmdOrCtrl+Alt+W", Click: command(app.win.Close)},
		}},
		{Label: "Edit", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleUndo}, {Role: mygo.RoleRedo}, mygo.Separator(),
			{Role: mygo.RoleCut}, {Role: mygo.RoleCopy}, {Role: mygo.RolePaste}, {Role: mygo.RoleSelectAll},
			mygo.Separator(),
			{Label: "Find…", Accelerator: "CmdOrCtrl+F", Click: command(func() { app.showFind(false) })},
			{Label: "Find and Replace…", Accelerator: "CmdOrCtrl+Alt+F", Click: command(func() { app.showFind(true) })},
			{Label: "Fold Selection", Accelerator: "CmdOrCtrl+.", Click: command(func() {
				if app.sourceEditing() {
					app.code.Run(editor.ToggleFold)
				}
			})},
		}},
		{Label: "Format", Submenu: []*mygo.MenuItem{
			{Label: "Increase Font Size", Accelerator: "CmdOrCtrl+Plus", Click: command(func() { app.changeFontSize(1) })},
			{Label: "Decrease Font Size", Accelerator: "CmdOrCtrl+-", Click: command(func() { app.changeFontSize(-1) })},
			{Label: "Reset Font Size", Accelerator: "CmdOrCtrl+0", Click: command(func() { app.changeFontSize(0) })},
		}},
		{Label: "View", Submenu: []*mygo.MenuItem{
			{Label: "Editor Only", Accelerator: "CmdOrCtrl+1", Click: command(func() { app.setMode(sourceMode) })},
			{Label: "Split View", Accelerator: "CmdOrCtrl+2", Click: command(func() { app.setMode(splitMode) })},
			{Label: "Preview Only", Accelerator: "CmdOrCtrl+3", Click: command(func() { app.setMode(previewMode) })},
			{Label: "WYSIWYG", Accelerator: "CmdOrCtrl+4", Click: command(func() { app.setMode(wysiwygMode) })},
			{Label: "Multi-file View", Click: command(app.toggleMultiFile)},
			mygo.Separator(),
			{Label: "Toggle Preview", Accelerator: "CmdOrCtrl+\\", Click: command(app.togglePreview)},
			{Label: "Toggle PDF Preview", Accelerator: "CmdOrCtrl+Shift+P", Click: command(app.togglePDFPreview)},
			{Label: "Toggle Snippets", Accelerator: "CmdOrCtrl+Alt+S", Click: command(app.toggleSnippets)},
			{Label: "Toggle Outline", Accelerator: "CmdOrCtrl+Shift+O", Click: command(app.toggleOutline)},
			{Label: "Toggle Focus Mode", Accelerator: "CmdOrCtrl+Shift+F", Click: command(app.toggleFocus)},
			app.sourceHighlightMenu,
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Label: "Insert", Submenu: app.nativeInsertMenu()},
		{Label: "Vault", Submenu: []*mygo.MenuItem{
			{Label: "New Folder…", Click: command(func() { app.createFolder(".", false) })},
			app.installVaultStatsMenu(),
			{Label: "Restore Example Vault", Click: command(app.restoreExampleVault)},
			{Label: "Refresh Vault", Click: command(app.refreshVault)},
			{Label: "Reveal Vault in File Manager", Click: command(app.revealVault)},
			{Label: "Open Vault Trash", Click: command(app.openVaultTrash)},
			{Label: "Graph", Click: command(func() {
				if app.vault != nil {
					app.toggleGraph()
				}
			})},
		}},
		{Role: mygo.RoleWindowMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleMinimize}, {Role: mygo.RoleZoom},
			mygo.Separator(),
			{Label: "Show Previous Tab", Accelerator: "Ctrl+Shift+Tab", Click: command(func() { app.moveTab(-1) })},
			{Label: "Show Next Tab", Accelerator: "Ctrl+Tab", Click: command(func() { app.moveTab(1) })},
			app.moveTabMenu,
			mygo.Separator(), {Role: mygo.RoleFront},
		}},
		{Label: app.tr("Help", "帮助"), Submenu: []*mygo.MenuItem{
			{Label: applicationName + " " + app.tr("Help", "帮助"), Click: command(app.showFeatureGuide)},
			{Label: app.userGuideTitle(), Click: command(func() { app.openUserGuide(0, false) })},
			markdownSyntaxGuideMenuItem(app.interfaceLanguage()),
			mygo.Separator(),
			{Label: app.tr("Keyboard Shortcuts", "键盘快捷键"), Accelerator: "CmdOrCtrl+Shift+/", Click: command(app.showShortcuts)},
			{Label: app.tr("Show Welcome", "显示欢迎页"), Click: command(app.showWelcome)},
			mygo.Separator(),
			app.application.replaySplashMenuItem(app.interfaceLanguage()),
		}},
	})
	app.win.SetMenu(app.menu)
	app.updateRecentMenu()
	app.updateVaultStatsMenu()
}
