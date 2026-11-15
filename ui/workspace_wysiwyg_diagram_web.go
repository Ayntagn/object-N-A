package ui

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"cyberwriter/internal/core/mermaid"
	"cyberwriter/ui/wysiwyg"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

const visualDiagramEditURL = "cyberwriter-diagram://edit"
const visualVegaSelectURL = "cyberwriter-diagram://select"
const visualDiagramInteractionJS = `
document.querySelector("main").addEventListener("click", event => {
  if (!event.isTrusted || event.button !== 0) return;
  event.preventDefault();
  location.href = "` + visualDiagramEditURL + `";
}, true);
`

const visualDiagramMeasure = `
const stage = document.querySelector("main");
for (let n = 0; n < 200; n++) {
  if (stage.querySelector("[data-mermaid-state]") || !stage.querySelector("pre.mermaid")) break;
  await new Promise(resolve => setTimeout(resolve, 20));
}
document.body.style.background = "var(--card-background)";
return stage.getBoundingClientRect().height;
`

const visualVegaRender = `
async function renderVisualVega() {
  const stage = document.querySelector("main");
  const chart = stage.querySelector(vegaSelector);
  if (!chart.firstElementChild.textContent.trim()) {
    chart.dataset.vegaDone = "empty";
    const status = document.createElement("span");
    status.className = "vega-empty";
    status.setAttribute("role", "status");
    status.textContent = "(empty chart — click to edit)";
    chart.append(status);
  } else {
    await renderVegaCharts(stage, {
      errorText: message => "Chart error:\n" + message,
      blockLinks: true,
    });
  }
  stage.addEventListener("click", event => {
    if (event.target.closest?.("a")) event.preventDefault();
    if (event.isTrusted && event.button === 0) location.href = "` + visualVegaSelectURL + `";
  }, true);
  stage.addEventListener("auxclick", event => {
    if (event.target.closest?.("a")) event.preventDefault();
  }, true);
  return stage.getBoundingClientRect().height;
}
return await renderVisualVega();
`

const visualVegaMeasure = `
// Vega updates container-sized layouts on the viewport resize event.
const stage = document.querySelector("main");
const chart = stage.querySelector("[data-vega-preview]");
if (chart.dataset.vegaDone === "rendered") {
  window.dispatchEvent(new Event("resize"));
  await new Promise(resolve => setTimeout(resolve, 50));
}
return stage.getBoundingClientRect().height;
`

type visualDiagramWeb struct {
	view       *mygo.WebView
	cancel     context.CancelFunc
	stopLoad   func()
	generation uint64
	frame      visualDiagramFrame
	offset     int
	width      int
	height     float64
	ready      bool
	failed     bool
	origin     previewResourceOrigin
	chartFiles *previewChartFiles
	stopWatch  context.CancelFunc
}

type visualDiagramFrame struct {
	diagram              *wysiwyg.Diagram
	canvas, card, text   mygoui.Color
	link, codeBackground mygoui.Color
}

var visualDiagramScriptHash = sync.OnceValue(func() string {
	sum := sha256.Sum256([]byte(previewMermaidJS + visualDiagramInteractionJS))
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
})

func (stage *visualDiagramWeb) Close() error {
	if stage.cancel != nil {
		stage.cancel()
	}
	stage.watchChartFiles(nil, nil, nil)
	if stage.stopLoad != nil {
		stage.stopLoad()
		stage.stopLoad = nil
	}
	if stage.view != nil && !stage.view.IsDestroyed() {
		stage.view.Destroy()
	}
	stage.view = nil
	return nil
}

func (w *workspaceWYSIWYG) drawDiagramWeb(c *mygoui.Context, app *workspaceWindow, element mygoui.Element, offset int, frame visualDiagramFrame) {
	stage := mygoui.Local(element, "diagram WebView", func() visualDiagramWeb { return visualDiagramWeb{} })
	stage.offset = offset
	width := int(element.Bounds().W + 0.5)
	mode := frame.diagram.VegaMode()
	label := "Mermaid"
	var origin previewResourceOrigin
	if mode != "" {
		label = "Vega"
		origin = app.previewOrigin()
		origin.index = nil // Chart preparation only needs the Vault and note path.
	}
	if stage.frame != frame || stage.origin != origin {
		stage.load(app, w, frame, width, origin)
	} else if stage.ready && width > 0 && width != stage.width {
		stage.measure(app, w, width)
	}
	if stage.ready && w.reserveDiagramHeight(frame.diagram, stage.height) {
		c.Invalidate()
	}
	if stage.failed || stage.view == nil {
		status := mygoui.Text(c, label+" preview unavailable").Role(mygoui.RoleStatus).Padding(8)
		if mode != "" {
			status.FillWidth().Height(60).TextAlign(mygoui.Center).
				OnClick(func() { w.selectVegaDiagram(app, offset, frame.diagram) })
		}
		return
	}
	mygoui.WebView(c, stage.view).Fill().PassThrough().Label(label + " diagram preview").IgnoreInput(app.modalOpen)
}

func (stage *visualDiagramWeb) nextRequest() (context.Context, context.CancelFunc, uint64) {
	if stage.cancel != nil {
		stage.cancel()
	}
	if stage.stopLoad != nil {
		stage.stopLoad()
		stage.stopLoad = nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	stage.cancel = cancel
	stage.generation++
	return ctx, cancel, stage.generation
}

func (stage *visualDiagramWeb) load(app *workspaceWindow, w *workspaceWYSIWYG, frame visualDiagramFrame, width int, origin previewResourceOrigin) {
	stage.frame, stage.width, stage.ready, stage.failed = frame, width, false, false
	stage.origin = origin
	stage.watchChartFiles(nil, nil, nil)
	win := app.win
	if win == nil || !mygo.WebViewAvailable() {
		stage.failed = true
		return
	}
	if stage.view == nil || stage.view.IsDestroyed() {
		view, err := win.NewWebView(mygo.WebViewOptions{
			Transparent:       true,
			ScrollToContainer: true,
			Page:              mygo.PageOptions{Untrusted: true, DevTools: mygo.DevToolsDisabled},
		})
		if err != nil {
			stage.failed = true
			return
		}
		stage.view = view
		view.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
			e.PreventDefault()
			if e.URL != visualDiagramEditURL && (e.URL != visualVegaSelectURL || !e.UserInitiated) {
				return
			}
			win.Update(func() {
				if stage.view == view && &app.wys == w {
					diagram := stage.frame.diagram
					if diagram.VegaMode() == "" && e.URL == visualDiagramEditURL {
						w.beginDiagramEdit(app, stage.offset, diagram)
					} else if diagram.VegaMode() != "" && e.URL == visualVegaSelectURL {
						focusWorkspaceSurface(win)
						w.selectVegaDiagram(app, stage.offset, diagram)
					}
				}
			})
		})
	}
	page := stage.view.Page()
	ctx, cancel, generation := stage.nextRequest()
	go func() {
		defer cancel()
		markup, render, loader, files := prepareVisualDiagram(frame, origin)
		loaded := make(chan struct{})
		win.Update(func() {
			if stage.view != nil && stage.generation == generation && ctx.Err() == nil {
				stage.stopLoad = page.OnDidFinishLoad(func() {
					if stage.view != nil && stage.generation == generation && stage.stopLoad != nil {
						stage.stopLoad()
						stage.stopLoad = nil
						close(loaded)
					}
				})
				page.LoadHTML(markup, "")
			}
		})
		select {
		case <-loaded:
		case <-ctx.Done():
			stage.finish(app, w, generation, nil, nil, ctx.Err())
			return
		}
		var err error
		if loader != nil {
			var script string
			script, err = loader()
			if err == nil {
				_, err = page.EvalContext(ctx, script)
			}
		}
		var height any
		if err == nil {
			height, err = page.EvalContext(ctx, render)
		}
		stage.finish(app, w, generation, height, files, err)
	}()
}

func prepareVisualDiagram(frame visualDiagramFrame, origin previewResourceOrigin) (markup, render string, loader func() (string, error), files *previewChartFiles) {
	mode := frame.diagram.VegaMode()
	if mode == "" {
		return visualDiagramHTML(frame), visualDiagramMeasure, mermaidLoader, nil
	}
	preparer := previewMarkup{relative: origin.note, resources: origin.resources()}
	source := preparer.preparedVega(frame.diagram.Source, mode)
	if preparer.resources != nil {
		files = preparer.resources.chartFiles
	}
	if strings.TrimSpace(source) != "" {
		loader = vegaLoader
	}
	return visualVegaHTML(frame, source), previewVegaJS + visualVegaRender, loader, files
}

func (stage *visualDiagramWeb) measure(app *workspaceWindow, w *workspaceWYSIWYG, width int) {
	stage.width = width
	page := stage.view.Page()
	ctx, cancel, generation := stage.nextRequest()
	measure := visualDiagramMeasure
	if stage.frame.diagram.VegaMode() != "" {
		measure = visualVegaMeasure
	}
	files := stage.chartFiles
	go func() {
		defer cancel()
		height, err := page.EvalContext(ctx, measure)
		stage.finish(app, w, generation, height, files, err)
	}()
}

func (stage *visualDiagramWeb) finish(app *workspaceWindow, w *workspaceWYSIWYG, generation uint64, height any, files *previewChartFiles, err error) {
	app.win.Update(func() {
		if stage.view == nil || stage.generation != generation {
			return
		}
		if stage.stopLoad != nil {
			stage.stopLoad()
			stage.stopLoad = nil
		}
		stage.height, stage.ready = height.(float64)
		stage.failed = err != nil || !stage.ready
		stage.watchChartFiles(app, w, files)
	})
}

func (stage *visualDiagramWeb) watchChartFiles(app *workspaceWindow, w *workspaceWYSIWYG, files *previewChartFiles) {
	if stage.chartFiles == files {
		return
	}
	if stage.stopWatch != nil {
		stage.stopWatch()
		stage.stopWatch = nil
	}
	stage.chartFiles = files
	if files == nil {
		return
	}
	ctx, stop := context.WithCancel(context.Background())
	stage.stopWatch = stop
	win := app.win
	go func() {
		ticks := time.NewTicker(2 * time.Second)
		defer ticks.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticks.C:
				if !files.changed() {
					continue
				}
				win.Update(func() {
					if stage.view != nil && stage.chartFiles == files && ctx.Err() == nil {
						stage.load(app, w, stage.frame, stage.width, stage.origin)
					}
				})
				return
			}
		}
	}()
}

func visualDiagramColor(color mygoui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", color.R, color.G, color.B)
}

func visualVegaHTML(frame visualDiagramFrame, source string) string {
	errorInk, errorPaper := "#b30000", "rgba(255,80,80,.08)"
	if frame.canvas.R < 64 && frame.canvas.G < 64 && frame.canvas.B < 64 {
		errorInk = "#ffaaf7"
		if frame.codeBackground.A != 0 {
			errorPaper = visualDiagramColor(frame.codeBackground)
		}
	}
	return `<!doctype html><meta charset="utf-8"><title>Vega diagram</title>
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src ` + vegaScriptHashes + `; connect-src https: http: data:; img-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<style>
html,body{margin:0;padding:0}body{font:15px system-ui,sans-serif;color:` + visualDiagramColor(frame.text) + `;background:` + visualDiagramColor(frame.card) + `}
` + previewVegaCSS + `
main{min-height:60px}
[data-vega-preview]{box-sizing:border-box;overflow:hidden;min-height:60px;text-align:center}
[data-vega-preview]>.vega-source{display:none}
[data-vega-preview]>.vega-stage{display:block;width:100%;text-align:center}
.vega-stage>svg{display:block;max-width:100%;height:auto;margin:0 auto}
[data-vega-done="rendered"]{padding-bottom:4.296875px}
[data-vega-done="error"],[data-vega-done="empty"]{display:flex;justify-content:center;align-items:flex-start}
.vega-empty{color:rgba(127,127,127,.7);font-style:italic;font-size:12px}
pre.vega-error{max-width:100%;box-sizing:border-box;padding:10px;font:11px ui-monospace,SFMono-Regular,monospace;line-height:19px;min-height:60px;color:` + errorInk + `;background:` + errorPaper + `;text-align:left;overflow-wrap:anywhere;border:1px solid rgba(255,80,80,.3);border-radius:4px}
</style><main><div class="` + frame.diagram.VegaMode() + `"` + vegaBlockMarker + `<code class="vega-source">` + html.EscapeString(source) + `</code></div></main>`
}

func visualDiagramHTML(frame visualDiagramFrame) string {
	diagram := frame.diagram
	background, _ := mermaid.Background(diagram.Info)
	return `<!doctype html><meta charset="utf-8"><title>Mermaid diagram</title>
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src '` + visualDiagramScriptHash() + `' '` + mermaidScriptHash + `'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<style>
html,body{margin:0;padding:0}body{font:15px system-ui,sans-serif;color:` + visualDiagramColor(frame.text) + `;background:` + visualDiagramColor(frame.canvas) + `;--mermaid-link:` + visualDiagramColor(frame.link) + `;--card-background:` + visualDiagramColor(frame.card) + `}
main{min-height:60px;cursor:pointer}pre.mermaid{box-sizing:border-box;margin:0;padding:6px 0;min-height:60px;background:transparent;white-space:pre-wrap;overflow-wrap:anywhere}
` + previewMermaidCSS + `
pre.mermaid[data-mermaid-state="rendered"]{border:0;padding:0}
pre.mermaid svg{overflow:visible}
</style><main>` + mermaidBlockStart + html.EscapeString(background) + `">` + html.EscapeString(diagram.Source) + `</pre></main>
<script>` + previewMermaidJS + visualDiagramInteractionJS + `</script>`
}
