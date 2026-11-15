package ui

// spec: FEAT-FEATURE-GUIDE / EV-UI-122 / EV-UI-123

import (
	"slices"
	"testing"
)

func TestFeatureGuideSearch(t *testing.T) {
	guide := workspaceFeatureGuide{selected: "wikilinks"}
	guide.search("  ALIAS  ")
	if !slices.Equal(guide.matches, []string{"wikilinks", "wikibuilder"}) || guide.selected != "wikilinks" {
		t.Fatalf("alias results %v, selected %q", guide.matches, guide.selected)
	}
	guide.search("table")
	wantTables := []string{"headings", "tables", "figures", "pagebreak", "api", "query", "embeds",
		"custom-css", "toc", "slides", "smartpaste", "selectiontoolbar", "wysiwyg", "ai-panel"}
	if !slices.Equal(guide.matches, wantTables) || guide.selected != "headings" {
		t.Fatalf("table results %v, selected %q", guide.matches, guide.selected)
	}
	guide.move(1)
	if guide.selected != "tables" {
		t.Fatalf("Down should open Tables, got %q", guide.selected)
	}
	guide.search("PRESENT & EXPORT")
	want := []string{"export", "custom-css", "themes", "toc", "slides", "flashcards", "focus"}
	if !slices.Equal(guide.matches, want) || guide.selected != "export" {
		t.Fatalf("section results %v, selected %q", guide.matches, guide.selected)
	}
	guide.search("mermaid background")
	if !slices.Equal(guide.matches, []string{"mermaid-bg", "export"}) || guide.selected != "export" {
		t.Fatalf("multi-word results %v, selected %q", guide.matches, guide.selected)
	}
	guide.move(-1)
	guide.move(-1)
	if guide.selected != "mermaid-bg" {
		t.Fatalf("selection should stop at the first result, got %q", guide.selected)
	}
	guide.search("no-results-2026")
	guide.move(1)
	if len(guide.matches) != 0 || guide.selected != "mermaid-bg" {
		t.Fatalf("empty search should keep detail, got %v and %q", guide.matches, guide.selected)
	}
}
