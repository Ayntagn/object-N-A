package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
)

func (app *workspaceWindow) generateVaultHealth() {
	if app.vault == nil || app.modalOpen {
		return
	}
	for _, window := range app.documentWindows() {
		if window.root == app.root && window.vaultHealthBuilding {
			return
		}
	}
	app.vaultHealthBuilding = true
	vault, win, name := app.vault, app.win, filepath.Base(app.root)
	go func() {
		source, err := vaultindex.HealthReport(vault, name, time.Now())
		win.Update(func() {
			if app.vault != vault || win.IsDestroyed() {
				return
			}
			app.vaultHealthBuilding = false
			if err != nil {
				app.problem = fmt.Sprintf("Could not generate Vault health report. %v", err)
				return
			}
			app.writeVaultHealth(source)
		})
	}()
}

func (app *workspaceWindow) writeVaultHealth(source string) {
	if !app.vaultHealthCanWrite() {
		return
	}
	vault := app.vault
	current, err := vault.ReadDocument(workspace.VaultHealthFile)
	if errors.Is(err, fs.ErrNotExist) {
		created, createErr := vault.CreateDocument(workspace.VaultHealthFile, source)
		app.finishVaultHealth(created, createErr)
		return
	}
	if err != nil {
		app.finishVaultHealth(workspace.Document{}, err)
		return
	}
	save := func() {
		if app.vault != vault || !app.vaultHealthCanWrite() {
			return
		}
		doc, err := vault.SaveDocument(current, source)
		app.finishVaultHealth(doc, err)
	}
	if !vaultindex.IsGeneratedHealthReport(current.Text) {
		app.confirm("Replace Vault Health.md?",
			"Replace the existing note with the Vault health report?",
			"Replace", save)
		return
	}
	save()
}

func (app *workspaceWindow) vaultHealthCanWrite() bool {
	if app.modalOpen {
		return false
	}
	absolute := filepath.Join(app.root, workspace.VaultHealthFile)
	for _, window := range app.documentWindows() {
		for _, tab := range window.tabs {
			if window.absoluteDocument(tab) != absolute {
				continue
			}
			if tab == window.workspaceTab {
				window.wys.flush(window)
			}
			if tab.dirty() {
				app.problem = "Save or close the edited Vault Health.md before regenerating it."
				return false
			}
		}
	}
	return true
}

func (app *workspaceWindow) finishVaultHealth(doc workspace.Document, err error) {
	if err != nil {
		app.problem = fmt.Sprintf("Could not write Vault health report. %v", err)
		return
	}
	app.refreshVault()
	var reportWindow *workspaceWindow
	var reportTab *workspaceTab
	absolute := filepath.Join(app.root, workspace.VaultHealthFile)
	for _, window := range app.documentWindows() {
		for _, tab := range window.tabs {
			if window.absoluteDocument(tab) != absolute {
				continue
			}
			storage := window.vault
			if tab.storage != nil {
				storage = tab.storage
			}
			if storage == app.vault {
				window.setTabDocument(tab, doc)
			} else if updated, readErr := storage.ReadDocument(doc.Path); readErr == nil {
				window.setTabDocument(tab, updated)
			}
			if tab == window.workspaceTab {
				window.activateTab(tab)
				window.win.Invalidate()
			}
			if reportWindow == nil || window == app {
				reportWindow, reportTab = window, tab
			}
		}
	}
	if reportWindow == nil {
		app.openDocument(doc.Path)
		return
	}
	if reportTab != reportWindow.workspaceTab {
		reportWindow.activateTab(reportTab)
		reportWindow.win.Invalidate()
	}
	if reportWindow != app {
		reportWindow.win.Focus()
	}
}
