package ui

import (
	"unicode/utf8"

	"github.com/egoist/mygo/transfer"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

var visualImageFileDropOptions = transfer.DropOptions{Formats: []transfer.Format{transfer.FileList}}

func (w *workspaceWYSIWYG) dropInVisualNote(app *workspaceWindow, edit mygoui.Element) {
	drop, ok := mygoui.DropData(edit, noteDropOptions)
	if !ok {
		return
	}
	tab, session := app.workspaceTab, w.session
	doc := session.Document
	at, ok := doc.OffsetAtPoint(drop.X, drop.Y)
	if !ok {
		return
	}
	before, version := doc.Text(), doc.Version()
	offset := at
	var style richtext.ParagraphStyle
	prepare := func() bool {
		if tab != app.workspaceTab || session != w.session || !w.editable(app) || app.displayedMode() != wysiwygMode {
			return false
		}
		if request := app.ai.request; request != nil && request.visualWriteback != nil &&
			request.visualWriteback.previewFor(app, doc) != nil {
			return false
		}
		text := doc.Text()
		if doc.Version() != version {
			current := text.String()
			byteOffset, mapped := rebaseNoteDropOffset(before.String(), current, len(before.Slice(0, offset).String()))
			if !mapped {
				return false
			}
			at = utf8.RuneCountInString(current[:byteOffset])
		}
		start, end := text.ParagraphRange(at)
		style = text.ParagraphStyleAt(start)
		return !style.CodeBlock && !(end == start+1 && text.AttributesAt(start).Attachment != nil)
	}
	if !prepare() {
		app.problem = app.tr("Drop into an editable text paragraph.", "请拖入可编辑的文字段落。")
		return
	}
	app.applyNoteDrop(tab, drop.Data, prepare, func(markdown string, images bool) {
		if images {
			if !session.InsertMarkdownAt(at, at, markdown) {
				app.problem = app.tr("Could not insert the dropped images.", "无法插入拖入的图片。")
				return
			}
		} else {
			// An inline drop at the first character keeps the heading, list, or quote style.
			link := richtext.ParseMarkdown(markdown).FormatParagraphs(0, 0, func(p *richtext.ParagraphStyle) {
				*p = style
			})
			doc.Edit(func(change *richtext.Edit) {
				change.Replace(at, at, link)
				end := at + link.Len()
				change.Select(end, end)
			})
		}
		w.tableSelection = visualTableSelection{}
		w.sync(app)
		w.focus = true
	})
}
