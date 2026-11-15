package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type workspaceMentions struct {
	path       string
	index      *vaultindex.Index
	cancel     context.CancelFunc
	result     vaultindex.UnlinkedMentionResult
	ready      bool
	refreshing bool
}

func (app *workspaceWindow) unlinkedMentions(c *mygoui.Context) {
	path := app.currentVaultPath()
	if app.vault == nil || !workspace.IsMarkdownPath(path) {
		app.clearUnlinkedMentions()
		return
	}
	if path != app.mentions.path {
		app.clearUnlinkedMentions()
		app.mentions.path = path
	}
	if app.mentions.cancel == nil && !app.mentions.ready {
		app.requestUnlinkedMentions(150 * time.Millisecond)
	} else if app.mentions.ready && !app.mentions.refreshing && app.preferences.VaultMentionsExpanded && app.mentions.index != app.index.data {
		app.requestUnlinkedMentions(1500 * time.Millisecond)
	}
	app.drawUnlinkedMentions(c, app.mentions.result)
}

func (app *workspaceWindow) clearUnlinkedMentions() {
	if app.mentions.cancel != nil {
		app.mentions.cancel()
	}
	app.mentions = workspaceMentions{}
}

func (app *workspaceWindow) requestUnlinkedMentions(delay time.Duration) {
	state := &app.mentions
	if state.path == "" || app.win == nil {
		return
	}
	if state.cancel != nil {
		state.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	state.cancel, state.index = cancel, app.index.data
	vault, win, path := app.vault, app.win, state.path
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		result, err := vaultindex.ScanUnlinkedMentions(ctx, vault, path)
		win.Update(func() {
			if ctx.Err() != nil || app.vault != vault || win.IsDestroyed() || app.mentions.path != path {
				return
			}
			app.mentions.cancel, app.mentions.ready, app.mentions.refreshing = nil, true, false
			cancel()
			if err != nil {
				app.mentions.result = vaultindex.UnlinkedMentionResult{}
				return
			}
			app.mentions.result = result
		})
	}()
}

func (app *workspaceWindow) toggleUnlinkedMentions() {
	app.preferences.VaultMentionsExpanded = !app.preferences.VaultMentionsExpanded
	app.writePreferences()
	for _, window := range app.documentWindows() {
		if window.win != nil && window != app {
			window.win.Invalidate()
		}
	}
}

func (app *workspaceWindow) openUnlinkedMention(path string) {
	app.openDocument(path)
}

func (app *workspaceWindow) linkUnlinkedMentions(hits []vaultindex.UnlinkedMentionHit) {
	state := &app.mentions
	if app.vault == nil || app.modalOpen || state.refreshing || state.path != app.currentVaultPath() || len(hits) == 0 {
		return
	}
	delay := 150 * time.Millisecond
	if state.result.Truncated && len(hits) >= 2 && len(hits) == len(state.result.Hits) {
		delay = time.Second
	}
	sources := make(map[string][]vaultindex.UnlinkedMentionHit)
	var paths []string
	for _, hit := range hits {
		if _, exists := sources[hit.Path]; !exists {
			paths = append(paths, hit.Path)
		}
		sources[hit.Path] = append(sources[hit.Path], hit)
	}
	var failures []string
	linked := make(map[vaultindex.UnlinkedMentionHit]bool, len(hits))
	for _, path := range paths {
		if err := app.linkUnlinkedSource(path, sources[path]); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		for _, hit := range sources[path] {
			linked[hit] = true
		}
	}
	state.result.Hits = slices.DeleteFunc(state.result.Hits, func(hit vaultindex.UnlinkedMentionHit) bool { return linked[hit] })
	state.result.Total = len(state.result.Hits)
	if len(linked) != 0 {
		for _, window := range app.documentWindows() {
			if window.root == app.root {
				window.requestIndex()
			}
		}
	}
	if len(failures) != 0 {
		app.problem = fmt.Sprintf("Could not link mentions in %s.", failures[0])
		if more := len(failures) - 1; more > 0 {
			app.problem += fmt.Sprintf(" %d other notes could not be updated.", more)
		}
	} else {
		app.problem = ""
	}
	state.refreshing = true
	app.requestUnlinkedMentions(delay)
}

type openMentionSource struct {
	window *workspaceWindow
	tab    *workspaceTab
	edits  []editor.Edit
}

func (app *workspaceWindow) linkUnlinkedSource(path string, hits []vaultindex.UnlinkedMentionHit) error {
	doc, err := app.vault.ReadDocument(path)
	if err != nil {
		return err
	}
	text, err := vaultindex.ApplyUnlinkedMentions(doc, hits)
	if err != nil {
		return err
	}
	absolute := filepath.Join(app.root, filepath.FromSlash(path))
	var open []openMentionSource
	for _, window := range app.documentWindows() {
		for _, tab := range window.tabs {
			if window.absoluteDocument(tab) != absolute || tab.draft == nil {
				continue
			}
			if tab == window.workspaceTab {
				window.wys.flush(window)
			}
			if tab.dirty() {
				return errors.New("save or close its edited tab before linking")
			}
			edits, err := vaultindex.UnlinkedMentionBufferEdits(doc, tab.draft.String(), hits)
			if err != nil {
				return err
			}
			source := openMentionSource{window: window, tab: tab, edits: make([]editor.Edit, 0, len(edits))}
			for _, edit := range edits {
				source.edits = append(source.edits, editor.Edit{Start: edit.Start, End: edit.End, Text: edit.Text})
			}
			open = append(open, source)
		}
	}
	saved, err := app.vault.SaveDocument(doc, text)
	if err != nil {
		return err
	}
	for _, source := range open {
		window, tab := source.window, source.tab
		storage := tab.storage
		if storage == nil {
			storage = window.vault
		}
		revision := saved
		if storage != app.vault {
			var readErr error
			revision, readErr = storage.ReadDocument(tab.document.Path)
			if readErr != nil || revision.Text != saved.Text {
				continue
			}
		}
		tab.draft.Edit(source.edits...)
		tab.draft.MarkSaved()
		tab.document = revision
		if tab == window.workspaceTab {
			window.syncDocumentEdited()
			window.win.Invalidate()
		}
	}
	return nil
}
