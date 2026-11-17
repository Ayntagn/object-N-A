package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
)

const attachmentRenameDetail = "Links pointing to the old name will not be rewritten."

// EV-UI-182/183. Attachment references include the full Vault-relative name.
func attachmentCopyLinks(relative string) (wiki, embed string) {
	if workspace.IsMarkdownPath(relative) {
		wiki = "[[" + noteName(path.Base(relative)) + "]]"
		return wiki, "!" + wiki
	}
	embed = "![[" + relative + "]]"
	return embed, embed
}

func (app *workspaceWindow) duplicateAttachment(relative string) {
	if app.modalOpen || app.vault == nil {
		return
	}
	vault := app.vault
	go func() {
		_, err := vault.DuplicateAttachment(relative)
		app.win.Update(func() {
			if app.win.IsDestroyed() || app.vault != vault {
				return
			}
			if err != nil {
				app.attachmentError("Couldn't duplicate file", err.Error())
				return
			}
			app.refreshAttachmentRows()
		})
	}()
}

func (app *workspaceWindow) renameAttachment(relative string) {
	if app.modalOpen || app.vault == nil {
		return
	}
	vault := app.vault
	info, err := vault.StatEntry(relative)
	if err != nil {
		app.attachmentError("Couldn't rename", err.Error())
		return
	}
	app.modalOpen = true
	basename, stem := path.Base(relative), noteName(path.Base(relative))
	app.win.Update(func() {
		if app.win.IsDestroyed() || app.vault != vault {
			app.modalOpen = false
			return
		}
		prompt := workspaceNamePrompt{title: "Rename", heading: "Rename " + basename, detail: attachmentRenameDetail,
			field: "File name", initial: stem, action: "Rename"}
		showWorkspaceNamePrompt(app.win, prompt, func(input string, confirmed bool) {
			app.win.Update(func() {
				app.modalOpen = false
				if !confirmed || app.win.IsDestroyed() || app.vault != vault {
					return
				}
				name := strings.TrimSpace(input)
				if name == "" || name == stem {
					return
				}
				if name == "." || name == ".." || strings.ContainsAny(name, "/\x00") {
					app.attachmentError("Couldn't rename", "Enter a single file name.")
					return
				}
				target := path.Join(path.Dir(relative), name+path.Ext(basename))
				if err := app.moveAttachment(relative, target, info); err != nil {
					detail := err.Error()
					if errors.Is(err, fs.ErrExist) {
						detail = "A file with that name already exists in this folder."
					}
					app.attachmentError("Couldn't rename", detail)
				}
			})
		})
	})
}

func (app *workspaceWindow) moveAttachment(from, to string, expected fs.FileInfo) error {
	if from == to {
		return nil
	}
	previous := filepath.Join(app.root, filepath.FromSlash(from))
	target := filepath.Join(app.root, filepath.FromSlash(to))
	for _, owner := range app.documentWindows() {
		for _, tab := range owner.tabs {
			if owner.absoluteDocument(tab) == target {
				return fmt.Errorf("%q is already open in another tab: %w", to, fs.ErrExist)
			}
		}
	}
	moved, err := app.vault.MoveAttachment(from, to, expected)
	if err != nil {
		return err
	}
	for _, owner := range app.documentWindows() {
		for _, tab := range owner.tabs {
			if owner.absoluteDocument(tab) != previous {
				continue
			}
			if tab.drawing != nil {
				tab.drawing.file = tab.drawing.file.Relocated(moved)
				tab.document.Path = tab.drawing.file.Path
			} else {
				tab.document = moved.RelocateDocument(tab.document)
			}
			if tab.storage != nil {
				// The individually opened file may have a handle rooted at
				// the old parent. Rebind it without touching its draft.
				old := tab.storage
				tab.storage, tab.folderRoot = app.vault, app.root
				if owner.vault == app.vault {
					tab.storage, tab.folderRoot = nil, ""
				}
				if tab.drawing != nil {
					tab.drawing.storage = app.vault
				}
				_ = owner.closeUnusedVault(old)
			}
			if win := owner.win; win != nil {
				if tab == owner.workspaceTab {
					win.SetTitle(tab.title())
					win.SetRepresentedFile(owner.absoluteDocument(tab))
				}
				win.Invalidate()
			}
		}
		if owner.root == app.root {
			for i, recent := range owner.recentPaths {
				if recent == from {
					owner.recentPaths[i] = to
				}
			}
		}
	}
	if app.application != nil {
		app.application.relocatePDFWindow(previous, target)
		app.application.relocateImageWindow(previous, target)
	}
	app.refreshAttachmentRows()
	if workspace.CanBrowseVaultVersions(from) {
		app.moveVersionHistory(from, to)
	}
	if app.win != nil {
		app.persistSession()
	}
	return nil
}

func (app *workspaceWindow) trashAttachment(relative string) {
	if app.modalOpen || app.vault == nil {
		return
	}
	vault := app.vault
	original, err := vault.StatEntry(relative)
	if err != nil {
		app.attachmentError("Couldn't trash file", err.Error())
		return
	}
	app.modalOpen = true
	options := mygo.MessageOptions{
		Type: mygo.MessageWarning, Message: "Move " + path.Base(relative) + " to Trash?",
		Detail:       "The file will be moved to the system Trash. You can restore it from there.",
		Buttons:      []string{"Move to Trash", "Cancel"},
		CancelButton: 1,
	}
	if runtime.GOOS != "darwin" {
		options.Parent = app.win
	}
	go func() {
		result, err := mygo.Dialog.Message(options)
		app.win.Update(func() {
			app.modalOpen = false
			if app.win.IsDestroyed() || app.vault != vault {
				return
			}
			if err != nil {
				app.attachmentError("Couldn't trash file", err.Error())
				return
			}
			if result.Button != 0 {
				return
			}
			if err := vault.TrashAttachment(relative, original, mygo.Shell.TrashItem); err != nil {
				app.attachmentError("Couldn't trash file", err.Error())
				return
			}
			// Keep any open editor and its draft, just as the original does.
			app.refreshAttachmentRows()
		})
	}()
}

func (app *workspaceWindow) refreshAttachmentRows() {
	app.refreshVault()
	for _, owner := range app.documentWindows() {
		if owner != app && owner.vault == app.vault {
			if owner.win != nil {
				owner.win.Update(owner.refreshVault)
			} else {
				owner.refreshVault()
			}
		}
	}
}

func (app *workspaceWindow) attachmentError(title, detail string) {
	app.modalOpen = true
	options := mygo.MessageOptions{Type: mygo.MessageWarning, Message: title, Detail: detail}
	if runtime.GOOS != "darwin" {
		options.Parent = app.win
	}
	go func() {
		_, err := mygo.Dialog.Message(options)
		app.win.Update(func() {
			app.modalOpen = false
			if err != nil {
				app.problem = title + ". " + detail
			}
		})
	}()
}
