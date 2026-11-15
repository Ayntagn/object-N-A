package ui

import (
	"path"
	"strconv"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func (app *workspaceWindow) drawBacklinks(c *mygoui.Context) {
	if app.index.data == nil {
		return
	}
	links := app.index.data.WikiBacklinks(app.currentVaultPath())
	if len(links) == 0 {
		return
	}
	colors := app.vaultSectionColors(c)
	expanded := app.preferences.VaultBacklinksExpanded
	count := strconv.Itoa(len(links))
	mygoui.Column(c.Key("backlinks")).Margin(0, 3, 0, 4).Shrink(0).Children(func() {
		mygoui.Box(c).FillWidth().Height(1).Shrink(0).Background(colors.border.Alpha(.2))
		vaultSectionHeading(c, "Backlinks", count, "link", expanded, app.modalOpen, colors, app.toggleBacklinks)
		if !expanded {
			return
		}
		mygoui.Scroll(c.Key("rows")).FillWidth().MaxHeight(240).Shrink(0).Label("Backlinks").Children(func() {
			mygoui.Column(c).FillWidth().Shrink(0).Children(func() {
				for _, entry := range links {
					app.drawBacklinkRow(c, entry, colors)
				}
			})
		})
	})
}

func (app *workspaceWindow) toggleBacklinks() {
	app.preferences.VaultBacklinksExpanded = !app.preferences.VaultBacklinksExpanded
	app.writePreferences()
	for _, window := range app.documentWindows() {
		if window.win != nil && window != app {
			window.win.Invalidate()
		}
	}
}

func (app *workspaceWindow) drawBacklinkRow(c *mygoui.Context, entry workspace.Entry, colors vaultSectionColors) {
	title := app.noteLabel(entry.Name)
	folder := path.Dir(entry.Path)
	label, height := title, float32(24)
	if folder != "." {
		label, height = title+", "+folder, 39
	}
	row := mygoui.ButtonBase(c.Key(entry.Path)).FillWidth().Height(height).Shrink(0).Padding(4, 12).
		Gap(8).Justify(mygoui.Start).AlignItems(mygoui.Center).Label(label).Disabled(app.modalOpen).
		OnClick(func() { app.openDocument(entry.Path) }).
		ContextMenu(func(menu *mygoui.Menu) { app.vaultFileMenu(menu, entry.Path) })
	if row.Hovered() {
		row.Background(colors.secondary.Alpha(.08))
	}
	symbol, ink := app.vaultFileRowIcon(c, entry.Path, colors, false)
	colors.accent = ink
	row.Children(func() {
		drawVaultFileRowContents(c, title, folder, symbol, colors, app.vaultFileRowEdited(entry.Path))
	})
}

func (app *workspaceWindow) vaultFileRowEdited(relative string) bool {
	for _, window := range app.documentWindows() {
		if window.root == app.root {
			if tab := window.findVaultTab(relative); tab != nil && tab.dirty() {
				return true
			}
		}
	}
	return false
}
