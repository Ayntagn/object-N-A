package ui

import (
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

const welcomeWidth, welcomeHeight = 660, 692

type workspaceWelcomeDocument struct{ Path, Title, Folder string }

type workspaceWelcome struct {
	win, returnTo *mygo.Window
	workspace     *workspaceWindow
	documents     []workspaceWelcomeDocument
	afterClose    func()
}

func (app *workspaceWindow) showWelcome() {
	application := app.application
	if application == nil || app.modalOpen {
		return
	}
	welcome := &application.welcome
	if welcome.win != nil && !welcome.win.IsDestroyed() {
		if mygo.FocusedWindow() != welcome.win {
			welcome.returnTo = app.win
		}
		welcome.workspace, welcome.documents = app, app.welcomeDocuments()
		welcome.win.Focus()
		welcome.win.Invalidate()
		return
	}
	*welcome = workspaceWelcome{workspace: app, returnTo: app.win, documents: app.welcomeDocuments()}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title: "Welcome Back", Width: welcomeWidth, Height: welcomeHeight, Hidden: true,
		TitleBarStyle: mygo.TitleBarHiddenInset, TitleBarHeight: 32,
		TrafficLightPosition: &mygo.Point{X: 8, Y: 8},
		DisableResize:        true, DisableMinimize: true, DisableMaximize: true, DisableFullScreen: true,
		Content: mygoui.View(func(c *mygoui.Context) {
			workspace := welcome.workspace
			workspace.applyAppearance(c)
			closeWelcome := func() { welcome.dismiss(nil) }
			c.OnWindowShortcut(mygoui.Cmd, mygoui.KeyW, closeWelcome)
			drawWelcomeContent(c, workspace.blackout, welcome.documents, func(path string) {
				welcome.dismiss(func() { workspace.openWelcomeDocument(path) })
			}, func() {
				welcome.dismiss(func() {
					workspace.newDocument()
					if workspace.win != nil && !workspace.win.IsDestroyed() {
						workspace.win.Show()
					}
				})
			}, closeWelcome)
		}),
	})
	welcome.win = win
	if runtime.GOOS == "darwin" {
		win.SetTitle("")
	}
	win.OnClosed(func() {
		if welcome.win != win {
			return
		}
		next, action := welcome.returnTo, welcome.afterClose
		*welcome = workspaceWelcome{}
		if action != nil {
			action()
			return
		}
		if next == nil || next.IsDestroyed() {
			if active := application.activeWorkspace(); active != nil {
				next = active.win
			}
		}
		if next != nil && !next.IsDestroyed() {
			next.Show()
		}
	})
	win.Show()
}

func (welcome *workspaceWelcome) dismiss(action func()) {
	if welcome.win != nil && !welcome.win.IsDestroyed() {
		welcome.afterClose = action
		welcome.win.Close()
	}
}

func (app *workspaceWindow) closeWelcomeIfFocused() bool {
	if app.application != nil {
		welcome := &app.application.welcome
		if welcome.win != nil && mygo.FocusedWindow() == welcome.win {
			welcome.dismiss(nil)
			return true
		}
	}
	return false
}

// The native AX window is 660 × 692 pt. The list scrolls independently of
// the fixed header and actions.
func drawWelcomeContent(c *mygoui.Context, blackout bool, documents []workspaceWelcomeDocument, openDocument func(string), newDocument, continueToWorkspace func()) {
	theme := c.Theme()
	background, titleInk, secondaryInk := theme.Background, theme.Text, theme.TextMuted
	footerInk, actionFill := theme.TextMuted.Alpha(.65), theme.Surface
	accent, primaryFill, primaryInk := theme.Accent, theme.Accent, theme.AccentText
	if blackout && !c.Preferences().HighContrast {
		secondaryInk, footerInk = mygoui.Hex("#c4c6d0"), mygoui.Hex("#66666d")
		actionFill, accent = mygoui.Hex("#101011"), mygoui.Hex("#0095ff")
		primaryFill, primaryInk = mygoui.Hex("#a9c7ff"), mygoui.Hex("#05284c")
	}
	mygoui.Column(c).Fill().Background(background).Children(func() {
		mygoui.Box(c).Height(32).FillWidth().Shrink(0).DragWindow()
		mygoui.Row(c).FillWidth().Height(48).Shrink(0).Margin(20, 0, 16).Padding(0, 24).
			Gap(12).AlignItems(mygoui.Center).Children(func() {
			mygoui.Box(c.Key("Welcome logo")).Size(48, 48).Shrink(0).Center().Role(mygoui.RoleImage).Label(applicationName).Children(func() {
				mygoui.Box(c).Size(40, 40).Center().Radius(10).Background(mygoui.Hex("#1b1309")).
					Border(1, mygoui.Hex("#31261a")).Children(func() {
					sfsymbols.Symbol(c, "hurricane").Size(31).Natural().Weight(sfsymbols.Semibold).
						Color(mygoui.Hex("#f9a21b")).Fallback(chromeIconFallbacks["hurricane"]).Role(mygoui.RoleNone)
				})
			})
			mygoui.Column(c).Gap(2).Children(func() {
				mygoui.Text(c, "Welcome Back").FontSize(20).FixedLineHeight(26).Bold().TextColor(titleInk)
				mygoui.Text(c, "The markdown editor of the future").FontSize(12).FixedLineHeight(15).TextColor(secondaryInk)
			})
		})
		mygoui.Scroll(c.Key("Welcome recent documents")).FillWidth().Grow(1).MinHeight(0).Label("Recent Documents").Children(func() {
			if len(documents) == 0 {
				return
			}
			mygoui.Column(c).FillWidth().Padding(12, 16, 0).Gap(2).Children(func() {
				mygoui.Text(c, "RECENT DOCUMENTS").FontSize(12).FixedLineHeight(16).FontWeight(700).
					TextColor(secondaryInk).Margin(0, 8, 6)
				for _, document := range documents {
					drawWelcomeDocument(c, document, titleInk, secondaryInk, accent, openDocument)
				}
			})
		})
		mygoui.Row(c).FillWidth().Height(43).Shrink(0).Margin(12, 0, 0).Padding(0, 24).Gap(12).Children(func() {
			for _, action := range [...]struct {
				label     string
				fill, ink mygoui.Color
				click     func()
			}{
				{"New Document", actionFill, secondaryInk, newDocument},
				{"Continue", primaryFill, primaryInk, continueToWorkspace},
			} {
				mygoui.ButtonBase(c.Key(action.label)).Grow(1).MinWidth(0).FillHeight().Radius(8).
					Background(action.fill).Label(action.label).OnClick(action.click).Children(func() {
					mygoui.Text(c, action.label).FontSize(12).FontWeight(600).TextColor(action.ink).Role(mygoui.RoleNone)
				})
			}
		})
		mygoui.Text(c, "Help > Show Welcome  ·  Help > What's New").Height(13).Shrink(0).FontSize(10).FixedLineHeight(13).
			TextColor(footerInk).AlignSelf(mygoui.Center).Margin(12, 0)
	})
}

func drawWelcomeDocument(c *mygoui.Context, document workspaceWelcomeDocument, ink, secondary, accent mygoui.Color, openDocument func(string)) {
	const folderFontSize = 11
	const textWidth = welcomeWidth - 32 - 24 - 32 - 24 - 14
	label := document.Title + ", " + document.Folder
	row := mygoui.ButtonBase(c.Key(document.Path)).FillWidth().Height(52).Shrink(0).MinWidth(0).
		Padding(0, 12).Gap(12).Justify(mygoui.Start).Radius(7).Label(label).
		OnClick(func() { openDocument(document.Path) })
	if row.Hovered() {
		row.Background(c.Theme().SurfaceHover)
	}
	row.Children(func() {
		mygoui.Box(c).Size(32, 32).Shrink(0).Center().Children(func() {
			chromeIconColor(c, "doc.text.fill", 27, accent)
		})
		mygoui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			mygoui.Text(c, document.Title).FontSize(13).FixedLineHeight(16).FontWeight(600).
				SingleLine().Ellipsis("…").TextColor(ink).Role(mygoui.RoleNone)
			// Fit the middle ellipsis with the folder's font so the last directory stays visible.
			var folder string
			const available = textWidth - 2
			for budget := float32(available); ; {
				folder = truncateFileTitle(c, document.Folder, budget*13/folderFontSize)
				measured, _ := c.MeasureText(0, mygoui.Span{Text: folder, Size: folderFontSize})
				if measured <= available || folder == "…" {
					break
				}
				budget -= max(1, measured-available)
			}
			mygoui.Text(c, folder).FontSize(folderFontSize).FixedLineHeight(14).
				SingleLine().TextColor(secondary).Role(mygoui.RoleNone)
		})
		chromeIconColor(c, "chevron.right", 14, secondary.Alpha(.5))
	})
}
