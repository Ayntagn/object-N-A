package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"cyberwriter/internal/core/vaultcontext"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
)

// EV-UI-295. The generated Markdown opens as a new untitled document.
// The temporary file path is the fallback when no document window is available.
func (app *workspaceWindow) showAIVaultMap() {
	state := app.aiVaultContextState()
	if state == nil || state.index == nil || app.aiVaultMapBusy || app.win == nil || app.win.IsDestroyed() {
		return
	}
	app.aiVaultMapBusy, app.aiVaultMapProblem = true, ""
	app.invalidateAISettings()
	index, indexed, duration, skipped := state.index, state.indexed, state.buildDuration, state.skipped
	indexRoot, clearRevision := state.root, state.clearRevision
	root, vault, win := app.root, app.vault, app.win
	settings := app.settings
	go func() {
		file, _ := aiVaultContextCacheFile(indexRoot)
		options := vaultcontext.MapReportOptions{
			VaultRoot: root, IndexPath: file, GeneratedAt: time.Now(), BuildDuration: duration,
			ModifiedAt: make(map[string]time.Time, len(indexed)), Skipped: skipped,
		}
		if info, err := os.Stat(file); err == nil {
			options.IndexSize = info.Size()
		}
		for path, stamp := range indexed {
			options.ModifiedAt[path] = time.Unix(0, stamp.Modified)
		}
		report := index.VaultMapMarkdown(options)
		win.Update(func() {
			app.aiVaultMapBusy = false
			if win.IsDestroyed() {
				return
			}
			defer app.invalidateAISettings()
			// Clear retires pending reports from this index, including when a
			// fresh build completes before the report is delivered.
			if app.vault != vault || app.aiVaultContextState() != state || state.clearRevision != clearRevision ||
				settings != nil && (app.settings != settings || settings.IsDestroyed()) {
				return
			}
			if !app.openAIVaultMapDraft(report) {
				if err := openTemporaryAIVaultMap(report); err != nil {
					app.aiVaultMapProblem = fmt.Sprintf("Could not open the Vault embeddings map. %v", err)
				}
			}
		})
	}()
}

func (app *workspaceWindow) openAIVaultMapDraft(markdown string) bool {
	if app.application == nil || app.win == nil || app.win.IsDestroyed() {
		return false
	}
	tab := app.blankTab()
	tab.mode = app.preferences.DefaultView
	app.setTabDocument(tab, workspace.Document{})
	tab.draft.Set(markdown)
	tab.code.Select(0, 0)
	next := app.application.newWindow(tab, app)
	if next.win.IsDestroyed() {
		return false
	}
	next.win.Show()
	app.application.activate(next)
	return true
}

func openTemporaryAIVaultMap(markdown string) error {
	file, err := os.CreateTemp("", "vault-map-*.md")
	if err != nil {
		return err
	}
	filename := file.Name()
	_, writeErr := io.WriteString(file, markdown)
	err = errors.Join(writeErr, file.Close())
	if err == nil {
		err = mygo.Shell.OpenPath(filename)
	}
	if err != nil {
		_ = os.Remove(filename)
	}
	return err
}
