package ui

import "math"

const (
	titleBarHeight    = 52
	documentBarHeight = 38
	statusBarHeight   = 26
	vaultPanelWidth   = 348
	outlinePanelWidth = 200
)

type paneMaximization uint8

const (
	paneNotMaximized paneMaximization = iota
	editorMaximized
	previewMaximized
)

func (app *workspaceWindow) displayedMode() int {
	if app.multiFile.active {
		if !multiFileTabEligible(app.workspaceTab) {
			return previewMode
		}
		if !app.multiFile.rendered {
			return sourceMode
		}
		if app.multiFile.editing != nil && app.multiFile.editing == app.workspaceTab {
			return wysiwygMode
		}
		return previewMode
	}
	if app.mode != wysiwygMode {
		switch app.maximizedPane {
		case editorMaximized:
			return sourceMode
		case previewMaximized:
			return previewMode
		}
	}
	return app.mode
}

func (app *workspaceWindow) visibleSidebars() (vault, outline bool) {
	if app.multiFile.active {
		return app.vaultPanelVisible(), false
	}
	if app.draft != nil && app.maximizedPane != paneNotMaximized {
		return false, false
	}
	return app.vaultPanelVisible(), app.outlineVisible
}

// EV-UI-057: maximization overrides the layout of the current document.
func (app *workspaceWindow) togglePaneMaximization(pane paneMaximization) {
	if app.draft == nil || app.modalOpen {
		return
	}
	if app.maximizedPane == pane {
		app.maximizedPane = paneNotMaximized
	} else {
		app.maximizedPane = pane
	}
	app.modeChanged()
}

type documentPaneLayout struct {
	leading, trailing, gap float32
	source, preview        float32
	divider                float32
}

func (app *workspaceWindow) documentPaneWidths(width float32, vault, outline, split bool) documentPaneLayout {
	panes := documentPanes(width, vault, outline, split)
	if split {
		if panes.gap >= 1 {
			panes.divider = panes.leading + panes.source + panes.gap/2
		}
		panes = panes.withSplitRatio(width, vault, outline, app.split.ratio)
	}
	return panes
}

func documentPanes(windowWidth float32, vault, outline, split bool) documentPaneLayout {
	width := windowWidth
	leading, trailing := float32(8), float32(9)
	if vault {
		width -= vaultPanelWidth
		leading, trailing = 12, 11
	}
	if outline {
		width -= outlinePanelWidth
		if split {
			return splitPanesWithOutline(width, vault)
		}
	}
	if !split {
		if vault {
			trailing = 10
		}
		return documentPaneLayout{leading: leading, trailing: trailing}
	}

	const gap = 16.5
	const narrowSource = 393
	preview := max(0, (width-leading-trailing-gap-0.5)/2)
	balancedSource := preview + 0.5
	// In the narrower samples the source holds its width while the leading
	// and center gaps shrink. Clip the source slot before it overlaps the
	// other pane or sidebar at still narrower sizes.
	preferredSource := max(narrowSource, balancedSource)
	leading = max(0, leading-(preferredSource-balancedSource)/2)
	source := min(preferredSource, max(0, width-leading-trailing-preview))
	center := max(0, width-leading-trailing-source-preview)
	return documentPaneLayout{leading: leading, trailing: trailing, gap: center, source: source, preview: preview}
}

// EV-UI-233: the viewport allocation deducts pane chrome and sidebars from
// the window width. The drag gesture itself uses the full window geometry.
func (panes documentPaneLayout) withSplitRatio(windowWidth float32, vault, outline bool, ratio float32) documentPaneLayout {
	if ratio == 0 {
		ratio = splitDefaultRatio
	}
	const cardGap = 17
	const narrowPane = 240
	room := windowWidth - 16 - 8 - splitDividerHitWidth
	contentWidth := windowWidth
	leading, minSource := float32(8), float32(393)
	if vault {
		room -= 340 + 6 + 8
		contentWidth -= vaultPanelWidth
		leading += 4
	}
	if outline {
		room -= outlinePanelWidth + 8
		contentWidth -= outlinePanelWidth
		leading += 6
		if !vault {
			minSource = 449
		}
	}
	if room < 2*narrowPane {
		return panes
	}
	// Retain the measured default frames except where the native preview
	// uses its 420-point minimum as soon as both logical panes fit.
	if ratio == splitDefaultRatio && (room < splitMinSourceWidth+splitMinPreviewWidth || room >= 2*splitMinPreviewWidth) {
		return panes
	}
	logicalSource := clampSplitSource(room, room*ratio)
	if room < splitMinSourceWidth+splitMinPreviewWidth {
		logicalSource = min(max(room*ratio, narrowPane), room-narrowPane)
	}
	logicalPreview := room - logicalSource
	const minPreview = 246
	sourceWidth, previewWidth := max(minSource, logicalSource), max(minPreview, logicalPreview)
	sourceX := roundPaneHalfPoint(leading - (sourceWidth-logicalSource)/2)
	previewX := roundPaneHalfPoint(leading + logicalSource + cardGap - (previewWidth-logicalPreview)/2)
	panes.source, panes.preview = roundPaneHalfPoint(sourceWidth), roundPaneHalfPoint(previewWidth)
	if !outline {
		// The native Web view rounds its inner frame to whole points.
		previewX = float32(math.Floor(float64(previewX)))
		panes.preview = float32(math.Floor(float64(panes.preview)))
	}
	panes.leading = sourceX
	panes.gap = previewX - sourceX - panes.source
	panes.trailing = contentWidth - previewX - panes.preview
	if panes.divider > 0 || panes.gap >= 1 {
		panes.divider = leading + roundPaneHalfPoint(logicalSource) + cardGap/2.0
	}
	return panes
}

// EV-UI-216: the outline changes the preferred pane widths and spacing.
func splitPanesWithOutline(width float32, vault bool) documentPaneLayout {
	const balancedGap = 17
	leading, trailing := float32(14), float32(10)
	minSource, minBalance, minPreview := float32(449), float32(0), float32(0)
	minObserved := float32(1094 - outlinePanelWidth)
	if vault {
		leading, trailing = 18, 12
		minSource, minBalance, minPreview = 393, 240, 246
		minObserved = 1030 - vaultPanelWidth - outlinePanelWidth
	}
	reference := max(width, minObserved)
	equal := (reference - leading - trailing - balancedGap) / 2
	balance := max(equal, minBalance)
	source, preview := max(minSource, balance), max(equal, minPreview)
	previewX := roundPaneHalfPoint(reference - trailing - equal - (preview-balance)/2)
	leading = roundPaneHalfPoint(leading - (source-balance)/2)
	gap := previewX - leading - source
	trailing = reference - previewX - preview
	// Below the narrowest measured window, share the lost space between
	// the panes while holding the observed margins and overlap.
	shrink := (reference - width) / 2
	return documentPaneLayout{
		leading: leading, trailing: trailing, gap: gap,
		source: max(0, source-shrink), preview: max(0, preview-shrink),
	}
}

func roundPaneHalfPoint(value float32) float32 {
	return float32(math.Floor(float64(2*value)+0.5) / 2)
}
