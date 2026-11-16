package ui

import (
	"context"
	"time"

	"cyberwriter/internal/core/vaultcontext"
	"cyberwriter/internal/core/vaultindex"
)

const vaultRelatedDebounce = 250 * time.Millisecond

type vaultRelatedSearch struct {
	index   *vaultcontext.Index
	cancel  context.CancelFunc
	matches []vaultcontext.Match
}

type vaultRelatedPresentation struct {
	buildHint bool
	matches   []vaultcontext.Match
}

func (app *workspaceWindow) presentVaultRelatedSearch() vaultRelatedPresentation {
	state := &app.sidebarSearch
	if state.relatedRefreshDeferred {
		return state.presentedRelated
	}
	var presentation vaultRelatedPresentation
	index := app.aiVaultContextState()
	switch {
	case index != nil && index.request != nil:
	case index == nil || index.index == nil:
		presentation.buildHint = true
	case state.related != nil && state.related.index == index.index:
		presentation.matches = state.related.matches
	}
	state.presentedRelated = presentation
	return presentation
}

func (s *vaultRelatedSearch) stop() {
	if s != nil && s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// EV-UI-187 / EV-UI-294 / EV-UI-296. Files runs a separate local query
// after a 250 ms wait. It uses the same five matched chunks as AI Chat.
func (app *workspaceWindow) startVaultRelatedSearch(input, query string) {
	previous := app.sidebarSearch.related
	previous.stop()
	app.sidebarSearch.related = nil
	if vaultindex.SidebarSearchQueryLength(query) < 3 {
		return
	}
	state := app.aiVaultContextState()
	if state == nil || state.index == nil || state.request != nil {
		return
	}
	search := &vaultRelatedSearch{index: state.index}
	// EV-UI-313. Carry the published matches while the next query waits on the same index.
	if previous != nil && previous.index == search.index {
		search.matches = previous.matches
	}
	app.sidebarSearch.related = search
	win, vault := app.win, app.vault
	if win == nil {
		search.matches, _ = searchAIVaultIndex(context.Background(), search.index, query)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	search.cancel = cancel
	go func() {
		timer := time.NewTimer(vaultRelatedDebounce)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		matches, _ := searchAIVaultIndex(ctx, search.index, query)
		if ctx.Err() != nil {
			return
		}
		win.Update(func() {
			defer cancel()
			if ctx.Err() != nil || win.IsDestroyed() || app.vault != vault ||
				app.fileSearch != input || app.sidebarSearch.related != search || state.index != search.index {
				return
			}
			search.cancel, search.matches = nil, matches
			app.publishVaultSearchGraph()
		})
	}()
}

func (app *workspaceWindow) dropOutdatedVaultRelatedSearch() {
	search := app.sidebarSearch.related
	if search == nil {
		return
	}
	if state := app.aiVaultContextState(); state != nil && state.index == search.index {
		return
	}
	// EV-UI-318: a new index state invalidates the sidebar rows. Graph keeps
	// the last search emphasis until the next query publishes a result.
	search.stop()
	app.sidebarSearch.related = nil
}

// EV-UI-187. Both canvases use the maximum of the lexical weight and the
// relative Concept Search score. The sidebar keeps the ranked chunks.
func (app *workspaceWindow) publishVaultSearchGraph() {
	results, related := app.sidebarSearch.results, app.sidebarSearch.related
	highlight := make(map[string]float64, len(results))
	for _, result := range results {
		highlight[result.Entry.Path] = 1
	}
	if related != nil && len(related.matches) != 0 {
		best := float32(related.matches[0].Score)
		for _, hit := range related.matches {
			weight := float64(float32(hit.Score) / best)
			highlight[hit.Path] = max(highlight[hit.Path], weight)
		}
	}
	if len(highlight) == 0 {
		highlight = nil
	}
	app.graph.highlight = highlight
	app.invalidateGraphWindow()
}
