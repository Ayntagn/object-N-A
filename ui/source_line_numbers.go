package ui

import (
	"math"
	"strconv"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	sourceTextTopInset    = 15
	sourceLineNumberWidth = 37
)

// EV-UI-232: the ruler and its trailing rule share the source viewport.
func drawSourceLineNumbers(c *mygoui.Context, viewport mygoui.Element, source *editor.State) {
	mygoui.Box(c.Key("source line numbers")).Width(sourceLineNumberWidth).Shrink(0).
		Role(mygoui.RoleGroup).Label("Source line numbers").HandleInput(func(event mygoui.InputEvent) bool {
		if event.Kind != mygoui.InputScroll || event.DY == 0 {
			return false
		}
		y := source.ScrollY()
		source.SetScrollY(y + float64(event.DY))
		return source.ScrollY() != y
	})
	viewport.DrawOver(func(p *mygoui.Painter, r mygoui.Rect) {
		r.W = min(r.W, sourceLineNumberWidth)
		paintSourceLineNumbers(p, r, source)
	})
}

func paintSourceLineNumbers(p *mygoui.Painter, r mygoui.Rect, source *editor.State) {
	bg := source.Theme.Background
	p.Fill(r, bg, 0)
	ink := mygoui.RGBA(255, 255, 255, 0.1)
	if 299*int(bg.R)+587*int(bg.G)+114*int(bg.B) >= 128000 {
		ink = mygoui.RGBA(0, 0, 0, 0.1)
	}
	p.Fill(mygoui.Rect{X: r.X + r.W - 1.5, Y: r.Y, W: 1, H: r.H}, ink, 0)

	textArea := mygoui.Rect{X: r.X, Y: r.Y + sourceTextTopInset, W: r.W, H: max(0, r.H-sourceTextTopInset)}
	first, last := source.VisibleLines()
	scroll := source.ScrollY()
	spacing := source.LineHeight
	if spacing <= 0 {
		spacing = 1.5
	}
	scale := max(1, p.Scale())
	lineHeight := float32(math.Round(float64(source.Font.Size*spacing*scale))) / scale
	textMetrics := source.Font.Metrics()
	baseline := (lineHeight-textMetrics.Ascent-textMetrics.Descent)/2 + textMetrics.Ascent
	// EV-UI-232: the original uses 11-point system tabular digits.
	font := mygoui.Font{Family: "system-ui, monospace", Size: 11, Features: "tnum"}
	p.Clip(textArea, 0, func() {
		for line := max(0, first); line <= last && line < source.Buffer.LineCount(); line++ {
			top := source.LineTop(line)
			if source.LineTop(line+1) <= top {
				continue // A folded line takes no room in the source layout.
			}
			number := strconv.Itoa(line + 1)
			layout := mygoui.ShapeText(number, font, 0)
			width, _ := layout.Size()
			const padding = 8.5
			if available := r.W - padding - 4; available > 0 && width > available {
				fittedFont := font
				fittedFont.Size *= available / width
				layout = mygoui.ShapeText(number, fittedFont, 0)
				width, _ = layout.Size()
			}
			x := r.X + r.W - padding - width
			y := textArea.Y + float32(top-scroll) + baseline - layout.Lines()[0].Baseline
			p.TextLayout(layout, x, y, source.Theme.LineNumber)
		}
	})
}
