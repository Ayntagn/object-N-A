//go:build !darwin

package ui

import (
	"bufio"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
)

//go:embed workspace_attachment_pdf.js
var workspacePDFModule string

const workspacePDFHeader = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="%s">
<title>PDF</title>
<style>
  html, body { height: 100%%; margin: 0; }
  body { color-scheme: light dark; background: Canvas; color: CanvasText; font: 14px system-ui, sans-serif; }
  #pages { box-sizing: border-box; height: 100%%; overflow: auto; container-type: inline-size; }
  .pdf-page {
    --page-scale: var(--manual-scale, calc(100cqw / var(--spread-width)));
    --display-width: calc(var(--manual-width, var(--page-width)) * var(--page-scale));
    width: var(--display-width);
    aspect-ratio: var(--page-aspect);
    position: relative; margin: calc(9.5 * var(--page-scale)) auto 0; background: white;
    overflow: hidden; box-shadow: 0 1px 3px #0008;
  }
  #pages.zoomed .pdf-page {
    --manual-width: var(--actual-width, var(--page-width));
    margin-inline: max(calc(4 * var(--page-scale)), calc((100%% - var(--display-width)) / 2));
  }
  .pdf-page:first-child { margin-top: calc(4.75 * var(--page-scale)); }
  .pdf-page:last-child { margin-bottom: calc(4.75 * var(--page-scale)); }
  .pdf-page canvas { position: absolute; inset: 0; width: 100%%; height: 100%%; }
  .text-layer {
    position: absolute; inset: 0; text-align: left; line-height: 1;
    transform-origin: 0 0; text-size-adjust: none; -webkit-text-size-adjust: none;
    --min-font-size: 1;
    --scale-round-x: 1px;
    --scale-round-y: 1px;
    --text-scale: calc(var(--total-scale-factor) * var(--min-font-size));
  }
  .text-layer :is(span, br) {
    position: absolute; color: transparent; white-space: pre; cursor: text; transform-origin: 0 0;
  }
  .text-layer span {
    font-size: calc(var(--text-scale) * var(--font-height, 0px));
    transform: rotate(var(--rotate, 0deg)) scaleX(var(--scale-x, 1)) scale(calc(1 / var(--min-font-size)));
  }
  .text-layer[data-main-rotation="90"] { transform: rotate(90deg) translateY(-100%%); }
  .text-layer[data-main-rotation="180"] { transform: rotate(180deg) translate(-100%%, -100%%); }
  .text-layer[data-main-rotation="270"] { transform: rotate(270deg) translateX(-100%%); }
  .text-layer ::selection { background: #2680e76b; }
  #status, #failure { position: absolute; inset: 0; align-content: center; text-align: center; margin: 0; padding: 24px; }
  #status { pointer-events: none; }
  #failure h1 { margin: 0 0 6px; font-size: 18px; }
  #failure p { margin: 0; color: color-mix(in srgb, CanvasText 72%%, Canvas); overflow-wrap: anywhere; }
  #failure p + p { margin-top: 4px; }
</style>
</head>
<body>
<main id="pages" aria-label="PDF pages" tabindex="0" aria-busy="true"></main>
<p id="status" role="status">Opening PDF…</p>
<section id="failure" role="alert" hidden>
  <h1>Could not open PDF</h1>
  <p>%s</p>
  <p>The file may be corrupt or password-protected.</p>
</section>`

func (viewer *workspacePDFWindow) mount(data []byte) error {
	win := viewer.win
	page := win.Page()
	if page == nil {
		return errors.New("the PDF viewer has no page")
	}
	dir, err := os.MkdirTemp("", "cyberwriter-vault-pdf-")
	if err != nil {
		return err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	defer func() {
		if err != nil {
			cleanup()
		}
	}()

	filename := filepath.Join(dir, "attachment.html")
	err = writeWorkspacePDFFile(filename, win.Title(), data)
	if err != nil {
		return err
	}
	// Install the close handler and start navigation in one main-loop turn.
	// The caller prepares bytes on a worker and may close the window meanwhile.
	err = errors.New("the PDF viewer closed before loading")
	mygo.RunOnMain(func() {
		if win.IsDestroyed() {
			return
		}
		page.OnPageTitleUpdated(func(e *mygo.TitleEvent) { e.PreventDefault() })
		page.OnWillNavigate(func(e *mygo.NavigateEvent) { e.PreventDefault() })
		if err = page.LoadFile(filename); err == nil {
			win.OnClosed(cleanup)
			viewer.zoom = func(action workspacePDFZoom) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_, _ = page.EvalContext(ctx, fmt.Sprintf("window.adjustAttachmentPDFZoom?.(%q)", action))
			}
		}
	})
	return err
}

func writeWorkspacePDFFile(filename, title string, data []byte) error {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(file)
	err = writeWorkspacePDF(w, title, data)
	return errors.Join(err, w.Flush(), file.Close())
}

func writeWorkspacePDF(w io.Writer, title string, data []byte) error {
	if _, err := fmt.Fprintf(w, workspacePDFHeader, pdfContentPolicy(workspacePDFModule), html.EscapeString(title)); err != nil {
		return err
	}
	if err := writePortablePDFBlobs(context.Background(), w, data); err != nil {
		return err
	}
	_, err := io.WriteString(w, `<script type="module">`+workspacePDFModule+`</script></body></html>`)
	return err
}
