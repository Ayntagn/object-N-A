package ui

import (
	"strconv"

	mygoui "github.com/egoist/mygo/ui"
)

func (app *workspaceWindow) generalSettings(c *mygoui.Context) {
	theme := *c.Theme()
	if app.blackout && !c.Preferences().HighContrast {
		theme.TextMuted = mygoui.Hex("#c1c2cc")
	}
	mygoui.Themed(c, &theme).FillWidth().Gap(20).Children(func() {
		app.settingsSection(c, "Editor", "text.cursor", func() {
			app.settingsCard(c).Padding(10, 16).Children(func() {
				app.generalEditorGrid(c)
			})
		})
		app.settingsSection(c, "Vault", "building.columns", func() {
			app.settingsCard(c).Padding(16).Children(func() {
				app.generalVaultSettings(c)
			})
		})
		view := func() {
			app.settingsSection(c, "View", "rectangle.split.3x1", func() {
				app.settingsCard(c).Padding(8, 16).Children(func() {
					app.generalViewSettings(c)
				})
			})
		}
		startup := func() {
			app.settingsSection(c, "Startup", "power", func() {
				app.settingsCard(c).Padding(8, 16).Children(func() {
					app.settingsSwitch(c, "Welcome Screen", "Show on launch", "hand.wave", 60, &app.preferences.ShowWelcomeOnLaunch)
					drawAutoSaveStatus(c)
				})
			})
		}
		width, _ := c.Size()
		if width < 735 {
			view()
			startup()
		} else {
			mygoui.Row(c.Key("View and Startup")).FillWidth().Gap(16).AlignItems(mygoui.Start).Children(func() {
				mygoui.Column(c).Basis(0).Grow(1).MinWidth(0).Children(view)
				mygoui.Column(c).Basis(0).Grow(1).MinWidth(0).Children(startup)
			})
		}
		// Keep the editor extension beneath the native sections so its preview
		// can expand without moving the Vault and View controls.
		app.settingsCard(c.Key("Typewriter settings")).Padding(10, 16).Children(func() {
			app.typewriterSettings(c)
		})
	})
}

// EV-UI-247: Auto-Save is a read-only status in the Startup card.
func drawAutoSaveStatus(c *mygoui.Context) {
	theme := c.Theme()
	mygoui.Row(c.Key("Auto-Save")).FillWidth().MinHeight(60).Gap(16).AlignItems(mygoui.Center).Children(func() {
		mygoui.Box(c).Size(20, 20).Shrink(0).Center().Role(mygoui.RoleNone).Children(func() {
			chromeIconColor(c, "doc.badge.clock", 17, theme.TextMuted)
		})
		mygoui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			mygoui.Text(c, "Auto-Save").FontSize(13).FontWeight(600)
			mygoui.Text(c, "Always enabled").FontSize(11).TextColor(theme.TextMuted)
		})
		mygoui.Text(c, "ALWAYS ON").FontSize(10).FontWeight(650).TextColor(theme.TextMuted).
			SingleLine().Padding(4, 7).Radius(5).Background(theme.SurfaceHover).Shrink(0)
	})
}

// EV-UI-022: Supported Editor options retain their positions in the two-column grid.
func (app *workspaceWindow) generalEditorGrid(c *mygoui.Context) {
	p := app.preferences
	rows := [][2]func(){
		{
			func() { app.settingsSwitch(c, "Line Numbers", "Show in editor", "number", 60, &p.LineNumbers) },
			func() {
				size := app.sourceFontSize()
				settingsRow(c, "Font Size", strconv.Itoa(size)+" pt", "textformat.size", 60, func() {
					app.settingsFontStepper(c, size)
				})
			},
		},
		{
			func() { app.sourceLineSpacingSettings(c) },
			func() { settingsMenuRow(c, "Editor Font", app.editorFontLabel(), "textformat", app.editorFontMenu) },
		},
		{
			func() {
				settingsMenuRow(c, "Color Theme", sourceThemeForID(p.EditorTheme).label, "paintbrush", app.sourceThemeMenu)
			},
			func() { app.sourceSpellSettings(c) },
		},
		{
			func() {
				app.settingsSwitch(c, "Collapse Hex Colors", "Show as color blocks", "paintpalette", 60, &p.CollapseHexColors)
			},
			func() {
				app.settingsSwitch(c, "Number Figures", "Prefix image captions with Figure 1., Figure 2.", "photo.badge.plus", 60, &p.NumberFigures)
			},
		},
	}
	width, _ := c.Size()
	for index, row := range rows {
		if width < 640 {
			for _, draw := range row {
				if draw != nil {
					draw()
				}
			}
			continue
		}
		mygoui.Row(c.Key(strconv.Itoa(index))).FillWidth().Gap(8).Children(func() {
			for _, draw := range row {
				cell := mygoui.Column(c).Basis(0).Grow(1).MinWidth(0)
				if draw != nil {
					cell.Children(draw)
				}
			}
		})
	}
}

func settingsMenuRow(c *mygoui.Context, title, value, symbol string, menu func(*mygoui.Menu)) {
	settingsMenuRowWithDetail(c, title, value, value, symbol, menu)
}

func settingsMenuRowWithDetail(c *mygoui.Context, title, value, detail, symbol string, menu func(*mygoui.Menu)) {
	settingsRow(c, title, detail, symbol, 60, func() {
		theme := c.Theme()
		mygoui.ButtonBase(c.Key(title)).Role(mygoui.RoleMenuButton).Label(title).Value(value).Menu(menu).
			Size(144, 26).MinWidth(72).Shrink(1).AlignItems(mygoui.Center).Justify(mygoui.SpaceBetween).
			Padding(0, 10).Gap(6).Radius(6).Border(1, theme.Border).Background(theme.Surface).
			Children(func() {
				mygoui.Text(c, value).FontSize(12).FontWeight(600).SingleLine().Ellipsis("…").
					Shrink(1).MinWidth(0).Role(mygoui.RoleNone)
				chromeIconColor(c, "chevron.up.chevron.down", 9, theme.TextMuted).Shrink(0)
			})
	})
}

func (app *workspaceWindow) settingsFontStepper(c *mygoui.Context, size int) {
	mygoui.Row(c).Size(90, 26).Shrink(0).Border(1, c.Theme().Border).Radius(6).Children(func() {
		chromeButton(c.Key("Decrease Font Size"), "minus", "Decrease Font Size", func() {
			app.changeToolbarFontSize(-1)
		}).Size(28, 26).Disabled(size <= sourceFontMenuMin)
		mygoui.ButtonBase(c.Key("Reset Font Size")).Size(34, 26).Center().Label("Reset Font Size").
			Tooltip("Reset Font Size").OnClick(func() { app.changeFontSize(0) }).Children(func() {
			mygoui.Text(c, strconv.Itoa(size)).FontSize(13).Bold().Role(mygoui.RoleNone)
		})
		chromeButton(c.Key("Increase Font Size"), "plus", "Increase Font Size", func() {
			app.changeToolbarFontSize(1)
		}).Size(28, 26).Disabled(size >= sourceFontMax)
	})
}

func (app *workspaceWindow) generalVaultSettings(c *mygoui.Context) {
	p := app.preferences
	app.drawVaultOpenPreferences(c)
	newDocument := "Blank doc; saves into the open vault"
	if p.NewInVault {
		newDocument = "Create a note in the open vault"
	}
	app.vaultSettingsSegments(c, "New Document", newDocument, "doc.badge.plus", 138, p.NewInVault,
		[]string{"Blank", "In vault"}, func(value int) {
			p.NewInVault = value == 1
			app.preferencesChanged()
		})
	sidebarFiles := "Supported file types only"
	if p.VaultShowAllFiles {
		sidebarFiles = "All files - unsupported ones open in their default app"
	}
	app.vaultSettingsSegments(c, "Show in Sidebar", sidebarFiles, "doc.on.doc", 178, p.VaultShowAllFiles,
		[]string{"Supported", "All"}, func(value int) {
			p.VaultShowAllFiles = value == 1
			app.preferencesChanged()
			for _, window := range app.documentWindows() {
				if window.vault != nil {
					window.refreshVault()
				}
			}
		})
	extensions := "Hidden on .md notes, shown on everything else"
	if p.ShowExtension {
		extensions = "Show file extensions"
	}
	app.vaultSettingsSegments(c, "File Extensions", extensions, "textformat", 159, p.ShowExtension,
		[]string{"Hide .md", "Show All"}, func(value int) {
			p.ShowExtension = value == 1
			app.preferencesChanged()
		})
}

func (app *workspaceWindow) vaultSettingsSegments(c *mygoui.Context, title, detail, symbol string, width float32, second bool, values []string, change func(int)) {
	selected := 0
	if second {
		selected = 1
	}
	settingsRow(c, title, detail, symbol, 48, func() {
		mygoui.Row(c).Width(220).MinWidth(0).Shrink(1).Justify(mygoui.Center).Children(func() {
			app.settingsSegments(c, title, values, width, selected, change)
		})
	})
}

func (app *workspaceWindow) generalViewSettings(c *mygoui.Context) {
	p := app.preferences
	mygoui.Column(c).FillWidth().MinHeight(60).Gap(5).Padding(6, 0, 6, 36).Children(func() {
		mygoui.Text(c, "Default View").FontSize(13).FontWeight(600).Role(mygoui.RoleNone)
		mygoui.Row(c).FillWidth().Children(func() {
			app.settingsSegments(c, "Default View", []string{"Editor", "Split", "Preview"}, 218, p.DefaultView, func(value int) {
				p.DefaultView = value
				app.preferencesChanged()
			})
		})
	})
	app.settingsSwitch(c, "Scroll Sync", "Sync editor & preview", "arrow.up.arrow.down", 60, &p.ScrollSyncEnabled)
	app.settingsSwitch(c, "Scroll Past End", "Extra space at bottom", "arrow.down.to.line", 60, &p.ScrollPastEnd)
}
