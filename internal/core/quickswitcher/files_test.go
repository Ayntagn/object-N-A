package quickswitcher

// spec: FEAT-QUICK-SWITCHER / EV-UI-118 / EV-UI-119

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
)

func TestFileSearch(t *testing.T) {
	files := Files([]workspace.Entry{
		{Path: "Summary.md"},
		{Path: "Résumé.md"},
		{Path: "Research", IsDir: true, Children: []workspace.Entry{
			{Path: "Research/Summary.md"},
			{Path: "Research/Daily Meeting.md"},
			{Path: "Research/Deep", IsDir: true, Children: []workspace.Entry{
				{Path: "Research/Deep/Summary.md"},
			}},
		}},
		{Path: "Journal", IsDir: true, Children: []workspace.Entry{{Path: "Journal/Weekly Agenda.md"}}},
		{Path: "Media", IsDir: true, Children: []workspace.Entry{{Path: "Media/Diagram.excalidraw"}}},
		{Path: "Meeting Notes.md"},
		{Path: "Long extension.markdown"},
		{Path: "CamelCaseDocument.md"},
		{Path: "Agenda.md"},
		{Path: "Agenda Outline.md"},
	}, strings.Compare)
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"Summary", []string{"Research/Deep/Summary.md", "Research/Summary.md", "Summary.md"}},
		{"Research", []string{"Research/Daily Meeting.md", "Research/Deep/Summary.md", "Research/Summary.md"}},
		{"Agenda", []string{"Agenda.md", "Agenda Outline.md", "Journal/Weekly Agenda.md"}},
		{"MN", []string{"CamelCaseDocument.md", "Meeting Notes.md", "Research/Daily Meeting.md"}},
		{"RÉSUMÉ", []string{"Résumé.md"}},
		{"resume", nil},
		{" Summary ", nil},
		{"Research/Summary", nil},
		{"Long extension", []string{"Long extension.markdown"}},
		{"Diagram.excalidraw", []string{"Media/Diagram.excalidraw"}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			var paths []string
			for _, result := range Search(files, tc.query) {
				paths = append(paths, result.Path)
			}
			if !slices.Equal(paths, tc.want) {
				t.Fatalf("paths = %q, want %q", paths, tc.want)
			}
		})
	}
}

func TestEmptyQueryLimit(t *testing.T) {
	var entries []workspace.Entry
	for number := 1; number <= 25; number++ {
		entries = append(entries, workspace.Entry{Path: fmt.Sprintf("Bulk/Bulk %02d.md", number)})
	}
	files := Files(entries, strings.Compare)
	if empty := Search(files, ""); len(empty) != 20 || empty[19].Path != "Bulk/Bulk 20.md" {
		t.Fatalf("default results = %v, want first 20 files", empty)
	}
	if matching := Search(files, "Bulk"); len(matching) != 25 || matching[24].Path != "Bulk/Bulk 25.md" {
		t.Fatalf("matching results = %v, want all 25 files", matching)
	}
}
