package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/richtext"
)

// Backticks are handled by Markdown code span and fence input.
var editorPairs = map[rune]rune{
	'(': ')', '[': ']', '{': '}', '"': '"', '\'': '\'',
	'（': '）', '［': '］', '｛': '｝', '【': '】', '〔': '〕',
	'〈': '〉', '《': '》', '「': '」', '『': '』', '“': '”', '‘': '’',
}

const editorClosers = ")]}\"'）］｝】〕〉》」』”’"

type pairInput uint8

const (
	noPair pairInput = iota
	insertPair
	skipCloser
	deletePair
)

// Zero with ok reports a plain Backspace. Non-pair keystrokes can bypass
// the buffer and document inspection.
func pairInputRune(event mygoui.InputEvent) (typed rune, ok bool) {
	switch event.Kind {
	case mygoui.InputKeyDown:
		return 0, event.Key == mygoui.KeyBackspace && event.Mods == 0
	case mygoui.InputText:
		r, size := utf8.DecodeRuneInString(event.Text)
		return r, len(event.Text) == size && (editorPairs[r] != 0 || strings.ContainsRune(editorClosers, r))
	default:
		return 0, false
	}
}

func pairInputAction(typed, left, right rune, selected bool) (pairInput, rune, rune) {
	if typed == 0 {
		if !selected && right != 0 && editorPairs[left] == right {
			return deletePair, left, right
		}
		return noPair, 0, 0
	}
	if !selected && typed == right && strings.ContainsRune(editorClosers, typed) {
		return skipCloser, 0, typed
	}
	close, ok := editorPairs[typed]
	if !ok {
		return noPair, 0, 0
	}
	if !selected {
		if typed == '\'' && (unicode.IsLetter(left) || unicode.IsNumber(left) || unicode.IsMark(left) || left == '_') {
			return noPair, 0, 0
		}
		if right != 0 && !unicode.IsSpace(right) && !strings.ContainsRune(editorClosers+".,?!;:。，、！？；：", right) {
			return noPair, 0, 0
		}
	}
	return insertPair, typed, close
}

var escapePairSnippet = strings.NewReplacer("\\", "\\\\", "$", "\\$", "}", "\\}")

func (app *workspaceWindow) pairSourceInput(event mygoui.InputEvent) bool {
	typed, ok := pairInputRune(event)
	if !ok {
		return false
	}
	sels := app.code.Selections()
	if len(sels) != 1 {
		return false
	}
	selection := sels[0]
	start, end := selection.Start(), selection.End()
	snapshot := app.draft.Snapshot()
	var left, right rune
	if start > 0 {
		left, _ = utf8.DecodeLastRuneInString(snapshot.Slice(max(0, start-utf8.UTFMax), start))
	}
	if end < snapshot.Len() {
		right, _ = utf8.DecodeRuneInString(snapshot.Slice(end, min(snapshot.Len(), end+utf8.UTFMax)))
	}
	action, open, close := pairInputAction(typed, left, right, start != end)
	if action == noPair {
		return false
	}
	openAt := start
	if action == deletePair {
		openAt -= utf8.RuneLen(open)
	}
	if openAt > 0 && snapshot.Slice(openAt-1, openAt) == "\\" {
		line := snapshot.Slice(snapshot.LineStart(snapshot.LineOf(openAt)), openAt)
		if !mixedSpacingUnescaped(line, len(line)) {
			return false
		}
	}
	switch action {
	case insertPair:
		if start != end {
			// A final snippet stop keeps the selected text selected after
			// wrapping and redo, without starting a tab-stop session.
			snippet, err := editor.ParseSnippet(string(open) + "${0:" + escapePairSnippet.Replace(snapshot.Slice(start, end)) + "}" + string(close))
			return err == nil && app.code.InsertSnippet(snippet)
		}
		opening, caret := app.sourceMarkdownInsertion(snapshot, start, end, string(open))
		app.code.ReplaceTextInput(nil, opening+string(close), caret)
	case skipCloser:
		after := start + utf8.RuneLen(close)
		app.code.Select(after, after)
	case deletePair:
		leftEnd := mygoui.UTF16Len(snapshot.Slice(0, start))
		target := mygoui.TextInputRange{
			Start: leftEnd - mygoui.UTF16Len(string(open)),
			End:   leftEnd + mygoui.UTF16Len(string(close)),
		}
		app.code.ReplaceTextInput(&target, "", 0)
	}
	return true
}

func (w *workspaceWYSIWYG) pairVisualInput(app *workspaceWindow, doc *richtext.Document, event mygoui.InputEvent) bool {
	typed, ok := pairInputRune(event)
	if !ok || !w.editable(app) || w.session.Document != doc || app.inspection ||
		app.document.Path != "" && !workspace.IsMarkdownPath(app.document.Path) {
		return false
	}
	anchor, caret := doc.Selection()
	start, end := min(anchor, caret), max(anchor, caret)
	text := doc.Text()
	action, open, close := pairInputAction(typed, text.RuneAt(start-1), text.RuneAt(end), start != end)
	if action == noPair {
		return false
	}
	openAt := start
	if action == deletePair {
		openAt--
	}
	if text.RuneAt(openAt-1) == '\\' {
		lineStart, _ := text.ParagraphRange(openAt)
		before := text.Slice(lineStart, openAt).String()
		if !mixedSpacingUnescaped(before, len(before)) {
			return false
		}
	}
	attrs := doc.TypingAttributes()
	if attrs.Attachment != nil || attrs.Tag == wysiwyg.FormulaColorTag {
		return false
	}
	switch action {
	case insertPair:
		// Insert around the selection to preserve its character styles.
		doc.Edit(func(edit *richtext.Edit) {
			edit.ReplaceString(end, end, string(close), attrs)
			edit.ReplaceString(start, start, string(open), attrs)
			edit.Select(anchor+1, caret+1)
		})
	case skipCloser:
		doc.SetSelection(start+1, start+1)
		return true
	case deletePair:
		doc.Edit(func(edit *richtext.Edit) {
			edit.ReplaceString(openAt, end+1, "", attrs)
			edit.Select(openAt, openAt)
		})
	}
	w.sync(app)
	return true
}
