package presentation

// spec: EV-UI-084 / EV-UI-085

import (
	"strings"
	"testing"
	"time"
)

func TestSlideSections(t *testing.T) {
	source := "# Aurora Opening\n\n```text\n---\n```\n\n***\n\n## Boreal\n\n<!-- pagebreak -->\n\n----\n\n# Citrus"
	deck := Slides(source, "title: \"Presentation Atlas\"\ntags: [study-probe]\n", "Note", time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC))
	if deck.Title != "“Presentation Atlas”" || deck.Subtitle != "November 12, 2026 · study-probe" {
		t.Fatalf("cover = %q / %q", deck.Title, deck.Subtitle)
	}
	if len(deck.Pages) != 3 || !strings.Contains(deck.Pages[0], "```text\n---\n```") || !strings.Contains(deck.Pages[1], "<!-- pagebreak -->") {
		t.Fatalf("pages = %q", deck.Pages)
	}

	deck = Slides("# Orchard Seminar\n\nIntro\n\n## Apple\n\n### Lime\n\n#### Keep\n\n## Kiwi", "", "Note", time.Time{})
	if deck.Title != "Orchard Seminar" || len(deck.Pages) != 4 || deck.Pages[2] != "### Lime\n\n#### Keep" {
		t.Fatalf("headings = %#v", deck)
	}
}

func TestFlashcardReview(t *testing.T) {
	const source = "## Alloy prompt\n\nWhich symbol stands for gold?\n\n???\n\nThe answer is **Au**.\n\n---\n\nPrompt only."
	cards := Flashcards(source)
	if len(cards) != 2 || cards[0].Key() != "d3f8e958f0976911" || cards[0].Answer != "The answer is **Au**." || cards[1].Answer != "" {
		t.Fatalf("cards = %#v", cards)
	}
	if key := (Card{Question: "## 中文 😀 café\n\nWhere is the byte boundary?"}).Key(); key != "d05da0cfd126d182" {
		t.Fatalf("UTF-8 question key = %q", key)
	}
	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		rating      Rating
		ease        float64
		days, count int
	}{{Again, 2.3, 1, 0}, {Hard, 2.35, 1, 1}, {Good, 2.5, 1, 1}, {Easy, 2.65, 4, 1}} {
		states := Progress{}
		states.Rate(cards[0], tc.rating, now)
		state := states[cards[0].Key()]
		if state.Ease != tc.ease || state.Interval != tc.days || state.Repetitions != tc.count ||
			state.Last == nil || *state.Last != 816134400 || state.Next-*state.Last != float64(tc.days*86400) ||
			states.Due(cards, now) != 1 {
			t.Errorf("rating %d: %+v, due %d", tc.rating, state, states.Due(cards, now))
		}
	}
	states := Progress{}
	for range 8 {
		states.Rate(cards[0], Again, now)
	}
	if got := states[cards[0].Key()].Ease; got != 1.3 {
		t.Errorf("minimum ease = %v, want 1.3", got)
	}
}
