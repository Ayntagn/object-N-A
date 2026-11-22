package ui

import (
	"slices"
	"unicode/utf8"

	"cyberwriter/ui/wysiwyg"
	"github.com/egoist/mygo/transfer"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

func (w *workspaceWYSIWYG) pasteVisualNote(app *workspaceWindow, services mygoui.Services, start, end int) bool {
	if !w.editable(app) || !app.acceptsNoteImages(app.workspaceTab) {
		return false
	}
	if w.pasteVisualImage(app, services, start, end) {
		return true
	}
	document := w.session.Document
	text := document.Text()
	style := text.ParagraphStyleAt(start)
	attrs := document.TypingAttributes()
	if style.CodeBlock || attrs.Code || attrs.Link != "" || attrs.Tag == wysiwyg.FormulaColorTag {
		return false
	}

	formats := services.ClipboardFormats()
	if slices.Contains(formats, richtext.Format) {
		return false
	}
	clipboard := readNotePasteText(services)
	// The rich-text clipboard parser has no editable grid for pasted tables.
	if table, ok := pasteMarkdownTable(clipboard.markdown); ok {
		return w.insertVisualTable(app, table, false)
	}
	paragraphStart, _ := text.ParagraphRange(start)
	if clipboard.kind != plainNotePaste || clipboard.format == transfer.Markdown {
		markdown := clipboard.markdown
		blocks := clipboard.kind == blockNotePaste
		if app.autoIndentParagraphs() && clipboard.format == transfer.Text {
			newParagraph := blocks || (start == paragraphStart && style.Heading == 0 && style.List == richtext.NoList && !style.Quote)
			markdown = indentPastedParagraphs(markdown, newParagraph)
		}
		if w.session.PasteMarkdownAt(start, end, markdown, blocks) {
			w.tableSelection = visualTableSelection{}
			w.sync(app)
			return true
		}
	}
	if !app.autoIndentParagraphs() ||
		style.Heading != 0 || style.List != richtext.NoList || style.Indent != 0 || style.Quote || style.SoftBreak ||
		clipboard.format != transfer.Text || slices.Contains(formats, transfer.HTML) || slices.Contains(formats, richtext.RTF) {
		return false
	}
	replacement := indentPastedParagraphs(clipboard.markdown, start == paragraphStart)
	if replacement == clipboard.markdown {
		return false
	}
	document.Edit(func(edit *richtext.Edit) {
		edit.ReplaceString(start, end, replacement, attrs)
		caret := start + utf8.RuneCountInString(replacement)
		edit.Select(caret, caret)
	})
	w.sync(app)
	return true
}
