package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

// workspaceMathImage uses DIPs. Height includes descent below Ascent.
// Image is nil when the formula cannot be captured at this size.
type workspaceMathImage struct {
	Width, Height, Ascent float32
	Image                 mygoui.ImageSource
	Valid                 bool
	Error                 string
}

type workspaceMathKey struct {
	Source  string  `json:"source"`
	Display bool    `json:"display"`
	Size    float32 `json:"size"`
	Wrap    int     `json:"wrap"`
	Ink     mygoui.Color
}

type workspaceMathEntry struct {
	image workspaceMathImage
	bytes int
	used  uint64
}

// Each workspace keeps one hidden rendering window for its visible formulas.
// The queue and results are owned by the main thread, and each atlas is built
// serially off that thread. None of the renderer state is part of the document.
type workspaceMathRender struct {
	parent  *mygo.Window
	window  *mygo.Window
	entries map[workspaceMathKey]workspaceMathEntry
	queue   []workspaceMathKey
	pending map[workspaceMathKey]bool
	cancel  context.CancelFunc
	bytes   int
	used    uint64
	busy    bool
	closed  bool
	failed  bool
	opaque  bool
}

const workspaceMathBatch = 12
const workspaceMathQueue = 128
const workspaceMathCacheCount = 128
const workspaceMathCacheBytes = 16 << 20

// formula looks up or requests a formula without blocking the editor. The
// second return value means the result has settled, including TeX errors.
func (r *workspaceMathRender) formula(win *mygo.Window, source string, display bool, size float32, ink mygoui.Color, wrap int) (workspaceMathImage, bool) {
	if r.closed || r.failed || win == nil || win.IsDestroyed() || len(source) > 16<<10 ||
		r.parent != nil && r.parent != win || !(size > 0 && size <= 96) || strings.TrimSpace(source) == "" {
		return workspaceMathImage{}, false
	}
	key := workspaceMathKey{Source: source, Display: display, Size: size, Ink: ink, Wrap: wrap}
	if entry, ok := r.entries[key]; ok {
		r.used++
		entry.used = r.used
		r.entries[key] = entry
		return entry.image, true
	}
	if !r.pending[key] && len(r.queue) < workspaceMathQueue {
		if r.pending == nil {
			r.pending = make(map[workspaceMathKey]bool)
		}
		r.pending[key] = true
		r.queue = append(r.queue, key)
	}
	if !r.busy && len(r.queue) > 0 {
		if r.window == nil && !mygo.WebViewAvailable() {
			r.fail()
			return workspaceMathImage{}, false
		}
		r.busy = true
		r.parent = win
		// Collect the rest of this frame's requests before starting the atlas.
		win.Update(func() { r.renderBatch(win) })
	}
	return workspaceMathImage{}, false
}

func (r *workspaceMathRender) Close() error {
	r.closed = true
	if r.cancel != nil {
		r.cancel()
	}
	if r.window != nil && !r.window.IsDestroyed() {
		r.window.Destroy()
	}
	r.parent, r.window = nil, nil
	r.entries, r.pending, r.queue = nil, nil, nil
	return nil
}

func (r *workspaceMathRender) renderBatch(parent *mygo.Window) {
	if r.closed || parent.IsDestroyed() {
		return
	}
	n := min(workspaceMathBatch, len(r.queue))
	if n == 0 {
		r.busy = false
		return
	}
	batch := append([]workspaceMathKey(nil), r.queue[:n]...)
	r.queue = r.queue[n:]
	var ready chan bool
	if r.window == nil || r.window.IsDestroyed() {
		r.window = mygo.NewWindow(mygo.WindowOptions{
			Title: "Formula renderer", Parent: parent, Hidden: true, Transparent: true, SkipTaskbar: true, Frameless: true,
			UseContentSize: true, Width: 1600, Height: 680,
			Page: mygo.PageOptions{Untrusted: true, DevTools: mygo.DevToolsDisabled, BackgroundThrottling: mygo.ThrottlingOff},
		})
		ready = loadWorkspaceMathPage(r.window)
	}
	window, opaque := r.window, r.opaque
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	r.cancel = cancel
	go func() {
		defer cancel()
		if ready != nil {
			select {
			case ok := <-ready:
				if !ok {
					parent.Update(func() { r.fail() })
					return
				}
			case <-ctx.Done():
				parent.Update(func() { r.fail() })
				return
			}
		}
		images, opaque, err := captureWorkspaceMath(ctx, window, batch, opaque)
		parent.Update(func() {
			if r.closed {
				return
			}
			if err != nil {
				r.fail()
				return
			}
			r.opaque = opaque
			for i, rendered := range images {
				r.remember(batch[i], rendered)
				delete(r.pending, batch[i])
			}
			if len(images) < len(batch) {
				r.queue = append(batch[len(images):], r.queue...)
			}
			r.renderBatch(parent)
		})
	}()
}

func (r *workspaceMathRender) fail() {
	if r.closed {
		return
	}
	r.failed, r.busy = true, false
	r.pending, r.queue = nil, nil
	if r.window != nil && !r.window.IsDestroyed() {
		r.window.Destroy()
	}
	r.window = nil
}

func (r *workspaceMathRender) remember(key workspaceMathKey, rendered workspaceMathCrop) {
	if r.entries == nil {
		r.entries = make(map[workspaceMathKey]workspaceMathEntry)
	}
	if rendered.pixels != nil {
		rendered.result.Image = mygoui.NewBitmap(rendered.pixels)
	}
	r.used++
	r.entries[key] = workspaceMathEntry{image: rendered.result, bytes: rendered.bytes, used: r.used}
	r.bytes += rendered.bytes
	for len(r.entries) > workspaceMathCacheCount || r.bytes > workspaceMathCacheBytes {
		var oldestKey workspaceMathKey
		oldest := r.used + 1
		for key, entry := range r.entries {
			if entry.used < oldest {
				oldestKey, oldest = key, entry.used
			}
		}
		r.bytes -= r.entries[oldestKey].bytes
		delete(r.entries, oldestKey)
	}
}

func loadWorkspaceMathPage(window *mygo.Window) chan bool {
	ready := make(chan bool, 1)
	if window.IsDestroyed() {
		ready <- false
		return ready
	}
	page := window.Page()
	loaded := func(ok bool) {
		select {
		case ready <- ok:
		default:
		}
	}
	page.OnDidFinishLoad(func() { loaded(true) })
	page.OnDidFailLoad(func(*mygo.LoadError) { loaded(false) })
	page.OnRenderProcessGone(func(string) { loaded(false) })
	page.OnWillNavigate(func(event *mygo.NavigateEvent) { event.PreventDefault() })
	window.OnClosed(func() { loaded(false) })
	page.LoadHTML(workspaceMathPage, "")
	return ready
}

const workspaceMathPage = `<!doctype html><meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; font-src data:; img-src 'none'; connect-src 'none'; base-uri 'none'; form-action 'none'">
<style>
html,body{margin:0;padding:0;background:transparent;overflow:hidden}
body{font:15px/1.25 system-ui,sans-serif}
main{zoom:2;padding:3px;white-space:nowrap}
.sample{display:block;width:max-content;padding:3px}
.sample .math-inline{display:inline-block;white-space:break-spaces}
.sample .katex-display{display:inline-block;margin:0}
.baseline{display:inline-block;width:0;height:0;vertical-align:baseline}
</style><style id="custom-stylesheet"></style><main></main>`

// Keep the existing preview engine and fonts in the page after the first
// batch. The input is a JSON value and reaches the page only as textContent.
const workspaceMathAtlas = `
  const stage = document.querySelector("main");
  stage.replaceChildren();
  for (const item of items) {
    const sample = document.createElement("div");
    sample.className = "sample";
    sample.style.fontSize = item.size + "px";
    sample.style.color = "rgba(" + [item.Ink.R,item.Ink.G,item.Ink.B,item.Ink.A/255].join(",") + ")";
    const formula = document.createElement("span");
    formula.className = item.display ? "math-display" : "math-inline";
    if (item.wrap > 0) formula.style.maxWidth = item.wrap + "px";
    formula.textContent = item.source;
    sample.append(formula);
    stage.append(sample);
  }
  window.renderWorkspaceMath(stage);
  for (const sample of stage.children) {
    const marker = document.createElement("i");
    marker.className = "baseline";
    (sample.querySelector(".katex-html .base") || sample.firstElementChild).append(marker);
  }
  // Flush layout before waiting for the WOFF2 faces it needs.
  stage.getBoundingClientRect();
  await document.fonts.ready;
  const width = innerWidth, height = innerHeight;
  const boxes = [];
  for (const sample of stage.children) {
    const baseline = sample.querySelector(".baseline").getBoundingClientRect().top;
    const error = sample.querySelector(".katex-error");
    const struts = Array.from(sample.querySelectorAll(".katex-html .katex-strut"));
    const content = sample.firstElementChild.getBoundingClientRect();
    const lines = struts.length ? struts.map(strut => strut.getBoundingClientRect()) : [content];
    // Keep one logical point of glyph overhang around the struts.
    const left = Math.floor(content.left - 2);
    const top = Math.floor(Math.min(...lines.map(r => r.top)) - 2);
    const right = Math.ceil(content.right + 2);
    const bottom = Math.ceil(Math.max(...lines.map(r => r.bottom)) + 2);
    const rowHeight = sample.getBoundingClientRect().height;
    const box = {left, top, right, bottom, baseline, error: error?.title || "", visible: true};
    if (right > width || rowHeight + 12 > height) {
      box.visible = false;
    } else if (bottom > height && boxes.length) {
      break;
    } else if (left < 0 || top < 0 || bottom > height || right <= left || bottom <= top) {
      box.visible = false;
    }
    boxes.push(box);
  }
  return {width, height, boxes};
`

type workspaceMathBox struct {
	Left, Top, Right, Bottom int
	Baseline                 float64
	Error                    string
	Visible                  bool
}

type workspaceMathAtlasMetrics struct {
	Width, Height int
	Boxes         []workspaceMathBox
}

type workspaceMathCrop struct {
	result workspaceMathImage
	pixels *image.RGBA
	bytes  int
}

func captureWorkspaceMath(ctx context.Context, window *mygo.Window, items []workspaceMathKey, opaque bool) ([]workspaceMathCrop, bool, error) {
	page := window.Page()
	// This checks for the installed function without re-sending the 600 KB
	// KaTeX script and fonts on subsequent atlases.
	installed, err := page.EvalContext(ctx, `typeof window.renderWorkspaceMath === "function"`)
	if err != nil {
		return nil, opaque, err
	}
	if installed != true {
		if _, err := page.EvalContext(ctx, `(function(){`+previewMathScript()+`window.renderWorkspaceMath = renderMath; return true;})()`); err != nil {
			return nil, opaque, err
		}
	}
	literal, err := json.Marshal(items)
	if err != nil {
		return nil, opaque, err
	}
	rawMetrics, err := page.EvalContext(ctx,
		`(async()=>{const items=`+string(literal)+`;return JSON.stringify(await (async()=>{`+workspaceMathAtlas+`})());})()`)
	if err != nil {
		return nil, opaque, err
	}
	var metrics workspaceMathAtlasMetrics
	encoded, ok := rawMetrics.(string)
	if !ok || json.Unmarshal([]byte(encoded), &metrics) != nil || len(metrics.Boxes) == 0 ||
		len(metrics.Boxes) > len(items) || metrics.Width < 100 || metrics.Height < 100 {
		return nil, opaque, fmt.Errorf("formula atlas has invalid metrics")
	}
	result := make([]workspaceMathCrop, len(metrics.Boxes))
	var needsSnapshot bool
	for i, box := range metrics.Boxes {
		result[i].result.Error = box.Error
		needsSnapshot = needsSnapshot || box.Visible
	}
	if !needsSnapshot {
		return result, opaque, nil
	}
	if ctx.Err() != nil {
		return nil, opaque, ctx.Err()
	}
	var dark, light image.Image
	if !opaque {
		dark, err = captureWorkspaceMathPNG(window)
		if err != nil {
			return nil, opaque, err
		}
		// The atlas leaves the origin clear. Some backends flatten that
		// corner to an opaque color, so use black and white mattes below.
		_, _, _, alpha := dark.At(0, 0).RGBA()
		opaque = alpha == 0xffff
	}
	if opaque {
		for i, background := range []string{"black", "white"} {
			if _, err := page.EvalContext(ctx, `(document.documentElement.style.background="`+background+`")`); err != nil {
				return nil, opaque, err
			}
			bitmap, err := captureWorkspaceMathPNG(window)
			if err != nil {
				return nil, opaque, err
			}
			if i == 0 {
				dark = bitmap
			} else {
				light = bitmap
			}
		}
	}
	for i, box := range metrics.Boxes {
		if box.Visible {
			result[i] = cropWorkspaceMath(metrics, box, dark, light)
		}
	}
	return result, opaque, nil
}

func captureWorkspaceMathPNG(window *mygo.Window) (image.Image, error) {
	data, err := window.CapturePage()
	if err != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(data))
}

func cropWorkspaceMath(metrics workspaceMathAtlasMetrics, box workspaceMathBox, dark, light image.Image) workspaceMathCrop {
	const zoom = 2
	scaleX := float64(dark.Bounds().Dx()) / float64(metrics.Width)
	scaleY := float64(dark.Bounds().Dy()) / float64(metrics.Height)
	minX, minY := int(math.Floor(float64(box.Left)*scaleX)), int(math.Floor(float64(box.Top)*scaleY))
	maxX, maxY := int(math.Ceil(float64(box.Right)*scaleX)), int(math.Ceil(float64(box.Bottom)*scaleY))
	area := image.Rect(minX, minY, maxX, maxY).Intersect(dark.Bounds())
	result := workspaceMathCrop{result: workspaceMathImage{Error: box.Error}}
	if area.Empty() || area.Dx()*area.Dy()*4 > workspaceMathCacheBytes || light != nil && !area.In(light.Bounds()) {
		return result
	}
	pixels := image.NewRGBA(image.Rect(0, 0, area.Dx(), area.Dy()))
	if light == nil {
		draw.Draw(pixels, pixels.Bounds(), dark, area.Min, draw.Src)
	} else {
		for y := range area.Dy() {
			for x := range area.Dx() {
				px, py := x+area.Min.X, y+area.Min.Y
				br, bg, bb, _ := dark.At(px, py).RGBA()
				wr, wg, wb, _ := light.At(px, py).RGBA()
				coverage := 255 - (max(int(wr+wg+wb)-int(br+bg+bb), 0)+385)/771
				a := uint8(coverage)
				pixels.SetRGBA(x, y, color.RGBA{R: min(uint8(br>>8), a), G: min(uint8(bg>>8), a), B: min(uint8(bb>>8), a), A: a})
			}
		}
	}
	var painted bool
	for i := 3; i < len(pixels.Pix); i += 4 {
		if pixels.Pix[i] != 0 {
			painted = true
			break
		}
	}
	if !painted {
		return result
	}
	result.result.Width = float32(float64(area.Dx()) / scaleX / zoom)
	result.result.Height = float32(float64(area.Dy()) / scaleY / zoom)
	result.result.Ascent = min(result.result.Height, max(1, float32((box.Baseline-float64(area.Min.Y)/scaleY)/zoom)))
	result.result.Valid = box.Error == ""
	result.pixels = pixels
	result.bytes = len(pixels.Pix)
	return result
}
