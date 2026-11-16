package ui

import (
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const systemSourceThemeID = "system"

type sourceThemeChoice struct {
	id, label string
	palette   *editor.Theme
	codeTheme string
}

type sourceSyntaxStyle struct {
	color        uint32
	bold, italic bool
}

var sourceThemesByID = func() map[string]sourceThemeChoice {
	themes := make(map[string]sourceThemeChoice, len(darkSourceThemes)+len(lightSourceThemes)+1)
	themes[systemSourceThemeID] = sourceThemeChoice{id: systemSourceThemeID, label: "System"}
	for _, group := range [][]sourceThemeChoice{darkSourceThemes, lightSourceThemes} {
		for _, choice := range group {
			themes[choice.id] = choice
		}
	}
	return themes
}()

func sourceThemeForID(id string) sourceThemeChoice {
	if theme, found := sourceThemesByID[id]; found {
		return theme
	}
	return sourceThemesByID[systemSourceThemeID]
}

func (app *workspaceWindow) sourceThemeMenu(m *mygoui.Menu) {
	app.drawSourceThemeMenu(m, false)
}

func (app *workspaceWindow) sourceThemeSettingsLabel(choice sourceThemeChoice) string {
	if choice.id == systemSourceThemeID {
		return app.tr(choice.label, "系统")
	}
	return choice.label
}

func (app *workspaceWindow) drawSourceThemeMenu(m *mygoui.Menu, localize bool) {
	current := sourceThemeForID(app.preferences.EditorTheme).id
	add := func(choice sourceThemeChoice) {
		label := choice.label
		if localize {
			label = app.sourceThemeSettingsLabel(choice)
		}
		if m.Item(label).Checked(choice.id == current).Chosen() {
			app.preferences.EditorTheme = choice.id
			app.preferencesChanged()
		}
	}
	add(sourceThemeForID(systemSourceThemeID))
	m.Separator()
	dark, light := "Dark", "Light"
	if localize {
		dark, light = app.tr(dark, "深色"), app.tr(light, "浅色")
	}
	m.Item(dark).Disabled(true)
	for _, choice := range darkSourceThemes {
		add(choice)
	}
	m.Item(light).Disabled(true)
	for _, choice := range lightSourceThemes {
		add(choice)
	}
}

func (app *workspaceWindow) sourceEditorTheme(tab *workspaceTab, dark bool) *editor.Theme {
	choice := sourceThemeForID(app.preferences.EditorTheme)
	if choice.id == systemSourceThemeID {
		switch {
		case app.blackout:
			choice = sourceThemesByID["blackout"]
		case app.appAppearance() == appearancePLATO:
			choice = sourceThemesByID["plato"]
		case dark:
			choice = sourceThemesByID["nabu-dark"]
		default:
			choice = lightSystemSourceTheme
		}
	}
	if choice.palette == nil {
		// The original Odsibian key has no matching CSS. Highlightr keeps
		// the last successfully loaded stylesheet in this editor.
		if tab.sourceThemeFallback != nil {
			return tab.sourceThemeFallback
		}
		return initialSourceThemeFallback
	}
	tab.sourceThemeFallback = choice.palette
	if choice.codeTheme != "" {
		tab.sourceThemeFallback = sourceThemesByID[choice.codeTheme].palette
	}
	return choice.palette
}

func sourceThemeFromPalette(background, text uint32, styles map[editor.TokenKind]sourceSyntaxStyle) *editor.Theme {
	rgb := func(color uint32) mygoui.Color {
		return mygoui.RGB(uint8(color>>16), uint8(color>>8), uint8(color))
	}
	foreground := rgb(text)
	theme := &editor.Theme{
		Background: rgb(background), Text: foreground,
		LineNumber: foreground.Alpha(0.55), ActiveLineNumber: foreground,
		UprightFencedCode: true,
	}
	for kind := range theme.Tokens {
		theme.Tokens[kind].Color = foreground
	}
	for kind, style := range styles {
		theme.Tokens[kind] = editor.TokenStyle{Color: rgb(style.color), Bold: style.bold, Italic: style.italic}
	}
	return theme
}
