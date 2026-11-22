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
	"object-na.local/vaultdelta"
)

type workspaceIndex struct {
	data         *vaultindex.Index
	wikiImages   *wikiImageNames
	wikiPDFs     *wikiPDFIndex
	attachment   uint64
	generation   uint64
	building     bool
	scanning     bool
	pendingHints vaultdelta.Scope
	stopWatch    context.CancelFunc
	bodyCache    string
	cancelBody   context.CancelFunc
}

func (app *workspaceWindow) requestIndex() {
	app.index.generation++
	app.buildIndex(app.index.data)
	app.refreshSidebarBodyIndex()
	app.syncAIVaultContext()
}

func (app *workspaceWindow) buildIndex(previous *vaultindex.Index) {
	if app.vault == nil || app.win == nil || app.index.building {
		return
	}
	app.index.building = true
	vault, entries, win := app.vault, app.entries, app.win
	attachment, generation := app.index.attachment, app.index.generation
	go func() {
		index, err := previous.RefreshMetadata(vault, entries)
		win.Update(func() {
			if win.IsDestroyed() || app.vault != vault || app.index.attachment != attachment {
				return
			}
			app.index.building = false
			if app.index.generation != generation {
				// The intermediate result stays private. Its per-file
				// read stamps can still be reused by the pending scan.
				app.buildIndex(index)
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
	scan := func(hints vaultdelta.Scope) {
		mygo.RunOnMain(func() {
			if ctx.Err() == nil && !win.IsDestroyed() {
				app.scanVault(hints)
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
		if err == nil {
			// Reconcile the interval between the initial snapshot and enrollment.
			scan(vaultdelta.Hints("."))
			for {
				events, err := watch.Next(ctx)
				if err != nil {
					break
				}
				directories := make([]string, 0, len(events))
				for _, event := range events {
					if event.Op != mygo.FileHint || !event.IsDir {
						directories = []string{"."}
						break
					}
					directories = append(directories, event.Path)
				}
				if len(directories) == 0 {
					directories = append(directories, ".")
				}
				scan(vaultdelta.Hints(directories...))
			}
			watch.Close()
		}
		// Reconcile any changes lost before switching to polling.
		scan(vaultdelta.Hints("."))
		pollVault(ctx, scan)
	}()
}

func pollVault(ctx context.Context, scan func(vaultdelta.Scope)) {
	ticks := time.NewTicker(2 * time.Second)
	defer ticks.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks.C:
			scan(vaultdelta.Scope{})
		}
	}
}

func (app *workspaceWindow) scanVault(hints vaultdelta.Scope) {
	if app.index.scanning {
		app.index.pendingHints = app.index.pendingHints.Merge(hints)
		return
	}
	vault, win := app.vault, app.win
	previous := app.entries
	attachment, generation := app.index.attachment, app.index.generation
	all := app.preferences.VaultShowAllFiles
	var chartFiles *previewChartFiles
	if app.previewPane.tab == app.workspaceTab && app.previewPane.path == app.document.Path {
		chartFiles = app.previewPane.chartFiles
	}
	app.index.scanning = true
	go func() {
		entries, _, err := vault.RefreshSidebarEntries(previous, all, hints)
		changed := err == nil && !reflect.DeepEqual(previous, entries)
		var count int
		if changed {
			count = countVaultFiles(entries)
		}
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
				// scan. Apply its hints again to the current snapshot.
				app.index.pendingHints = app.index.pendingHints.Merge(hints)
			} else if err == nil {
				if changed {
					app.vaultFileCount = count
					app.updateEntries(entries)
					win.Invalidate()
				}
				if changed || !hints.Empty() {
					app.requestIndex()
				}
			}
			if pending := app.index.pendingHints; !pending.Empty() {
				app.index.pendingHints = vaultdelta.Scope{}
				app.scanVault(pending)
			}
		})
	}()
}
