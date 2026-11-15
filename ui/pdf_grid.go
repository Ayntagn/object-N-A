package ui

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-061: the grid shows the generated PDF's first 500 pages.
const pdfGridMaxPages = 500
const pdfGridThumbnailSide = 800

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
  .page-card { display: flex; flex-direction: column; align-items: center; gap: 8px; min-width: 0; }
  .page-card img { max-width: 100%; height: auto; background: white; border-radius: 4px; box-shadow: 0 4px 12px rgba(0,0,0,.4); }
  .page-number, #status { color: #888; font-size: 12px; }
  body.export-pages { padding: 16px 8px; background: var(--page-list-background); }
  body.export-pages #pages { grid-template-columns: 1fr; gap: 16px; }
  body.export-pages .page-card { gap: 6px; }
  body.export-pages .page-card img { width: auto; max-width: min(120px, 100%); max-height: 155px; object-fit: contain;
    outline: 1px solid #777; outline-offset: -1px; border-radius: 5px; box-shadow: none; }
  body.export-pages .page-number { color: var(--page-list-text); font-size: 11px; line-height: 14px; }
  body.export-pages.two-columns { padding-top: 8px; }
  body.export-pages.two-columns #pages { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 11px 8px; }
  body.export-pages.two-columns .page-card img { max-width: min(77px, 100%); max-height: 100px; }
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
	if p.view != nil && p.requested == p.generated {
		p.problem = ""
	}
}

func (p *pdfViewer) setExportThumbnails(style pdfThumbnailStyle) {
	p.setGridLayout(true)
	if p.thumbnailStyle == style {
		return
	}
	p.clearThumbnails()
	p.thumbnailStyle = style
}

func (p *pdfViewer) clearThumbnails() {
	if p.gridCancel != nil {
		p.gridCancel()
		p.gridCancel = nil
	}
	if p.gridFile != "" {
		_ = os.RemoveAll(filepath.Dir(p.gridFile))
	}
	p.gridFile, p.gridProblem = "", ""
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
	p.gridCancel = cancel
	view, payload, style := p.view, p.payload, p.thumbnailStyle
	go func() {
		file, err := writePDFGrid(ctx, payload, style)
		if win.IsDestroyed() {
			cancel()
			if file != "" {
				_ = os.RemoveAll(filepath.Dir(file))
			}
			return
		}
		win.Update(func() {
			defer cancel()
			if ctx.Err() != nil || p.view != view || win.IsDestroyed() {
				if file != "" {
					_ = os.RemoveAll(filepath.Dir(file))
				}
				return
			}
			p.gridCancel = nil
			if err != nil {
				p.gridProblem = "Could not create PDF thumbnails. " + err.Error()
				return
			}
			p.gridFile = file
		})
	}()
	return ""
}

func writePDFGrid(ctx context.Context, pdf []byte, style pdfThumbnailStyle) (string, error) {
	dir, err := os.MkdirTemp("", "cyberwriter-pdf-grid-")
	if err != nil {
		return "", err
	}
	name := filepath.Join(dir, "grid.html")
	file, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	w := bufio.NewWriter(file)
	writeErr := renderPlatformPDFGrid(ctx, w, pdf, style)
	err = errors.Join(writeErr, w.Flush(), file.Close(), ctx.Err())
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return name, nil
}

func pdfContentPolicy(script string) string {
	if script == "" {
		return "default-src 'none'; style-src 'unsafe-inline'; img-src data:; base-uri 'none'; form-action 'none'"
	}
	digest := sha256.Sum256([]byte(script))
	return "default-src 'none'; style-src 'unsafe-inline'; img-src data: blob:; font-src data: blob:; worker-src blob:; base-uri 'none'; form-action 'none'; script-src blob: 'sha256-" +
		base64.StdEncoding.EncodeToString(digest[:]) + "'"
}

func writePDFGridHeader(w io.Writer, script string, style pdfThumbnailStyle) error {
	var bodyClass, variables string
	if style.layout != pdfThumbnailsGrid {
		bodyClass = "export-pages"
		if style.layout == pdfThumbnailsSheetGrid {
			bodyClass += " two-columns"
		}
		variables = fmt.Sprintf(":root { --page-list-background: %s; --page-list-text: %s; }", style.background, style.text)
	}
	_, err := fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="%s">
<title>PDF Preview</title><style>%s%s</style></head><body class="%s">`, pdfContentPolicy(script), pdfGridCSS, variables, bodyClass)
	return err
}
