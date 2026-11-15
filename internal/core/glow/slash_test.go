// spec: FEAT-001 / EV-UI-078 / EV-UI-079
package glow

import (
	"slices"
	"testing"
)

func TestSlashSuggestions(t *testing.T) {
	for _, example := range []struct {
		input string
		want  []string
		more  int
	}{
		{"/", []string{"editor", "split", "preview", "wys", "focus", "slides"}, 39},
		{"/SP", []string{"split", "spell", "transcribe", "dictate"}, 0},
		{"/exp", []string{"export-pdf", "export", "pdf-preview"}, 0},
		{"/note", []string{"voice", "meeting-notes", "snippets"}, 0},
		{"/zzq", nil, 0},
	} {
		menu := SlashSuggestions(example.input)
		var slugs []string
		for _, cmd := range menu.Rows {
			slugs = append(slugs, cmd.Slug)
		}
		if !menu.Open || !slices.Equal(slugs, example.want) || menu.More != example.more {
			t.Errorf("%q: open=%t, rows=%v, more=%d", example.input, menu.Open, slugs, menu.More)
		}
	}
	if SlashSuggestions("/split ").Open {
		t.Error("an ASCII space should close the suggestions")
	}
}
