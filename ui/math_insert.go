package ui

import mygoui "github.com/egoist/mygo/ui"

type mathInsertCommand struct {
	label, example, prefix, suffix string
	display                        bool
}

// EV-UI-153: Add and the source context menu use the same template enum.
var mathCommands = [...]mathInsertCommand{
	{label: "Math (inline)", example: "e = mc^2", prefix: "$", suffix: "$"},
	{label: "Math (display)", prefix: "$$\n", suffix: "\n$$", display: true},
	{label: "Colored Math", example: "x", prefix: "$", suffix: "$:lightgreen"},
}

func (app *workspaceWindow) mathMenu(m *mygoui.Menu) {
	for _, command := range mathCommands {
		if m.Item(command.label).Chosen() {
			app.insertMath(command)
		}
	}
}

func (app *workspaceWindow) insertMath(command mathInsertCommand) {
	if app.sourceEditing() {
		value := app.code.SelectedText()
		caret := len(command.prefix)
		if value == "" {
			value = command.example
		} else if !command.display {
			caret += len(value) + 1
		}
		app.insertSourceSnippetAt(command.prefix+value+command.suffix, caret)
		return
	}
	if app.displayedMode() != wysiwygMode || !app.wys.editable(app) {
		return
	}
	session := app.wys.session
	value := session.SelectionMarkdown()
	if command.display {
		if value == "" {
			return
		}
		doc := session.Document
		text := doc.Text()
		a, z := doc.Selection()
		start, end := min(a, z), max(a, z)
		paragraphStart, _ := text.ParagraphRange(start)
		_, paragraphEnd := text.ParagraphRange(end)
		if start != paragraphStart || end != paragraphEnd {
			// Crepe opens the inserted slice inside the active paragraph.
			// The selected Markdown and its terminal LF land as body text.
			doc.InsertText(value)
		} else if formula, ok := session.InsertDisplayFormula(value); ok {
			app.wys.math.focusBlock = formula.ID
			app.wys.focus = false
		}
	} else {
		if value == "" {
			value = command.example
		}
		session.InsertInlineMathTemplate(value, command.suffix[1:])
	}
	app.wys.sync(app)
}
