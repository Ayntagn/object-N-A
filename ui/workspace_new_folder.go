package ui

import (
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
)

const newFolderDefaultName = "Untitled"

type workspaceNamePrompt struct {
	title, heading, detail, field, initial, action, icon string
}

// EV-UI-124/125: the same prompt serves the toolbar, both context menus and commands.
func (app *workspaceWindow) createFolder(parent string, expandParent bool) {
	if app.vault == nil || app.modalOpen || app.win == nil || app.win.IsDestroyed() {
		return
	}
	vault := app.vault
	origin := mygo.FocusedWindow()
	if origin == nil || (origin != app.win && (app.application == nil || origin != app.application.glow.window)) {
		origin = app.win
	}
	parentName := path.Base(parent)
	if parent == "." {
		parentName = filepath.Base(app.root)
	}

	app.modalOpen = true
	done := func(name string, confirmed bool) {
		app.win.Update(func() {
			defer func() { app.modalOpen = false }()
			if !confirmed || app.win.IsDestroyed() {
				return
			}
			if app.vault != vault {
				app.showNewFolderError(origin, "The active Vault changed while the dialog was open.")
				return
			}
			created, err := vault.CreateNamedFolder(parent, name)
			switch {
			case errors.Is(err, workspace.ErrFolderNameSeparators):
				app.showNewFolderError(origin, "Folder names can't contain “/” or “:”.")
			case errors.Is(err, fs.ErrExist):
				app.showNewFolderError(origin, "An item with that name already exists here.")
			case err != nil:
				app.showNewFolderError(origin, err.Error())
			case created:
				if expandParent {
					*app.folder(parent) = true
				}
				app.refreshVault()
			}
		})
	}
	app.win.Update(func() {
		if app.win.IsDestroyed() || origin.IsDestroyed() || app.vault != vault {
			app.modalOpen = false
			return
		}
		showWorkspaceNamePrompt(origin, workspaceNamePrompt{
			title: "New Folder", heading: "New Folder in " + parentName, field: "Folder name",
			initial: newFolderDefaultName, action: "Create", icon: "folder.badge.plus",
		}, done)
	})
}

func (app *workspaceWindow) showNewFolderError(origin *mygo.Window, detail string) {
	options := mygo.MessageOptions{
		Type: mygo.MessageWarning, Message: "Couldn't create folder", Detail: detail,
		Buttons: []string{"OK"}, CancelButton: -1,
	}
	if runtime.GOOS != "darwin" {
		options.Parent = origin
	}
	if _, err := mygo.Dialog.Message(options); err != nil {
		app.problem = options.Message + ". " + detail
	}
}
