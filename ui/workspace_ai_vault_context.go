package ui

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"cyberwriter/internal/core/vaultcontext"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
)

type workspaceAIVaultContext struct {
	root          string
	legacyRoots   map[string]map[string]aiVaultFileStamp
	clearRevision uint64
	index         *vaultcontext.Index
	files         map[string]aiVaultFileStamp
	indexed       map[string]aiVaultFileStamp
	builtAt       time.Time
	buildDuration time.Duration
	skipped       []vaultcontext.MapSkippedFile
	loaded        bool
	request       *workspaceAIVaultBuild
	problem       string
	assetsMissing bool
	assetFailure  time.Time
}

type aiVaultRequestKind uint8

const (
	aiVaultBuild aiVaultRequestKind = iota
	aiVaultLoad
	aiVaultPrune
	aiVaultDownload
)

type workspaceAIVaultBuild struct {
	cancel    context.CancelFunc
	kind      aiVaultRequestKind
	automatic bool
	progress  vaultcontext.BuildProgress
}

func (app *workspaceWindow) aiVaultContextRoot() string {
	if app.vault == nil {
		return ""
	}
	if app.vaultEmbeddingOfferPath != "" {
		return app.vaultEmbeddingOfferPath
	}
	return normalizedVaultEmbeddingOfferPath(app.root)
}

// One published, immutable index is shared by all spellings of a mounted Vault.
func (app *workspaceWindow) aiVaultContextState() *workspaceAIVaultContext {
	root := app.aiVaultContextRoot()
	if root == "" {
		return nil
	}
	var state *workspaceAIVaultContext
	if a := app.application; a != nil {
		state = a.aiVaultContexts[root]
		if state == nil {
			if a.aiVaultContexts == nil {
				a.aiVaultContexts = make(map[string]*workspaceAIVaultContext)
			}
			state = &workspaceAIVaultContext{root: root}
			a.aiVaultContexts[root] = state
		}
	} else {
		if app.aiVaultContext == nil {
			app.aiVaultContext = &workspaceAIVaultContext{root: root}
		}
		state = app.aiVaultContext
	}
	if app.root != root {
		if state.legacyRoots == nil {
			state.legacyRoots = make(map[string]map[string]aiVaultFileStamp)
		}
		if _, seen := state.legacyRoots[app.root]; !seen {
			state.legacyRoots[app.root] = nil
		}
	}
	return state
}

func (app *workspaceWindow) detachAIVaultContext(root string) {
	if root == "" {
		return
	}
	if a := app.application; a != nil {
		for _, window := range a.windows {
			if window.aiVaultContextRoot() == root {
				return
			}
		}
		if state := a.aiVaultContexts[root]; state != nil {
			state.cancel()
			delete(a.aiVaultContexts, root)
		}
	} else if state := app.aiVaultContext; state != nil && state.root == root {
		state.cancel()
		app.aiVaultContext = nil
	}
}

func (state *workspaceAIVaultContext) cancel() {
	if state.request != nil {
		state.request.cancel()
		state.request = nil
	}
}

// EV-UI-289 / EV-UI-292. The native list includes Markdown and TXT, omits
// the generated Vault-root statistics report, and ignores attachments.
func aiVaultContextFiles(entries []workspace.Entry) map[string]aiVaultFileStamp {
	files := make(map[string]aiVaultFileStamp)
	var scan func([]workspace.Entry)
	scan = func(entries []workspace.Entry) {
		for _, entry := range entries {
			if entry.IsDir {
				scan(entry.Children)
			} else if entry.Path != workspace.VaultStatsFile &&
				(workspace.IsMarkdownPath(entry.Path) || strings.EqualFold(path.Ext(entry.Path), ".txt")) {
				files[entry.Path] = aiVaultFileStamp{Modified: entry.ModTime().UnixNano(), Size: entry.Size()}
			}
		}
	}
	scan(entries)
	return files
}

func (app *workspaceWindow) recommendedAIVaultContextDepth() vaultcontext.Depth {
	if runtime.GOOS != "darwin" {
		return vaultcontext.Quick
	}
	var count int
	if state := app.aiVaultContextState(); state != nil && state.files != nil {
		count = len(state.files)
	} else {
		count = len(aiVaultContextFiles(app.entries))
	}
	switch {
	case count < 100:
		return vaultcontext.Deep
	case count < 1500:
		return vaultcontext.Balanced
	default:
		return vaultcontext.Quick
	}
}

func (app *workspaceWindow) aiVaultContextDepth() vaultcontext.Depth {
	switch depth := app.preferences.AIVaultContextDepth; depth {
	case vaultcontext.Quick, vaultcontext.Balanced, vaultcontext.Deep:
		return depth
	}
	return app.recommendedAIVaultContextDepth()
}

func (app *workspaceWindow) setAIVaultContextDepth(depth vaultcontext.Depth, rebuild bool) {
	switch depth {
	case vaultcontext.Quick, vaultcontext.Balanced, vaultcontext.Deep:
	default:
		return
	}
	app.preferences.AIVaultContextDepth = depth
	app.preferencesChanged()
	if rebuild {
		app.buildAIVaultContext()
	}
}

func (p workspacePreferences) aiVaultContextAutoUpdate() bool {
	return p.AIVaultContextAutoUpdate == nil || *p.AIVaultContextAutoUpdate
}

func (app *workspaceWindow) syncAIVaultContext() {
	if app.win == nil || app.win.IsDestroyed() {
		return
	}
	state := app.aiVaultContextState()
	if state == nil {
		return
	}
	state.files = aiVaultContextFiles(app.entries)
	if state.loaded {
		app.autoUpdateAIVaultContext(state)
		return
	}
	state.loaded = true
	root, mounted, files := state.root, app.root, state.files
	ctx, cancel := context.WithCancel(context.Background())
	request := &workspaceAIVaultBuild{cancel: cancel, kind: aiVaultLoad}
	state.request = request
	go func() {
		defer cancel()
		cache, legacy, err := readAIVaultContextCacheForMount(root, mounted, files)
		var target, stage string
		var migrationErr error
		if legacy && ctx.Err() == nil {
			target, stage, migrationErr = stageAIVaultContextCache(root, cache)
		}
		if ctx.Err() != nil {
			if stage != "" {
				_ = os.Remove(stage)
			}
			return
		}
		mygo.RunOnMain(func() {
			if stage != "" {
				defer os.Remove(stage)
			}
			if state.request != request {
				return
			}
			state.request = nil
			if err != nil {
				state.problem = fmt.Sprintf("Could not read the Vault index. %v", err)
			} else {
				if legacy {
					state.legacyRoots[mounted] = cache.Files
				}
				if stage != "" {
					migrationErr = os.Rename(stage, target)
				}
				if migrationErr != nil {
					state.problem = fmt.Sprintf("Could not migrate the Vault index. %v", migrationErr)
				}
				state.index, state.indexed, state.builtAt = cache.Index, cache.Files, cache.BuiltAt
				state.buildDuration = cache.BuildDuration
				app.autoUpdateAIVaultContext(state)
			}
			app.invalidateAIVaultContext(root)
		})
	}()
}

func (app *workspaceWindow) autoUpdateAIVaultContext(state *workspaceAIVaultContext) {
	if state.index == nil || state.request != nil || maps.Equal(state.files, state.indexed) {
		return
	}
	if app.pruneAIVaultContext(state) {
		return
	}
	if app.preferences.aiVaultContextAutoUpdate() &&
		(state.assetFailure.IsZero() || time.Since(state.assetFailure) >= 30*time.Second) {
		app.startAIVaultContextBuild(state, state.index.Depth(), true)
	}
}

// EV-UI-298. Deletions update the snapshot independently of Auto-update.
// Retain the old stamps for the surviving source so new and changed files
// are still pending when Auto-update is disabled.
func (app *workspaceWindow) pruneAIVaultContext(state *workspaceAIVaultContext) bool {
	files := make(map[string]aiVaultFileStamp, len(state.indexed))
	for path, stamp := range state.indexed {
		if _, present := state.files[path]; present {
			files[path] = stamp
		}
	}
	if len(files) == len(state.indexed) {
		return false
	}
	previous := state.index
	var paths []string
	for _, path := range previous.Paths() {
		if _, present := files[path]; present {
			paths = append(paths, path)
		}
	}
	var skips []vaultcontext.MapSkippedFile
	for _, skip := range state.skipped {
		if _, present := files[skip.Path]; present {
			skips = append(skips, skip)
		}
	}
	app.startAIVaultIndexTask(state, &workspaceAIVaultBuild{kind: aiVaultPrune}, files, func(ctx context.Context) (*vaultcontext.Index, []vaultcontext.MapSkippedFile, error) {
		index, err := vaultcontext.Prune(ctx, previous, paths)
		return index, skips, err
	})
	return true
}

func (app *workspaceWindow) buildAIVaultContext() {
	state := app.aiVaultContextState()
	if state == nil || state.request != nil || app.win == nil || app.win.IsDestroyed() {
		return
	}
	state.loaded = true
	state.files = aiVaultContextFiles(app.entries)
	app.startAIVaultContextBuild(state, app.aiVaultContextDepth(), false)
}

func (app *workspaceWindow) startAIVaultContextBuild(state *workspaceAIVaultContext, depth vaultcontext.Depth, automatic bool) {
	owner := app
	for _, window := range app.documentWindows() {
		if window.aiVaultContextRoot() == state.root && window.win != nil && !window.win.IsDestroyed() {
			owner = window
			break
		}
	}
	if owner.aiVaultContextRoot() != state.root {
		return
	}
	snapshot := aiVaultBuildSnapshot{
		vault: owner.vault, files: state.files, depth: depth, previous: state.index,
		previousFiles: state.indexed, previousSkips: state.skipped,
	}
	request := &workspaceAIVaultBuild{kind: aiVaultBuild, automatic: automatic}
	app.startAIVaultIndexTask(state, request, snapshot.files, snapshot.build)
}

func (app *workspaceWindow) startAIVaultIndexTask(state *workspaceAIVaultContext, request *workspaceAIVaultBuild, files map[string]aiVaultFileStamp,
	work func(context.Context) (*vaultcontext.Index, []vaultcontext.MapSkippedFile, error)) {
	root, previous, builtAt, duration := state.root, state.index, state.builtAt, state.buildDuration
	kind := request.kind
	ctx, cancel := context.WithCancel(context.Background())
	request.cancel = cancel
	if kind == aiVaultBuild {
		request.progress.Total = len(files)
		ctx = vaultcontext.WithBuildProgress(ctx, func(progress vaultcontext.BuildProgress) {
			mygo.RunOnMain(func() {
				if state.request != request {
					return
				}
				request.progress = progress
				for _, window := range app.documentWindows() {
					if window.aiVaultContextRoot() == root {
						window.invalidateAISettings()
						if window.showingAI() && !request.automatic {
							window.invalidateAIChat()
						}
					}
				}
			})
		})
	}
	state.request, state.problem = request, ""
	app.invalidateAIVaultContext(state.root)
	go func() {
		defer cancel()
		started := time.Now()
		index, skipped, err := work(ctx)
		changed := kind == aiVaultPrune || index != previous
		cache := aiVaultContextCache{Index: index, Files: files, BuiltAt: builtAt, BuildDuration: duration}
		if kind == aiVaultBuild {
			cache.BuiltAt, cache.BuildDuration = time.Now(), time.Since(started)
		}
		var target, stage string
		if err == nil && changed && ctx.Err() == nil {
			target, stage, err = stageAIVaultContextCache(root, cache)
		}
		if ctx.Err() != nil {
			if stage != "" {
				_ = os.Remove(stage)
			}
			return
		}
		mygo.RunOnMain(func() {
			if stage != "" {
				defer os.Remove(stage)
			}
			if state.request != request {
				return
			}
			state.request = nil
			if err == nil && changed {
				if kind == aiVaultBuild && aiVaultPathsDisappeared(files, state.files) {
					app.startAIVaultContextBuild(state, index.Depth(), request.automatic)
					return
				}
				err = os.Rename(stage, target)
			}
			if kind == aiVaultBuild {
				state.assetsMissing = errors.Is(err, errVaultContextualAssetsMissing)
				if err == nil {
					state.assetFailure = time.Time{}
				}
			}
			if err != nil {
				if !errors.Is(err, errVaultContextualAssetsMissing) {
					state.problem = fmt.Sprintf("Could not update the Vault index. %v", err)
				}
			} else {
				if changed {
					state.index, state.indexed, state.builtAt = cache.Index, cache.Files, cache.BuiltAt
					state.buildDuration, state.skipped = cache.BuildDuration, skipped
				}
				app.autoUpdateAIVaultContext(state)
			}
			app.invalidateAIVaultContext(root)
		})
	}()
}

func aiVaultPathsDisappeared(old, current map[string]aiVaultFileStamp) bool {
	for path := range old {
		if _, present := current[path]; !present {
			return true
		}
	}
	return false
}

func (app *workspaceWindow) clearAIVaultContext() {
	state := app.aiVaultContextState()
	if state == nil {
		return
	}
	state.cancel()
	if err := clearAIVaultContextCache(state.root, state.files, state.legacyRoots); err != nil {
		state.problem = fmt.Sprintf("Could not clear the Vault index. %v", err)
	} else {
		*state = workspaceAIVaultContext{
			root: state.root, legacyRoots: state.legacyRoots, clearRevision: state.clearRevision + 1,
			files: state.files, loaded: true,
		}
	}
	app.invalidateAIVaultContext(state.root)
}

func (app *workspaceWindow) cancelAIVaultContextBuild() {
	state := app.aiVaultContextState()
	if state != nil && state.request != nil && state.request.kind == aiVaultBuild {
		state.cancel()
		app.invalidateAIVaultContext(state.root)
	}
}

func (app *workspaceWindow) invalidateAIVaultContext(root string) {
	// EV-UI-318: under Settings, Files keeps its last concept-search frame
	// until its document window becomes active. Index tasks still use live state.
	settingsFocused := app.settings != nil && mygo.FocusedWindow() == app.settings
	for _, window := range app.documentWindows() {
		if window.aiVaultContextRoot() == root {
			if settingsFocused {
				window.sidebarSearch.relatedRefreshDeferred = true
			}
			window.dropOutdatedVaultRelatedSearch()
			window.invalidateAIChat()
			window.invalidateAISettings()
		}
	}
}
