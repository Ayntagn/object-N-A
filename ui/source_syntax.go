package ui

import (
	"runtime"

	"github.com/egoist/mygo/ui/editor"
)

var sourceMarkdownLanguage = editor.LanguageFor("note.md")

var sourceMarkdownWithoutHighlighting = func() *editor.Language {
	language := *sourceMarkdownLanguage
	language.Tokenizer = nil
	language.Highlighter = nil
	return &language
}()

func (app *workspaceWindow) sourceLanguage() *editor.Language {
	if app.preferences.DisableSyntaxHighlighting {
		return sourceMarkdownWithoutHighlighting
	}
	if app.preferences.CollapseHexColors {
		return sourceMarkdownCollapsedHex
	}
	return sourceMarkdownExpandedHex
}

// EV-UI-058: the toolbar and the View menu write the same preference.
func (app *workspaceWindow) toggleSourceHighlighting() {
	app.preferences.DisableSyntaxHighlighting = !app.preferences.DisableSyntaxHighlighting
	if item := app.sourceHighlightMenu; item != nil {
		item.SetLabel(app.sourceHighlightMenuLabel())
	}
	app.preferencesChanged()
}

func (app *workspaceWindow) sourceHighlightMenuLabel() string {
	if app.preferences.DisableSyntaxHighlighting {
		return "Enable Syntax Highlighting"
	}
	return "Disable Syntax Highlighting"
}

func (app *workspaceWindow) sourceHighlightTooltip() string {
	shortcut := "Ctrl+Shift+H"
	if runtime.GOOS == "darwin" {
		shortcut = "⌘⇧H"
	}
	action := "Disable"
	if app.preferences.DisableSyntaxHighlighting {
		action = "Enable"
	}
	return action + " syntax highlighting (" + shortcut + ")"
}
