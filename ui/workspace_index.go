package ui

import (
	"context"
	"path"
	"reflect"
	"strings"
	"time"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
)

type workspaceIndex struct {
	data        *vaultindex.Index
	attachment  uint64
	generation  uint64
	building    bool
	scanning    bool
	pendingHint bool
	stopWatch   context.CancelFunc
}

func (app *workspaceWindow) requestIndex() {
	app.index.generation++
	app.buildIndex()
	app.syncAIVaultContext()
}

func (app *workspaceWindow) buildIndex() {
	if app.vault == nil || app.win == nil || app.index.building {
		return
	}
	app.index.building = true
	vault, entries, win := app.vault, app.entries, app.win
	attachment, generation := app.index.attachment, app.index.generation
	go func() {
		index, err := vaultindex.Build(vault, entries)
		win.Update(func() {
			if win.IsDestroyed() || app.vault != vault || app.index.attachment != attachment {
				return
			}
			app.index.building = false
			if app.index.generation != generation {
				app.buildIndex()
				return
			}
			if err == nil {
				app.index.data = index
				app.previewPane.refreshPending = true
				app.invalidateGraphWindow()
			}
		})
	}()
}

func readWorkspaceEntries(vault *workspace.Vault, all bool) ([]workspace.Entry, int, error) {
	entries, err := vault.SidebarEntries(all)
	if err != nil {
		return nil, 0, err
	}
	return entries, countVaultFiles(entries), nil
}

func countVaultFiles(entries []workspace.Entry) int {
	count := 0
	for _, entry := range entries {
		if entry.IsDir {
			count += countVaultFiles(entry.Children)
		} else {
			count++
		}
	}
	return count
}

func (app *workspaceWindow) startVaultWatch() {
	if app.vault == nil || app.win == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	app.index.stopWatch = cancel
	win, root := app.win, app.root
	scan := func(hint bool) {
		mygo.RunOnMain(func() {
			if ctx.Err() == nil && !win.IsDestroyed() {
				app.scanVault(hint)
			}
		})
	}
	go func() {
		// An index refresh only needs a dirty directory. A write can preserve
		// every file attribute seen by the polling scan (EV-UI-188).
		watch, err := mygo.WatchFiles(ctx, root, mygo.FileWatchOptions{
			Recursive: true, HintsOnly: true,
			Debounce: 500 * time.Millisecond, MaxDelay: time.Second,
			Ignore: func(relative string, _ bool) bool {
				return strings.HasPrefix(path.Base(relative), ".")
			},
		})
		if err != nil {
			pollVault(ctx, scan)
			return
		}
		// Refresh once the watch is enrolled to cover the interval between
		// the initial directory snapshot and the first observable event.
		scan(true)
		for {
			if _, err := watch.Next(ctx); err != nil {
				break
			}
			scan(true)
		}
		watch.Close()
		pollVault(ctx, scan)
	}()
}

func pollVault(ctx context.Context, scan func(bool)) {
	ticks := time.NewTicker(2 * time.Second)
	defer ticks.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks.C:
			scan(false)
		}
	}
}

func (app *workspaceWindow) scanVault(hint bool) {
	if app.index.scanning {
		app.index.pendingHint = app.index.pendingHint || hint
		return
	}
	vault, win := app.vault, app.win
	attachment, generation := app.index.attachment, app.index.generation
	all := app.preferences.VaultShowAllFiles
	var chartFiles *previewChartFiles
	if app.previewPane.tab == app.workspaceTab && app.previewPane.path == app.document.Path {
		chartFiles = app.previewPane.chartFiles
	}
	app.index.scanning = true
	go func() {
		entries, count, err := readWorkspaceEntries(vault, all)
		chartChanged := chartFiles.changed()
		mygo.RunOnMain(func() {
			if win.IsDestroyed() || app.vault != vault || app.index.attachment != attachment {
				return
			}
			app.index.scanning = false
			if chartChanged && app.previewPane.chartFiles == chartFiles {
				app.previewPane.refreshPending = true
				win.Invalidate()
			}
			if generation != app.index.generation {
				// A newer explicit refresh or save may have superseded this
				// scan. Carry its filesystem hint to the current snapshot.
				app.index.pendingHint = app.index.pendingHint || hint
			} else if err == nil {
				changed := !reflect.DeepEqual(app.entries, entries)
				if changed {
					app.vaultFileCount = count
					app.updateEntries(entries)
					win.Invalidate()
				}
				if changed || hint {
					app.requestIndex()
				}
			}
			if app.index.pendingHint {
				app.index.pendingHint = false
				app.scanVault(true)
			}
		})
	}()
}
