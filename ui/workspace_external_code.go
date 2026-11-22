package ui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"cyberwriter/internal/core/externalcode"
	"cyberwriter/internal/core/markdownfence"
	"cyberwriter/internal/core/workspace"
	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type externalCodeEdit struct {
	draft    *editor.Buffer
	snapshot editor.Snapshot
	fence    markdownfence.Fence
}

func externalCodeShortcut(event mygoui.InputEvent) bool {
	return event.Kind == mygoui.InputKeyDown && event.Key == mygoui.KeyE &&
		event.Mods == mygoui.Cmd|mygoui.Alt && !event.Repeat
}

func (app *workspaceWindow) canEditCodeExternally() bool {
	return !app.modalOpen && !app.inspection && app.draft != nil &&
		(app.document.Path == "" || workspace.IsMarkdownPath(app.document.Path)) &&
		(!app.documentType.overridden || app.documentType.kind == documentMarkdown)
}

func (app *workspaceWindow) editSourceCodeExternally() bool {
	if !app.sourceEditing() || !app.canEditCodeExternally() {
		return false
	}
	selections := app.code.Selections()
	if len(selections) != 1 {
		return false
	}
	snapshot := app.draft.Snapshot()
	fence, ok := markdownfence.Find(snapshot.String(), selections[0].Head)
	if ok {
		app.openExternalCode(snapshot, fence)
	}
	return ok
}

func (w *workspaceWYSIWYG) editVisualCodeExternally(app *workspaceWindow, offset int) bool {
	if !w.editable(app) || !app.canEditCodeExternally() {
		return false
	}
	w.sync(app)
	fence, ok := w.session.ExternalCodeFenceAt(offset)
	if ok {
		app.openExternalCode(app.draft.Snapshot(), fence)
	}
	return ok
}

func (app *workspaceWindow) openExternalCode(snapshot editor.Snapshot, fence markdownfence.Fence) {
	tab := app.workspaceTab
	if app.win == nil || app.win.IsDestroyed() {
		return
	}
	if tab.externalCode != nil {
		app.problem = app.tr("Close the code file already open in VS Code for this note.",
			"请先关闭这篇笔记已经在 VS Code 中打开的代码文件。")
		return
	}
	edit := &externalCodeEdit{tab.draft, snapshot, fence}
	tab.externalCode = edit
	app.problem = ""
	go func() {
		session, err := externalcode.Open(fence.Language, fence.Code)
		if err != nil {
			app.win.Update(func() {
				owner := app.externalCodeOwner(tab, edit)
				if owner == nil {
					return
				}
				owner.problem = fmt.Sprintf(app.tr("Could not open VS Code. %v", "无法打开 VS Code。%v"), err)
				if errors.Is(err, externalcode.ErrVSCodeNotFound) {
					owner.problem = app.tr("Install Visual Studio Code or add its code command to your PATH.",
						"请安装 Visual Studio Code，或把 code 命令加入 PATH。")
				}
				owner.win.Invalidate()
			})
			return
		}
		result := <-session.Result
		app.win.Update(func() { app.finishExternalCode(tab, edit, session, result) })
	}()
}

// Find the original tab even if it moved to another window during editing.
func (app *workspaceWindow) externalCodeOwner(tab *workspaceTab, edit *externalCodeEdit) *workspaceWindow {
	if tab.externalCode != edit {
		return nil
	}
	tab.externalCode = nil
	for _, window := range app.documentWindows() {
		if !window.win.IsDestroyed() && slices.Contains(window.tabs, tab) && tab.draft == edit.draft {
			return window
		}
	}
	return nil
}

func (app *workspaceWindow) finishExternalCode(tab *workspaceTab, edit *externalCodeEdit, session *externalcode.Session, result externalcode.Result) {
	owner := app.externalCodeOwner(tab, edit)
	report := owner
	if report == nil && !app.win.IsDestroyed() {
		report = app
	}
	if report != nil {
		defer report.win.Invalidate()
	}
	switch {
	case result.Err != nil:
		if report != nil {
			report.problem = fmt.Sprintf(app.tr("Could not read the edited code. Check %s. %v", "无法读取外部编辑结果。请查看 %s。%v"),
				session.Path, result.Err)
		}
		return
	case result.Text == edit.fence.Code:
		// Closing an untouched file never changes the draft.
	case owner == nil:
		if report != nil {
			report.problem = fmt.Sprintf(app.tr("The original note closed. The edited code is in %s.",
				"原笔记已经关闭。编辑后的代码保存在 %s。"), session.Path)
		}
		return
	default:
		// The visual editor can have a change waiting to be synced.
		if owner.workspaceTab == tab {
			owner.wys.sync(owner)
		}
		if err := edit.apply(result.Text, tab.wys.session); err != nil {
			owner.problem = fmt.Sprintf(app.tr("Could not update the original code block. The edited code is in %s.",
				"无法更新原代码块。编辑后的代码保存在 %s。"), session.Path)
			return
		}
		if owner.workspaceTab == tab {
			owner.syncDocumentEdited()
		}
	}
	if err := session.Cleanup(); err != nil && report != nil {
		report.problem = fmt.Sprintf(app.tr("The temporary file is still available at %s. %v",
			"临时文件仍保存在 %s。%v"), session.Path, err)
	}
}

func (edit *externalCodeEdit) apply(code string, visual *wysiwyg.Session) error {
	fence, ok := edit.currentFence(edit.draft.Snapshot())
	if !ok {
		return errors.New("original code block changed")
	}
	replacement, err := fence.Replacement(code)
	if err != nil {
		return err
	}
	if replacement != fence.Source && (visual == nil || !visual.ApplyExternalCodeFence(fence, replacement, code)) {
		edit.draft.Edit(editor.Edit{Start: fence.Start, End: fence.End, Text: replacement})
	}
	return nil
}

func (edit *externalCodeEdit) currentFence(current editor.Snapshot) (markdownfence.Fence, bool) {
	fence := edit.fence
	if current.Version() == edit.snapshot.Version() {
		return fence, true
	}
	before, after := edit.snapshot.String(), current.String()
	if strings.Count(before, fence.Source) != strings.Count(after, fence.Source) {
		return markdownfence.Fence{}, false
	}
	at := fence.Start
	switch {
	case strings.HasPrefix(after, before[:fence.End]):
	case strings.HasSuffix(after, before[fence.Start:]):
		at += len(after) - len(before)
	default:
		return markdownfence.Fence{}, false
	}
	found, ok := markdownfence.Find(after, at)
	return found, ok && found.Start == at && found.Source == fence.Source
}
