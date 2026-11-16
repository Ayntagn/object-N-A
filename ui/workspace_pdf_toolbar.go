package ui

import (
	"fmt"
	"math"

	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-044: the native actions round a five-point change and clamp at 70–150%.
const (
	pdfFontStep = 5
	pdfFontMin  = 70
	pdfFontMax  = 150

	pdfToolbarInlineWidth = 550
	pdfToolbarTypeWidth   = 660
)

func (app *workspaceWindow) pdfToolbarLayout(c *mygoui.Context, width float32, enabled, essential bool, ink mygoui.Color) {
	p := &app.pdfPreview
	landscape := p.form.layout.Landscape
	scale := p.form.layout.FontScale
	// Reserve the trailing PDF controls when the full parameter row would overflow.
	if width < pdfToolbarInlineWidth {
		chromeMenu(c, "gearshape", "PDF Layout", func(m *mygoui.Menu) {
			m.Submenu("Paper Size", p.paperMenu)
			if m.Item("Landscape").Checked(landscape).Chosen() {
				p.form.layout.Landscape = !landscape
			}
			m.Submenu("Margins", p.marginMenu)
			m.Submenu(fmt.Sprintf("Font Size · %.0f%%", scale), func(m *mygoui.Menu) {
				if m.Item("Decrease Text Size").Disabled(scale <= pdfFontMin).Chosen() {
					p.stepFontScale(-pdfFontStep)
				}
				if m.Item("Increase Text Size").Disabled(scale >= pdfFontMax).Chosen() {
					p.stepFontScale(pdfFontStep)
				}
			})
			m.Submenu("Font", p.fontMenu)
			if essential {
				m.Separator()
				if m.Item("Refresh Preview").Chosen() {
					app.refreshDocumentPreview()
				}
				m.Submenu("Share", app.shareMenu)
				if m.Item(app.pdfLayoutTarget().menuLabel).Chosen() {
					app.togglePDFGridLayout()
				}
				if m.Item(app.paneMaximizeLabel(previewMaximized)).Chosen() {
					app.togglePaneMaximization(previewMaximized)
				}
			}
		}).Size(38, 22).Disabled(!enabled)
		return
	}

	paper := pdfTextMenu(c, p.form.paper, "Paper Size", "doc", 68, enabled, ink, p.paperMenu).MarginEnd(5)
	if width >= pdfToolbarTypeWidth {
		paper.MarginStart(4)
	}
	icon, orientation := "rectangle.portrait", "Portrait Rectangle"
	if landscape {
		icon, orientation = "rectangle", "Landscape Rectangle"
	}
	orientationButton := chromeButtonIcon(c, icon, orientation, paneAction(enabled, func() {
		p.form.layout.Landscape = !p.form.layout.Landscape
	}), 13, ink).Size(22.5, 22).MarginEnd(5).Radius(6).Tooltip("Page orientation")
	blackout := app.blackout && !c.Preferences().HighContrast
	if blackout && !orientationButton.Hovered() && !orientationButton.Pressed() {
		orientationButton.Background(mygoui.Hex("#222222")).Border(1, mygoui.Hex("#303030"))
	}
	margin := fmt.Sprintf("%.0f pt", p.form.layout.Top)
	switch p.form.layout.Top {
	case 54:
		margin = `¾"`
	case 72:
		margin = `1"`
	}
	pdfTextMenu(c, margin, "Margins", "rectangle.dashed", 54, enabled, ink, p.marginMenu).MarginEnd(6)
	scaleFill := c.Theme().Surface
	if blackout {
		scaleFill = blackoutPreviewZoomSurface
	}
	mygoui.Row(c).Size(62.5, 19).Shrink(0).MarginEnd(6).Radius(5).Background(scaleFill).
		AlignItems(mygoui.Center).Children(func() {
		chromeButtonIcon(c, "textformat.size.smaller", "Decrease Text Size",
			paneAction(enabled && scale > pdfFontMin, func() { p.stepFontScale(-pdfFontStep) }), 9, ink).Size(18, 19)
		mygoui.Text(c, fmt.Sprintf("%.0f%%", scale)).FontSize(10).FontWeight(600).Width(26.5).
			TextAlign(mygoui.Center).TextColor(ink).SingleLine()
		chromeButtonIcon(c, "textformat.size.larger", "Increase Text Size",
			paneAction(enabled && scale < pdfFontMax, func() { p.stepFontScale(pdfFontStep) }), 12, ink).Size(18, 19)
	})
	pdfTextMenu(c, "Font", "PDF Font", "textformat", 52, enabled, ink, p.fontMenu)
}

func pdfTextMenu(c *mygoui.Context, label, help, icon string, width float32, enabled bool, ink mygoui.Color, build func(*mygoui.Menu)) mygoui.Element {
	return chromeMenuControl(c, label, build).Size(width, 18).Gap(4).ClipX().Disabled(!enabled).Tooltip(help).
		Children(func() {
			chromeIconColor(c, icon, 14, ink)
			mygoui.Text(c, label).FontSize(11).FontWeight(600).SingleLine().MinWidth(0).Shrink(1).
				Ellipsis("…").TextColor(ink).Role(mygoui.RoleNone)
			if label != "Font" {
				chromeIconColor(c, "chevron.down", 8, ink)
			}
		})
}

func (app *workspaceWindow) pdfToolbarToggle(c *mygoui.Context, enabled bool) {
	theme := c.Theme()
	button := chromeControl(c, "PDF Preview", !enabled, "Exit PDF preview").Size(27, 23).Radius(6).
		AccessSelected(true).OnClick(app.togglePDFPreview)
	surface, ink := theme.Accent, theme.AccentText
	if app.blackout && !c.Preferences().HighContrast {
		surface, ink = mygoui.Hex("#a1caff"), mygoui.RGB(255, 255, 255)
	}
	if button.Pressed() {
		surface = surface.Mix(theme.Background, .12)
	} else if button.Hovered() {
		surface = surface.Mix(ink, .12)
	}
	button.Background(surface).Children(func() { chromeIconColor(c, "doc.fill", 14, ink) })
}

func (p *workspacePDFPreview) stepFontScale(step float64) {
	p.form.layout.FontScale = min(pdfFontMax, max(pdfFontMin, math.Round(p.form.layout.FontScale+step)))
}

func (p *workspacePDFPreview) paperMenu(m *mygoui.Menu) {
	for _, paper := range exportPaperNames {
		if m.Item(paper).Radio(p.form.paper == paper).Chosen() {
			p.form.paper = paper
			p.form.setPaperSize()
		}
	}
}

func (p *workspacePDFPreview) marginMenu(m *mygoui.Menu) {
	for _, preset := range exportMarginPresets {
		if m.Item(preset.label).Radio(p.form.layout.Top == preset.points).Chosen() {
			p.form.setMargins(preset.points)
		}
	}
}

func (p *workspacePDFPreview) fontMenu(m *mygoui.Menu) {
	for _, label := range pdfInheritedFontChoices {
		font := pdfFontChoice(label)
		if m.Item(label).Radio(p.form.layout.FontFamily == font).Chosen() {
			p.form.layout.FontFamily = font
		}
	}
}
