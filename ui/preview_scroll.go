package ui

import (
	_ "embed"
	"log"
	"sort"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/editor"
)

const previewScrollEvent = "cyberwriter:preview-scroll"
const previewPaddingEvent = "cyberwriter:preview-padding"

var previewScrolled = mygo.NewEvent[previewScrollTarget](previewScrollEvent)
var previewPaddingChanged = mygo.NewEvent[bool](previewPaddingEvent)

type previewScrollTarget struct {
	Document   uint64  `json:"document"`
	Revision   uint64  `json:"revision"`
	Percentage float64 `json:"percentage"`
	Line       int     `json:"line,omitempty"`
}

type sourceScrollPosition struct {
	editor        *editor.State
	page          *mygo.Page
	width         float32
	height, y     float64
	lineBased     bool
	scrollPastEnd bool
}

// EV-UI-059: the main window routes source motion to the mounted preview.
func (p *workspacePreview) syncSource(app *workspaceWindow, width float32) {
	if app.multiFile.active || app.workspaceTab == nil || app.code == nil || app.drawing != nil ||
		app.displayedMode() != splitMode || !app.preferences.ScrollSyncEnabled {
		p.sourceScroll = sourceScrollPosition{}
		return
	}
	kind := app.previewDocumentType()
	var page *mygo.Page
	if kind == documentHTML {
		if p.html.ready && p.html.view != nil && !p.html.view.IsDestroyed() {
			page = p.html.view.Page()
		}
	} else if !app.pdfPreview.enabled && p.ready && p.view != nil && !p.view.IsDestroyed() {
		page = p.view.Page()
	}
	if page == nil {
		p.sourceScroll = sourceScrollPosition{}
		return
	}
	vault, outline := app.visibleSidebars()
	sourceWidth := app.documentPaneWidths(width, vault, outline, true).source
	target, moved := p.sourceScroll.observe(app.code, page, sourceWidth, *app.preferences)
	if moved {
		target.Document, target.Revision = p.document, p.revision
		if err := previewScrolled.EmitPage(page, target); err != nil {
			log.Printf("Scroll document preview: %v", err)
		}
	}
}

func (previous *sourceScrollPosition) observe(source *editor.State, page *mygo.Page, width float32, prefs workspacePreferences) (previewScrollTarget, bool) {
	current := sourceScrollPosition{
		editor: source, page: page, width: width, height: source.ViewportHeight(), y: source.ScrollY(),
		lineBased: prefs.UseLineBasedScrollSync, scrollPastEnd: prefs.ScrollPastEnd,
	}
	last := *previous
	*previous = current
	// Establish a baseline when the document, viewport or sync mode changes.
	moved := last.y != current.y
	last.y = current.y
	if last != current || current.height <= 0 || !moved {
		return previewScrollTarget{}, false
	}
	lines := source.Buffer.LineCount()
	if current.lineBased {
		line := sort.Search(lines, func(i int) bool { return source.LineTop(i) > current.y })
		return previewScrollTarget{Line: max(1, line)}, true
	}
	extent := source.LineTop(lines) - current.height
	percentage := 0.0
	if extent > 0 {
		percentage = min(1, max(0, current.y/extent))
	}
	return previewScrollTarget{Percentage: percentage}, true
}

//go:embed preview_scroll.js
var previewScrollJS string

const previewScrollCSS = `
html[data-cw-scroll-past-end] body::after { content: ""; display: block; height: 70vh; }
`

func htmlPreviewScrollScript() string {
	return `(() => {` + previewScrollJS + `
window.mygo.on("` + previewScrollEvent + `", scrollPreview);
window.mygo.on("` + previewPaddingEvent + `", setPreviewScrollPadding);
})();`
}
