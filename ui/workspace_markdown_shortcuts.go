package ui

import (
	"runtime"

	"cyberwriter/internal/core/mdshortcuts"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// MyGo performs all placeholder edits and Tab traversal. This guard gives an
// active expansion priority over any new rule matching the text in its stops.
type markdownShortcutSession struct {
	code          *editor.State
	start, end    int
	documentSize  int
	stop, stopMax int
	undoVersion   uint64
	original      editor.Selection
}

func (app *workspaceWindow) applySourceMarkdownShortcut(event mygoui.InputEvent) bool {
	if app.preferences == nil {
		return false
	}
	config := &app.preferences.MarkdownShortcuts
	if !config.Enabled || app.draft == nil || !app.sourceEditing() ||
		app.document.Path != "" && !workspace.IsMarkdownPath(app.document.Path) {
		app.markdownShortcutSession = markdownShortcutSession{}
		return false
	}
	session := &app.markdownShortcutSession
	if session.code == app.code && session.undoVersion != 0 && session.undoVersion == app.draft.Version() &&
		markdownShortcutUndo(event) {
		// The snippet was applied to a temporary selection over the trigger.
		// Undo should restore the user's caret or original selection.
		selection := session.original
		*session = markdownShortcutSession{}
		if app.code.Run(editor.Undo) {
			app.code.SetSelections(selection)
			return true
		}
	}
	if session.active(app.code, event) {
		return false
	}
	var typed string
	switch {
	case event.Kind == mygoui.InputText:
		typed = event.Text
		if typed == "" {
			return false
		}
	case event.Kind == mygoui.InputKeyDown && event.Key == mygoui.KeyTab && event.Mods == 0:
	default:
		return false
	}
	if len(config.Rules) == 0 {
		return false
	}
	selections := app.code.Selections()
	if len(selections) != 1 {
		return false
	}
	program, err := config.Program()
	if err != nil {
		return false
	}
	original := selections[0]
	match, ok := program.Match(mdshortcuts.Input{
		Markdown: app.workingText(), Start: original.Start(), End: original.End(), Typed: typed,
	})
	if !ok {
		return false
	}
	snippet, err := editor.ParseSnippet(match.Template)
	if err != nil {
		return false
	}
	beforeVersion := app.draft.Version()
	app.code.Select(match.Start, match.End)
	if !app.code.InsertSnippet(snippet) {
		app.code.SetSelections(original)
		return false
	}
	*session = markdownShortcutSession{
		code: app.code, start: match.Start, end: match.Start + len(snippet.Text()),
		documentSize: app.draft.Len(), stopMax: match.TabStops, original: original,
	}
	if version := app.draft.Version(); version != beforeVersion {
		session.undoVersion = version
	}
	return true
}

func markdownShortcutUndo(event mygoui.InputEvent) bool {
	if event.Kind == mygoui.InputCommand {
		return event.Text == "undo"
	}
	modifier := mygoui.Ctrl
	if runtime.GOOS == "darwin" {
		modifier = mygoui.Cmd
	}
	return event.Kind == mygoui.InputKeyDown && event.Key == mygoui.KeyZ && event.Mods == modifier
}

func (session *markdownShortcutSession) active(code *editor.State, event mygoui.InputEvent) bool {
	if session.code == nil {
		return false
	}
	if session.stopMax == 0 {
		return false
	}
	if session.code != code || event.Kind == mygoui.InputPointerDown ||
		event.Kind == mygoui.InputKeyDown && (event.Key == mygoui.KeyEscape || event.Key == mygoui.KeyZ && event.Mods&(mygoui.Cmd|mygoui.Ctrl) != 0) ||
		event.Kind == mygoui.InputCommand && (event.Text == "undo" || event.Text == "redo") {
		*session = markdownShortcutSession{}
		return false
	}
	size := code.Buffer.Len()
	session.end += size - session.documentSize
	session.documentSize = size
	for _, selected := range code.Selections() {
		if selected.Start() < session.start || selected.End() > session.end {
			*session = markdownShortcutSession{}
			return false
		}
	}
	if event.Kind == mygoui.InputKeyDown && event.Key == mygoui.KeyTab {
		switch event.Mods {
		case 0:
			session.stop++
			if session.stop == session.stopMax {
				session.stopMax = 0
			}
		case mygoui.Shift:
			session.stop = max(0, session.stop-1)
		}
	}
	return true
}
