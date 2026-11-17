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

func (app *workspaceWindow) typeSourceMarkdown(text string) bool {
	selections := app.code.Selections()
	if len(selections) != 1 {
		return false
	}
	sel := selections[0]
	replacement, caret := app.sourceMarkdownInsertion(app.draft.Snapshot(), sel.Start(), sel.End(), text)
	if replacement == text {
		return false
	}
	app.code.ReplaceTextInput(nil, replacement, caret)
	return true
}

func (app *workspaceWindow) sourceMarkdownTyping() bool {
	return app.sourceEditing() && app.draft != nil && !app.inspection &&
		(app.document.Path == "" || workspace.IsMarkdownPath(app.document.Path)) &&
		(app.markdownShortcutSession.code != app.code || app.markdownShortcutSession.stopMax <= 0)
}

func (app *workspaceWindow) sourceMarkdownInsertion(snapshot editor.Snapshot, start, end int, text string) (string, int) {
	replacement, trailingSpace := mixedLanguageInsertion(snapshot, start, end, text)
	if app.preferences != nil && app.preferences.AutoIndentParagraphs && start == end && canStartIndentedParagraph(text) {
		line := snapshot.LineOf(start)
		if start == snapshot.LineStart(line) && start == snapshot.LineEnd(line) && startsSourceParagraph(snapshot.String(), start, text) {
			replacement = paragraphIndent + replacement
		}
	}
	caret := len(replacement)
	if trailingSpace {
		caret--
	}
	return replacement, caret
}

// Keep the original selection and context while the IME updates its preedit.
type sourceMarkdownComposition struct {
	before     editor.Snapshot
	start, end int
	version    uint64
	mark       mygoui.TextInputRange
}

type sourceMarkdownTextInput struct {
	mygoui.TextInputClient
	app     *workspaceWindow
	code    *editor.State
	pending *sourceMarkdownComposition
}

func (input *sourceMarkdownTextInput) wrap(client mygoui.TextInputClient) mygoui.TextInputClient {
	input.TextInputClient = client
	return input
}

func (input *sourceMarkdownTextInput) enabled() bool {
	return input.app.code == input.code && input.app.sourceMarkdownTyping()
}

func (input *sourceMarkdownTextInput) SetMarkedText(r *mygoui.TextInputRange, text string, selected mygoui.TextInputRange) {
	_, marked := input.MarkedRange()
	if !marked {
		input.pending = nil
		if text != "" && input.enabled() && (r == nil || *r == input.Selection().Range) {
			if selections := input.code.Selections(); len(selections) == 1 {
				before := input.code.Buffer.Snapshot()
				input.pending = &sourceMarkdownComposition{before: before, start: selections[0].Start(), end: selections[0].End(), version: before.Version()}
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

func (input *sourceMarkdownTextInput) ReplaceText(r *mygoui.TextInputRange, text string) {
	p := input.pending
	input.pending = nil
	if p != nil && input.enabled() {
		_, marked := input.MarkedRange()
		if input.matches(p, r, marked) {
			replacement, caret := input.app.sourceMarkdownInsertion(p.before, p.start, p.end, text)
			if replacement != text {
				input.code.ReplaceTextInput(r, replacement, caret)
				return
			}
		}
	}
	input.TextInputClient.ReplaceText(r, text)
}

func (input *sourceMarkdownTextInput) matches(p *sourceMarkdownComposition, r *mygoui.TextInputRange, marked bool) bool {
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
