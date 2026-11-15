// spec: FEAT-001 / EV-UI-009, EV-UI-010, EV-UI-032, EV-UI-216
package ui

import "testing"

func TestSplitPaneWidthsFromWindowSamples(t *testing.T) {
	for _, sample := range []struct {
		name            string
		width           float32
		vault           bool
		source, preview float32
	}{
		{"expanded", 1294, true, 453.5, 453},
		{"collapsed", 1294, false, 630.5, 630},
		{"narrow", 1140, true, 393, 376},
	} {
		t.Run(sample.name, func(t *testing.T) {
			layout := documentPanes(sample.width, sample.vault, false, true)
			if layout.source != sample.source || layout.preview != sample.preview {
				t.Fatalf("widths = %v and %v, want %v and %v", layout.source, layout.preview, sample.source, sample.preview)
			}
		})
	}
}

func TestSplitPaneFramesWithOutline(t *testing.T) {
	for _, sample := range []struct {
		window, sourceX, sourceWidth, previewX, previewWidth float32
		vault                                                bool
	}{
		{1030, 489.5, 393, 797.5, 246, true},
		{1045, 489.5, 393, 805, 246, true},
		{1060, 489.5, 393, 812.5, 246, true},
		{1070, 489.5, 393, 817.5, 246, true},
		{1080, 491, 393, 824, 246, true},
		{1094, 494.5, 393, 832.5, 249.5, true},
		{1144, 507, 393, 857.5, 274.5, true},
		{1194, 519.5, 393, 882.5, 299.5, true},
		{1244, 532, 393, 907.5, 324.5, true},
		{1294, 544.5, 393, 932.5, 349.5, true},
		{1319, 550.5, 393, 945, 362, true},
		{1344, 557, 393, 957.5, 374.5, true},
		{1374, 564.5, 393, 972.5, 389.5, true},
		{1404, 566, 404.5, 987.5, 404.5, true},
		{1460, 566, 432.5, 1015.5, 432.5, true},
		{1094, 203, 449, 657.5, 426.5, false},
		{1144, 214, 451.5, 682.5, 451.5, false},
		{1294, 214, 526.5, 757.5, 526.5, false},
	} {
		layout := documentPanes(sample.window, sample.vault, true, true)
		sourceX := outlinePanelWidth + layout.leading
		if sample.vault {
			sourceX += vaultPanelWidth
		}
		previewX := sourceX + layout.source + layout.gap
		if sourceX != sample.sourceX || layout.source != sample.sourceWidth ||
			previewX != sample.previewX || layout.preview != sample.previewWidth {
			t.Errorf("window %.1f, vault %v: source %.1f×%.1f, preview %.1f×%.1f; want %.1f×%.1f, %.1f×%.1f",
				sample.window, sample.vault, sourceX, layout.source, previewX, layout.preview,
				sample.sourceX, sample.sourceWidth, sample.previewX, sample.previewWidth)
		}
	}
}
