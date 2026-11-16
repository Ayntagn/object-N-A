package ui

import (
	"strings"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const sourceCodeFence = "```"

type sourceFenceCompletion struct {
	code                   *editor.State
	version                uint64
	openingEnd, closingEnd int
	opener, body, closer   string
}

func (app *workspaceWindow) completeSourceCodeFence(event mygoui.InputEvent) bool {
	pair := &app.sourceFence
	if pair.code != nil && (pair.code != app.code || event.Kind == mygoui.InputPointerDown || markdownShortcutUndo(event)) {
		*pair = sourceFenceCompletion{}
	}
	enter := event.Kind == mygoui.InputKeyDown && event.Key == mygoui.KeyEnter && event.Mods == 0 && pair.code != nil
	backquote := event.Kind == mygoui.InputText && event.Text == "`"
	if !(enter || backquote) || app.draft == nil || !app.sourceEditing() || app.inspection ||
		app.document.Path != "" && !workspace.IsMarkdownPath(app.document.Path) ||
		app.markdownShortcutSession.code == app.code && app.markdownShortcutSession.stopMax > 0 {
		return false
	}
	selections := app.code.Selections()
	if len(selections) != 1 || selections[0].Anchor != selections[0].Head {
		return false
	}
	caret := selections[0].Head
	snapshot := app.draft.Snapshot()
	if enter {
		bodyEnd, ok := pair.bodyPosition(snapshot, caret)
		*pair = sourceFenceCompletion{}
		if ok {
			app.code.Select(bodyEnd, bodyEnd)
		}
		return ok
	}
	if pair.code != nil && pair.version == snapshot.Version() && pair.openingEnd == caret {
		// Widen the closing fence of this just-inserted pair in the same edit.
		app.draft.Edit(
			editor.Edit{Start: caret, End: caret, Text: event.Text},
			editor.Edit{Start: pair.closingEnd, End: pair.closingEnd, Text: event.Text},
		)
		pair.openingEnd++
		pair.closingEnd += 2
		pair.version = app.draft.Version()
		pair.opener += event.Text
		pair.closer += event.Text
		app.code.Select(pair.openingEnd, pair.openingEnd)
		return true
	}

	line := snapshot.LineOf(caret)
	if caret != snapshot.LineEnd(line) {
		return false
	}
	current := snapshot.Line(line)
	if !strings.HasSuffix(current, sourceCodeFence[:2]) {
		return false
	}
	lead := current[:len(current)-2]
	if line == 0 {
		// The Markdown body excludes a leading byte-order mark.
		lead = strings.TrimPrefix(lead, "\ufeff")
	}
	if strings.Trim(lead, " \t>+-*.)0123456789") != "" {
		return false
	}
	prefix, _, markdown := workspace.SplitFrontmatter(snapshot.String())
	marker := caret - len(prefix) - 2
	if marker < 0 {
		return false
	}
	end := marker + 2
	parsed := parser.Parse(markdown[:end] + event.Text + markdown[end:])
	opening := sourceFenceIndexAt(parsed, marker)
	if opening == 0 || parsed.Block(opening).Start != marker {
		return false
	}
	if closing := sourceClosingFence(parsed, opening); closing >= 0 {
		// Reuse a stand-alone closer, but keep both lines of any complete
		// code block that follows the new fence.
		before := parser.Parse(markdown)
		original := sourceFenceIndexAt(before, closing-1)
		if original == 0 || sourceClosingFence(before, original) < 0 {
			return false
		}
	}

	// Keep quote markers on each line and indent through any list markers.
	indent := strings.Map(func(char rune) rune {
		if char == '>' || char == '\t' || char == ' ' {
			return char
		}
		return ' '
	}, lead)
	snippet, err := editor.ParseSnippet(event.Text + "$0\n" + indent + "\n" + indent + sourceCodeFence)
	if err != nil || !app.code.InsertSnippet(snippet) {
		return false
	}
	*pair = sourceFenceCompletion{
		code: app.code, version: app.draft.Version(),
		openingEnd: caret + 1, closingEnd: caret + len(snippet.Text()),
		opener: current + event.Text, body: indent, closer: indent + sourceCodeFence,
	}
	return true
}

// Enter enters the generated empty body after typing a language. Other lines
// still use the editor's normal newline, including a body already filled in.
func (pair *sourceFenceCompletion) bodyPosition(snapshot editor.Snapshot, caret int) (int, bool) {
	line := snapshot.LineOf(caret)
	if line+2 >= snapshot.LineCount() || caret != snapshot.LineEnd(line) ||
		snapshot.LineStart(line)+len(pair.opener) != pair.openingEnd {
		return 0, false
	}
	info, ok := strings.CutPrefix(snapshot.Line(line), pair.opener)
	if !ok || strings.ContainsRune(info, '`') || snapshot.Line(line+1) != pair.body || snapshot.Line(line+2) != pair.closer {
		return 0, false
	}
	return snapshot.LineEnd(line + 1), true
}

func sourceFenceIndexAt(document *parser.Document, offset int) int {
	for index := 1; index < document.Len(); index++ {
		block := document.Block(index)
		if block.Start <= offset && offset < block.End && block.Kind == parser.CodeBlock && block.Fenced {
			return index
		}
	}
	return 0
}

func sourceClosingFence(document *parser.Document, index int) int {
	opening := document.Block(index)
	if opening.Open {
		return -1
	}
	source := document.Source()
	start := strings.LastIndexByte(source[:opening.End], '\n') + 1
	if start <= opening.Start {
		return -1
	}
	// A parent container can end an unclosed fence. The parser excludes an
	// explicit closing line from the body, even inside quotes and list items.
	if count := document.LineCount(index); count > 0 {
		if _, last := document.Line(index, count-1); last >= start {
			return -1
		}
	}
	marker := source[opening.Start : opening.Start+1]
	opener := source[opening.Start:start]
	length := len(opener) - len(strings.TrimLeft(opener, marker))
	line := strings.TrimRight(source[start:opening.End], " \t")
	beforeClosing := len(strings.TrimRight(line, marker))
	if len(line)-beforeClosing < length {
		return -1
	}
	return start + beforeClosing
}
