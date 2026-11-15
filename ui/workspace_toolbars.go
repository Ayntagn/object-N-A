package ui

import (
	"fmt"
	"math"
	"strconv"

	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

var (
	blackoutToolbarGlyphInk = mygoui.RGB(138, 141, 169)
	blackoutToolbarMenuInk  = mygoui.RGB(222, 222, 222)
)

func (app *workspaceWindow) paneTools(c *mygoui.Context, key, label string, inset float32, build func()) mygoui.Element {
	surface := c.Theme().Surface
	if app.blackout {
		surface = c.Theme().Background
	}
	return mygoui.Toolbar(c.Key(key), build).Height(documentBarHeight).Shrink(0).
		FillWidth().Padding(0, inset).Gap(0).AlignItems(mygoui.Center).
		Background(surface).Label(label)
}

func paneAction(enabled bool, action func()) func() {
	if enabled {
		return action
	}
	return nil
}

// EV-UI-009: the Vault row has a leading group and a right-aligned group.
func (app *workspaceWindow) vaultToolbar(c *mygoui.Context) {
	if app.showingProperties() {
		app.paneTools(c, "properties-tools", "Vault tabs", 12, func() { app.vaultTabs(c, true) }).
			Padding(4, 12, 0).Background(mygoui.Transparent).
			BorderWidth(0, 0, 1, 0).BorderColor(app.propertyDivider(c))
		return
	}
	bar := app.paneTools(c, "vault-tools", "Vault tools", 16, func() {
		app.vaultTabs(c, false)
		mygoui.Spacer(c)
		ink, menuInk := c.Theme().Text, c.Theme().Text
		if app.blackout && !c.Preferences().HighContrast {
			// EV-UI-265: the enabled buttons and menu icons use different Blackout colors.
			ink, menuInk = blackoutToolbarGlyphInk, blackoutToolbarMenuInk
		}
		chromeButtonIcon(c, "plus.square", "Add", paneAction(!app.modalOpen, app.addVaultNote), 16, ink).
			Size(26, 22).MarginEnd(4)
		chromeButtonIcon(c, "pencil.and.outline", "Annotate", paneAction(!app.modalOpen, app.newDrawing), 16.5, ink).
			Size(26, 22).MarginEnd(4)
		chromeButtonIcon(c, "folder.badge.plus", "New Folder", paneAction(!app.modalOpen, func() {
			app.createFolder(".", false)
		}), 19, ink).
			Size(26, 22).MarginEnd(3).KeepFocus()
		chromeMenuControl(c, "Folder With A Gear Badge", app.vaultGearMenu).Size(28, 17).MarginEnd(2).
			Disabled(app.modalOpen).Children(func() { chromeIconColor(c, "folder.badge.gearshape", 20.5, menuInk) })
		chromeMenuControl(c, "More", app.vaultMoreMenu).Size(22, 16).MarginEnd(3).
			Disabled(app.modalOpen).Children(func() { chromeIconColor(c, "ellipsis", 17, menuInk) })
		chromeButtonIcon(c, "xmark", "Close", paneAction(!app.modalOpen, func() {
			app.setVaultPanelVisible(false)
		}), 16.5, ink).Size(26, 22).Tooltip("Close Vault sidebar")
	})
	if app.blackout {
		bar.Background(mygoui.Transparent)
	} else {
		bar.Background(workspaceChromeSurface(c))
	}
	app.drawVaultGuide(c, bar)
}

func (app *workspaceWindow) showingProperties() bool {
	return app.sidebarTab == workspacePropertiesTab && app.draft != nil && app.drawing == nil && app.permitsActiveDocument()
}

func (app *workspaceWindow) vaultTabs(c *mygoui.Context, labeled bool) {
	for i, tab := range [...]struct {
		label, icon     string
		width, expanded float32
		action          func()
	}{
		{"Files", "folder", 31.5, 60.5, app.openFilesSidebar},
		{"AI", "sparkles", 28, 43, paneAction(app.drawing == nil, app.openAIWorkspace)},
		{"Properties", "list.bullet.rectangle", 32, 92, paneAction(app.draft != nil && app.permitsActiveDocument(), func() {
			app.vaultGuideOpen = false
			app.sidebarTab = workspacePropertiesTab
			app.leaveVaultFiles()
		})},
	} {
		selected := i == 0 && !labeled && !app.showingAI() || i == 1 && app.showingAI() || i == 2 && labeled
		width := tab.width
		if labeled {
			width = tab.expanded
		}
		control := mygoui.ButtonBase(c.Key(tab.label)).Label(tab.label).Tooltip(tab.label).
			Size(width, 22).Radius(6).Gap(5).Center().Shrink(0).
			AccessSelected(selected).Disabled(tab.action == nil || app.modalOpen)
		ink := c.Theme().Text
		if selected {
			ink = c.Theme().Accent
			control.Background(c.Theme().SurfacePressed)
			if app.blackout {
				ink = blackoutTitleAccent
				control.Background(mygoui.Hex("#151f2b"))
			}
		}
		if tab.action != nil {
			control.OnClick(tab.action)
		}
		control.Children(func() {
			size := float32(15)
			if labeled {
				size = 12
			}
			chromeIconColor(c, tab.icon, size, ink)
			if labeled {
				mygoui.Text(c, tab.label).FontSize(11).FontWeight(600).TextColor(ink).Role(mygoui.RoleNone)
			}
		})
	}
}

// EV-UI-009/010/021: the seven format controls share one slot when
// the editor pane cannot fit the measured full row.
func (app *workspaceWindow) formatToolbar(c *mygoui.Context) {
	sourceWidth, _ := app.paneToolWidths(c)
	// Leading controls take 427 points expanded or 257 collapsed.
	// The trailing controls take 108 points in both states.
	expandedWidth, collapsedWidth := float32(427+108+16), float32(257+108)
	if !app.vaultPanelVisible() {
		expandedWidth += 56
		collapsedWidth += 56
	}
	expanded := sourceWidth >= expandedWidth
	inset := min(8, max(0, (sourceWidth-collapsedWidth)/2))
	canFind := app.code != nil && !app.modalOpen
	canEdit := canFind && !app.code.ReadOnly
	var menu func(*mygoui.Menu)
	var format func(markdownFormat) bool
	if canEdit {
		menu, format = app.sourceFormattingMenu, app.applyFormatting
	}
	bar := app.paneTools(c, "source-tools", "Source tools", inset, func() {
		if !app.vaultPanelVisible() {
			// EV-UI-196 confirms that the folder button reopens the sidebar.
			chromeButtonIcon(c, "folder", "Move", paneAction(app.vault != nil && !app.modalOpen,
				app.openFilesSidebar), chromeIconSize, c.Theme().Text, "Show Vault").
				Size(26, 22).MarginEnd(2)
			chromeButton(c, "bubble.left.and.text.bubble.right", "Left Speech Bubble And Right Text Bubble", nil).
				Size(26, 22).MarginEnd(2)
		}
		app.editorViewToggle(c, "list.bullet.indent", "Toggle Outline", paneAction(canFind, app.toggleOutline), app.outlineVisible).
			MarginEnd(2)
		app.editorViewToggle(c, "list.number", "Toggle Line Numbers", paneAction(canFind, app.toggleLineNumbers), app.preferences.LineNumbers).
			MarginEnd(1)
		sizeMenu := app.sourceToolbarMenu(c, "textformat.size", "AA", 17.5, mygoui.RGB(196, 198, 208), app.editorFontSizeMenu)
		sizeMenu.Size(24, 16).Disabled(!canFind).Tooltip("Font Size")
		app.sourceToolbarMenu(c.Key("editor-font"), "textformat", "Text Formatting", 17, blackoutToolbarMenuInk, app.editorFontMenu).
			Size(26, 16).Disabled(!canFind).Tooltip("Editor Font")
		brush := "paintbrush"
		if sourceThemeForID(app.preferences.EditorTheme).id != systemSourceThemeID {
			brush = "paintbrush.fill"
		}
		app.sourceToolbarMenu(c.Key("editor-theme"), brush, "Editor Theme", 18, blackoutToolbarMenuInk, app.sourceThemeMenu).
			Size(25, 20).MarginEnd(5).Disabled(!canFind)
		if expanded {
			app.paneInsertMenus(c, 1, canEdit)
			app.paneFormatButtons(c, format, canEdit)
		} else {
			app.paneInsertMenus(c, 0, canEdit)
			anchor := app.sourceToolbarMenu(c.Key("format-overflow"), "textformat", "Text Formatting", 17.5, blackoutToolbarMenuInk, menu).
				Size(26, 16).MarginEnd(1)
			app.drawTableGrid(c, anchor, canEdit)
		}
		app.paneSnippetsButton(c)
		app.paneEditorEnd(c, paneAction(canFind, func() { app.showFind(false) }))
	})
	if !app.blackout {
		bar.BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border)
	}
}

// EV-UI-204/206/210/220: the view toggles keep focus in the editor. In Blackout,
// hover brightens the symbol and selection adds an accent surface.
func (app *workspaceWindow) editorViewToggle(c *mygoui.Context, symbol, label string, action func(), checked bool) mygoui.Element {
	verb := "Show"
	if checked {
		verb = "Hide"
	}
	help := verb + " line numbers"
	if symbol == "list.bullet.indent" {
		help = verb + " outline (" + commandPaletteShortcut("⌘⇧O") + ")"
	}
	button := chromeControl(c, label, action == nil, help).KeepFocus().Checked(checked)
	if action != nil {
		button.OnClick(action)
	}
	if !app.blackout || c.Preferences().HighContrast || action == nil {
		return button.Children(func() {
			sfsymbols.Symbol(c, symbol).Size(chromeIconSize).Natural().Color(c.Theme().Text).
				Fallback(editorViewToggleFallbacks[symbol]).Role(mygoui.RoleNone)
		})
	}
	ink := blackoutToolbarGlyphInk
	if checked {
		ink = mygoui.RGB(169, 199, 255)
		button.Radius(4.5).Background(mygoui.RGB(25, 32, 43))
	} else if paneToolbarFeedback(button) {
		ink = blackoutToolbarHoverInk
	}
	return button.Children(func() {
		editorViewToggleSymbol(c, symbol, ink)
	})
}

func editorViewSymbol(c *mygoui.Context, symbol string, ink mygoui.Color) sfsymbols.SymbolElement {
	// Fit MyGo's SF Symbol rendering to the measured Blackout glyph boxes.
	size, weight := float32(18), sfsymbols.Regular
	if symbol == "xmark" {
		size, weight = 16, sfsymbols.Medium
	}
	glyph := sfsymbols.Symbol(c, symbol).Size(size).Natural().Weight(weight).Color(ink).Fallback(chromeIconFallbacks[symbol])
	glyph.Role(mygoui.RoleNone)
	return glyph
}

// The two toolbar fallbacks retain the stepped bullets and all three numbers.
var editorViewToggleFallbacks = map[string]*mygoui.SVG{
	"list.bullet.indent": editorViewToggleFallback("editor-outline"),
	"list.number":        editorViewToggleFallback("editor-line-numbers"),
}

func editorViewToggleFallback(name string) *mygoui.SVG {
	data, err := chromeIconFiles.ReadFile("chrome_icons/" + name + ".svg")
	if err != nil {
		panic(err)
	}
	return mygoui.MustParseSVG(data)
}

func editorViewToggleSymbol(c *mygoui.Context, symbol string, ink mygoui.Color) {
	slot, size, weight := float32(18), float32(18), sfsymbols.Medium
	numbered := symbol == "list.number"
	if numbered {
		slot, size, weight = 16.5, 16, sfsymbols.Semibold
	}
	mygoui.Box(c).Size(slot, slot).Shrink(0).Role(mygoui.RoleNone).Draw(func(p *mygoui.Painter, r mygoui.Rect) {
		// These bitmap sizes and weights fit MyGo's raster output to the native glyphs.
		pixels := int(math.Ceil(float64(size * 2 * max(1, p.Scale()))))
		if glyph, ok := sfsymbols.Image(symbol, pixels, ink, sfsymbols.Options{Natural: true, Weight: weight}); ok {
			if numbered {
				left := r.X + (r.W-size)/2
				top := r.Y + (r.H-size)/2 + 0.5
				// The number glyph lands on a whole-point X even when Outline shifts the toolbar by a half point.
				r = mygoui.Rect{X: float32(math.Floor(float64(left))), Y: top, W: size, H: size}
			}
			p.Image(glyph, r, mygoui.Contain)
		} else if fallback := editorViewToggleFallbacks[symbol]; fallback != nil {
			p.Icon(fallback, r, ink)
		}
	})
}

// Blackout source menus use a single glyph in the compact slot.
func (app *workspaceWindow) sourceToolbarMenu(c *mygoui.Context, symbol, label string, size float32, ink mygoui.Color, build func(*mygoui.Menu)) mygoui.Element {
	if !app.blackout || c.Preferences().HighContrast {
		return chromeMenu(c, symbol, label, build)
	}
	return chromeMenuControl(c, label, build).Children(func() {
		if label == "AA" {
			mygoui.Text(c, "AA").FontSize(11).FontWeight(600).TextColor(ink).SingleLine().Role(mygoui.RoleNone)
		} else {
			chromeIconColor(c, symbol, size, ink)
		}
	})
}

func (app *workspaceWindow) sourceFormattingMenu(m *mygoui.Menu) {
	for _, command := range formattingCommands {
		if command.label == insertTableLabel {
			if m.Item(command.label).Chosen() {
				app.tableGrid = workspaceTableGrid{open: true, focus: true}
			}
			continue
		}
		item := m.Item(command.label).Disabled(command.format == "")
		if command.key != mygoui.KeyUnknown {
			item.Shortcut(mygoui.Cmd, command.key)
		}
		if item.Chosen() {
			app.applyFormatting(command.format)
		}
	}
}

func (app *workspaceWindow) paneInsertMenus(c *mygoui.Context, trailing float32, canEdit bool) {
	app.sourceToolbarMenu(c, "plus", "Add", 17.5, blackoutToolbarMenuInk, app.insertMenu).Size(22, 16).Disabled(!canEdit)
	app.sourceToolbarMenu(c, "exclamationmark.bubble", "Alert", 17.5, blackoutToolbarMenuInk, app.calloutMenu).Size(25, 18).
		Disabled(!app.sourceEditing() && (app.displayedMode() != wysiwygMode || !app.wys.editable(app)))
	app.microphoneControl(c).MarginEnd(trailing + 2)
}

func (app *workspaceWindow) paneFormatButtons(c *mygoui.Context, apply func(markdownFormat) bool, canInsertTable bool) {
	for _, command := range formattingCommands {
		key := c.Key(command.symbol)
		if command.label == insertTableLabel {
			app.tableGridButton(key, command.symbol, canInsertTable).Size(26, 22).MarginEnd(2)
			continue
		}
		action := paneAction(apply != nil && command.format != "", func() { apply(command.format) })
		chromeButton(key, command.symbol, command.label, action).Size(26, 22).MarginEnd(2)
	}
}

// EV-UI-049 / EV-UI-235: the scissors reveal the shade without an active fill.
func (app *workspaceWindow) paneSnippetsButton(c *mygoui.Context) {
	button := chromeControl(c, "Snippets", app.modalOpen, "Toggle Snippets").
		AccessSelected(app.snippets.open).OnClick(app.toggleSnippets)
	ink := c.Theme().Text
	if !c.Preferences().HighContrast {
		switch app.appAppearance() {
		case appearanceBlackout:
			ink = blackoutToolbarGlyphInk
			if paneToolbarFeedback(button) {
				ink = blackoutToolbarHoverInk
			}
		case appearanceCyber:
			ink = mygoui.Hex("#8a8da8")
			if button.Hovered() {
				ink = mygoui.Hex("#e8e9f3")
			}
		case appearancePLATO:
			ink = mygoui.Hex("#8f6d40")
			if button.Hovered() {
				ink = mygoui.Hex("#ffd9a0")
			}
		default:
			if !button.Hovered() {
				ink = c.Theme().TextMuted
			}
		}
	}
	button.Children(func() {
		sfsymbols.Symbol(c, "scissors").Size(18).Natural().Weight(sfsymbols.Medium).Color(ink).
			Fallback(chromeIconFallbacks["scissors"]).Role(mygoui.RoleNone)
	})
}

func (app *workspaceWindow) paneEditorEnd(c *mygoui.Context, find func()) {
	mygoui.Spacer(c)
	chromeButtonIcon(c, "sparkles", "Sparkle", paneAction(app.canOpenQuickActions(), app.openQuickActions), 17, c.Theme().Text).
		Size(26, 22).MarginEnd(1).KeepFocus().Tooltip("AI Quick Actions")
	var presentationMenu func(*mygoui.Menu)
	if app.canPresent() {
		presentationMenu = app.presentationMenu
	}
	chromeMenuControl(c, "Slideshow", presentationMenu).Size(26, 16).MarginEnd(1).
		Children(func() { chromeIcon(c, "play.rectangle.fill", 17.5) })
	search := chromeControl(c, "Search", find == nil).Size(26, 22).MarginEnd(2).
		KeyShortcuts(mygoui.Cmd, mygoui.KeyF).OnClick(find)
	ink := c.Theme().Text
	if app.blackout && find != nil && !c.Preferences().HighContrast {
		ink = blackoutToolbarGlyphInk
		if paneToolbarFeedback(search) {
			ink = blackoutToolbarHoverInk
		}
	}
	search.Children(func() { chromeIconColor(c, "magnifyingglass", 16.5, ink) })
	app.paneMaximizeButton(c, editorMaximized, 16.5, c.Theme().Text)
}

func (app *workspaceWindow) paneMaximizeLabel(pane paneMaximization) string {
	name := "editor"
	if pane == previewMaximized {
		name = "preview"
	}
	if app.maximizedPane == pane {
		return "Exit maximized " + name
	}
	return "Maximize " + name
}

func (app *workspaceWindow) paneMaximizeButton(c *mygoui.Context, pane paneMaximization, size float32, ink mygoui.Color) mygoui.Element {
	active := app.maximizedPane == pane
	symbol := "arrow.up.left.and.arrow.down.right"
	if active {
		symbol = "arrow.down.right.and.arrow.up.left"
	}
	return chromeButtonIcon(c, symbol, app.paneMaximizeLabel(pane), paneAction(!app.modalOpen, func() {
		app.togglePaneMaximization(pane)
	}), size, ink).AccessSelected(active)
}

// EV-UI-021/204: WYSIWYG replaces the line number button and style menus with
// the text-size and page-width controls.
func (app *workspaceWindow) wysToolbar(c *mygoui.Context) {
	canView := !app.modalOpen
	canEdit := canView && app.wys.editable(app)
	canInsertTable := app.wys.canInsertTable(app)
	var format func(markdownFormat) bool
	if canEdit {
		format = func(command markdownFormat) bool { return app.wys.format(app, command) }
	}
	app.paneTools(c, "wys-tools", "WYSIWYG tools", 8, func() {
		app.editorViewToggle(c, "list.bullet.indent", "Toggle Outline", paneAction(canView, app.toggleOutline), app.outlineVisible).
			MarginEnd(2)
		chromeButton(c, "textformat.size.smaller", "Decrease Text Size",
			paneAction(canView, func() { app.wys.changeFontSize(-1) })).Size(26, 22).MarginEnd(4)
		mygoui.Text(c, strconv.Itoa(app.wys.fontSize())).FontSize(11).Width(14).SingleLine().
			TextAlign(mygoui.Center).MarginEnd(4)
		chromeButton(c, "textformat.size.larger", "Increase Text Size",
			paneAction(canView, func() { app.wys.changeFontSize(1) })).Size(26, 22).MarginEnd(2)
		chromeButton(c, "rectangle.portrait.arrowtriangle.2.inward", "Portrait Rectangle With Two Inward Triangles",
			paneAction(canView, app.wys.togglePageWidth)).Size(26, 22).MarginEnd(6)
		app.paneInsertMenus(c, 1, canEdit || canInsertTable)
		app.paneFormatButtons(c, format, canInsertTable)
		app.paneSnippetsButton(c)
		app.paneEditorEnd(c, paneAction(canView, func() { app.showFind(false) }))
	})
}

func (app *workspaceWindow) previewToolbar(c *mygoui.Context) {
	_, previewWidth := app.paneToolWidths(c)
	markdownWidth, pdfWidth := previewLabelWidths(previewWidth)
	kind := app.previewDocumentType()
	typeLabel, typeIcon := documentTypeLabels[kind], documentTypeIcons[kind]
	pdfMode := app.pdfPreview.enabled && kind != documentHTML
	compactPDFType := pdfMode && previewWidth >= pdfToolbarInlineWidth && previewWidth < pdfToolbarTypeWidth
	showEdgeControls := !pdfMode || previewWidth >= 360
	refreshGap := float32(5)
	if compactPDFType {
		markdownWidth, refreshGap = 11, 0
	}
	enabled := !app.modalOpen
	labelInk := c.Theme().Text
	iconInk, paneInk := labelInk, labelInk
	blackoutInk := app.blackout && !c.Preferences().HighContrast
	blackoutDocumentTools := blackoutInk && !pdfMode && kind == documentMarkdown
	if blackoutInk {
		labelInk = blackoutPreviewLabelInk
		iconInk = blackoutPreviewIconInk
		paneInk = blackoutPaneToolInk
	}
	bar := app.paneTools(c, "preview-tools", "Preview toolbar", 8, func() {
		if showEdgeControls {
			chromeButtonIcon(c, "arrow.counterclockwise", "Counterclockwise Curved Arrow",
				paneAction(enabled, app.refreshDocumentPreview), 16.5, paneInk, "Return to preview").
				Size(26, 22).MarginEnd(refreshGap)
			typeMenu := chromeMenuControl(c, typeLabel, app.documentTypeMenu).
				Size(markdownWidth, 18).Gap(3).Disabled(!enabled).
				Tooltip("Document type (affects preview rendering). Click to override auto-detection.")
			if compactPDFType {
				typeMenu.Children(func() { chromeIconColor(c, typeIcon, 14, iconInk) })
			} else {
				iconSize := float32(15)
				if !pdfMode && kind == documentMarkdown {
					iconSize = 18
				}
				typeMenu.ClipX().Children(func() {
					chromeIconColor(c, typeIcon, iconSize, iconInk)
					mygoui.Text(c, typeLabel).FontSize(11).SingleLine().MinWidth(0).Shrink(1).
						Ellipsis("…").TextColor(labelInk).Role(mygoui.RoleNone)
					chromeIconColor(c, "chevron.down", 9, labelInk)
				})
			}
		}
		if pdfMode {
			app.pdfToolbarLayout(c, previewWidth, enabled, !showEdgeControls, labelInk)
		}
		mygoui.Spacer(c)
		app.previewZoomControls(c, enabled)
		if showEdgeControls {
			shareSize, shareInk := float32(16), iconInk
			maximizeSize, maximizeInk := float32(16), paneInk
			if blackoutDocumentTools {
				shareSize, shareInk = 18, blackoutToolbarMenuInk
				if app.maximizedPane != previewMaximized {
					maximizeSize, maximizeInk = 16.5, blackoutToolbarGlyphInk
				}
			}
			chromeMenuControl(c, "Share", app.shareMenu).Size(36, 20).MarginEnd(5).Gap(3).
				Disabled(!enabled).Children(func() {
				icon := chromeIconColor(c, "square.and.arrow.up", shareSize, shareInk)
				if blackoutDocumentTools {
					icon.Translate(.25, .5).IgnoreInput(true)
					sfsymbols.Symbol(c, "chevron.down").Size(11).Natural().Weight(sfsymbols.Heavy).
						Color(blackoutToolbarMenuInk).Fallback(chromeIconFallbacks["chevron.down"]).Role(mygoui.RoleNone).
						Translate(-3, .25).IgnoreInput(true)
				} else {
					chromeIconColor(c, "chevron.down", 9, labelInk)
				}
			})
			app.paneMaximizeButton(c, previewMaximized, maximizeSize, maximizeInk).MarginEnd(6)
			if pdfMode {
				target := app.pdfLayoutTarget()
				chromeButtonIcon(c, target.symbol, target.buttonLabel, paneAction(enabled, app.togglePDFGridLayout),
					16, paneInk, target.help).Size(26, 22).MarginEnd(6)
			}
		}
		if pdfMode {
			app.pdfToolbarToggle(c, enabled)
			return
		}
		if kind == documentHTML {
			chromeControl(c, "Open in Browser", !enabled).Size(pdfWidth, 23).Gap(4).Padding(0, 4).ClipX().
				Tooltip("Open in default browser for printing or viewing complex HTML").
				OnClick(app.openPreviewInBrowser).Children(func() {
				chromeIconColor(c, "safari", 15, iconInk)
				mygoui.Text(c, "Open in Browser").FontSize(11).SingleLine().MinWidth(0).Shrink(1).
					Ellipsis("…").TextColor(labelInk).Role(mygoui.RoleNone)
			})
			return
		}
		pdf := chromeControl(c, "PDF Preview", !enabled).Size(pdfWidth, 23).Gap(4).Padding(0, 4).ClipX().
			AccessSelected(false).OnClick(app.togglePDFPreview)
		pdfInk, pdfIconSize := labelInk, float32(15)
		if app.blackout {
			pdf.Border(1, blackoutPreviewPDFEdge).Radius(6)
			if !pdf.Hovered() && !pdf.Pressed() {
				pdf.Background(blackoutPreviewPDFSurface)
				if blackoutInk {
					pdfInk = blackoutPreviewPDFInk
					if blackoutDocumentTools {
						pdfInk = mygoui.RGB(110, 110, 110)
						pdfIconSize = 16.5
					}
				}
			}
		}
		pdf.Children(func() {
			chromeIconColor(c, "doc", pdfIconSize, pdfInk)
			mygoui.Text(c, "PDF Preview").FontSize(11).SingleLine().MinWidth(0).Shrink(1).
				Ellipsis("…").TextColor(pdfInk).Role(mygoui.RoleNone)
		})
	})
	if app.displayedMode() == splitMode {
		// The toolbar is inset from the outer pane, whose edges extend beyond
		// the web viewport by half a point on the left and one point on the right.
		bar.Padding(0, 7, 0, 8.5)
	}
}

func (app *workspaceWindow) previewZoomControls(c *mygoui.Context, enabled bool) {
	percentage := normalizedPreviewZoom(app.previewZoom)
	background := c.Theme().Surface
	iconInk, textInk := c.Theme().TextMuted, c.Theme().Text
	if app.blackout {
		background = blackoutPreviewZoomSurface
		if !c.Preferences().HighContrast {
			iconInk, textInk = blackoutPreviewZoomInk, blackoutPreviewZoomInk
		}
	}
	mygoui.Row(c).Height(18).Shrink(0).Padding(0, 5).Gap(6.25).MarginEnd(5).
		AlignItems(mygoui.Center).Radius(5).Background(background).Children(func() {
		previewZoomButton(c, "minus.magnifyingglass", "Zoom Out", "Zoom out (min 25%)",
			paneAction(enabled && percentage > previewZoomMin, func() {
				app.setPreviewZoom(normalizedPreviewZoom(app.previewZoom) - previewZoomStep)
			}), iconInk)
		zoom := fmt.Sprintf("%d%%", percentage)
		chromeControl(c, zoom, !enabled, "Click to reset to 100%").Size(29.5, 13).
			OnClick(func() { app.setPreviewZoom(previewZoomDefault) }).Children(func() {
			mygoui.Text(c, zoom).FontSize(10).TextColor(textInk).FontFeatures("tnum").SingleLine().Shrink(0)
		})
		previewZoomButton(c, "plus.magnifyingglass", "Zoom In", "Zoom in (max 400%)",
			paneAction(enabled && percentage < previewZoomMax, func() {
				app.setPreviewZoom(normalizedPreviewZoom(app.previewZoom) + previewZoomStep)
			}), iconInk)
	})
}

func previewZoomButton(c *mygoui.Context, symbol, label, help string, action func(), ink mygoui.Color) {
	const width, height, symbolSize = 10.5, 11, 15
	chromeControl(c, label, action == nil, help).Size(width, height).OnClick(action).Children(func() {
		// The SF Symbol box includes padding beyond its visible edges.
		chromeIconColor(c, symbol, symbolSize, ink).Translate((width-symbolSize)/2, 0).IgnoreInput(true)
	})
}

// EV-UI-043: use the observed label widths at each nominal preview slot.
// Interpolation keeps the controls resizing with the continuous pane layout.
func previewLabelWidths(paneWidth float32) (markdown, pdf float32) {
	samples := [...]struct{ pane, markdown, pdf float32 }{
		{336, 62, 57.5}, {356, 71.5, 68}, {361, 76.5, 68},
		{366, 77.5, 72}, {371, 82.5, 72}, {376, 81.5, 78},
		{396, 89, 87}, {401, 89, 87}, {406, 89, 93.5},
		{411, 89, 100.5}, {416, 89, 100.5}, {436, 89, 107},
	}
	previous := samples[0]
	for _, next := range samples[1:] {
		if paneWidth <= next.pane {
			fraction := max(0, paneWidth-previous.pane) / (next.pane - previous.pane)
			return previous.markdown + (next.markdown-previous.markdown)*fraction,
				previous.pdf + (next.pdf-previous.pdf)*fraction
		}
		previous = next
	}
	return previous.markdown, previous.pdf
}

func (app *workspaceWindow) paneToolWidths(c *mygoui.Context) (source, preview float32) {
	width, _ := c.Size()
	split := app.displayedMode() == splitMode
	vault, outline := app.visibleSidebars()
	panes := app.documentPaneWidths(width, vault, outline, split)
	if split {
		return panes.source, panes.preview
	}
	if vault {
		width -= vaultPanelWidth
	}
	if outline {
		width -= outlinePanelWidth
	}
	width = max(0, width-panes.leading-panes.trailing)
	return width, width
}
