package ui

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"object-na.local/pdfthumb"
)

// EV-UI-061: the grid shows the generated PDF's first 500 pages.
const pdfGridMaxPages = 500
const pdfGridThumbnailSide = 800
const pdfGridCacheBytes = 24 << 20

//go:embed pdf_grid_viewport.js
var pdfGridViewport string

type pdfThumbnailLayout int

const (
	pdfThumbnailsGrid pdfThumbnailLayout = iota
	pdfThumbnailsSheetColumn
	pdfThumbnailsSheetGrid
)

const pdfGridCSS = `
  *, *::before, *::after { box-sizing: border-box; }
  body { margin: 0; padding: 20px; background: #10141a; font-family: -apple-system, sans-serif; }
  #pages { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 20px; justify-items: center; }
  .page-card { position: relative; display: flex; flex-direction: column; align-items: center; gap: 8px; min-width: 0; }
  .page-card img { max-width: 100%; height: auto; background: white; border-radius: 4px; box-shadow: 0 4px 12px rgba(0,0,0,.4); }
  .page-card img.thumbnail-placeholder { width: calc(var(--thumbnail-width) * 1px);
    aspect-ratio: var(--thumbnail-width) / var(--thumbnail-height); color: transparent; }
  .thumbnail-error { position: absolute; inset: 2px 2px 22px; display: grid; place-items: center;
    color: #444; font-size: 11px; line-height: 1.1; text-align: center; overflow-wrap: anywhere; pointer-events: none; }
  .page-number, #status { color: #888; font-size: 12px; }
  body.export-pages { padding: 16px 8px; background: var(--page-list-background); }
  body.export-pages #pages { grid-template-columns: 1fr; gap: 16px; }
  body.export-pages .page-card { gap: 6px; }
  body.export-pages .page-card img { width: auto; max-width: min(120px, 100%); max-height: 155px; object-fit: contain;
    outline: 1px solid #777; outline-offset: -1px; border-radius: 5px; box-shadow: none; }
  body.export-pages .page-card img.thumbnail-placeholder {
    width: min(120px, calc(var(--thumbnail-width) * 1px), calc(155px * var(--thumbnail-width) / var(--thumbnail-height))); }
  body.export-pages .page-number { color: var(--page-list-text); font-size: 11px; line-height: 14px; }
  body.export-pages.two-columns { padding-top: 8px; }
  body.export-pages.two-columns #pages { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 11px 8px; }
  body.export-pages.two-columns .page-card img { max-width: min(77px, 100%); max-height: 100px; }
  body.export-pages.two-columns .page-card img.thumbnail-placeholder {
    width: min(77px, calc(var(--thumbnail-width) * 1px), calc(100px * var(--thumbnail-width) / var(--thumbnail-height))); }
`

type pdfThumbnailStyle struct {
	layout           pdfThumbnailLayout
	background, text string
}

func pdfCSSColor(color mygoui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", color.R, color.G, color.B)
}

func (style pdfThumbnailStyle) maxSide() int {
	if style.layout != pdfThumbnailsGrid {
		return 360
	}
	return pdfGridThumbnailSide
}

func (p *pdfViewer) setGridLayout(grid bool) {
	if p.grid == grid {
		return
	}
	p.grid = grid
	p.gridProblem = ""
	if !grid {
		p.clearThumbnails()
	}
	if p.view != nil && p.requested == p.generated {
		p.problem = ""
	}
}

func (p *pdfViewer) setExportThumbnails(style pdfThumbnailStyle) {
	p.setGridLayout(true)
	if p.thumbnailStyle == style {
		return
	}
	if p.thumbnailStyle.maxSide() != style.maxSide() {
		p.clearThumbnails()
	}
	p.thumbnailStyle = style
	p.thumbnailStyleVersion++
}

func (p *pdfViewer) clearThumbnails() {
	if p.gridCancel != nil {
		p.gridCancel()
		p.gridCancel = nil
	}
	p.gridContext = nil
	if p.view != nil {
		unpublishPDFGrid(p.view.Page())
	}
	if p.gridFile != "" {
		_ = os.RemoveAll(filepath.Dir(p.gridFile))
	}
	p.gridFile, p.gridProblem = "", ""
}

func (style pdfThumbnailStyle) bodyClass() string {
	switch style.layout {
	case pdfThumbnailsSheetColumn:
		return "export-pages"
	case pdfThumbnailsSheetGrid:
		return "export-pages two-columns"
	default:
		return ""
	}
}

// Both export layouts request 360 px. Change the CSS in place so the page,
// its first viewport and its working set are still available after a switch.
func (p *pdfViewer) applyThumbnailStyle() {
	style := p.thumbnailStyle
	if p.gridFile == "" || p.activeFile != p.gridFile || p.gridContext == nil || p.gridRenderedStyle == style {
		return
	}
	p.gridRenderedStyle = style
	version := p.thumbnailStyleVersion
	page, ctx := p.view.Page(), p.gridContext
	script := fmt.Sprintf(`(() => {
		if (!document.getElementById("pages") || (globalThis.pdfGridStyleVersion ?? 0) > %[1]d) return;
		globalThis.pdfGridStyleVersion = %[1]d;
		document.body.className = %q;
		document.body.style.setProperty("--page-list-background", %q);
		document.body.style.setProperty("--page-list-text", %q);
		globalThis.refreshPDFThumbnailWindow?.();
	})()`, version, style.bodyClass(), style.background, style.text)
	go func() {
		_, _ = page.EvalContext(ctx, script)
	}()
}

// A layout change consumes the cached payload and keeps the preview WebView.
func (p *pdfViewer) layoutFile(win *mygo.Window) string {
	if !p.grid {
		return filepath.Join(p.directory, "preview.pdf")
	}
	if p.gridFile != "" || p.gridCancel != nil || p.gridProblem != "" {
		return p.gridFile
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.gridContext, p.gridCancel = ctx, cancel
	if p.thumbnailCache == nil {
		p.thumbnailCache = pdfthumb.New(pdfGridCacheBytes, 1)
	}
	cache := p.thumbnailCache
	view, payload, style := p.view, p.payload, p.thumbnailStyle
	go func() {
		file, source, err := writePDFGrid(ctx, payload, style, cache)
		if win.IsDestroyed() {
			cancel()
			if file != "" {
				_ = os.RemoveAll(filepath.Dir(file))
			}
			return
		}
		win.Update(func() {
			if ctx.Err() != nil || p.view != view || win.IsDestroyed() {
				cancel()
				if file != "" {
					_ = os.RemoveAll(filepath.Dir(file))
				}
				return
			}
			if err != nil {
				cancel()
				p.gridContext, p.gridCancel = nil, nil
				p.gridProblem = "Could not create PDF thumbnails. " + err.Error()
				return
			}
			if source != nil {
				publishPDFGrid(view.Page(), source)
			}
			p.gridRenderedStyle = style
			p.gridFile = file
		})
	}()
	return ""
}

type pdfGridSource struct {
	ctx    context.Context
	route  string
	count  int
	digest [sha256.Size]byte
	side   int
	render func(context.Context, int) ([]byte, error)
	cache  *pdfthumb.Scheduler
}

func writePDFGrid(ctx context.Context, pdf []byte, style pdfThumbnailStyle, cache *pdfthumb.Scheduler) (string, *pdfGridSource, error) {
	dir, err := os.MkdirTemp("", "cyberwriter-pdf-grid-")
	if err != nil {
		return "", nil, err
	}
	name := filepath.Join(dir, "grid.html")
	file, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	w := bufio.NewWriter(file)
	route := "/" + rand.Text() + "/"
	source, writeErr := renderPlatformPDFGrid(ctx, w, pdf, style, route)
	err = errors.Join(writeErr, w.Flush(), file.Close(), ctx.Err())
	if err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	if source != nil {
		source.ctx, source.route, source.side = ctx, route, style.maxSide()
		source.digest = sha256.Sum256(pdf)
		source.cache = cache
	}
	return name, source, nil
}

func pdfContentPolicy(script string) string {
	if script == "" {
		return "default-src 'none'; style-src 'unsafe-inline'; img-src data:; base-uri 'none'; form-action 'none'"
	}
	digest := sha256.Sum256([]byte(script))
	return "default-src 'none'; style-src 'unsafe-inline'; img-src data: blob:; font-src data: blob:; worker-src blob:; base-uri 'none'; form-action 'none'; script-src blob: 'sha256-" +
		base64.StdEncoding.EncodeToString(digest[:]) + "'"
}

func writePDFGridHeader(w io.Writer, script string, style pdfThumbnailStyle, imageOrigin string) error {
	var variables string
	if style.layout != pdfThumbnailsGrid {
		variables = fmt.Sprintf(":root { --page-list-background: %s; --page-list-text: %s; }", style.background, style.text)
	}
	policy := pdfContentPolicy(script)
	if imageOrigin != "" {
		policy = strings.Replace(policy, "img-src data: blob:", "img-src data: blob: "+imageOrigin, 1)
	}
	_, err := fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="%s">
<title>PDF Preview</title><style>%s%s</style></head><body class="%s">`, policy, pdfGridCSS, variables, style.bodyClass())
	return err
}
