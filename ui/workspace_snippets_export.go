package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cyberwriter/internal/core/exportdoc"
	"cyberwriter/internal/core/snippets"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type snippetFileExport struct {
	label, extension string
	format           snippets.ExportFormat
}

// EV-UI-238/240. The formats share one native save sheet and filename stem.
var snippetFileExports = [...]snippetFileExport{
	{"Export All as Markdown…", "md", snippets.ExportMarkdown},
	{"Export All as JSON…", "json", snippets.ExportJSON},
	{"Export All as Text…", "txt", snippets.ExportText},
}

func (app *workspaceWindow) snippetShareButton(c *mygoui.Context, floating bool) {
	owner := app.win
	if floating {
		owner = app.snippets.window
	}
	chromeMenuControl(c.Key("share snippets"), "Share", func(menu *mygoui.Menu) {
		app.snippetShareMenu(menu, owner, c.Now())
	}).Size(21, 18).MarginEnd(8).KeepFocus().Disabled(app.modalOpen).Children(func() {
		chromeIcon(c, "square.and.arrow.up", 18)
	})
}

func (app *workspaceWindow) snippetShareMenu(menu *mygoui.Menu, owner *mygo.Window, now time.Time) {
	s := &app.snippets
	selectedTitle := "Export Snippet to Vault"
	if s.notes {
		selectedTitle = "Export Note to Vault"
	}
	selected, found := snippetByID(s.items, s.selected)
	if menu.Item(selectedTitle).
		Disabled(app.vault == nil || !found || selected.IsFolder).Chosen() {
		app.exportSnippetsToVault(selected.ID, now)
	}
	if menu.Item("Export All to Vault").
		Disabled(app.vault == nil).Chosen() {
		app.exportSnippetsToVault("", now)
	}
	menu.Separator()
	for _, choice := range snippetFileExports {
		if menu.Item(choice.label).Chosen() {
			app.saveSnippetCollection(owner, choice)
		}
	}
}

// EV-UI-239. Save into the current Vault without replacing an existing note.
func (app *workspaceWindow) exportSnippetsToVault(selectedID string, now time.Time) {
	s := &app.snippets
	s.exportNotice = ""
	source, err := snippetFile(s.notes)
	var notes []snippets.VaultExportNote
	if err == nil {
		notes, err = snippets.VaultExportNotes(source, selectedID)
	}
	for _, note := range notes {
		if err = app.vault.EnsureFolder(path.Dir(note.Path)); err != nil {
			break
		}
		candidate := note.Path
		extension := path.Ext(note.Path)
		stem := strings.TrimSuffix(note.Path, extension)
		for suffix := 2; ; suffix++ {
			_, err = app.vault.CreateDocument(candidate, note.Content)
			if !errors.Is(err, fs.ErrExist) {
				break
			}
			candidate = fmt.Sprintf("%s %d%s", stem, suffix, extension)
		}
		if err != nil {
			break
		}
	}
	if err != nil {
		s.problem = fmt.Sprintf("Could not export to vault. %v", err)
		return
	}
	app.refreshVault()
	s.problem = ""
	s.exportNotice = "Exported to vault"
	if selectedID == "" {
		s.exportNotice = "Exported all to vault"
	}
	s.exportNoticeUntil = now.Add(1500 * time.Millisecond)
}

func (s *workspaceSnippets) snippetExportNotice(c *mygoui.Context) {
	if s.exportNotice == "" || !c.Now().Before(s.exportNoticeUntil) {
		return
	}
	c.After(s.exportNoticeUntil.Sub(c.Now()))
	mygoui.Row(c.Key("snippet export notice")).Absolute().Bottom(20).Left(0).Right(0).
		Height(14).Center().PassThrough().Children(func() {
		mygoui.Text(c, s.exportNotice).Role(mygoui.RoleStatus).FontSize(11).
			FixedLineHeight(14).TextColor(c.Theme().Text).PassThrough()
	})
}

func (app *workspaceWindow) saveSnippetCollection(owner *mygo.Window, choice snippetFileExport) {
	if owner == nil || owner.IsDestroyed() || app.modalOpen {
		return
	}
	source, err := snippetFile(app.snippets.notes)
	if err != nil {
		app.snippets.problem = fmt.Sprintf("Could not export snippets. %v", err)
		return
	}
	options := mygo.SaveDialogOptions{
		Parent: owner, Title: "export snippets", ButtonLabel: "Save", CreateDirectories: true,
		DefaultPath: filepath.Join(app.exportDirectory(), "Snippets."+choice.extension),
		Filters:     []mygo.FileFilter{{Extensions: []string{choice.extension}}},
	}
	app.modalOpen = true
	go func() {
		data, err := snippets.ExportCollection(source, choice.format)
		var destination string
		if err == nil && !owner.IsDestroyed() {
			destination, err = mygo.Dialog.Save(options)
		}
		if err == nil && destination != "" && !owner.IsDestroyed() {
			if !strings.EqualFold(filepath.Ext(destination), "."+choice.extension) {
				err = fmt.Errorf("choose a filename ending in .%s", choice.extension)
			} else {
				err = exportdoc.WriteFile(destination, data)
			}
		}
		app.win.Update(func() {
			app.modalOpen = false
			if err != nil {
				app.snippets.problem = fmt.Sprintf("Could not export snippets. %v", err)
			} else if destination != "" && !owner.IsDestroyed() {
				app.snippets.problem = ""
				app.rememberExport(destination)
			}
			if owner != app.win && !owner.IsDestroyed() {
				owner.Invalidate()
			}
		})
	}()
}
