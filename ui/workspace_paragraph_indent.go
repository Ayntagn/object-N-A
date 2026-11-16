package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const paragraphIndent = "\u3000\u3000"

func (app *workspaceWindow) autoIndentSourceParagraph(event mygoui.InputEvent) bool {
	if event.Kind != mygoui.InputText || !app.sourceParagraphIndentEnabled() || !canStartIndentedParagraph(event.Text) {
		return false
	}
	snapshot, caret, emptyLine := sourceParagraphCaret(app.code)
	if !emptyLine || !startsSourceParagraph(snapshot.String(), caret, event.Text) {
		return false
	}
	// Insert the prefix and the first input as one undo step.
	app.code.Insert(paragraphIndent + event.Text)
	return true
}

func (app *workspaceWindow) sourceParagraphIndentEnabled() bool {
	return app.preferences != nil && app.preferences.AutoIndentParagraphs && app.sourceEditing() && app.draft != nil &&
		(app.document.Path == "" || workspace.IsMarkdownPath(app.document.Path)) &&
		(app.markdownShortcutSession.code != app.code || app.markdownShortcutSession.stopMax <= 0)
}

func sourceParagraphCaret(code *editor.State) (editor.Snapshot, int, bool) {
	selections := code.Selections()
	if len(selections) != 1 || selections[0].Start() != selections[0].End() {
		return editor.Snapshot{}, 0, false
	}
	caret := selections[0].Head
	snapshot := code.Buffer.Snapshot()
	line := snapshot.LineOf(caret)
	return snapshot, caret, caret == snapshot.LineStart(line) && caret == snapshot.LineEnd(line)
}

// Snapshot the empty line before preedit changes the buffer and selection.
type sourceParagraphComposition struct {
	before  editor.Snapshot
	at      int
	version uint64
	mark    mygoui.TextInputRange
}

type sourceParagraphTextInput struct {
	mygoui.TextInputClient
	app     *workspaceWindow
	code    *editor.State
	pending *sourceParagraphComposition
}

func (input *sourceParagraphTextInput) wrap(client mygoui.TextInputClient) mygoui.TextInputClient {
	input.TextInputClient = client
	return input
}

func (input *sourceParagraphTextInput) enabled() bool {
	return input.app.code == input.code && input.app.sourceParagraphIndentEnabled()
}

func (input *sourceParagraphTextInput) SetMarkedText(r *mygoui.TextInputRange, text string, selected mygoui.TextInputRange) {
	_, marked := input.MarkedRange()
	if !marked {
		input.pending = nil
		if text != "" && input.enabled() && (r == nil || *r == input.Selection().Range) {
			if before, at, emptyLine := sourceParagraphCaret(input.code); emptyLine {
				input.pending = &sourceParagraphComposition{before: before, at: at, version: before.Version()}
			}
		}
	} else if p := input.pending; p != nil && !input.matches(p, r, true) {
		input.pending = nil
	}
	input.TextInputClient.SetMarkedText(r, text, selected)
	if p := input.pending; p != nil {
		if mark, ok := input.MarkedRange(); ok {
			p.mark, p.version = mark, input.code.Buffer.Version()
		} else {
			input.pending = nil
		}
	}
}

func (input *sourceParagraphTextInput) ReplaceText(r *mygoui.TextInputRange, text string) {
	p := input.pending
	input.pending = nil
	if p != nil && input.enabled() && canStartIndentedParagraph(text) {
		_, marked := input.MarkedRange()
		if input.matches(p, r, marked) && startsSourceParagraph(p.before.String(), p.at, text) {
			// Let MyGo replace the marked text, move the caret and commit
			// the composition and the prefix in its existing undo group.
			text = paragraphIndent + text
		}
	}
	input.TextInputClient.ReplaceText(r, text)
}

func (input *sourceParagraphTextInput) matches(p *sourceParagraphComposition, r *mygoui.TextInputRange, marked bool) bool {
	if input.code.Buffer.Version() != p.version {
		return false
	}
	if !marked {
		// Some input methods end preedit before replacing its range.
		return r != nil && *r == p.mark
	}
	return r == nil || *r == p.mark || *r == input.Selection().Range
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
