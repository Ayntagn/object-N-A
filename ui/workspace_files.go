package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

func (app *workspaceWindow) vaultForFolder(root string) (*workspace.Vault, error) {
	for _, window := range app.documentWindows() {
		if root == window.root && window.vault != nil {
			return window.vault, nil
		}
		for _, tab := range window.tabs {
			if tab.storage != nil && tab.folderRoot == root {
				return tab.storage, nil
			}
		}
	}
	return workspace.Open(root)
}

func (app *workspaceWindow) closeUnusedVault(vault *workspace.Vault) error {
	if vault == nil {
		return nil
	}
	for _, window := range app.documentWindows() {
		if vault == window.vault || slices.ContainsFunc(window.tabs, func(tab *workspaceTab) bool { return tab.storage == vault }) {
			return nil
		}
	}
	return vault.Close()
}

func (app *workspaceWindow) releaseTabStorage(tab *workspaceTab) {
	vault := tab.storage
	tab.storage, tab.folderRoot = nil, ""
	app.closeUnusedVault(vault)
}

func (app *workspaceWindow) absoluteDocument(tab *workspaceTab) string {
	if tab.document.Path == "" {
		return ""
	}
	root := app.root
	if tab.storage != nil {
		root = tab.folderRoot
	}
	return filepath.Join(root, filepath.FromSlash(tab.document.Path))
}

func (app *workspaceWindow) relativeToVault(absolute string) (string, bool) {
	if app.vault == nil {
		return "", false
	}
	relative, err := filepath.Rel(app.root, absolute)
	relative = filepath.ToSlash(relative)
	if err != nil || !fs.ValidPath(relative) || strings.HasPrefix(relative, "../") || relative == ".." {
		return "", false
	}
	return relative, true
}

const documentPickerTitle = "Open Document"

var documentPickerFilter = mygo.FileFilter{
	Name:       "Notes, Drawings, PDFs and Images",
	Extensions: []string{"md", "markdown", "excalidraw", "pdf", "png", "jpg", "jpeg", "gif", "webp"},
}

func (app *workspaceWindow) filePicker() {
	open := func(filename string) { app.openPickedDocument(filename, false) }
	if app.multiFile.active {
		app.pickDocument("Open Markdown", mygo.FileFilter{Name: "Markdown", Extensions: []string{"md", "markdown"}}, open)
		return
	}
	app.pickDocument(documentPickerTitle, documentPickerFilter, open)
}

func (app *workspaceWindow) drawingPicker() {
	if app.multiFile.active {
		app.problem = multiFileMarkdownOnly
		return
	}
	app.pickDocument("Open Drawing", mygo.FileFilter{Name: "Drawings", Extensions: []string{"excalidraw"}}, app.openPickedFile)
}

func (app *workspaceWindow) pickDocument(title string, filter mygo.FileFilter, open func(string)) {
	if app.modalOpen {
		return
	}
	app.modalOpen = true
	options := mygo.OpenDialogOptions{Parent: app.win, Title: title, DefaultPath: app.root,
		Filters: []mygo.FileFilter{filter}}
	go func() {
		files, err := mygo.Dialog.Open(options)
		app.win.Update(func() {
			app.modalOpen = false
			if err != nil {
				app.problem = fmt.Sprintf("Could not show the file picker. %v", err)
				return
			}
			if len(files) == 0 {
				return
			}
			open(files[0])
		})
	}()
}

func (app *workspaceWindow) openPickedFile(absolute string) {
	app.openPickedDocument(absolute, true)
}

func (app *workspaceWindow) openPickedDocument(absolute string, replaceInitial bool) {
	if app.multiFile.active && !workspace.IsMarkdownPath(absolute) {
		app.problem = multiFileMarkdownOnly
		return
	}
	if workspace.IsPDFPath(absolute) {
		app.openPickedPDF(absolute)
		return
	}
	if workspace.IsRasterImagePath(absolute) {
		app.openPickedImage(absolute)
		return
	}
	if relative, ok := app.relativeToVault(absolute); ok {
		app.openNote(relative, (app.draft != nil || app.drawing != nil) && !(replaceInitial && app.canReplaceInitialTab()))
		return
	}
	for _, tab := range app.tabs {
		if absolute == app.absoluteDocument(tab) {
			app.activateTab(tab)
			return
		}
	}
	root := filepath.Dir(absolute)
	storage, err := app.vaultForFolder(root)
	if err != nil {
		app.problem = fmt.Sprintf("Could not open %q. %v", absolute, err)
		return
	}
	if isDrawingPath(absolute) {
		draw, err := loadWorkspaceDrawing(storage, filepath.Base(absolute))
		if err != nil {
			app.closeUnusedVault(storage)
			app.problem = fmt.Sprintf("Could not open %q. %v", absolute, err)
			return
		}
		tab := app.availableTab(replaceInitial)
		tab.storage, tab.folderRoot = storage, root
		app.setTabDrawing(tab, draw)
		app.activateTab(tab)
		return
	}
	sourceText := workspace.IsSourceTextPath(absolute)
	read := storage.ReadDocument
	if sourceText {
		read = storage.ReadTextDocument
	}
	doc, err := read(filepath.Base(absolute))
	if err != nil {
		app.closeUnusedVault(storage)
		app.problem = fmt.Sprintf("Could not open %q. %v", absolute, err)
		return
	}
	tab := app.availableTab(replaceInitial)
	tab.storage, tab.folderRoot = storage, root
	app.setTabDocument(tab, doc)
	if sourceText {
		tab.mode = splitMode
	}
	app.activateTab(tab)
}

func (app *workspaceWindow) availableTab(replaceInitial bool) *workspaceTab {
	if app.draft == nil && app.drawing == nil {
		return app.workspaceTab
	}
	if replaceInitial && app.canReplaceInitialTab() {
		return app.workspaceTab
	}
	tab := app.blankTab()
	app.tabs = append(app.tabs, tab)
	return tab
}

func (app *workspaceWindow) newDocument() {
	if app.vault != nil && app.preferences.NewInVault {
		app.addVaultNote()
		return
	}
	if app.modalOpen {
		return
	}
	tab := app.availableTab(true)
	tab.mode = app.preferences.DefaultView
	app.setTabDocument(tab, workspace.Document{})
	app.activateTab(tab)
}

func (app *workspaceWindow) offerMissingDocumentSave(tab *workspaceTab, next func()) {
	app.modalOpen = true
	options := mygo.MessageOptions{
		Parent: app.win, Type: mygo.MessageWarning,
		Title:         fmt.Sprintf("Could not save %q", tab.title()),
		Message:       "The original file was moved or deleted. Save the current edits as a new file.",
		Buttons:       []string{"Cancel", "Save As…"},
		DefaultButton: 1,
	}
	go func() {
		choice, err := mygo.Dialog.Message(options)
		app.win.Update(func() {
			app.modalOpen = false
			if err != nil {
				app.problem = fmt.Sprintf("Could not show the save prompt. %v", err)
				return
			}
			if choice.Button == 1 {
				app.saveAs(tab, "Untitled.md", next)
			}
		})
	}()
}

func (app *workspaceWindow) saveAs(tab *workspaceTab, target string, next func()) {
	if tab.draft == nil || app.modalOpen {
		return
	}
	app.wys.flush(app)
	app.modalOpen = true
	dir := app.root
	if absolute := app.absoluteDocument(tab); absolute != "" {
		dir = filepath.Dir(absolute)
		if target == "" {
			target = noteName(filepath.Base(absolute)) + " Copy.md"
		}
	}
	if target == "" {
		if tab.inspection {
			target = inspectionSaveFilename(tab.inspectionTextForSave())
		} else {
			target = "Untitled.md"
		}
	}
	options := mygo.SaveDialogOptions{
		Parent: app.win, Title: "Save Markdown", DefaultPath: filepath.Join(dir, target),
		CreateDirectories: true,
		Filters:           []mygo.FileFilter{{Name: "Markdown", Extensions: []string{"md", "markdown"}}},
	}
	go func() {
		chosen, err := mygo.Dialog.Save(options)
		app.win.Update(func() {
			app.modalOpen = false
			if err != nil {
				app.problem = fmt.Sprintf("Could not show the save picker. %v", err)
				return
			}
			if chosen == "" || !app.writeNoteAt(tab, markdownFilename(chosen)) {
				return
			}
			if next != nil {
				next()
			}
		})
	}()
}

func (app *workspaceWindow) writeNoteAt(tab *workspaceTab, absolute string) bool {
	for _, other := range app.tabs {
		if other != tab && absolute == app.absoluteDocument(other) {
			app.problem = fmt.Sprintf("%q is already open in another tab.", absolute)
			return false
		}
	}
	relative, inVault := app.relativeToVault(absolute)
	storage := app.vault
	if !inVault {
		var err error
		storage, err = app.vaultForFolder(filepath.Dir(absolute))
		if err != nil {
			app.problem = fmt.Sprintf("Could not access %q. %v", absolute, err)
			return false
		}
		relative = filepath.Base(absolute)
	}
	content := tab.draft.String()
	if tab.inspection {
		content = tab.inspectionTextForSave()
	}
	doc, err := storage.CreateDocument(relative, content)
	if err != nil {
		app.closeUnusedVault(storage)
		app.problem = fmt.Sprintf("Could not save %q. %v", absolute, err)
		return false
	}
	if tab.storage != storage {
		app.releaseTabStorage(tab)
	}
	if !inVault {
		tab.storage, tab.folderRoot = storage, filepath.Dir(absolute)
	}
	tab.document = doc
	tab.draft.MarkSaved()
	if inVault {
		app.refreshVault()
		app.revealFolder(path.Dir(relative))
	}
	if tab == app.workspaceTab {
		app.activateTab(tab)
	}
	return true
}

func (app *workspaceWindow) addVaultNote() {
	if app.vault == nil || app.modalOpen {
		return
	}
	for number := 0; ; number++ {
		name := "Untitled"
		if number > 0 {
			name = fmt.Sprintf("Untitled %d", number)
		}
		doc, err := app.vault.CreateDocument(name+".md", "# "+name+"\n\n")
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			app.problem = fmt.Sprintf("Could not create a Vault note. %v", err)
			return
		}
		app.openCreatedNote(doc)
		return
	}
}

func (app *workspaceWindow) createNote(folder, content string) {
	if app.vault == nil {
		return
	}
	app.promptName("New Document", "Note name", "Untitled", func(name string) error {
		doc, err := app.vault.CreateDocument(path.Join(folder, markdownFilename(name)), content)
		if err != nil {
			return err
		}
		app.openCreatedNote(doc)
		return nil
	})
}

func (app *workspaceWindow) openCreatedNote(doc workspace.Document) {
	tab := app.availableTab(true)
	tab.mode = app.preferences.DefaultView
	app.setTabDocument(tab, doc)
	app.refreshVault()
	app.revealFolder(path.Dir(doc.Path))
	app.activateTab(tab)
}

func (app *workspaceWindow) promptName(title, label, initial string, accept func(string) error) {
	if app.modalOpen {
		return
	}
	app.modalOpen = true
	name, problem := initial, ""
	focus := true
	var dialog *mygo.Window
	submit := func() {
		value := strings.TrimSpace(name)
		if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `/\`) {
			problem = "Enter a single file or folder name."
			return
		}
		if err := accept(value); err != nil {
			problem = err.Error()
			return
		}
		dialog.Close()
	}
	dialog = mygo.NewWindow(mygo.WindowOptions{
		Title: title, Parent: app.win, Modal: true, Width: 400, Height: 170,
		DisableMaximize: true, DisableMinimize: true, DisableResize: true,
		Content: mygoui.View(func(c *mygoui.Context) {
			c.OnWindowShortcut(0, mygoui.KeyEscape, func() { dialog.Close() })
			mygoui.Column(c).Fill().Padding(16).Gap(10).Children(func() {
				mygoui.Field(c, label, func() {
					input := mygoui.TextInput(c, &name).Label(label).FillWidth().OnSubmit(submit)
					if focus {
						input.Focus()
						focus = false
					}
				})
				if problem != "" {
					mygoui.Text(c, problem).Role(mygoui.RoleAlert).Wrap().TextColor(c.Theme().Danger)
				}
				mygoui.Spacer(c)
				mygoui.Row(c).Gap(8).Justify(mygoui.End).Children(func() {
					mygoui.Button(c, "Cancel").OnClick(func() { dialog.Close() })
					mygoui.Button(c, title).OnClick(submit)
				})
			})
		}),
	})
	dialog.OnClosed(func() {
		app.modalOpen = false
		app.win.Invalidate()
	})
}

func markdownFilename(name string) string {
	if workspace.IsMarkdownPath(name) {
		return name
	}
	return name + ".md"
}

func isDrawingPath(name string) bool { return strings.EqualFold(path.Ext(name), ".excalidraw") }

func (app *workspaceWindow) duplicateNote(relative string) {
	doc, err := app.vault.DuplicateDocument(relative)
	if err != nil {
		app.problem = fmt.Sprintf("Could not duplicate %q. %v", relative, err)
		return
	}
	app.refreshVault()
	app.revealFolder(path.Dir(doc.Path))
	app.openNote(doc.Path, app.draft != nil)
}

func (app *workspaceWindow) renameNote(relative string) {
	app.promptName("Rename", "Note name", path.Base(relative), func(name string) error {
		target := path.Join(path.Dir(relative), markdownFilename(name))
		return app.moveNote(relative, target)
	})
}

func (app *workspaceWindow) chooseMoveFolder(relative string) {
	if app.modalOpen || app.vault == nil {
		return
	}
	vault := app.vault
	options := mygo.OpenDialogOptions{Parent: app.win, Title: "Move to Folder", Directory: true, DefaultPath: app.root}
	attachment := !workspace.IsMarkdownPath(relative)
	var selected fs.FileInfo
	if attachment {
		var err error
		selected, err = vault.StatEntry(relative)
		if err != nil {
			app.attachmentError("Couldn't move file", err.Error())
			return
		}
		options.Title = "Open"
		options.Message = "Choose a destination folder inside the vault"
		options.ButtonLabel = "Move"
		options.CreateDirectories = true
		if runtime.GOOS == "darwin" {
			options.Parent = nil
		}
	}
	app.modalOpen = true
	go func() {
		folders, err := mygo.Dialog.Open(options)
		app.win.Update(func() {
			app.modalOpen = false
			if err != nil {
				app.problem = fmt.Sprintf("Could not show the folder picker. %v", err)
				return
			}
			if len(folders) == 0 || app.vault != vault || app.win.IsDestroyed() {
				return
			}
			folder, ok := app.relativeToVault(folders[0])
			if !ok {
				if attachment {
					app.attachmentError("Couldn't move file", "Choose a folder inside the current Vault.")
					return
				}
				app.problem = "Choose a folder inside the current Vault."
				return
			}
			target := path.Join(folder, path.Base(relative))
			if attachment {
				if err := app.moveAttachment(relative, target, selected); err != nil {
					app.attachmentError("Couldn't move file", err.Error())
				}
				return
			}
			if err := app.moveNote(relative, target); err != nil {
				app.problem = fmt.Sprintf("Could not move %q. %v", relative, err)
			}
		})
	}()
}

func (app *workspaceWindow) moveNote(from, to string) error {
	if from == to {
		return nil
	}
	if app.findVaultTab(to) != nil {
		return fmt.Errorf("%q is already open in another tab", to)
	}
	app.wys.sync(app)
	moved, err := app.vault.MoveDocument(from, to)
	if err != nil {
		return err
	}
	for _, tab := range app.tabs {
		if tab.storage == nil && tab.document.Path == from {
			tab.document = tab.document.Relocated(moved)
		}
	}
	if app.storage == nil && app.document.Path == to {
		app.win.SetTitle(app.title())
		app.win.SetRepresentedFile(app.absoluteDocument(app.workspaceTab))
	}
	for i, recent := range app.recentPaths {
		if recent == from {
			app.recentPaths[i] = to
		}
	}
	app.refreshVault()
	app.revealFolder(path.Dir(to))
	app.moveVersionHistory(from, to)
	return nil
}

func (app *workspaceWindow) trashNote(relative string) {
	var tabs []*workspaceTab
	for _, tab := range app.tabs {
		if tab.storage == nil && tab.document.Path == relative {
			tabs = append(tabs, tab)
		}
	}
	remove := func() {
		if err := app.vault.TrashDocument(relative); err != nil {
			app.problem = fmt.Sprintf("Could not move %q to Vault Trash. %v", relative, err)
			return
		}
		for _, tab := range tabs {
			app.removeTab(tab)
		}
		app.refreshVault()
		app.retireVersionHistory(relative)
	}
	app.confirm("Move to Vault Trash", fmt.Sprintf("Move %q to the recoverable Vault Trash?", relative),
		"Move to Vault Trash", func() { app.beforeLeavingTabs(tabs, remove) })
}

func (app *workspaceWindow) confirm(title, message, action string, run func()) {
	if app.modalOpen {
		return
	}
	app.modalOpen = true
	go func() {
		choice, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent: app.win, Title: title, Type: mygo.MessageWarning, Message: message,
			Buttons: []string{"Cancel", action}, DestructiveButtons: []int{1},
		})
		app.win.Update(func() {
			app.modalOpen = false
			if err != nil {
				app.problem = fmt.Sprintf("Could not show the confirmation. %v", err)
			} else if choice.Button == 1 {
				run()
			}
		})
	}()
}

func (app *workspaceWindow) reloadNote() {
	if app.document.Path == "" || app.drawing != nil {
		return
	}
	load := func() {
		vault := app.vault
		if app.storage != nil {
			vault = app.storage
		}
		doc, err := vault.ReadTextDocument(app.document.Path)
		if err != nil {
			app.problem = fmt.Sprintf("Could not reload %q. %v", app.document.Path, err)
			return
		}
		app.setTabDocument(app.workspaceTab, doc)
		app.activateTab(app.workspaceTab)
	}
	app.wys.sync(app)
	if app.dirty() {
		app.confirm("Revert to Saved", "Discard changes in the current tab and load the note from disk?", "Revert", load)
	} else {
		load()
	}
}

func (app *workspaceWindow) findVaultTab(relative string) *workspaceTab {
	if app.workspaceTab != nil && app.storage == nil && app.document.Path == relative {
		return app.workspaceTab
	}
	index := slices.IndexFunc(app.tabs, func(tab *workspaceTab) bool {
		return tab.storage == nil && tab.document.Path == relative
	})
	if index >= 0 {
		return app.tabs[index]
	}
	return nil
}

func (app *workspaceWindow) currentVaultPath() string {
	if app.storage != nil || app.drawing != nil || !app.permitsActiveDocument() {
		return ""
	}
	return app.document.Path
}

func (app *workspaceWindow) vaultFileMenu(m *mygoui.Menu, relative string) {
	absolute := filepath.Join(app.root, filepath.FromSlash(relative))
	markdown := workspace.IsMarkdownPath(relative)
	if m.Item("Open in New Tab").Chosen() {
		if markdown || isDrawingPath(relative) || workspace.IsPDFPath(relative) ||
			workspace.IsRasterImagePath(relative) || workspace.IsSourceTextPath(relative) {
			app.openVaultRow(relative, true)
		}
	}
	if m.Item("Open in Default App").Chosen() {
		app.openVaultFileInDefaultApp(relative)
	}
	m.Submenu("Open As", func(open *mygoui.Menu) {
		if open.Item("Plain Text").Chosen() {
			app.inspectVaultFile(relative, workspace.InspectPlainText)
		}
		if open.Item("Hex Dump").Chosen() {
			app.inspectVaultFile(relative, workspace.InspectHexDump)
		}
	})
	if workspace.CanBrowseVaultVersions(relative) {
		if m.Item("Browse Versions…").Chosen() {
			app.showNoteVersionHistory(relative)
		}
	}
	m.Separator()
	if m.Item("Duplicate").Chosen() {
		if markdown {
			app.duplicateNote(relative)
		} else {
			app.duplicateAttachment(relative)
		}
	}
	if m.Item("Rename…").Chosen() {
		if markdown {
			app.renameNote(relative)
		} else {
			app.renameAttachment(relative)
		}
	}
	if m.Item("Move to Folder…").Chosen() {
		app.chooseMoveFolder(relative)
	}
	if m.Item("New Folder Here…").Chosen() {
		app.createFolder(path.Dir(relative), false)
	}
	m.Separator()
	m.Submenu("Copy", func(copy *mygoui.Menu) {
		wiki, embed := attachmentCopyLinks(relative)
		if copy.Item("Wikilink").Chosen() {
			mygo.Clipboard.WriteText(wiki)
		}
		if copy.Item("Embed").Chosen() {
			mygo.Clipboard.WriteText(embed)
		}
		copy.Separator()
		if copy.Item("Relative Path").Chosen() {
			mygo.Clipboard.WriteText(relative)
		}
		if copy.Item("Absolute Path").Chosen() {
			mygo.Clipboard.WriteText(absolute)
		}
	})
	if m.Item(vaultRevealLabel()).Chosen() {
		mygo.Shell.ShowItemInFolder(absolute)
	}
	m.Separator()
	if m.Item("Move to Trash").Chosen() {
		if markdown {
			app.trashNote(relative)
		} else {
			app.trashAttachment(relative)
		}
	}
}

func (app *workspaceWindow) openVaultFileInDefaultApp(relative string) {
	absolute := filepath.Join(app.root, filepath.FromSlash(relative))
	if err := mygo.Shell.OpenPath(absolute); err != nil {
		app.problem = fmt.Sprintf("Could not open %q. %v", relative, err)
	}
}

func vaultRevealLabel() string {
	if runtime.GOOS == "darwin" {
		return "Reveal in Finder"
	}
	return "Reveal in File Manager"
}

func (app *workspaceWindow) menuCommand(action func()) func(*mygo.MenuItem, *mygo.Window) {
	return func(_ *mygo.MenuItem, _ *mygo.Window) {
		app.win.Update(func() {
			if !app.modalOpen {
				action()
			}
		})
	}
}

const maxRecent = 8

func (app *workspaceWindow) recentMenuItems() []*mygo.MenuItem {
	app.recentSlots = make([]*mygo.MenuItem, maxRecent)
	for i := range app.recentSlots {
		app.recentSlots[i] = &mygo.MenuItem{
			Hidden: true,
			Click: app.menuCommand(func() {
				if i < len(app.recentPaths) {
					app.openDocument(app.recentPaths[i])
				}
			}),
		}
	}
	return append(slices.Clone(app.recentSlots), &mygo.MenuItem{
		Label: "Clear Menu", Click: app.menuCommand(func() {
			app.recentPaths = nil
			app.updateRecentMenu()
		}),
	})
}

func (app *workspaceWindow) updateRecentMenu() {
	for i, item := range app.recentSlots {
		visible := i < len(app.recentPaths)
		if visible {
			label := app.recentPaths[i]
			if item.Label != label {
				item.SetLabel(label)
			}
		}
		if item.IsVisible() != visible {
			item.SetVisible(visible)
		}
	}
}
