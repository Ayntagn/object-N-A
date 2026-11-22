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
	snapshot := app.draft.Snapshot()
	if replacement, caret, preceding := app.sourceManualIndentInsertion(snapshot, sel.Start(), sel.End(), text); preceding != 0 {
		end := mygoui.UTF16Len(snapshot.Slice(0, sel.End()))
		app.code.ReplaceTextInput(&mygoui.TextInputRange{Start: end - preceding, End: end}, replacement, caret)
		return true
	}
	replacement, caret := app.sourceMarkdownInsertion(snapshot, sel.Start(), sel.End(), text)
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
	if app.autoIndentParagraphs() && start == end && canStartIndentedParagraph(text) {
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

func (app *workspaceWindow) autoIndentParagraphs() bool {
	return app.preferences != nil && app.preferences.AutoIndentParagraphs
}

func (app *workspaceWindow) sourceManualIndentInsertion(snapshot editor.Snapshot, start, end int, text string) (replacement string, caret, preceding int) {
	if !app.autoIndentParagraphs() || start != end {
		return "", 0, 0
	}
	line := snapshot.LineOf(start)
	lineStart, lineEnd := snapshot.LineStart(line), snapshot.LineEnd(line)
	switch start - lineStart {
	case 3:
		// Adding the fourth space before existing prose would turn it
		// into an indented code block.
		if text != " " || snapshot.Slice(lineStart, start) != "   " {
			return "", 0, 0
		}
		right := snapshot.Slice(start, lineEnd)
		if !canStartIndentedParagraph(right) || !strings.ContainsFunc(right, isHan) ||
			!startsSourceParagraph(snapshot.String(), start, "") {
			return "", 0, 0
		}
		return paragraphIndent, len(paragraphIndent), 3
	case 4:
		// On an empty line, wait until the first prose character so
		// typing four spaces can still start deliberate code.
		first, _ := utf8.DecodeRuneInString(text)
		if start != lineEnd || !isHan(first) || !canStartIndentedParagraph(text) ||
			snapshot.Slice(lineStart, start) != "    " ||
			!isolatedSourceIndent(snapshot.String(), start, text) {
			return "", 0, 0
		}
		replacement, trailingSpace := mixedLanguageInsertion(snapshot, lineStart, end, paragraphIndent+text)
		caret := len(replacement)
		if trailingSpace {
			caret--
		}
		return replacement, caret, 4
	default:
		return "", 0, 0
	}
}

func isHan(r rune) bool { return unicode.Is(unicode.Han, r) }

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
			if marked {
				if replacement, caret, preceding := input.app.sourceManualIndentInsertion(p.before, p.start, p.end, text); preceding != 0 {
					whole := mygoui.TextInputRange{Start: p.mark.Start - preceding, End: p.mark.End}
					input.code.ReplaceTextInput(&whole, replacement, caret)
					return
				}
			}
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

func sourceParagraphBody(source string) (body string, prefixLength int, ok bool) {
	prefix, _, body := workspace.SplitFrontmatter(source)
	head, _, _ := strings.Cut(body, "\n")
	switch strings.TrimRight(head, " \t\r") {
	case "---", "+++":
		// While a leading front matter block has no closing delimiter, its
		// lines may still be metadata.
		return "", 0, false
	}
	return body, len(prefix), true
}

func startsSourceParagraph(source string, caret int, input string) bool {
	block, _ := sourceRootBlockStartingAt(source, caret, input)
	return block != nil && block.Kind == parser.Paragraph
}

func isolatedSourceIndent(source string, caret int, input string) bool {
	block, offset := sourceRootBlockStartingAt(source, caret, input)
	return block != nil && block.Kind == parser.CodeBlock && !block.Fenced && block.End == offset+len(input)
}

func sourceRootBlockStartingAt(source string, caret int, input string) (*parser.Block, int) {
	body, prefix, ok := sourceParagraphBody(source)
	offset := caret - prefix
	if !ok || offset < 0 {
		return nil, 0
	}
	document := parser.New(webPreviewExtensions)
	document.Set(body[:offset] + input + body[offset:])
	for index := range document.Children(0) {
		block := document.Block(index)
		if block.Start >= offset {
			if block.Start == offset {
				return block, offset
			}
			break
		}
	}
	return nil, 0
}

// sourceAllowsBlockPaste keeps a block paste from breaking out of the
// Markdown structure in which the caret sits.
func sourceAllowsBlockPaste(snapshot editor.Snapshot, start, end int) bool {
	body, prefix, ok := sourceParagraphBody(snapshot.String())
	if !ok || start < prefix {
		return false
	}
	doc := parser.New(webPreviewExtensions)
	doc.Set(body)
	at := start - prefix
	line := snapshot.LineOf(start)
	lineStart := snapshot.LineStart(line) - prefix
	lineEnd := snapshot.LineEnd(line) - prefix
	for index := range doc.Children(0) {
		block := doc.Block(index)
		if block.Start > lineEnd {
			break
		}
		if block.End < at || block.End == at && at == lineStart && !block.Open ||
			start < end && at <= block.Start && end-prefix >= block.End {
			continue
		}
		switch block.Kind {
		case parser.CodeBlock, parser.HTMLBlock, parser.MathBlock, parser.Quote, parser.List, parser.Table, parser.FootnoteDefinition:
			return false
		case parser.Paragraph, parser.Heading:
			if mixedSpacingInMarkdown(doc, at) || (start != end && mixedSpacingInMarkdown(doc, end-prefix)) {
				return false
			}
		}
	}
	return true
}

func indentPastedSource(snapshot editor.Snapshot, start, end int, text string) string {
	before := snapshot.Slice(0, start)
	after := snapshot.Slice(end, snapshot.Len())
	wholeLine := start == snapshot.LineStart(snapshot.LineOf(start)) && end == snapshot.LineEnd(snapshot.LineOf(end))
	return indentPastedRootParagraphs(before+text+after, start, text, wholeLine)
}

// first reports whether the clipboard begins in a new editor paragraph.
func indentPastedParagraphs(text string, first bool) string {
	return indentPastedRootParagraphs(text, 0, text, first)
}

func indentPastedRootParagraphs(source string, start int, text string, first bool) string {
	body, prefix, ok := sourceParagraphBody(source)
	if !ok || start < prefix {
		return text
	}
	doc := parser.New(webPreviewExtensions)
	doc.Set(body)
	var out strings.Builder
	last := 0
	for index := range doc.Children(0) {
		block := doc.Block(index)
		at := prefix + block.Start - start
		if at >= len(text) {
			break
		}
		if at < 0 || at == 0 && !first || block.Kind != parser.Paragraph ||
			block.Start > 0 && !strings.ContainsRune("\r\n", rune(body[block.Start-1])) ||
			!startsPastedProse(text[at:]) {
			continue
		}
		out.WriteString(text[last:at])
		out.WriteString(paragraphIndent)
		last = at
	}
	if out.Len() == 0 {
		return text
	}
	out.WriteString(text[last:])
	return out.String()
}

func startsPastedProse(text string) bool {
	first, _ := utf8.DecodeRuneInString(text)
	if unicode.IsDigit(first) {
		return true // The complete pasted block has already parsed as prose.
	}
	if end := strings.IndexAny(text, "\r\n"); end >= 0 {
		text = text[:end]
	}
	return canStartIndentedParagraph(text)
}
