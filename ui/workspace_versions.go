package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

const currentNoteVersion = "Current Document"

func workspaceVersionHistory() (workspace.VersionHistory, error) {
	file, err := preferenceFile()
	return workspace.VersionHistory{Directory: filepath.Join(filepath.Dir(file), "note-versions")}, err
}

func (app *workspaceWindow) recordVersion(tab *workspaceTab, previous workspace.Document) {
	if previous.Path == "" || previous.Text == tab.document.Text {
		return
	}
	history, err := workspaceVersionHistory()
	if err == nil {
		err = history.Record(app.absoluteDocument(tab), previous.Text)
	}
	if err != nil {
		app.problem = fmt.Sprintf("%q was saved. Could not record the previous version. %v", tab.title(), err)
	}
}

func (app *workspaceWindow) moveVersionHistory(from, to string) {
	history, err := workspaceVersionHistory()
	if err == nil {
		err = history.Move(filepath.Join(app.root, filepath.FromSlash(from)), filepath.Join(app.root, filepath.FromSlash(to)))
	}
	if err != nil {
		app.problem = fmt.Sprintf("%q was moved. Could not move its version history. %v", from, err)
	}
}

func (app *workspaceWindow) retireVersionHistory(relative string) {
	history, err := workspaceVersionHistory()
	if err == nil {
		err = history.Retire(filepath.Join(app.root, filepath.FromSlash(relative)))
	}
	if err != nil {
		app.problem = fmt.Sprintf("%q was moved to Vault Trash. Could not archive its version history. %v", relative, err)
	}
}

type workspaceVersionBrowser struct {
	app       *workspaceWindow
	win       *mygo.Window
	tab       *workspaceTab
	history   workspace.VersionHistory
	path      string
	labels    []string
	versions  map[string]workspace.NoteVersion
	selection string
	current   string
	preview   string
	changed   bool
	problem   string
}

func (app *workspaceWindow) canBrowseVersions() bool {
	return !app.modalOpen && app.workspaceTab != nil && app.draft != nil && app.document.Path != "" && app.permitsActiveDocument()
}

func (app *workspaceWindow) showVersionHistory() {
	if app.canBrowseVersions() {
		app.browseVersions(app.workspaceTab)
	}
}

func (app *workspaceWindow) showNoteVersionHistory(relative string) {
	if app.modalOpen || app.vault == nil || isDrawingPath(relative) {
		return
	}
	if app.multiFile.active && !workspace.IsMarkdownPath(relative) {
		app.problem = multiFileMarkdownOnly
		return
	}
	tab := app.findVaultTab(relative)
	if tab == nil {
		doc, err := app.vault.ReadTextDocument(relative)
		if err != nil {
			app.problem = fmt.Sprintf("Could not read %q. %v", relative, err)
			return
		}
		tab = app.blankTab()
		app.setTabDocument(tab, doc)
	}
	app.browseVersions(tab)
}

func (app *workspaceWindow) browseVersions(tab *workspaceTab) {
	app.wys.sync(app)
	current := tab.draft.String()
	history, err := workspaceVersionHistory()
	browser := &workspaceVersionBrowser{
		app: app, tab: tab, history: history, path: app.absoluteDocument(tab),
		selection: currentNoteVersion, labels: []string{currentNoteVersion},
		versions: make(map[string]workspace.NoteVersion), current: current, preview: current,
	}
	if err == nil {
		var versions []workspace.NoteVersion
		versions, err = history.List(browser.path)
		for i, version := range versions {
			label := version.RecordedAt.Local().Format("Jan 2, 2006 · 15:04:05.000")
			if _, exists := browser.versions[label]; exists {
				label = fmt.Sprintf("%s (%d)", label, i+1)
			}
			browser.labels = append(browser.labels, label)
			browser.versions[label] = version
		}
	}
	if err != nil {
		browser.problem = fmt.Sprintf("Could not load version history. %v", err)
	}

	app.modalOpen = true
	browser.win = mygo.NewWindow(mygo.WindowOptions{
		Title: "Version History", Width: 720, Height: 530, MinWidth: 560, MinHeight: 360,
		Parent: app.win, Modal: true, DisableMinimize: true, DisableMaximize: true,
		Content: mygoui.View(browser.draw),
	})
	browser.win.OnClosed(func() {
		app.modalOpen = false
		app.win.Invalidate()
	})
	app.win.Invalidate()
}

func (b *workspaceVersionBrowser) draw(c *mygoui.Context) {
	b.app.applyAppearance(c)
	done := func() { b.win.Close() }
	c.OnWindowShortcut(0, mygoui.KeyEscape, done)
	mygoui.Column(c).Fill().Padding(20).Gap(10).Background(c.Theme().Background).Children(func() {
		mygoui.Text(c, b.tab.document.Path).FontSize(19).Bold().Role(mygoui.RoleHeading).
			SingleLine().Ellipsis("…")
		mygoui.Select(c, &b.selection, b.labels).Label("Document version").
			Disabled(len(b.labels) == 1).OnChange(b.selectVersion)
		mygoui.TextArea(c.Key(b.selection), &b.preview).ReadOnly(true).Label(b.selection).
			Font("monospace").Grow(1).MinHeight(0).FillWidth()
		b.drawStatus(c)
		mygoui.Row(c).Role(mygoui.RoleToolbar).Label("Version Browsing Controls").
			FillWidth().Gap(8).Justify(mygoui.End).Children(func() {
			mygoui.Button(c, "Restore").Disabled(!b.canRestore()).OnClick(b.restore)
			mygoui.Button(c, "Done").OnClick(done)
		})
	})
}

func (b *workspaceVersionBrowser) drawStatus(c *mygoui.Context) {
	var status string
	switch {
	case b.problem != "":
		mygoui.Text(c, b.problem).Role(mygoui.RoleAlert).TextColor(c.Theme().Danger)
		return
	case len(b.labels) == 1:
		status = "No previous versions. Saving an edit keeps the previous saved text."
	case b.selection == currentNoteVersion:
		return
	case b.tab.dirty():
		status = "Save or discard the note’s unsaved changes before restoring a version."
	case !b.changed:
		status = "This version has the same text as the current document."
	default:
		status = "Restore loads an unsaved draft. Save the draft to update the file."
	}
	mygoui.Text(c, status).Role(mygoui.RoleStatus).TextColor(c.Theme().TextMuted)
}

func (b *workspaceVersionBrowser) selectVersion() {
	b.problem, b.preview, b.changed = "", b.current, false
	version, selected := b.versions[b.selection]
	if !selected {
		return
	}
	text, err := b.history.Read(b.path, version)
	if err != nil {
		b.preview = ""
		b.problem = fmt.Sprintf("Could not read this version. %v", err)
		return
	}
	b.preview = text
	// The editor uses LF internally and preserves the open note's line endings.
	b.changed = strings.ReplaceAll(text, "\r\n", "\n") != b.current
}

func (b *workspaceVersionBrowser) canRestore() bool {
	return b.problem == "" && b.changed && !b.tab.dirty()
}

func (b *workspaceVersionBrowser) restore() {
	app := b.app
	app.wys.sync(app)
	if !b.canRestore() {
		return
	}
	if app.absoluteDocument(b.tab) != b.path {
		b.problem = "The note moved. Reopen Version History to restore a version."
		return
	}
	vault := b.tab.storage
	if vault == nil {
		vault = app.vault
	}
	current, err := vault.ReadTextDocument(b.tab.document.Path)
	if err != nil {
		b.problem = fmt.Sprintf("Could not read the current document. %v", err)
		return
	}
	if current.Text != b.tab.document.Text {
		b.problem = "The note changed on disk. Reload it before restoring a version."
		return
	}

	b.tab.draft.Set(b.preview)
	b.tab.wys.reset()
	if !slices.Contains(app.tabs, b.tab) {
		app.tabs = append(app.tabs, b.tab)
	}
	if app.workspaceTab != b.tab {
		app.activateTab(b.tab)
	} else {
		app.syncDocumentEdited()
	}
	b.win.Close()
}
