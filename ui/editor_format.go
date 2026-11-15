package ui

import mygoui "github.com/egoist/mygo/ui"

var formattingCommands = [...]struct {
	label, symbol string
	format        markdownFormat
	key           mygoui.Key
}{
	{"Bold", "bold", boldFormat, mygoui.KeyB},
	{"Italic", "italic", italicFormat, mygoui.KeyI},
	{"Strike Through", "strikethrough", strikethroughFormat, mygoui.KeyUnknown},
	{"Embed Code", "chevron.left.forwardslash.chevron.right", inlineCodeFormat, mygoui.KeyBackquote},
	{"Link", "link", linkFormat, mygoui.KeyK},
	{"Highlight", "highlighter", "", mygoui.KeyUnknown},
	{"Table", "tablecells", "", mygoui.KeyUnknown},
}

func (app *workspaceWindow) formattingInput(event mygoui.InputEvent) bool {
	if event.Kind != mygoui.InputKeyDown || event.Mods != mygoui.Cmd || event.Key == mygoui.KeyUnknown {
		return false
	}
	for _, command := range formattingCommands {
		if event.Key == command.key {
			return app.applyFormatting(command.format)
		}
	}
	return false
}

func (app *workspaceWindow) applyFormatting(format markdownFormat) bool {
	if app.modalOpen || (app.mode != sourceMode && app.mode != splitMode) || !formatMarkdown(app.code, format) {
		return false
	}
	app.focusSource = true
	return true
}
