package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const welcomeDocumentLimit = 8

// EV-UI-254: Welcome Back lists recently used documents across windows.
func (app *workspaceWindow) recordWelcomeDocument(tab *workspaceTab) {
	if app.application == nil {
		return
	}
	filename := app.absoluteDocument(tab)
	recent := app.preferences.RecentDocuments
	if !filepath.IsAbs(filename) || len(recent) > 0 && recent[0] == filename {
		return
	}
	recent = slices.DeleteFunc(slices.Clone(recent), func(previous string) bool { return previous == filename })
	app.preferences.RecentDocuments = append([]string{filename}, recent[:min(len(recent), welcomeDocumentLimit-1)]...)
}

func (app *workspaceWindow) welcomeDocuments() []workspaceWelcomeDocument {
	recent := slices.Clone(app.preferences.RecentDocuments)
	// Existing preference files already track notes visited in the current Vault.
	for _, relative := range app.recentPaths {
		filename := filepath.Join(app.root, filepath.FromSlash(relative))
		if !slices.Contains(recent, filename) {
			recent = append(recent, filename)
		}
	}
	var documents []workspaceWelcomeDocument
	for _, filename := range recent {
		if !filepath.IsAbs(filename) {
			continue
		}
		info, err := os.Stat(filename)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		base := filepath.Base(filename)
		documents = append(documents, workspaceWelcomeDocument{
			Path: filename, Title: strings.TrimSuffix(base, filepath.Ext(base)), Folder: filepath.Dir(filename),
		})
		if len(documents) == welcomeDocumentLimit {
			break
		}
	}
	return documents
}

func (app *workspaceWindow) openWelcomeDocument(filename string) {
	for _, window := range app.documentWindows() {
		if window.win == nil || window.win.IsDestroyed() {
			continue
		}
		for _, tab := range window.tabs {
			if window.absoluteDocument(tab) != filename {
				continue
			}
			window.activateTab(tab)
			window.win.Focus()
			window.win.Invalidate()
			window.persistSession()
			return
		}
	}
	app.openPickedFile(filename)
	if app.win != nil && !app.win.IsDestroyed() {
		app.win.Focus()
		app.win.Invalidate()
		app.persistSession()
	}
}
