package ui

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

type workspaceAIWriteback struct {
	tab     *workspaceTab
	draft   *editor.Buffer
	code    *editor.State
	version uint64
	start   int
	end     int
	written string
}

// EV-UI-231 / EV-UI-242: Insert Text writes before the source selection unless Replace Selection is on.
func (app *workspaceWindow) beginAIWriteback() *workspaceAIWriteback {
	if !app.preferences.AIStreamIntoEditor {
		return nil
	}
	return app.aiWritebackAtSelection(app.ai.replaceSelection)
}

// EV-UI-242 / EV-UI-246: an explicit insertion uses the current selection.
func (app *workspaceWindow) insertAIReply(reply string) {
	if app.workspaceTab != nil && app.mode == wysiwygMode {
		if target := app.aiVisualWritebackAtSelection(true); target != nil {
			target.commit(app, reply)
		}
		return
	}
	if target := app.aiWritebackAtSelection(true); target != nil {
		target.apply(app, reply)
	}
}

func (app *workspaceWindow) aiWritebackAtSelection(replaceSelection bool) *workspaceAIWriteback {
	if app.workspaceTab == nil || app.draft == nil || !app.sourceEditing() {
		return nil
	}
	selection := app.code.Selection()
	start, end := selection.Start(), selection.Start()
	if replaceSelection {
		end = selection.End()
	}
	return &workspaceAIWriteback{
		tab: app.workspaceTab, draft: app.draft, code: app.code,
		version: app.draft.Version(), start: start, end: end,
	}
}

// apply runs on the UI thread. MyGo creates an undo step for each Buffer.Edit.
// Replace the turn's latest step only while its revision is still current, so
// one Undo takes back the whole turn without undoing any intervening user edit.
func (target *workspaceAIWriteback) apply(app *workspaceWindow, reply string) bool {
	tab := target.tab
	if app.modalOpen || !slices.Contains(app.tabs, tab) || tab.draft != target.draft ||
		tab.code != target.code || target.code.ReadOnly || target.draft.Version() != target.version {
		return false
	}
	sourceVisible := tab.mode == sourceMode || tab.mode == splitMode ||
		tab.mode != wysiwygMode && tab.maximizedPane == editorMaximized
	if !sourceVisible || tab.maximizedPane == previewMaximized {
		return false
	}
	reply = strings.ReplaceAll(reply, "\r\n", "\n")
	if reply == target.written {
		return true
	}
	if target.written != "" && !target.draft.Undo() {
		return false
	}
	// Insertion and removal are one step, even if the completed reply matches
	// the original selection after differing from it during streaming.
	target.draft.Edit(
		editor.Edit{Start: target.start, End: target.start, Text: reply},
		editor.Edit{Start: target.start, End: target.end},
	)
	target.version = target.draft.Version()
	target.written = reply
	caret := target.start + len(reply)
	target.code.Select(caret, caret)
	return true
}
