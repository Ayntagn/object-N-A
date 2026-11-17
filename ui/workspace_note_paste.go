package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/transfer"
	mygoui "github.com/egoist/mygo/ui"
)

func (app *workspaceWindow) acceptsNoteImages(tab *workspaceTab) bool {
	return !app.modalOpen && !tab.inspection &&
		(tab.document.Path == "" || workspace.IsMarkdownPath(tab.document.Path)) &&
		(!tab.documentType.overridden || tab.documentType.kind == documentMarkdown)
}

func imagePasteCommand(event mygoui.InputEvent) bool {
	return event.Kind == mygoui.InputCommand && event.Text == "paste" ||
		event.Kind == mygoui.InputKeyDown && event.Mods == mygoui.Cmd && event.Key == mygoui.KeyV
}

func (app *workspaceWindow) pasteSourceImage(tab *workspaceTab, services mygoui.Services) bool {
	if tab.code.ReadOnly || !app.acceptsNoteImages(tab) {
		return false
	}
	markdown, handled := app.clipboardImageMarkdown(tab, services)
	if markdown != "" {
		selections := tab.code.Selections()
		if len(selections) == 1 {
			selection := selections[0]
			markdown = sourceImageBlock(tab.draft.Snapshot(), selection.Start(), selection.End(), markdown)
		}
		tab.code.Insert(markdown)
		app.selectMultiFileTile(tab)
	}
	return handled
}

func (w *workspaceWYSIWYG) pasteVisualImage(app *workspaceWindow, services mygoui.Services, start, end int) bool {
	tab := app.workspaceTab
	if !w.editable(app) || !app.acceptsNoteImages(tab) ||
		w.session.Document.Text().ParagraphStyleAt(start).CodeBlock {
		return false
	}
	markdown, handled := app.clipboardImageMarkdown(tab, services)
	if !handled {
		markdown = strings.TrimSpace(services.ReadClipboard())
		if !strings.HasPrefix(markdown, "![[") {
			return false
		}
		links := vaultindex.Wikilinks(markdown)
		if len(links) != 1 || !links[0].Embed || links[0].End != len(markdown) ||
			!workspace.IsRasterImagePath(links[0].Target) {
			return false
		}
	}
	if markdown == "" {
		return handled
	}
	if !w.session.InsertMarkdownAt(start, end, markdown) {
		if !handled {
			return false
		}
		app.problem = app.tr("Could not insert the pasted image.", "无法插入粘贴的图片。")
		return true
	}
	w.tableSelection = visualTableSelection{}
	w.sync(app)
	return true
}

func (app *workspaceWindow) clipboardImageMarkdown(tab *workspaceTab, services mygoui.Services) (string, bool) {
	if !slices.Contains(services.ClipboardFormats(), transfer.PNG) {
		return "", false
	}
	png, err := services.ReadClipboardData(transfer.PNG).Read(transfer.PNG)
	if err != nil || len(png) == 0 {
		return "", false
	}
	vault, note := app.noteDropVault(tab), tab.document.Path
	if vault == nil || note == "" {
		app.problem = app.tr("Save the note before importing an image.", "请先保存笔记，再粘贴图片。")
		return "", true
	}
	destination, err := vault.ImportPastedPNG(note, png, time.Now())
	if err != nil {
		app.problem = fmt.Sprintf(app.tr("Could not import the image. %v", "无法导入图片。%v"), err)
		return "", true
	}
	app.problem = ""
	return importedImageMarkdown(note, destination, "", ""), true
}
