package ui

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type shortcutEntry struct {
	label, keys, hint string
}

const pendingShortcut = "This shortcut is pending in the replica."

// EV-UI-029 and EV-UI-065 preserve the archived reference text and order.
var shortcutSections = [...]struct {
	name    string
	entries []shortcutEntry
}{
	{"FILE", []shortcutEntry{
		{"New document", "⌘ N", ""},
		{"Open document", "⌘ O", ""},
		{"Quick Switcher - jump to file", "⌘ P", ""},
		{"Save", "⌘ S", ""},
		{"Save As...", "⌘ ⇧ S", "The File menu uses ⌘ ⌥ ⇧ S for Save As and ⌘ ⇧ S for Duplicate."},
		{"Export...", "⌘ E", "The File menu uses ⌘ E for Quick Export PDF and ⌘ ⇧ E for Export with options."},
	}},
	{"VIEW", []shortcutEntry{
		{"Editor only", "⌘ 1", ""},
		{"Split view", "⌘ 2", ""},
		{"Preview only", "⌘ 3", ""},
		{"WYSIWYG visual editing", "⌘ 4", ""},
		{"Toggle preview", "⌘ \\", ""},
		{"Toggle PDF preview", "⌘ ⇧ P", ""},
		{"Toggle outline sidebar", "⌘ ⇧ O", ""},
		{"Toggle focus mode", "⌘ ⇧ F", ""},
		{"Increase font size", "⌘ +", ""},
		{"Decrease font size", "⌘ -", ""},
		{"Reset font size", "⌘ 0", ""},
	}},
	{"FORMATTING", []shortcutEntry{
		{"Bold", "⌘ B", ""},
		{"Italic", "⌘ I", ""},
		{"Inline code", "⌘ \u0060", ""},
		{"Insert link", "⌘ K", ""},
		{"Insert Table of Contents", "⌘ ⇧ T", ""},
	}},
	{"EDITING", []shortcutEntry{
		{"Find", "⌘ F", ""},
		{"Find and Replace", "⌘ ⌥ F", ""},
		{"Undo", "⌘ Z", ""},
		{"Redo", "⌘ ⇧ Z", ""},
		{"Fold selection", "⌘ .", ""},
	}},
	{"HEADINGS", []shortcutEntry{
		{"Heading 1", "⌘ ⌃ 1", ""},
		{"Heading 2", "⌘ ⌃ 2", ""},
		{"Heading 3", "⌘ ⌃ 3", ""},
	}},
	{"AI WORKSPACE", []shortcutEntry{
		{"Toggle AI panel", "⌘ ⇧ A", pendingShortcut},
		{"AI Quick Actions on selection", "⌘ J", ""},
	}},
	{"DIAGRAMS", []shortcutEntry{
		{"Diagrams menu (16 Mermaid types)", "Insert +", ""},
	}},
	{"GENERAL", []shortcutEntry{
		{"Settings", "⌘ ,", ""},
		{"Command Palette", "⌘ /", ""},
		{"Show shortcuts", "⌘ ⇧ /", ""},
	}},
}

var portableShortcutLabels = strings.NewReplacer("⌘ ⌃ ", "Ctrl+Shift+", "⌘ ", "Ctrl+", "⌥ ", "Alt+", "⇧ ", "Shift+")

func shortcutLabel(text string) string {
	if runtime.GOOS == "darwin" {
		return text
	}
	return portableShortcutLabels.Replace(text)
}

func headingShortcut(level int) string {
	if level > 3 {
		return ""
	}
	if runtime.GOOS == "darwin" {
		return "CmdOrCtrl+Ctrl+" + strconv.Itoa(level)
	}
	return "CmdOrCtrl+Shift+" + strconv.Itoa(level)
}

func (app *workspaceWindow) showShortcuts() {
	if app.modalOpen || app.win == nil || app.win.IsDestroyed() {
		return
	}
	app.modalOpen = true
	var sheet *mygo.Window
	sheet = mygo.NewWindow(mygo.WindowOptions{
		Title: "Keyboard Shortcuts", Width: 340, Height: 700, UseContentSize: true,
		Parent: app.win, Modal: true, Hidden: true, DisableResize: true,
		TitleBarStyle: mygo.TitleBarHidden, HideWindowButtons: true,
		Content: mygoui.View(func(c *mygoui.Context) {
			app.shortcutSheetView(c, func() { sheet.Close() })
		}),
	})
	sheet.OnClosed(func() {
		app.modalOpen = false
		if !app.win.IsDestroyed() {
			app.win.Invalidate()
		}
	})
	sheet.Show()
	app.win.Invalidate()
}

func (app *workspaceWindow) shortcutSheetView(c *mygoui.Context, closeSheet func()) {
	app.applyAppearance(c)
	theme := c.Theme()
	c.OnWindowShortcut(0, mygoui.KeyEscape, closeSheet)
	mygoui.Column(c).Fill().Background(theme.Surface).Children(func() {
		mygoui.Row(c).FillWidth().Height(52).Shrink(0).Padding(0, 17.5, 0, 16).
			AlignItems(mygoui.Center).Children(func() {
			mygoui.Text(c, "Keyboard Shortcuts").Role(mygoui.RoleHeading).Level(1).FontSize(13).FontWeight(600)
			mygoui.Spacer(c)
			chromeButtonIcon(c, "xmark.circle.fill", "Close", closeSheet, 17, theme.Text).Size(17, 17)
		})
		mygoui.Row(c).FillWidth().Height(1).Shrink(0).Background(theme.Border)
		mygoui.Scroll(c).Label("Shortcut categories").Grow(1).MinHeight(0).FillWidth().Focusable().Children(func() {
			mygoui.Column(c).FillWidth().Padding(20).Gap(18).Children(func() {
				for _, section := range shortcutSections {
					mygoui.Column(c.Key(section.name)).FillWidth().Gap(8).Children(func() {
						mygoui.Text(c, section.name).Role(mygoui.RoleHeading).Level(2).FontSize(11).FontWeight(600).
							LetterSpacing(0.5).Height(14).AlignSelf(mygoui.Start)
						mygoui.Column(c).FillWidth().Gap(6).Children(func() {
							for _, entry := range section.entries {
								drawShortcutEntry(c, entry)
							}
						})
					})
				}
			})
		})
	})
}

func drawShortcutEntry(c *mygoui.Context, entry shortcutEntry) {
	row := mygoui.Row(c.Key(entry.label)).FillWidth().Height(23).AlignItems(mygoui.Center)
	if entry.hint != "" {
		row.Tooltip(shortcutLabel(entry.hint))
	}
	row.Children(func() {
		mygoui.Text(c, entry.label).FontSize(13).MinWidth(0).SingleLine().Ellipsis("…").Shrink(1)
		mygoui.Spacer(c).MinWidth(0)
		mygoui.Row(c).Height(23).Shrink(0).AlignItems(mygoui.Center).Padding(0, 8).
			Radius(4).Background(c.Theme().Text.Alpha(0.1)).Children(func() {
			mygoui.Text(c, shortcutLabel(entry.keys)).Font("monospace").FontSize(12).SingleLine()
		})
	})
}
