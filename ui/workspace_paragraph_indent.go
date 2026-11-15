package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const paragraphIndent = "\u3000\u3000"

func (app *workspaceWindow) autoIndentSourceParagraph(event mygoui.InputEvent) bool {
	if app.preferences == nil || !app.preferences.AutoIndentParagraphs ||
		event.Kind != mygoui.InputText || !app.sourceEditing() || app.draft == nil ||
		app.document.Path != "" && !workspace.IsMarkdownPath(app.document.Path) {
		return false
	}
	if !canStartIndentedParagraph(event.Text) {
		return false
	}
	selections := app.code.Selections()
	if len(selections) != 1 || selections[0].Start() != selections[0].End() {
		return false
	}
	caret := selections[0].Head
	snapshot := app.draft.Snapshot()
	line := snapshot.LineOf(caret)
	if caret != snapshot.LineStart(line) || caret != snapshot.LineEnd(line) {
		return false
	}
	if app.markdownShortcutSession.code == app.code && app.markdownShortcutSession.stopMax > 0 {
		return false
	}
	if !startsSourceParagraph(snapshot.String(), caret, event.Text) {
		return false
	}
	// Insert the prefix and the first input as one undo step.
	app.code.Insert(paragraphIndent + event.Text)
	return true
}

func canStartIndentedParagraph(text string) bool {
	if strings.ContainsAny(text, "\r\n\u2028\u2029") {
		return false
	}
	first, _ := utf8.DecodeRuneInString(text)
	// ASCII digits and Markdown punctuation can still become block markers.
	return unicode.IsLetter(first) || first > unicode.MaxASCII &&
		(unicode.IsPunct(first) || unicode.IsNumber(first) || unicode.IsSymbol(first))
}

func startsSourceParagraph(source string, caret int, input string) bool {
	prefix, _, body := workspace.SplitFrontmatter(source)
	offset := caret - len(prefix)
	if offset < 0 {
		return false
	}
	head, _, _ := strings.Cut(body, "\n")
	switch strings.TrimRight(head, " \t\r") {
	case "---", "+++":
		// While a leading front matter block has no closing delimiter, its
		// lines may still be metadata.
		return false
	}
	document := parser.Parse(body[:offset] + input + body[offset:])
	for index := range document.Children(0) {
		block := document.Block(index)
		if block.Start >= offset {
			return block.Start == offset && block.Kind == parser.Paragraph
		}
	}
	return false
}
