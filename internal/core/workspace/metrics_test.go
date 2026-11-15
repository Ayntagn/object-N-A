// spec: FEAT-001 / EV-UI-009, EV-UI-030, EV-UI-031
package workspace

import "testing"

func TestMeasureTextFromWindowSamples(t *testing.T) {
	tests := []struct {
		source       string
		words, chars int
	}{
		{"# AX 测试笔记\n\n这是主窗口 AX 采集用的一次性 Markdown 夹具。\n\n[[Notes/Child]]\n", 9, 58},
		{"# Nested probe\n\nNESTED_PROBE", 3, 28},
		{"# Root probe\n\nROOT_PROBE", 3, 24},
		{"# Root probe\n\nROOT_PROBE7", 3, 25},
	}
	for _, test := range tests {
		if got := MeasureText(test.source); got != (TextMetrics{Words: test.words, Characters: test.chars, ReadingMinutes: 1}) {
			t.Errorf("MeasureText(%q) = %+v, want %d words, %d chars and 1 min", test.source, got, test.words, test.chars)
		}
	}
}
