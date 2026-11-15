// spec: FEAT-001 / EV-UI-075 / EV-UI-076
package commandpalette

import (
	"slices"
	"testing"
)

func TestFrozenPaletteSearch(t *testing.T) {
	cases := []struct {
		query string
		want  []string
	}{
		{"preview", []string{"Preview Only", "Toggle PDF Preview", "Split View", "Custom Stylesheet", "Toggle Scroll Sync"}},
		{"view", []string{"Query Block", "API History", "Split View", "Preview Only", "Toggle PDF Preview", "Custom Stylesheet", "Toggle Scroll Sync"}},
		{"Strong emphasis", []string{"Bold", "Bold Italic"}},
		{"emphasis strong", []string{"Bold", "Bold Italic"}},
		{" spLiT ", []string{"Split View"}},
		{"heading", []string{"Heading 1", "Heading 2", "Heading 3", "Heading 4", "Heading 5", "Heading 6", "Table of Contents", "Toggle Outline", "Custom Stylesheet"}},
		{"glw", nil},
	}
	for _, test := range cases {
		t.Run(test.query, func(t *testing.T) {
			var names []string
			for _, command := range Search(test.query) {
				names = append(names, command.Name)
			}
			if !slices.Equal(names, test.want) {
				t.Errorf("Search(%q) = %q, want %q", test.query, names, test.want)
			}
		})
	}
}

func TestFrozenPaletteCatalog(t *testing.T) {
	commands := Search("")
	if len(commands) != 128 {
		t.Fatalf("default catalog has %d commands, want 128", len(commands))
	}
	var sections []string
	for _, match := range commands {
		if len(sections) == 0 || sections[len(sections)-1] != match.Section {
			sections = append(sections, match.Section)
		}
	}
	want := []string{
		"HEADINGS", "FORMATTING", "LISTS", "BLOCKS", "LINKS & MEDIA", "TABLES",
		"DIAGRAMS", "CSS STYLES", "INDENTATION", "TEMPLATES", "VIEW", "AI",
		"EXPORT", "SETTINGS", "HELP",
	}
	if !slices.Equal(sections, want) {
		t.Errorf("default sections = %q, want %q", sections, want)
	}
}
