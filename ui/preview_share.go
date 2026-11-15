package ui

import (
	"sync"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

var previewShareIcons = sync.OnceValue(func() map[string]mygoui.MenuIcon {
	icons := make(map[string]mygoui.MenuIcon)
	for _, symbol := range []string{"doc.richtext", "doc.text", "chevron.left.forwardslash.chevron.right",
		"slider.horizontal.3", "safari", "square.grid.2x2", "rectangle.grid.1x2"} {
		var icon *mygo.MenuIcon
		if png := nativeMenuSymbol(symbol); len(png) != 0 {
			icon, _ = mygo.NewMenuIcon(png, true)
		}
		if icon == nil && chromeIconFallbacks[symbol] != nil {
			icon, _ = mygo.NewMenuIconSVG(chromeIconFallbacks[symbol], true)
		}
		if icon != nil {
			icons[symbol] = icon
		}
	}
	return icons
})

// EV-UI-061: these commands act on the current preview document.
func (app *workspaceWindow) shareMenu(m *mygoui.Menu) {
	icons := previewShareIcons()
	for _, item := range []struct {
		label, symbol string
		format        exportFormat
	}{
		{"Export as PDF", "doc.richtext", exportPDF},
		{"Export as Word", "doc.text", exportDOCX},
		{"Export as HTML", "chevron.left.forwardslash.chevron.right", exportHTML},
	} {
		if m.Item(item.label).Icon(icons[item.symbol]).Chosen() {
			form := newExportPageForm(app.preferences.Export)
			if item.format == exportPDF || item.format == exportDOCX {
				form = app.documentPDFForm()
			}
			if app.pdfPreview.enabled && item.format != exportDOCX {
				form = app.pdfPreview.form
			}
			app.quickExport(item.format, form)
		}
	}
	m.Separator()
	if m.Item("Export with Options...").Icon(icons["slider.horizontal.3"]).Chosen() {
		app.showExport()
	}
	m.Separator()
	if m.Item("Open in Browser").Icon(icons["safari"]).Chosen() {
		app.openPreviewInBrowser()
	}
	m.Separator()
	target := app.pdfLayoutTarget()
	if m.Item(target.menuLabel).Icon(icons[target.symbol]).Chosen() {
		app.togglePDFGridLayout()
	}
}

type pdfLayoutControl struct{ menuLabel, buttonLabel, symbol, help string }

func (app *workspaceWindow) pdfLayoutTarget() pdfLayoutControl {
	if app.preferences.PDFPreviewGridLayout {
		return pdfLayoutControl{"Column Layout", "Column View", "rectangle.grid.1x2", "Switch to column layout"}
	}
	return pdfLayoutControl{"Grid Layout", "Grid View", "square.grid.2x2", "Switch to grid layout"}
}

func (app *workspaceWindow) togglePDFGridLayout() {
	if app.draft != nil && !app.modalOpen {
		app.preferences.PDFPreviewGridLayout = !app.preferences.PDFPreviewGridLayout
		app.preferencesChanged()
	}
}
