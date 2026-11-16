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
	sourceNumberPadding   = 8.5
)

// EV-UI-232: the ruler and its trailing rule share the source viewport.
func drawSourceLineNumbers(c *mygoui.Context, viewport mygoui.Element, source *editor.State) {
	ruler := mygoui.Box(c.Key("source line numbers")).Width(sourceLineNumberWidth).Shrink(0).
		Role(mygoui.RoleGroup).Label("Source line numbers").HandleInput(func(event mygoui.InputEvent) bool {
		if event.Kind != mygoui.InputScroll || event.DY == 0 {
			return false
		}
		y := source.ScrollY()
		source.SetScrollY(y + float64(event.DY))
		return source.ScrollY() != y
	})
	numbers := mygoui.Local(ruler, "line number layouts", func() sourceLineNumberCache {
		return sourceLineNumberCache{
			previous: make(map[int]sourceLineNumberLayout),
			current:  make(map[int]sourceLineNumberLayout),
		}
	})
	viewport.DrawOver(func(p *mygoui.Painter, r mygoui.Rect) {
		r.W = min(r.W, sourceLineNumberWidth)
		paintSourceLineNumbers(p, r, source, numbers)
	})
}

type sourceLineNumberLayout struct {
	text     *mygoui.TextLayout
	width    float32
	baseline float32
}

type sourceLineNumberCache struct {
	available float32
	previous  map[int]sourceLineNumberLayout
	current   map[int]sourceLineNumberLayout
}

func (cache *sourceLineNumberCache) layout(number int) sourceLineNumberLayout {
	layout, ok := cache.previous[number]
	if !ok {
		// EV-UI-232: the original uses 11-point system tabular digits.
		font := mygoui.Font{Family: "system-ui, monospace", Size: 11, Features: "tnum"}
		label := strconv.Itoa(number)
		shaped := mygoui.ShapeText(label, font, 0)
		width, _ := shaped.Size()
		if cache.available > 0 && width > cache.available {
			font.Size *= cache.available / width
			shaped = mygoui.ShapeText(label, font, 0)
			width, _ = shaped.Size()
		}
		layout = sourceLineNumberLayout{text: shaped, width: width, baseline: shaped.Lines()[0].Baseline}
	}
	cache.current[number] = layout
	return layout
}

func paintSourceLineNumbers(p *mygoui.Painter, r mygoui.Rect, source *editor.State, numbers *sourceLineNumberCache) {
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
	available := r.W - sourceNumberPadding - 4
	if numbers.available != available {
		numbers.available = available
		clear(numbers.previous)
	}
	last = min(last, source.Buffer.LineCount()-1)
	p.Clip(textArea, 0, func() {
		for line := max(0, first); line <= last; line++ {
			top := source.LineTop(line)
			if source.LineTop(line+1) <= top {
				continue // A folded line takes no room in the source layout.
			}
			layout := numbers.layout(line + 1)
			x := r.X + r.W - sourceNumberPadding - layout.width
			y := textArea.Y + float32(top-scroll) + baseline - layout.baseline
			p.TextLayout(layout.text, x, y, source.Theme.LineNumber)
		}
	})
	// Keep just the numbers this frame drew, including after a scroll jump.
	clear(numbers.previous)
	numbers.previous, numbers.current = numbers.current, numbers.previous
}
