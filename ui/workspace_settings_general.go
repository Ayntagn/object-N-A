package ui

import (
	"strconv"

	mygoui "github.com/egoist/mygo/ui"
)

func (app *workspaceWindow) generalSettings(c *mygoui.Context) {
	tr := app.tr
	theme := *c.Theme()
	if app.blackout && !c.Preferences().HighContrast {
		theme.TextMuted = mygoui.Hex("#c1c2cc")
	}
	mygoui.Themed(c, &theme).FillWidth().Gap(20).Children(func() {
		app.interfaceLanguageSettings(c)
		app.settingsSection(c, tr("Editor", "编辑器"), "text.cursor", func() {
			app.settingsCard(c).Padding(10, 16).Children(func() {
				app.generalEditorGrid(c)
			})
		})
		app.settingsSection(c, tr("Vault", "资料库"), "building.columns", func() {
			app.settingsCard(c).Padding(16).Children(func() {
				app.generalVaultSettings(c)
			})
		})
		view := func() {
			app.settingsSection(c, tr("View", "视图"), "rectangle.split.3x1", func() {
				app.settingsCard(c).Padding(8, 16).Children(func() {
					app.generalViewSettings(c)
				})
			})
		}
		startup := func() {
			app.settingsSection(c, tr("Startup", "启动"), "power", func() {
				app.settingsCard(c).Padding(8, 16).Children(func() {
					app.settingsSwitch(c, tr("Welcome Screen", "欢迎页"), tr("Show on launch", "启动时显示"), "hand.wave", 60, &app.preferences.ShowWelcomeOnLaunch)
					app.settingsSwitch(c, tr("Launch Animation", "启动动画"), tr("Play on launch", "启动时播放"), "sparkles", 60, &app.preferences.ShowSplashOnLaunch)
					app.drawAutoSaveStatus(c)
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
func (app *workspaceWindow) drawAutoSaveStatus(c *mygoui.Context) {
	theme := c.Theme()
	mygoui.Row(c.Key("Auto-Save")).FillWidth().MinHeight(60).Gap(16).AlignItems(mygoui.Center).Children(func() {
		mygoui.Box(c).Size(20, 20).Shrink(0).Center().Role(mygoui.RoleNone).Children(func() {
			chromeIconColor(c, "doc.badge.clock", 17, theme.TextMuted)
		})
		mygoui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			mygoui.Text(c, app.tr("Auto-Save", "自动保存")).FontSize(13).FontWeight(600)
			mygoui.Text(c, app.tr("Always enabled", "始终启用")).FontSize(11).TextColor(theme.TextMuted)
		})
		mygoui.Text(c, app.tr("ALWAYS ON", "始终开启")).FontSize(10).FontWeight(650).TextColor(theme.TextMuted).
			SingleLine().Padding(4, 7).Radius(5).Background(theme.SurfaceHover).Shrink(0)
	})
}

// EV-UI-022: Supported Editor options retain their positions in the two-column grid.
func (app *workspaceWindow) generalEditorGrid(c *mygoui.Context) {
	p := app.preferences
	tr := app.tr
	rows := [][2]func(){
		{
			func() {
				app.settingsSwitch(c, tr("Line Numbers", "行号"), tr("Show in editor", "在编辑器中显示"), "number", 60, &p.LineNumbers)
			},
			func() {
				size := app.sourceFontSize()
				settingsRow(c, tr("Font Size", "字号"), strconv.Itoa(size)+" pt", "textformat.size", 60, func() {
					app.settingsFontStepper(c, size)
				})
			},
		},
		{
			func() { app.sourceLineSpacingSettings(c) },
			func() {
				settingsMenuRow(c, tr("Editor Font", "编辑器字体"), app.editorFontLabel(), "textformat", func(menu *mygoui.Menu) {
					app.editorFontMenuWithSystem(menu, app.tr(systemMonoLabel, "系统等宽字体"))
				})
			},
		},
		{
			func() {
				settingsMenuRow(c, tr("Color Theme", "配色主题"), app.sourceThemeSettingsLabel(sourceThemeForID(p.EditorTheme)), "paintbrush", func(menu *mygoui.Menu) {
					app.drawSourceThemeMenu(menu, true)
				})
			},
			func() { app.sourceSpellSettings(c) },
		},
		{
			func() {
				app.settingsSwitch(c, tr("Collapse Hex Colors", "折叠十六进制颜色"), tr("Show as color blocks", "显示为色块"), "paintpalette", 60, &p.CollapseHexColors)
			},
			func() {
				app.settingsSwitch(c, tr("Number Figures", "图片编号"), tr("Prefix image captions with Figure 1., Figure 2.", "在图注前加上图 1、图 2 等编号"), "photo.badge.plus", 60, &p.NumberFigures)
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
	reset := app.tr("Reset Font Size", "重置字号")
	mygoui.Row(c).Size(90, 26).Shrink(0).Border(1, c.Theme().Border).Radius(6).Children(func() {
		chromeButton(c.Key("Decrease Font Size"), "minus", app.tr("Decrease Font Size", "减小字号"), func() {
			app.changeToolbarFontSize(-1)
		}).Size(28, 26).Disabled(size <= sourceFontMenuMin)
		mygoui.ButtonBase(c.Key("Reset Font Size")).Size(34, 26).Center().Label(reset).
			Tooltip(reset).OnClick(func() { app.changeFontSize(0) }).Children(func() {
			mygoui.Text(c, strconv.Itoa(size)).FontSize(13).Bold().Role(mygoui.RoleNone)
		})
		chromeButton(c.Key("Increase Font Size"), "plus", app.tr("Increase Font Size", "增大字号"), func() {
			app.changeToolbarFontSize(1)
		}).Size(28, 26).Disabled(size >= sourceFontMax)
	})
}

func (app *workspaceWindow) generalVaultSettings(c *mygoui.Context) {
	p := app.preferences
	tr := app.tr
	app.drawVaultOpenPreferences(c)
	newDocument := tr("Blank doc; saves into the open vault", "创建空白文档，保存时写入当前资料库")
	if p.NewInVault {
		newDocument = tr("Created directly in the open vault", "直接在当前资料库中创建")
	}
	app.vaultSettingsSegments(c, tr("New Document", "新建文档"), newDocument, "doc.badge.plus", 138, p.NewInVault,
		[]string{tr("Blank", "空白"), tr("In vault", "资料库内")}, func(value int) {
			p.NewInVault = value == 1
			app.preferencesChanged()
		})
	sidebarFiles := tr("Supported file types only", "只显示支持的文件类型")
	if p.VaultShowAllFiles {
		sidebarFiles = tr("All files - unsupported ones open in their default app", "显示全部文件，其他类型使用默认应用打开")
	}
	app.vaultSettingsSegments(c, tr("Show in Sidebar", "侧栏显示"), sidebarFiles, "doc.on.doc", 178, p.VaultShowAllFiles,
		[]string{tr("Supported", "支持的类型"), tr("All", "全部")}, func(value int) {
			p.VaultShowAllFiles = value == 1
			app.preferencesChanged()
			for _, window := range app.documentWindows() {
				if window.vault != nil {
					window.refreshVault()
				}
			}
		})
	extensions := tr("Hidden on .md notes, shown on everything else", "隐藏笔记的 .md 扩展名，显示其他扩展名")
	if p.ShowExtension {
		extensions = tr("Show file extensions", "显示文件扩展名")
	}
	app.vaultSettingsSegments(c, tr("File Extensions", "文件扩展名"), extensions, "textformat", 159, p.ShowExtension,
		[]string{tr("Hide .md", "隐藏 .md"), tr("Show All", "全部显示")}, func(value int) {
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
	tr := app.tr
	defaultView := tr("Default View", "默认视图")
	mygoui.Column(c).FillWidth().MinHeight(60).Gap(5).Padding(6, 0, 6, 36).Children(func() {
		mygoui.Text(c, defaultView).FontSize(13).FontWeight(600).Role(mygoui.RoleNone)
		mygoui.Row(c).FillWidth().Children(func() {
			app.settingsSegments(c, defaultView, []string{tr("Editor", "编辑"), tr("Split", "分栏"), tr("Preview", "预览")}, 218, p.DefaultView, func(value int) {
				p.DefaultView = value
				app.preferencesChanged()
			})
		})
	})
	app.settingsSwitch(c, tr("Scroll Sync", "同步滚动"), tr("Sync editor & preview", "同步编辑器和预览的滚动位置"), "arrow.up.arrow.down", 60, &p.ScrollSyncEnabled)
	app.settingsSwitch(c, tr("Scroll Past End", "滚过文末"), tr("Extra space at bottom", "在文末保留额外空间"), "arrow.down.to.line", 60, &p.ScrollPastEnd)
}
