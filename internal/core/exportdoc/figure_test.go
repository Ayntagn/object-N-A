// spec: EV-UI-134
package exportdoc

import "testing"

func TestFigureCaption(t *testing.T) {
	counter := 0
	for _, tc := range []struct {
		caption     string
		automatic   bool
		label, text string
		counter     int
	}{
		{" \t\n ", true, "", "", 0},
		{"  Hull plan  ", false, "", "Hull plan", 0},
		{"  FiG.\u00a007: Hull\u200b", false, "Figure 7.", "Hull", 7},
		{"Overview", true, "Figure 8.", "Overview", 8},
		{"3) Deck", false, "Figure 3.", "Deck", 3},
		{"3D rendering", true, "Figure 4.", "3D rendering", 4},
		{"ﬁgure 0002. Hatch", true, "Figure 2.", "Hatch", 2},
		{"Figure 12345. Outlier", false, "", "Figure 12345. Outlier", 2},
	} {
		label, text := FigureCaption(tc.caption, tc.automatic, &counter)
		if label != tc.label || text != tc.text || counter != tc.counter {
			t.Fatalf("caption %q with automatic=%t: got (%q, %q, %d), want (%q, %q, %d)",
				tc.caption, tc.automatic, label, text, counter, tc.label, tc.text, tc.counter)
		}
	}
}
