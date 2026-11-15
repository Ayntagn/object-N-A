package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"cyberwriter/internal/core/snippets"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type workspaceSnippets struct {
	open, focusSearch, notes bool
	query, selected, problem string
	items                    []snippets.Snippet
	order                    snippetSortOrder
	expanded                 map[string]bool
	copied                   string
	copiedUntil              time.Time
	exportNotice             string
	exportNoticeUntil        time.Time
	height                   float32
	heightGrip               mygoui.Handle
	resize                   snippetHeightDrag
	window                   *mygo.Window
	pinned                   string
	pinnedFromPreview        bool
}

func snippetFile(notes bool) (string, error) {
	file, err := preferenceFile()
	name := "snippets.json"
	if notes {
		name = "notes.json"
	}
	return filepath.Join(filepath.Dir(file), name), err
}

func (app *workspaceWindow) toggleSnippets() {
	if app.modalOpen {
		return
	}
	s := &app.snippets
	if s.window != nil && !s.window.IsDestroyed() {
		s.window.Focus()
		return
	}
	if s.open {
		s.open = false
		return
	}
	pinned, fromPreview := s.pinned, s.pinnedFromPreview
	s.open, s.focusSearch = true, true
	s.load(pinned != "" && s.notes)
	if _, found := snippetByID(s.items, pinned); found {
		s.pinned, s.selected, s.focusSearch = pinned, pinned, false
		s.pinnedFromPreview = fromPreview
	}
}

// EV-UI-236: a preview link opens the existing pinned reader in this window.
func (app *workspaceWindow) openPreviewSnippet(title string) {
	if app.modalOpen || app.draft == nil {
		return
	}
	library := app.snippets
	library.load(false)
	item, found := snippets.FindByTitle(library.items, title)
	if !found {
		return
	}
	library.open, library.focusSearch = true, false
	library.pinned, library.selected = item.ID, item.ID
	library.pinnedFromPreview = true
	app.snippets = library
}

func (s *workspaceSnippets) load(notes bool) {
	s.notes = notes
	s.query, s.selected, s.problem, s.copied = "", "", "", ""
	s.exportNotice = ""
	s.pinned = ""
	s.pinnedFromPreview = false
	s.expanded = nil
	file, err := snippetFile(notes)
	if err == nil {
		s.items, err = snippets.Load(file)
	}
	if err != nil {
		s.items = nil
		s.problem = fmt.Sprintf("Could not read snippets. %v", err)
	}
}

func (app *workspaceWindow) snippetCaptureMenu(m *mygoui.Menu) {
	content := app.snippetSelection()
	if strings.TrimSpace(content) == "" {
		return
	}
	m.Separator()
	if m.Item("Save as Snippet").Chosen() {
		app.saveSnippet(content)
	}
}

func (app *workspaceWindow) snippetSelection() string {
	if app.sourceEditing() {
		selection := app.code.Selection()
		return app.draft.Slice(selection.Start(), selection.End())
	}
	if app.mode == wysiwygMode && app.wys.editable(app) {
		doc := app.wys.session.Document
		anchor, caret := doc.Selection()
		return doc.Text().Slice(min(anchor, caret), max(anchor, caret)).PlainText()
	}
	return ""
}

func (app *workspaceWindow) saveSnippet(content string) {
	var fileName, pageTitle string
	if app.document.Path != "" {
		fileName = app.title()
		pageTitle = noteName(fileName)
	}
	item := snippets.Capture(content, fileName, pageTitle, time.Now())
	file, err := snippetFile(false)
	var items []snippets.Snippet
	if err == nil {
		items, err = snippets.Append(file, item)
	}
	if err != nil {
		app.problem = fmt.Sprintf("Could not save snippet. %v", err)
		return
	}
	if strings.HasPrefix(app.problem, "Could not save snippet.") {
		app.problem = ""
	}
	s := &app.snippets
	if !s.notes {
		s.items = items
		s.problem, s.query, s.selected, s.copied = "", "", item.ID, ""
	}
	app.refreshSnippetPreviews()
}

func (app *workspaceWindow) canInsertSnippet() bool {
	return app.sourceEditing() || app.mode == wysiwygMode && app.wys.editable(app)
}

func (app *workspaceWindow) insertSnippet(content string) {
	if app.sourceEditing() {
		if len(app.code.Selections()) > 1 {
			selection := app.code.Selection()
			app.code.Select(selection.Start(), selection.End())
		}
		app.code.Insert(content)
		app.focusSource = true
		app.snippets.problem = ""
		return
	}
	if app.mode == wysiwygMode && app.wys.editable(app) {
		if !app.wys.session.InsertMarkdown(content) {
			app.snippets.problem = "Switch to Source to insert this Markdown snippet."
			return
		}
		app.wys.sync(app)
		app.wys.focus = true
		app.snippets.problem = ""
	}
}
