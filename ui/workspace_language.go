package ui

import (
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type workspaceLanguage string

const (
	languageEnglish workspaceLanguage = "en"
	languageChinese workspaceLanguage = "zh-CN"
)

func (language workspaceLanguage) text(english, chinese string) string {
	if language == languageChinese {
		return chinese
	}
	return english
}

func (app *workspaceWindow) interfaceLanguage() workspaceLanguage {
	if app != nil && app.preferences != nil && app.preferences.InterfaceLanguage == languageChinese {
		return languageChinese
	}
	return languageEnglish
}

func (app *workspaceWindow) tr(english, chinese string) string {
	return app.interfaceLanguage().text(english, chinese)
}

func (app *workspaceWindow) interfaceLanguageSettings(c *mygoui.Context) {
	failed := mygoui.Local(c.Root(), "interface language save failed", func() bool { return false })
	label := app.tr("Settings & Help Language", "设置与帮助语言")
	app.settingsCard(c.Key("interface language")).Padding(8, 16).Children(func() {
		settingsRow(c, label, "", "globe", 42, func() {
			languages := [...]workspaceLanguage{languageEnglish, languageChinese}
			selected := 0
			if app.interfaceLanguage() == languageChinese {
				selected = 1
			}
			app.settingsSegments(c, label, []string{"English", "简体中文"}, 164, selected, func(index int) {
				*failed = app.setInterfaceLanguage(languages[index]) != nil
			})
		})
		if *failed {
			mygoui.Text(c, app.tr("Could not save the interface language.", "无法保存界面语言。")).
				Role(mygoui.RoleAlert).TextColor(c.Theme().Danger).Wrap()
		}
	})
}

func (app *workspaceWindow) setInterfaceLanguage(language workspaceLanguage) error {
	if app.interfaceLanguage() == language {
		return nil
	}
	prefs := app.preferences
	previous := prefs.InterfaceLanguage
	prefs.InterfaceLanguage = language
	if err := prefs.write(); err != nil {
		prefs.InterfaceLanguage = previous
		return err
	}
	for _, window := range app.documentWindows() {
		if window.win != nil && !window.win.IsDestroyed() {
			menuWasActive := window.menu != nil && mygo.App.Menu() == window.menu
			window.buildMenu()
			if menuWasActive {
				mygo.App.SetMenu(window.menu)
			}
			window.win.Invalidate()
		}
		if window.settings != nil && !window.settings.IsDestroyed() {
			window.settings.SetTitle(window.settingsWindowTitle())
			window.settings.Invalidate()
		}
		window.refreshShortcutsLanguage()
	}
	if app.application != nil {
		app.application.refreshHelpLanguage()
	}
	return nil
}

func (app *workspaceWindow) settingsWindowTitle() string {
	return applicationName + " " + app.tr("Settings", "设置")
}
