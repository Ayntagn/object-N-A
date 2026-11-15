package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
)

const exampleRestoreTitle = "Restore the Example Vault?"
const exampleRestoreMessage = "This permanently deletes the “cyberWriter Example Vault” folder and everything inside it - including any notes, drafts, or files you've saved there, not just edits to the example notes - and replaces it with a fresh copy. This can't be undone."

func (app *workspaceWindow) restoreExampleVault() {
	app.openExampleVault(true)
}

func (app *workspaceWindow) openExampleVault(restore bool) {
	if app.modalOpen {
		return
	}
	preferences, err := preferenceFile()
	if err != nil {
		app.problem = fmt.Sprintf("Could not locate the workspace data directory. %v", err)
		return
	}
	plan, err := workspace.PlanExampleVaultRestore(filepath.Dir(preferences))
	if err != nil {
		app.problem = fmt.Sprintf("Could not prepare the example Vault. %v", err)
		return
	}
	if plan.Existing() && !restore {
		if app.openVault(plan.Root()) {
			app.persistSession()
		}
		return
	}
	win := app.win
	app.modalOpen = true
	go func() {
		if plan.Existing() {
			options := mygo.MessageOptions{
				Title: exampleRestoreTitle, Message: exampleRestoreTitle, Detail: exampleRestoreMessage,
				Type:    mygo.MessageWarning,
				Buttons: []string{"Restore", "Cancel"}, DefaultButton: 1, CancelButton: 1,
				DestructiveButtons: []int{0},
			}
			if runtime.GOOS != "darwin" {
				options.Parent = win
			}
			answer, err := mygo.Dialog.Message(options)
			if err != nil || answer.Button != 0 {
				win.Update(func() {
					app.modalOpen = false
					if err != nil {
						app.problem = fmt.Sprintf("Could not show the example Vault confirmation. %v", err)
					}
				})
				return
			}
		}
		published, err := applyExampleVaultTemplate(plan)
		win.Update(func() {
			app.modalOpen = false
			if published && app.loadVault(plan.Root(), plan.Existing()) {
				app.persistSession()
			}
			if err != nil {
				app.problem = fmt.Sprintf("Could not finish restoring the example Vault. %v", err)
			}
		})
	}()
}

func applyExampleVaultTemplate(plan workspace.ExampleVaultRestore) (bool, error) {
	source := os.Getenv("CYBERWRITER_EXAMPLE_VAULT_DIR")
	if source == "" {
		builtin, err := workspace.BuiltinExampleVault()
		if err != nil {
			return false, err
		}
		return plan.Apply(builtin)
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return false, err
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return false, err
	}
	target := plan.Root()
	if parent, err := filepath.EvalSymlinks(filepath.Dir(target)); err == nil {
		target = filepath.Join(parent, filepath.Base(target))
	}
	_, targetInsideSource := exampleRelative(source, target)
	_, sourceInsideTarget := exampleRelative(target, source)
	if targetInsideSource || sourceInsideTarget {
		return false, errors.New("the template and restored example Vault must be separate directories")
	}
	template, err := os.OpenRoot(source)
	if err != nil {
		return false, err
	}
	defer template.Close()
	return plan.Apply(template.FS())
}

func exampleRelative(parent, filename string) (string, bool) {
	relative, err := filepath.Rel(parent, filename)
	relative = filepath.ToSlash(relative)
	return relative, err == nil && fs.ValidPath(relative)
}

func (app *workspaceWindow) rebindExampleTabs(root string, restored *workspace.Vault) {
	for _, tab := range slices.Clone(app.tabs) {
		if tab.document.Path == "" {
			continue
		}
		relative, inside := exampleRelative(root, app.absoluteDocument(tab))
		if !inside {
			continue
		}
		if tab.drawing != nil {
			draw, err := loadWorkspaceDrawing(restored, relative)
			if err != nil {
				app.removeTab(tab)
				continue
			}
			if tab.dirty() {
				tab.drawing.file, tab.drawing.storage = draw.file, restored
				tab.document.Path = relative
			} else {
				app.setTabDrawing(tab, draw)
			}
		} else {
			dirty := tab.dirty()
			doc, err := restored.ReadDocument(relative)
			if dirty && errors.Is(err, fs.ErrNotExist) {
				doc, err = restored.PrepareDocumentRecreation(relative)
			}
			if err != nil {
				if dirty {
					app.releaseTabStorage(tab)
					tab.document = workspace.Document{}
				} else {
					app.removeTab(tab)
				}
				continue
			}
			if dirty || tab.document.Text == doc.Text {
				tab.document = doc
			} else {
				app.setTabDocument(tab, doc)
			}
			if doc.NeedsRecreation() {
				app.autosaveRestoredDraft(tab, doc)
			}
		}
		app.releaseTabStorage(tab)
		tab.storage, tab.folderRoot = restored, root
	}
}

func (app *workspaceWindow) autosaveRestoredDraft(tab *workspaceTab, pending workspace.Document) {
	win := app.win
	// EV-UI-092 observed recreation shortly after the restored template appeared.
	time.AfterFunc(2*time.Second, func() {
		if win.IsDestroyed() {
			return
		}
		win.Update(func() {
			if !slices.Contains(app.tabs, tab) || tab.document != pending {
				return
			}
			if app.modalOpen {
				app.autosaveRestoredDraft(tab, pending)
				return
			}
			app.wys.sync(app)
			app.saveTab(tab, nil)
		})
	})
}
