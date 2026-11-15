package ui

import (
	"time"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const documentAutoSaveDelay = 15 * time.Second // EV-UI-255

type workspaceAutoSave struct {
	draft   *editor.Buffer
	version uint64
	due     time.Time
}

func (app *workspaceWindow) observeAutoSave(c *mygoui.Context) {
	now := c.Now()
	var failure string
	// The host schedules a frame for each pending deadline. Only the current
	// owner considers a tab, including after moving it into another window.
	for _, tab := range app.tabs {
		state := &tab.autoSave
		if !app.canAutoSave(tab) {
			*state = workspaceAutoSave{}
			continue
		}
		version := tab.draft.Version()
		if state.draft != tab.draft || state.version != version {
			*state = workspaceAutoSave{draft: tab.draft, version: version}
			if tab.draft.Modified() {
				state.due = now.Add(documentAutoSaveDelay)
			}
		}
		if state.due.IsZero() {
			continue
		}
		if remaining := state.due.Sub(now); remaining > 0 {
			c.After(remaining)
			continue
		}
		// On modal dismissal the next frame can process an existing deadline.
		if app.modalOpen || app.closeConfirmed {
			continue
		}
		state.due = time.Time{}
		if !app.saveTab(tab, nil) {
			failure = app.problem
		}
	}
	if failure != "" {
		app.problem = failure
	}
}

func (app *workspaceWindow) canAutoSave(tab *workspaceTab) bool {
	return tab.draft != nil && !tab.inspection && workspace.IsMarkdownPath(tab.document.Path) &&
		!tab.document.NeedsRecreation() && (tab.storage != nil || app.vault != nil)
}
