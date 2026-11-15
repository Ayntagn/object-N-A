package ui

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type shortcutEntry struct {
	label, chinese, keys string
	hint, hintChinese    string
}

const pendingShortcut = "This shortcut is pending in the replica."

// EV-UI-029 and EV-UI-065 preserve the archived reference text and order.
var shortcutSections = [...]struct {
	name, chinese string
	entries       []shortcutEntry
}{
	{"FILE", "文件", []shortcutEntry{
		{"New document", "新建文档", "⌘ N", "", ""},
		{"Open document", "打开文档", "⌘ O", "", ""},
		{"Quick Switcher - jump to file", "快速切换到文件", "⌘ P", "", ""},
		{"Save", "保存", "⌘ S", "", ""},
		{"Save As...", "另存为…", "⌘ ⇧ S",
			"The File menu uses ⌘ ⌥ ⇧ S for Save As and ⌘ ⇧ S for Duplicate.",
			"文件菜单中，另存为使用 ⌘ ⌥ ⇧ S，创建副本使用 ⌘ ⇧ S。"},
		{"Export...", "导出…", "⌘ E",
			"The File menu uses ⌘ E for Quick Export PDF and ⌘ ⇧ E for Export with options.",
			"文件菜单中，快速导出 PDF 使用 ⌘ E，带选项的导出使用 ⌘ ⇧ E。"},
	}},
	{"VIEW", "视图", []shortcutEntry{
		{"Editor only", "仅编辑器", "⌘ 1", "", ""},
		{"Split view", "分栏视图", "⌘ 2", "", ""},
		{"Preview only", "仅预览", "⌘ 3", "", ""},
		{"WYSIWYG visual editing", "所见即所得编辑", "⌘ 4", "", ""},
		{"Toggle preview", "切换预览", "⌘ \\", "", ""},
		{"Toggle PDF preview", "切换 PDF 预览", "⌘ ⇧ P", "", ""},
		{"Toggle outline sidebar", "切换大纲侧栏", "⌘ ⇧ O", "", ""},
		{"Toggle focus mode", "切换专注模式", "⌘ ⇧ F", "", ""},
		{"Increase font size", "增大字号", "⌘ +", "", ""},
		{"Decrease font size", "减小字号", "⌘ -", "", ""},
		{"Reset font size", "重设字号", "⌘ 0", "", ""},
	}},
	{"FORMATTING", "格式", []shortcutEntry{
		{"Bold", "粗体", "⌘ B", "", ""},
		{"Italic", "斜体", "⌘ I", "", ""},
		{"Inline code", "行内代码", "⌘ \u0060", "", ""},
		{"Insert link", "插入链接", "⌘ K", "", ""},
		{"Insert Table of Contents", "插入目录", "⌘ ⇧ T", "", ""},
	}},
	{"EDITING", "编辑", []shortcutEntry{
		{"Find", "查找", "⌘ F", "", ""},
		{"Find and Replace", "查找与替换", "⌘ ⌥ F", "", ""},
		{"Undo", "撤销", "⌘ Z", "", ""},
		{"Redo", "重做", "⌘ ⇧ Z", "", ""},
		{"Fold selection", "折叠选区", "⌘ .", "", ""},
	}},
	{"HEADINGS", "标题", []shortcutEntry{
		{"Heading 1", "一级标题", "⌘ ⌃ 1", "", ""},
		{"Heading 2", "二级标题", "⌘ ⌃ 2", "", ""},
		{"Heading 3", "三级标题", "⌘ ⌃ 3", "", ""},
	}},
	{"AI WORKSPACE", "AI 工作区", []shortcutEntry{
		{"Toggle AI panel", "切换 AI 面板", "⌘ ⇧ A", pendingShortcut, "此快捷键仍待实现。"},
		{"AI Quick Actions on selection", "对选区使用 AI 快速操作", "⌘ J", "", ""},
	}},
	{"DIAGRAMS", "图表", []shortcutEntry{
		{"Diagrams menu (16 Mermaid types)", "图表菜单（16 种 Mermaid 图表）", "Insert +", "", ""},
	}},
	{"GENERAL", "通用", []shortcutEntry{
		{"Settings", "设置", "⌘ ,", "", ""},
		{"Command Palette", "命令面板", "⌘ /", "", ""},
		{"Show shortcuts", "显示快捷键", "⌘ ⇧ /", "", ""},
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
		Title: app.tr("Keyboard Shortcuts", "键盘快捷键"), Width: 340, Height: 700, UseContentSize: true,
		Parent: app.win, Modal: true, Hidden: true, DisableResize: true,
		TitleBarStyle: mygo.TitleBarHidden, HideWindowButtons: true,
		Content: mygoui.View(func(c *mygoui.Context) {
			app.shortcutSheetView(c, func() { sheet.Close() })
		}),
	})
	if sheet == nil {
		app.modalOpen = false
		return
	}
	app.shortcutsSheet = sheet
	sheet.OnClosed(func() {
		app.shortcutsSheet = nil
		app.modalOpen = false
		if !app.win.IsDestroyed() {
			app.win.Invalidate()
		}
	})
	sheet.Show()
	app.win.Invalidate()
}

func (app *workspaceWindow) refreshShortcutsLanguage() {
	if sheet := app.shortcutsSheet; sheet != nil && !sheet.IsDestroyed() {
		sheet.SetTitle(app.tr("Keyboard Shortcuts", "键盘快捷键"))
		sheet.Invalidate()
	}
}

func (app *workspaceWindow) shortcutSheetView(c *mygoui.Context, closeSheet func()) {
	app.applyAppearance(c)
	theme := c.Theme()
	language := app.interfaceLanguage()
	c.OnWindowShortcut(0, mygoui.KeyEscape, closeSheet)
	mygoui.Column(c).Fill().Background(theme.Surface).Children(func() {
		mygoui.Row(c).FillWidth().Height(52).Shrink(0).Padding(0, 17.5, 0, 16).
			AlignItems(mygoui.Center).Children(func() {
			mygoui.Text(c, language.text("Keyboard Shortcuts", "键盘快捷键")).
				Role(mygoui.RoleHeading).Level(1).FontSize(13).FontWeight(600)
			mygoui.Spacer(c)
			chromeButtonIcon(c, "xmark.circle.fill", language.text("Close", "关闭"), closeSheet, 17, theme.Text).Size(17, 17)
		})
		mygoui.Row(c).FillWidth().Height(1).Shrink(0).Background(theme.Border)
		mygoui.Scroll(c).Label(language.text("Shortcut categories", "快捷键分类")).
			Grow(1).MinHeight(0).FillWidth().Focusable().Children(func() {
			mygoui.Column(c).FillWidth().Padding(20).Gap(18).Children(func() {
				for _, section := range shortcutSections {
					mygoui.Column(c.Key(section.name)).FillWidth().Gap(8).Children(func() {
						mygoui.Text(c, language.text(section.name, section.chinese)).
							Role(mygoui.RoleHeading).Level(2).FontSize(11).FontWeight(600).
							LetterSpacing(0.5).Height(14).AlignSelf(mygoui.Start)
						mygoui.Column(c).FillWidth().Gap(6).Children(func() {
							for _, entry := range section.entries {
								drawShortcutEntry(c, entry, language)
							}
						})
					})
				}
			})
		})
	})
}

func drawShortcutEntry(c *mygoui.Context, entry shortcutEntry, language workspaceLanguage) {
	row := mygoui.Row(c.Key(entry.label)).FillWidth().Height(23).AlignItems(mygoui.Center)
	if entry.hint != "" {
		row.Tooltip(shortcutLabel(language.text(entry.hint, entry.hintChinese)))
	}
	row.Children(func() {
		mygoui.Text(c, language.text(entry.label, entry.chinese)).FontSize(13).MinWidth(0).SingleLine().Ellipsis("…").Shrink(1)
		mygoui.Spacer(c).MinWidth(0)
		mygoui.Row(c).Height(23).Shrink(0).AlignItems(mygoui.Center).Padding(0, 8).
			Radius(4).Background(c.Theme().Text.Alpha(0.1)).Children(func() {
			mygoui.Text(c, shortcutLabel(entry.keys)).Font("monospace").FontSize(12).SingleLine()
		})
	})
}
