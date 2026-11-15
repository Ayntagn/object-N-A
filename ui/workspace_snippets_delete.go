package ui

import (
	"fmt"

	"cyberwriter/internal/core/snippets"
)

// EV-UI-241: the detail action removes the selected entry and clears details.
func (app *workspaceWindow) deleteSnippetLibraryItem(id string) {
	s := &app.snippets
	if s.selected != id {
		return
	}
	filename, err := snippetFile(s.notes)
	if err == nil {
		var items []snippets.Snippet
		items, err = snippets.Remove(filename, id)
		if err == nil {
			s.items, s.selected, s.problem, s.copied = items, "", "", ""
			if s.pinned == id {
				s.pinned, s.pinnedFromPreview = "", false
			}
			if !s.notes {
				app.refreshSnippetPreviews()
			}
			return
		}
	}
	s.problem = fmt.Sprintf("Could not delete item. %v", err)
}
