package ui

import (
	"fmt"
	"strings"

	"cyberwriter/internal/core/templates"
	"cyberwriter/ui/wysiwyg"
	"github.com/egoist/mygo/ui/editor"
)

type commandPaletteKey struct {
	section, name string
}

// The catalog also carries commands whose workspace feature is still pending.
func (app *workspaceWindow) commandPaletteActions() map[commandPaletteKey]func() {
	actions := map[commandPaletteKey]func(){
		{"VIEW", "Toggle Snippets"}:         app.toggleSnippets,
		{"SETTINGS", "Increase Font Size"}:  func() { app.changeFontSize(1) },
		{"SETTINGS", "Decrease Font Size"}:  func() { app.changeFontSize(-1) },
		{"SETTINGS", "Reset Font Size"}:     func() { app.changeFontSize(0) },
		{"SETTINGS", "Toggle Line Numbers"}: app.toggleLineNumbers,
		{"SETTINGS", "Toggle Scroll Sync"}:  app.toggleScrollSync,
		{"SETTINGS", "Custom Stylesheet"}:   func() { app.openFeatureGuide("custom-css", app.win) },
		{"HELP", "Keyboard Shortcuts"}:      app.showShortcuts,
		{"BLOCKS", "API History"}:           app.showAPIHistory,
	}
	if app.draft != nil {
		actions[commandPaletteKey{"VIEW", "Editor Only"}] = func() { app.setMode(sourceMode) }
		actions[commandPaletteKey{"VIEW", "Split View"}] = func() { app.setMode(splitMode) }
		actions[commandPaletteKey{"VIEW", "Preview Only"}] = func() { app.setMode(previewMode) }
		actions[commandPaletteKey{"VIEW", "WYSIWYG"}] = func() { app.setMode(wysiwygMode) }
		actions[commandPaletteKey{"VIEW", "Focus Mode"}] = app.toggleFocus
		actions[commandPaletteKey{"VIEW", "Toggle Outline"}] = app.toggleOutline
		actions[commandPaletteKey{"VIEW", "Toggle PDF Preview"}] = app.togglePDFPreview
		actions[commandPaletteKey{"EXPORT", "Quick Export PDF"}] = app.quickExportPDF
		actions[commandPaletteKey{"EXPORT", "Export with Options"}] = app.showExport
		actions[commandPaletteKey{"EXPORT", "Save as Template"}] = app.saveAsTemplate
		for _, builtin := range templates.General {
			actions[commandPaletteKey{"TEMPLATES", builtin.Label}] = func() { app.insertTemplate(builtin) }
		}
		actions[commandPaletteKey{"SETTINGS", "Find"}] = func() { app.showFind(false) }
		actions[commandPaletteKey{"SETTINGS", "Find & Replace"}] = func() { app.showFind(true) }
	}
	if app.canPresent() {
		actions[commandPaletteKey{"VIEW", "Play as Slides"}] = func() { app.openPresentation(false) }
		actions[commandPaletteKey{"VIEW", "Flash Cards"}] = func() { app.openPresentation(true) }
	}
	if app.canOpenQuickActions() {
		actions[commandPaletteKey{"AI", "AI Quick Actions"}] = app.openQuickActions
	}
	if app.vault != nil {
		actions[commandPaletteKey{"VIEW", "Toggle GLOW Terminal"}] = app.toggleGlow
		actions[commandPaletteKey{"EXPORT", "New Drawing"}] = app.newDrawing
		actions[commandPaletteKey{"EXPORT", "Map Vault"}] = app.mapVault
		actions[commandPaletteKey{"EXPORT", "Vault Health Report"}] = app.generateVaultHealth
		actions[commandPaletteKey{"EXPORT", "New Folder"}] = func() { app.createFolder(".", false) }
	}
	app.addPaletteEditorActions(actions)
	return actions
}

func (app *workspaceWindow) addPaletteEditorActions(actions map[commandPaletteKey]func()) {
	source := app.sourceEditing()
	rich := app.mode == wysiwygMode && app.wys.editable(app)
	if source {
		for level := 1; level <= 6; level++ {
			actions[commandPaletteKey{"HEADINGS", fmt.Sprintf("Heading %d", level)}] = func() {
				app.insertPaletteHeading(level)
			}
		}
		actions[commandPaletteKey{"VIEW", "Fold Selection"}] = func() { app.code.Run(editor.ToggleFold) }
	}
	if !source && !rich {
		return
	}
	actions[commandPaletteKey{"BLOCKS", wysiwyg.QueryBlockLabel}] = app.insertQueryBlock
	actions[commandPaletteKey{"BLOCKS", apiRequestLabel}] = app.insertAPIRequest
	for _, diagram := range diagramCommands {
		actions[commandPaletteKey{"DIAGRAMS", diagram.title}] = func() { app.insertDiagram(diagram) }
	}
	for _, chart := range chartCommands {
		actions[commandPaletteKey{"BLOCKS", chart.title}] = func() { app.insertChart(chart) }
	}
	for _, command := range mathCommands {
		actions[commandPaletteKey{"FORMATTING", command.label}] = func() { app.insertMath(command) }
	}
	formats := map[commandPaletteKey]markdownFormat{
		{"FORMATTING", "Bold"}:          boldFormat,
		{"FORMATTING", "Italic"}:        italicFormat,
		{"FORMATTING", "Strikethrough"}: strikethroughFormat,
		{"FORMATTING", "Inline Code"}:   inlineCodeFormat,
		{"LINKS & MEDIA", "Link"}:       linkFormat,
	}
	for key, format := range formats {
		actions[key] = func() {
			if rich {
				app.wys.format(app, format)
			} else {
				app.applyFormatting(format)
			}
		}
	}
	actions[commandPaletteKey{"LINKS & MEDIA", "Image"}] = app.insertImage
	if source {
		actions[commandPaletteKey{"LINKS & MEDIA", "HTML Image"}] = func() { app.insertHTMLImage(false) }
		actions[commandPaletteKey{"LINKS & MEDIA", "HTML Image (Sized)"}] = func() { app.insertHTMLImage(true) }
	}
	for _, entry := range app.insertionCommands() {
		if entry.source && !source && !(entry.label == insertTableLabel && app.wys.canInsertTable(app)) {
			continue
		}
		section := "BLOCKS"
		switch entry.label {
		case "Bullet List", "Numbered List", "Task List":
			section = "LISTS"
		case insertTableLabel:
			section = "TABLES"
		}
		actions[commandPaletteKey{section, entry.label}] = entry.action
	}
}

func (app *workspaceWindow) toggleLineNumbers() {
	app.preferences.LineNumbers = !app.preferences.LineNumbers
	app.preferencesChanged()
}

// EV-UI-075: the palette inserts the marker at the selection start and
// moves the caret after the retained text, including any existing heading.
func (app *workspaceWindow) insertPaletteHeading(level int) {
	if !app.sourceEditing() {
		return
	}
	prefix := strings.Repeat("#", level) + " "
	state := app.code
	selections, primary := state.Selections(), state.Selection()
	edits := make([]editor.Edit, 0, len(selections))
	carets := make([]editor.Selection, 0, len(selections))
	var primaryCaret editor.Selection
	for index, selected := range selections {
		start := selected.Start()
		edits = append(edits, editor.Edit{Start: start, End: start, Text: prefix})
		offset := selected.End() + (index+1)*len(prefix)
		caret := editor.Selection{Anchor: offset, Head: offset}
		if selected == primary {
			primaryCaret = caret
		} else {
			carets = append(carets, caret)
		}
	}
	state.Buffer.Edit(edits...)
	state.SetSelections(append(carets, primaryCaret)...)
	app.focusSource = true
}
