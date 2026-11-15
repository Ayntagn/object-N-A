package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"cyberwriter/internal/core/templates"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

type workspaceTemplates struct {
	entries       []templates.Entry
	saveMenu      *mygo.MenuItem
	pendingNotice string
}

const templateNoticeID = "saved template"

func templateDirectory() (string, error) {
	preferences, err := preferenceFile()
	return filepath.Join(filepath.Dir(preferences), "Templates"), err
}

func (app *workspaceWindow) refreshTemplateLibrary() {
	dir, err := templateDirectory()
	if err != nil {
		return
	}
	entries, err := templates.List(dir)
	if err != nil || slices.Equal(app.templates.entries, entries) {
		return
	}
	app.templates.entries = entries
	app.buildMenu()
	if mygo.FocusedWindow() == app.win {
		mygo.App.SetMenu(app.menu)
	}
}

func (app *workspaceWindow) templateMenuItems() []*mygo.MenuItem {
	var items []*mygo.MenuItem
	appendBuiltin := func(builtin templates.Builtin) {
		items = append(items, &mygo.MenuItem{Label: builtin.Label, Click: app.menuCommand(func() {
			app.newFromTemplate(builtin.Markdown(time.Now()))
		})})
	}
	for _, builtin := range templates.General {
		appendBuiltin(builtin)
	}
	items = append(items, mygo.Separator(), &mygo.MenuItem{Label: "Vault Templates", Disabled: true})
	for _, builtin := range templates.Vault {
		appendBuiltin(builtin)
	}
	if len(app.templates.entries) != 0 {
		items = append(items, mygo.Separator(), &mygo.MenuItem{Label: "My Templates", Disabled: true})
		for _, entry := range app.templates.entries {
			items = append(items, &mygo.MenuItem{Label: entry.Title, Click: app.menuCommand(func() {
				app.openCustomTemplate(entry)
			})})
		}
	}
	return append(items, mygo.Separator(), &mygo.MenuItem{
		Label: "Open Templates Folder", Click: app.menuCommand(app.openTemplateDirectory),
	})
}

func (app *workspaceWindow) openCustomTemplate(entry templates.Entry) {
	dir, err := templateDirectory()
	if err == nil {
		var markdown string
		markdown, err = templates.Load(dir, entry.Filename)
		if err == nil {
			app.newFromTemplate(markdown)
			return
		}
	}
	app.problem = fmt.Sprintf("Could not open template %q. %v", entry.Title, err)
	app.refreshTemplateLibrary()
}

func (app *workspaceWindow) openTemplateDirectory() {
	dir, err := templateDirectory()
	if err == nil {
		err = os.MkdirAll(dir, 0o700)
	}
	if err == nil {
		err = mygo.Shell.OpenPath(dir)
	}
	if err != nil {
		app.problem = fmt.Sprintf("Could not open the templates folder. %v", err)
	}
}

// EV-UI-116: File starts a new document in the current tab group. The
// command palette and GLOW instead insert into the active document.
func (app *workspaceWindow) newFromTemplate(markdown string) {
	tab := app.availableTab()
	tab.mode = app.preferences.DefaultView
	app.setTabDocument(tab, workspace.Document{})
	tab.templateTitle = templates.Heading(markdown)
	if strings.Contains(markdown, "\r\n") {
		tab.draft.SetLineEnding("\r\n")
	}
	tab.draft.Set(markdown)
	tab.code.Select(0, 0)
	app.activateTab(tab)
}

func (app *workspaceWindow) insertTemplate(builtin templates.Builtin) {
	if !app.insertMarkdownAtSelection(builtin.Markdown(time.Now())) {
		app.problem = "This template requires source editing."
	}
}

func (app *workspaceWindow) saveAsTemplate() {
	if app.draft == nil {
		return
	}
	app.wys.sync(app)
	dir, err := templateDirectory()
	var markdown strings.Builder
	if err == nil {
		_, err = app.draft.WriteTo(&markdown)
	}
	var title string
	if err == nil {
		title, err = templates.Save(dir, markdown.String())
	}
	if err != nil {
		app.problem = fmt.Sprintf("Could not save the template. %v", err)
		return
	}
	if title == "" {
		return
	}
	app.problem = ""
	for _, window := range app.documentWindows() {
		window.refreshTemplateLibrary()
	}
	app.templates.pendingNotice = "Saved template: " + title
}

// The native information banner covers the top of the document tools.
func (app *workspaceWindow) drawTemplateNotice(c *mygoui.Context) {
	if notice := app.templates.pendingNotice; notice != "" {
		app.templates.pendingNotice = ""
		c.AddToast(mygoui.Toast{ID: templateNoticeID, Title: notice, Data: app.workspaceTab})
	}
	theme := c.Theme()
	surface, border, accent, closeInk := theme.Surface, theme.Border, theme.Accent, theme.TextMuted
	if app.blackout && !c.Preferences().HighContrast {
		surface, border, accent = mygoui.RGB(24, 24, 28), mygoui.RGB(57, 63, 76), mygoui.RGB(60, 141, 248)
		closeInk = mygoui.RGB(104, 104, 106)
	}
	vault, outline := app.visibleSidebars()
	left := float32(48.5)
	if vault {
		left += vaultPanelWidth
	}
	if outline {
		left += outlinePanelWidth
	}
	top := float32(titleBarHeight + 3.5)
	if app.focusMode {
		top = 36 + 3.5
	} else if len(app.tabs) > 1 {
		top += tabBarHeight + tabBarGap
	}
	width, _ := c.Size()
	width = min(500, width-left-12)
	mygoui.ToastViewportBase(c, func(_ mygoui.Element, toasts []mygoui.Toast) {
		for _, notice := range toasts {
			if notice.Data != app.workspaceTab || width < 80 {
				continue
			}
			toast := mygoui.ToastBase(c, notice)
			toast.Root.Row().Absolute().Left(left).Top(top).Size(width, 48).
				AlignItems(mygoui.Center).Padding(0, 16, 0, 17).Gap(13.5).
				Background(surface).Border(1, border).Radius(8).Children(func() {
				mygoui.Box(c.Key("template info")).Size(20, 20).Shrink(0).Radius(10).Background(accent).
					Role(mygoui.RoleImage).Label("Info").Center().Children(func() {
					mygoui.Text(c, "ℹ").Role(mygoui.RoleNone).FontSize(15).Bold().TextColor(surface)
				})
				mygoui.Text(c, notice.Title).FontSize(13).MinWidth(0).Shrink(1).
					SingleLine().Ellipsis("…")
				mygoui.Spacer(c)
				toast.CloseButton().Label("Close").KeepFocus().Size(18, 18).Center().Shrink(0).Children(func() {
					sfsymbols.Symbol(c, "xmark.circle.fill").Size(18).Color(closeInk).
						Fallback(chromeIconFallbacks["xmark.circle.fill"]).Role(mygoui.RoleNone)
				})
			})
		}
	})
}
