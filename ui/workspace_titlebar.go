package ui

import (
	"runtime"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type modeControl struct {
	symbol string
	label  string
	name   string
}

var modeControls = [...]modeControl{
	sourceMode:  {symbol: "doc.text", label: "Plain Text Document", name: "Editor Only"},
	splitMode:   {symbol: "rectangle.split.2x1", label: "Rectangle Split Two By One", name: "Split View"},
	previewMode: {symbol: "eye", label: "Show", name: "Preview Only"},
	wysiwygMode: {symbol: "wand.and.stars", label: "Magic Wand With Stars", name: "WYSIWYG"},
}

func (app *workspaceWindow) toolbar(c *mygoui.Context) {
	bar := c.TitleBar()
	if app.focusMode {
		height := float32(36)
		if runtime.GOOS != "darwin" {
			height = max(height, bar.Height)
		}
		title := mygoui.Row(c).Height(height).Shrink(0).FillWidth().DragWindow().Background(app.titleSurface(c)).
			AlignItems(mygoui.Center).Padding(0, 10, 0, max(10, bar.Left)).Label("Document title")
		title.Children(func() {
			mygoui.Text(c, app.title()).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").TextAlign(mygoui.Center)
			chromeButton(c, "arrow.up.left.and.arrow.down.right", "Exit Focus Mode",
				paneAction(!app.modalOpen, app.toggleFocus)).Size(28, 24)
		})
		app.tabDropTarget(c, title, 6)
		return
	}

	width, _ := c.Size()
	// EV-UI-032 records these widths with each new trailing group visible.
	settingsVisible := width >= 1140
	trailingVisible := width >= 1190
	spacing := float32(36)
	if width < 1000 {
		spacing = 8
	}
	left, right := float32(12), float32(4)
	if bar.Left > 0 {
		left = max(92, bar.Left+8)
	}
	if !trailingVisible {
		right = 7
	}
	right = max(right, bar.Right)
	keyboardWidth, settingsWidth, tailWidth := float32(40), float32(36), float32(86)
	if !trailingVisible {
		keyboardWidth, settingsWidth, tailWidth = 46, 42, 49
	}
	if !settingsVisible {
		settingsWidth = 0
	}
	scrollGroupWidth := float32(72)
	if app.preferences.ScrollSyncEnabled {
		scrollGroupWidth += 37
	}
	leftControls := 347 + scrollGroupWidth + 3*spacing
	rightControls := 185 + spacing + keyboardWidth + settingsWidth + tailWidth

	mygoui.Column(c).Height(titleBarHeight).Shrink(0).FillWidth().Children(func() {
		toolbar := mygoui.Toolbar(c, func() {
			app.titleButton(c, "folder", "Vault", 43, func() {
				if app.vault == nil || app.showingAI() {
					app.openFilesSidebar()
					return
				}
				app.setVaultPanelVisible(!app.vaultPanelVisible())
			})
			app.titleButton(c, "sparkles", "AI", 36,
				paneAction(app.drawing == nil && (app.vault != nil || app.draft != nil), app.toggleAIWorkspace))
			titleGroup(c, 36, 0, 0, func() {
				app.titleMenu(c, "pencil.and.outline", "Drawing", app.drawingCommands).
					Size(30, 28).Disabled(app.modalOpen || app.multiFile.active)
			})
			app.titleDivider(c, spacing)
			titleGroup(c, 191, 9, 3, func() {
				for mode, control := range modeControls {
					selected := !app.multiFile.active && app.draft != nil && app.mode == mode
					button := app.titleIconButton(c, control.symbol, control.label, selected, func() { app.setMode(mode) }, control.name).
						Size(30, 28).Shrink(0).Disabled(app.draft == nil || app.modalOpen).
						AccessSelected(selected)
					if selected && (!app.blackout || c.Preferences().HighContrast) {
						button.Background(c.Theme().SurfacePressed).Radius(6)
					}
				}
				selected := app.multiFile.active
				button := app.titleIconButton(c, "square.grid.2x2", "Multi-file View", selected, app.toggleMultiFile, "Multi-file View").
					Size(30, 28).Shrink(0).Disabled(!app.canToggleMultiFile()).AccessSelected(selected)
				if selected {
					button.Tooltip(commandPaletteShortcut(multiFileShortcutHint)).ContextMenu(app.multiFileContextMenu)
				}
				if selected && (!app.blackout || c.Preferences().HighContrast) {
					button.Background(c.Theme().SurfacePressed).Radius(6)
				}
			})
			app.titleDivider(c, spacing)
			titleGroup(c, scrollGroupWidth, 7, 3, func() {
				app.scrollSyncToolbar(c)
				highlighting := !app.preferences.DisableSyntaxHighlighting
				button := app.titleIconButton(c, "highlighter", "Highlight", highlighting, app.toggleSourceHighlighting, app.sourceHighlightTooltip()).
					Size(30, 28).Shrink(0).Disabled(app.modalOpen).AccessSelected(highlighting).
					KeyShortcuts(mygoui.Cmd|mygoui.Shift, mygoui.KeyH)
				if highlighting && (!app.blackout || c.Preferences().HighContrast) {
					button.Background(c.Theme().SurfacePressed).Radius(6)
				}
			})
			app.titleDivider(c, spacing)
			app.titleButton(c, "clock.arrow.circlepath", "Version History", 41, app.showVersionHistory).
				Disabled(!app.canBrowseVersions())

			mygoui.Spacer(c).MinWidth(0)

			app.titleButton(c, "exclamationmark.triangle.fill", "Warning", 41, nil)
			titleGroup(c, 36, 0, 0, func() {
				app.appearanceButton(c).Size(30, 28).Disabled(app.modalOpen)
			})
			titleGroup(c, 36, 0, 0, func() {
				app.previewThemeButton(c).Size(30, 28).Disabled(app.modalOpen)
			})
			app.titleButton(c, "square.and.arrow.up", "Export", 36, app.showExport).
				Disabled(app.draft == nil || app.modalOpen || !app.permitsActiveDocument())
			app.titleDivider(c, spacing)
			app.titleButton(c, "questionmark.circle", app.tr("Help", "帮助"), 36, app.showFeatureGuide).KeepFocus()
			app.titleButton(c, "keyboard", app.tr("Keyboard Shortcuts", "键盘快捷键"), keyboardWidth, app.showShortcuts).KeepFocus()
			if settingsVisible {
				app.titleButton(c, "gearshape", app.tr("Settings", "设置"), settingsWidth, app.showSettings)
			}
			if trailingVisible {
				app.titleButton(c, "eyeglasses", "Focus", 45, app.toggleFocus).Disabled(app.draft == nil || app.modalOpen)
				app.titleButton(c, "plus", "New Tab", 41, app.newTab)
			} else {
				mygoui.Row(c).Width(11).Height(1).Shrink(0)
				chromeMenu(c, "ellipsis", "more toolbar items", func(m *mygoui.Menu) {
					if !settingsVisible && m.Item(app.tr("Settings", "设置")).Disabled(app.modalOpen).Chosen() {
						app.showSettings()
					}
					if m.Item("Focus").Disabled(app.draft == nil || app.modalOpen).Chosen() {
						app.toggleFocus()
					}
					if m.Item("New Tab").Disabled(app.modalOpen).Chosen() {
						app.newTab()
					}
				}).Size(38, 38).Shrink(0).Role(mygoui.RolePopUpButton).Disabled(app.modalOpen)
			}
		}).Height(titleBarHeight).Shrink(0).Gap(0).Padding(0, right, 0, left).AlignItems(mygoui.Center).
			Background(app.titleSurface(c)).Draw(app.titleMaterial(c, left, right, leftControls, rightControls)).
			Label("Main toolbar")
		if width > left+leftControls+right+rightControls {
			app.toolbarTitle(c, left+leftControls, right+rightControls)
		} else {
			app.tabDropTarget(c, toolbar, 6)
		}
	})
}

// EV-UI-059: the mode control occupies one slot only while sync is enabled.
func (app *workspaceWindow) scrollSyncToolbar(c *mygoui.Context) {
	prefs := app.preferences
	tooltip := "Enable scroll sync"
	if prefs.ScrollSyncEnabled {
		tooltip = "Disable scroll sync"
	}
	button := app.titleIconButton(c, "arrow.up.arrow.down", "Scroll Sync", prefs.ScrollSyncEnabled, app.toggleScrollSync, tooltip).
		Size(30, 28).Shrink(0).Disabled(app.modalOpen).AccessSelected(prefs.ScrollSyncEnabled)
	if !prefs.ScrollSyncEnabled {
		return
	}
	if !app.blackout || c.Preferences().HighContrast {
		button.Background(c.Theme().SurfacePressed).Radius(6)
	}
	symbol, tooltip := "percent", "Percentage sync (smooth)"
	if prefs.UseLineBasedScrollSync {
		symbol, tooltip = "text.line.first.and.arrowtriangle.forward", "Line-based sync (accurate)"
	}
	app.titleIconButton(c, symbol, "Scroll Sync Mode", false, func() {
		prefs.UseLineBasedScrollSync = !prefs.UseLineBasedScrollSync
		app.preferencesChanged()
	}, tooltip).Size(30, 28).Shrink(0).Disabled(app.modalOpen)
}

func (app *workspaceWindow) toggleScrollSync() {
	app.preferences.ScrollSyncEnabled = !app.preferences.ScrollSyncEnabled
	app.preferencesChanged()
}

func (app *workspaceWindow) toolbarTitle(c *mygoui.Context, left, right float32) {
	title := app.title()
	slot := mygoui.Row(c.Key("workspace-title")).Absolute().Left(left).Right(right).Top(0).
		Height(titleBarHeight).AlignItems(mygoui.Center).DragWindow().Label("Document title")
	slot.Children(func() {
		if app.canShowDocumentTitleActions() {
			if !app.blackout {
				slot.Justify(mygoui.Center)
			}
			app.drawDocumentTitle(c)
			return
		}
		label := mygoui.Text(c, title).Grow(1).MinWidth(0).Bold().FontSize(13).
			TextAlign(mygoui.Center).SingleLine().Ellipsis("…")
		if app.blackout {
			label.TextAlign(mygoui.Start).PaddingStart(8).FontSize(14).FontWeight(600)
		}
	})
	app.tabDropTarget(c, slot, 6)
}

func titleGroup(c *mygoui.Context, width, gap, leading float32, children func()) {
	group := mygoui.Row(c).Role(mygoui.RoleGroup).Width(width).Height(titleBarHeight).Shrink(0).Gap(gap).
		Padding(0, 0, 0, leading).AlignItems(mygoui.Center)
	if gap == 0 {
		group.Justify(mygoui.Center)
	}
	group.Children(children)
}

func (app *workspaceWindow) titleDivider(c *mygoui.Context, width float32) {
	gap := mygoui.Row(c).Width(width).Height(16).Shrink(0)
	if app.blackout {
		gap.Draw(func(p *mygoui.Painter, r mygoui.Rect) {
			p.Fill(mygoui.Rect{X: r.X + r.W/2 - 0.5, Y: r.Y, W: 1, H: r.H}, blackoutTitleRule, 0)
		})
	}
}

func (app *workspaceWindow) titleButton(c *mygoui.Context, symbol, label string, width float32, action func()) mygoui.Element {
	active := app.vaultVisible && (label == "AI" && app.showingAI() || label == "Vault" && !app.showingAI())
	return app.titleIconButton(c, symbol, label, active, action).
		Width(width).Height(titleBarHeight).
		Shrink(0).Disabled(action == nil || app.modalOpen)
}

func (app *workspaceWindow) titleIconButton(c *mygoui.Context, symbol, label string, active bool, action func(), help ...string) mygoui.Element {
	if !app.blackout {
		ink := c.Theme().Text
		if palette, ok := appearancePalettes[app.appAppearance()]; ok && label == "Vault" {
			ink = palette.muted
			if active {
				ink = palette.accent
			}
		}
		return chromeButtonIcon(c, symbol, label, action, chromeIconSize, ink, help...)
	}
	size, ink := float32(22), blackoutTitleInk
	if active {
		ink = blackoutTitleAccent
	}
	switch symbol {
	case "exclamationmark.triangle.fill":
		ink = mygoui.RGB(255, 146, 48)
	case "eyeglasses":
		size = 27
	}
	if label == "Settings" && !c.Preferences().HighContrast {
		button := chromeControl(c, label, action == nil || app.modalOpen, help...).Background(mygoui.Transparent)
		if button.Hovered() {
			ink = blackoutToolbarHoverInk
		}
		if toolbarPressInside(button) {
			button.Draw(drawTitleSettingsPress)
		}
		return button.OnClick(action).Children(func() { chromeIconColor(c, symbol, size, ink) })
	}
	return chromeButtonIcon(c, symbol, label, action, size, ink, help...)
}

func (app *workspaceWindow) titleMenu(c *mygoui.Context, symbol, label string, build func(*mygoui.Menu)) mygoui.Element {
	if !app.blackout {
		return chromeMenu(c, symbol, label, build)
	}
	return chromeMenuControl(c, label, build).Children(func() {
		chromeIconColor(c, symbol, 20, blackoutTitleInk)
	})
}

func (app *workspaceWindow) drawingCommands(m *mygoui.Menu) {
	if m.Item("New Drawing").Disabled(app.vault == nil).Chosen() {
		app.newDrawing()
	}
	if m.Item("Open Drawing…").Chosen() {
		app.drawingPicker()
	}
	if m.Item("Map Vault").Disabled(app.vault == nil || app.vaultMapBuilding).Chosen() {
		app.mapVault()
	}
}

func (app *workspaceWindow) showSettings() {
	if app.settings != nil && !app.settings.IsDestroyed() {
		app.settings.Focus()
		return
	}
	if app.settingsTab == "" {
		app.settingsTab = "General"
	}
	if app.settingsTab == "AI" {
		app.prepareAIConnectionSettings()
	}
	app.settings = mygo.NewWindow(mygo.WindowOptions{
		Title: app.settingsWindowTitle(), Width: 880, Height: 892, MinWidth: 500, MinHeight: 300,
		Parent: app.win, DisableMinimize: true, DisableMaximize: true, Vibrancy: workspaceVibrancy(),
		Content: mygoui.View(app.appearanceSettings),
	})
	app.settings.OnClose(func(e *mygo.CloseEvent) {
		if app.application != nil && !app.application.stylesheet.flush(app) {
			e.PreventDefault()
		}
	})
	app.settings.OnClosed(app.closeAIConnectionSettings)
}

func (app *workspaceWindow) openAISettings() {
	enteringAI := app.settingsTab != "AI"
	app.settingsTab = "AI"
	if app.settings != nil && !app.settings.IsDestroyed() {
		if enteringAI {
			app.prepareAIConnectionSettings()
		}
		app.settings.Invalidate()
	}
	app.showSettings()
}

func (app *workspaceWindow) appearanceSettings(c *mygoui.Context) {
	app.applyAppearance(c)
	host := mygoui.Row(c).Fill().AlignItems(mygoui.Stretch)
	host.Children(func() {
		mygoui.Column(c).Width(168).Shrink(0).Padding(16, 12).Gap(4).
			Background(workspaceChromeSurface(c)).Children(func() {
			mygoui.Text(c, app.tr("Settings", "设置")).FontSize(17).Bold().Role(mygoui.RoleHeading).
				Height(24).Padding(0, 8).Margin(0, 0, 6)
			for _, page := range []struct{ id, chinese, icon string }{
				{"General", "通用", "gearshape"}, {"Themes", "主题", "swatchpalette"}, {"Export", "导出", "square.and.arrow.up"},
				{"AI", "AI", "sparkles"}, {"API", "API", "network"}, {"Shortcuts", "快捷输入", "keyboard"},
			} {
				label := app.tr(page.id, page.chinese)
				active := app.settingsTab == page.id
				item := mygoui.ButtonBase(c.Key(page.id)).Label(label).AccessSelected(active).Height(36).
					FillWidth().AlignItems(mygoui.Center).Justify(mygoui.Start).Gap(12).Padding(0, 14).Radius(7).
					OnClick(func() {
						if app.settingsTab == page.id {
							return
						}
						app.settingsTab = page.id
						if page.id == "AI" {
							app.prepareAIConnectionSettings()
						}
					})
				ink := c.Theme().Text
				if active {
					item.Background(c.Theme().SurfacePressed)
					if app.blackout && !c.Preferences().HighContrast {
						ink = blackoutSettingsAccent
						item.Background(mygoui.Hex("#291a29"))
					}
				}
				item.Children(func() {
					if active && app.blackout && !c.Preferences().HighContrast {
						mygoui.Box(c).Absolute().Left(2).Top(10).Size(3, 16).Radius(1.5).
							Background(ink).Role(mygoui.RoleNone)
					}
					chromeIconColor(c, page.icon, 16, ink)
					mygoui.Text(c, label).FontSize(13).FontWeight(600).TextColor(ink)
				})
			}
		})
		panel := mygoui.Scroll(c.Key(app.settingsTab)).Grow(1).MinWidth(0).Padding(28, 30).Background(c.Theme().Background)
		switch app.settingsTab {
		case "General", "Themes", "Export":
			panel.Padding(24)
		case "AI":
			panel.Padding(24)
		}
		revealWorkspacePage(c, host, panel, app.settingsTab, c.Theme().Background)
		panel.Children(func() {
			mygoui.Column(c).Gap(12).Children(func() {
				switch app.settingsTab {
				case "General":
					app.generalSettings(c)
				case "Themes":
					app.previewThemeSettings(c)
				case "Export":
					app.exportSettings(c)
				case "AI":
					app.aiSettings(c)
				case "API":
					app.apiSettings(c)
				case "Shortcuts":
					app.drawMarkdownShortcutsSettings(c)
				}
			})
		})
	})
}

func (app *workspaceWindow) titleSurface(c *mygoui.Context) mygoui.Color {
	if app.blackout {
		return c.Theme().Background
	}
	if _, ok := appearancePalettes[app.appAppearance()]; ok {
		return c.Theme().Background
	}
	return workspaceChromeSurface(c)
}

func (app *workspaceWindow) titleMaterial(c *mygoui.Context, left, right, leftWidth, rightWidth float32) func(*mygoui.Painter, mygoui.Rect) {
	base, border := c.Theme().Surface, c.Theme().Border
	blackout := app.blackout
	active := c.WindowActive()
	return func(p *mygoui.Painter, r mygoui.Rect) {
		if left+right+leftWidth+rightWidth > r.W {
			return
		}
		for _, pill := range []mygoui.Rect{
			{X: r.X + left + 4, Y: r.Y + 8, W: leftWidth - 8, H: 36},
			{X: r.X + r.W - right - rightWidth + 4, Y: r.Y + 8, W: rightWidth - 8, H: 36},
		} {
			switch {
			case blackout && active:
				p.FillGradient(pill, mygoui.LinearGradient{Angle: 180, Stops: []mygoui.GradientStop{
					{Color: mygoui.RGB(39, 39, 39), At: 0},
					{Color: mygoui.RGB(29, 29, 29), At: 0.2},
					{Color: mygoui.RGB(30, 30, 30), At: 0.5},
					{Color: mygoui.RGB(30, 30, 30), At: 0.78},
					{Color: mygoui.RGB(38, 38, 38), At: 1},
				}}, 18)
				p.Stroke(pill, mygoui.RGB(61, 61, 61), 18, 1)
			case blackout:
				p.Fill(pill, mygoui.RGB(23, 23, 23), 18)
				p.Stroke(pill, mygoui.RGB(24, 24, 24), 18, 1)
			default:
				p.Fill(pill, base, 18)
				p.Stroke(pill, border, 18, 1)
			}
		}
	}
}

func (app *workspaceWindow) drawDocumentBorder(c *mygoui.Context) func(*mygoui.Painter, mygoui.Rect) {
	border := c.Theme().Border
	top := float32(0)
	if app.blackout {
		border = blackoutPaneEdge
		top = 4
	}
	return func(p *mygoui.Painter, r mygoui.Rect) {
		if r.W > 1 && r.H > top+1 {
			p.Stroke(mygoui.Rect{X: r.X + 0.5, Y: r.Y + top + 0.5, W: r.W - 1, H: r.H - top - 1}, border, 7, 1)
		}
	}
}
