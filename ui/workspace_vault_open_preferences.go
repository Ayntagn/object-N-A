package ui

import (
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-022/247. Vault has its own blue drawing switch and 120 pt file segments.
func (app *workspaceWindow) drawVaultOpenPreferences(c *mygoui.Context) {
	prefs := app.preferences
	tr := app.tr
	sidebar := tr("Sidebar Click - Open File", "从侧栏打开文件")
	selected := 0
	if prefs.SidebarClickNewWindow {
		selected = 1
	}
	modifier := "Ctrl"
	if runtime.GOOS == "darwin" {
		modifier = "⌘"
	}
	description := fmt.Sprintf(tr("Hold %s to open in a new window", "按住 %s 可在新窗口打开"), modifier)
	settingsRow(c, sidebar, description, "rectangle.on.rectangle", 48, func() {
		app.vaultOpenSegments(c, sidebar, selected)
	})
	drawings := tr("Drawings Open Separately", "单独打开绘图")
	drawingsDetail := tr("Drawings open in their own window/tab - and notes opened from a drawing's links do too, so neither ever replaces the other",
		"绘图以及从绘图链接打开的笔记会各自在独立的窗口或标签页中打开。")
	settingsRow(c, drawings, drawingsDetail, "pencil.and.outline", 48, func() {
		if !app.blackout || c.Preferences().HighContrast {
			mygoui.Switch(c.Key(drawings), &prefs.DrawingsOpenSeparately).Size(54, 24).Shrink(0).
				Label(drawings).OnChange(app.preferencesChanged)
			return
		}
		switcher := mygoui.SwitchBase(c.Key(drawings), &prefs.DrawingsOpenSeparately).Size(54, 24).Shrink(0).
			Label(drawings).FocusRing(true).OnChange(app.preferencesChanged)
		var target float32
		if prefs.DrawingsOpenSeparately {
			target = 1
		}
		position := switcher.Animate("drawing switch thumb", target, 140*time.Millisecond)
		switcher.Radius(12).Background(c.Theme().SurfaceHover.Mix(mygoui.Hex("#428CF7"), position)).
			Draw(func(p *mygoui.Painter, rect mygoui.Rect) {
				thumb := mygoui.Rect{X: rect.X + 2 + (rect.W-36)*position, Y: rect.Y + 2, W: 32, H: 20}
				p.Fill(thumb, mygoui.Hex("#E8F0FF"), 10)
			})
	})
}

func (app *workspaceWindow) vaultOpenSegments(c *mygoui.Context, label string, selected int) {
	theme := c.Theme()
	selection, selectedText, track := theme.Accent, theme.AccentText, theme.SurfaceHover
	if app.blackout && !c.Preferences().HighContrast {
		selection, selectedText, track = mygoui.Hex("#428CF7"), mygoui.Hex("#FFFFFF"), mygoui.Hex("#303032")
	}
	// The contiguous pair shares the narrower Vault controls' center.
	mygoui.RadioGroup(c.Key(label), func() {
		for index, title := range [...]string{app.tr("Replace current", "当前窗口"), app.tr("New window", "新窗口")} {
			button := mygoui.RadioBase(c.Key(title), &selected, index).Height(24).Basis(0).Grow(1).MinWidth(0).
				Center().Radius(6).Padding(0, 4).Label(title).OnChange(func() {
				app.preferences.SidebarClickNewWindow = selected == 1
				app.preferencesChanged()
			})
			ink := theme.Text
			if selected == index {
				button.Background(selection)
				ink = selectedText
			}
			button.Children(func() {
				mygoui.Text(c, title).FontSize(12).SingleLine().Ellipsis("…").TextColor(ink).Role(mygoui.RoleNone)
			})
		}
	}).Row().Gap(0).Size(240, 24).Shrink(1).MinWidth(0).Margin(0, -10, 0, 0).
		Radius(6).Background(track).Label(label)
}

// Sidebar file clicks and drawing links share the ordinary document policy.
func (app *workspaceWindow) openVaultFileByPreference(relative string, mods mygoui.Modifiers, fromDrawing bool) {
	app.openVaultFileFollowing(relative, mods, fromDrawing, nil)
}

func (app *workspaceWindow) openVaultFileFollowing(relative string, mods mygoui.Modifiers, fromDrawing bool, opened func(*workspaceWindow)) {
	if app.modalOpen || app.vault == nil {
		return
	}
	var finish func()
	if opened != nil {
		finish = func() { opened(app) }
	}
	if app.multiFile.active {
		app.openVaultRowFollowing(relative, true, finish)
		return
	}
	drawing := isDrawingPath(relative)
	editable := drawing || workspace.IsMarkdownPath(relative) || workspace.IsSourceTextPath(relative)
	separate := app.preferences.SidebarClickNewWindow || mods&mygoui.Cmd != 0 ||
		(app.preferences.DrawingsOpenSeparately && (drawing || fromDrawing))
	if editable && separate {
		if target := app.openVaultDocumentWindow(relative, drawing); target != nil && opened != nil {
			opened(target)
		}
		return
	}
	app.openVaultRowFollowing(relative, false, finish)
}

func (app *workspaceWindow) openVaultDocumentWindow(relative string, drawing bool) *workspaceWindow {
	absolute := filepath.Join(app.root, filepath.FromSlash(relative))
	for _, window := range app.documentWindows() {
		if window.win != nil && window.win.IsDestroyed() {
			continue
		}
		for _, tab := range window.tabs {
			if absolute != window.absoluteDocument(tab) {
				continue
			}
			if !window.modalOpen && window.workspaceTab != tab {
				window.activateTab(tab)
				if window.win != nil {
					window.win.Invalidate()
				}
			}
			if window.win != nil {
				window.win.Focus()
			}
			return window
		}
	}
	if app.application == nil || app.win == nil {
		return nil
	}

	tab := app.blankTab()
	tab.mode = app.preferences.DefaultView
	if drawing {
		draw, err := loadWorkspaceDrawing(app.vault, relative)
		if err != nil {
			app.problem = fmt.Sprintf("Could not open %q. %v", relative, err)
			return nil
		}
		app.setTabDrawing(tab, draw)
	} else {
		doc, err := app.vault.ReadTextDocument(relative)
		if err != nil {
			app.problem = fmt.Sprintf("Could not open %q. %v", relative, err)
			return nil
		}
		app.setTabDocument(tab, doc)
		if workspace.IsSourceTextPath(relative) {
			tab.mode = splitMode
		}
	}
	next := app.application.newWindow(tab, app)
	next.win.Show()
	app.application.activate(next)
	next.persistSession()
	return next
}
