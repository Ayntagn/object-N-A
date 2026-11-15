package ui

import (
	"math"
	"strconv"
	"strings"

	"cyberwriter/internal/core/drawing"
	mygoui "github.com/egoist/mygo/ui"
)

type drawingBounds struct {
	left, top, right, bottom float64
}

func elementDrawingBounds(e drawing.Element) (drawingBounds, bool) {
	b := drawingBounds{e.X, e.Y, e.X + e.Width, e.Y + e.Height}
	switch e.Type {
	case drawing.Rectangle, drawing.Diamond, drawing.Ellipse, drawing.Text:
		if e.Width < 0 || e.Height < 0 {
			return b, false
		}
	case drawing.Arrow, drawing.Line, drawing.FreeDraw:
		if len(e.Points) == 0 {
			return b, false
		}
		x, y := e.X+e.Points[0][0], e.Y+e.Points[0][1]
		b = drawingBounds{x, y, x, y}
		for _, p := range e.Points[1:] {
			x, y := e.X+p[0], e.Y+p[1]
			b.left, b.top = min(b.left, x), min(b.top, y)
			b.right, b.bottom = max(b.right, x), max(b.bottom, y)
		}
	default:
		return b, false
	}
	for _, value := range [4]float64{b.left, b.top, b.right, b.bottom} {
		if !(math.Abs(value) < math.MaxFloat32/32) {
			return b, false
		}
	}
	return b, true
}

func (b drawingBounds) center() drawing.Point {
	return drawing.Point{(b.left + b.right) / 2, (b.top + b.bottom) / 2}
}

func (d *workspaceDrawing) fitInitialView() {
	var extents drawingBounds
	found := false
	for _, element := range d.scene.Elements() {
		if element.IsDeleted || element.Opacity <= 0 {
			continue
		}
		b, valid := elementDrawingBounds(element)
		if !valid {
			continue
		}
		if !found {
			extents, found = b, true
		} else {
			extents.left = min(extents.left, b.left)
			extents.top = min(extents.top, b.top)
			extents.right = max(extents.right, b.right)
			extents.bottom = max(extents.bottom, b.bottom)
		}
	}
	width, height := d.viewport[0], d.viewport[1]
	if !found || (!d.linkedMap && extents.left >= 0 && extents.top >= 44 && extents.right <= width && extents.bottom <= height-44) {
		return
	}
	d.zoom = max(.1, min(1, (width-48)/max(1, extents.right-extents.left), (height-112)/max(1, extents.bottom-extents.top)))
	center := extents.center()
	d.offset = drawing.Point{width/2 - center[0]*d.zoom, height/2 - center[1]*d.zoom}
}

func (d *workspaceDrawing) hit(point drawing.Point, moveCards bool) int {
	elements := d.scene.Elements()
	for index := len(elements) - 1; index >= 0; index-- {
		e := elements[index]
		_, erased := d.gesture.erased[index]
		if erased || e.IsDeleted || e.Opacity <= 0 || (!e.Editable() && !(moveCards && e.MovableCard())) {
			continue
		}
		if hitDrawingElement(e, point, 6/d.zoom) {
			return index
		}
	}
	return -1
}

func hitDrawingElement(e drawing.Element, point drawing.Point, tolerance float64) bool {
	b, valid := elementDrawingBounds(e)
	if !valid {
		return false
	}
	center := b.center()
	if e.Angle != 0 {
		sin, cos := math.Sincos(-e.Angle)
		x, y := point[0]-center[0], point[1]-center[1]
		point = drawing.Point{center[0] + x*cos - y*sin, center[1] + x*sin + y*cos}
	}
	if point[0] < b.left-tolerance || point[0] > b.right+tolerance ||
		point[1] < b.top-tolerance || point[1] > b.bottom+tolerance {
		return false
	}
	switch e.Type {
	case drawing.Diamond, drawing.Ellipse:
		x := math.Abs(point[0]-center[0]) / (e.Width/2 + tolerance)
		y := math.Abs(point[1]-center[1]) / (e.Height/2 + tolerance)
		if e.Type == drawing.Diamond {
			return x+y <= 1
		}
		return x*x+y*y <= 1
	case drawing.Arrow, drawing.Line, drawing.FreeDraw:
		local := drawing.Point{point[0] - e.X, point[1] - e.Y}
		tolerance = max(tolerance, e.StrokeWidth/2)
		last := e.Points[0]
		for _, current := range e.Points {
			if drawingSegmentDistance(local, last, current) <= tolerance {
				return true
			}
			last = current
		}
		return false
	}
	return true
}

func drawingSegmentDistance(point, start, end drawing.Point) float64 {
	dx, dy := end[0]-start[0], end[1]-start[1]
	x, y := point[0]-start[0], point[1]-start[1]
	if dx == 0 && dy == 0 {
		return math.Hypot(x, y)
	}
	fraction := max(0, min(1, (x*dx+y*dy)/(dx*dx+dy*dy)))
	return math.Hypot(x-fraction*dx, y-fraction*dy)
}

func (d *workspaceDrawing) paint(p *mygoui.Painter, r mygoui.Rect, accent mygoui.Color, dark bool) {
	background := drawingColor(d.scene.Background(), mygoui.RGB(255, 255, 255))
	if dark {
		background = darkDrawingColor(background)
	}
	p.Fill(r, background, 0)
	p.Clip(r, 0, func() {
		elements := d.scene.Elements()
		var movingID string
		var delta drawing.Point
		if d.selected >= 0 && d.gesture.active && d.gesture.tool == drawingSelection {
			movingID = elements[d.selected].ID
			delta = d.gesture.delta()
		}
		for index, e := range elements {
			_, erased := d.gesture.erased[index]
			if e.IsDeleted || e.Opacity <= 0 || erased {
				continue
			}
			selected := index == d.selected
			if movingID != "" && (selected || e.ContainerID == movingID) {
				e.X, e.Y = e.X+delta[0], e.Y+delta[1]
			}
			d.paintDrawingElement(p, r, e, selected, accent, dark)
		}
		g := d.gesture
		if g.active && g.tool != drawingSelection && g.tool != drawingHand && g.tool != drawingEraser && g.tool != drawing.Text {
			d.paintDrawingElement(p, r, drawing.Shape(g.tool, g.points), false, accent, dark)
		}
	})
}

func (d *workspaceDrawing) paintDrawingElement(p *mygoui.Painter, r mygoui.Rect, e drawing.Element, selected bool, accent mygoui.Color, dark bool) {
	b, valid := elementDrawingBounds(e)
	if !valid {
		return
	}
	x := func(v float64) float32 { return r.X + float32(v*d.zoom+d.offset[0]) }
	y := func(v float64) float32 { return r.Y + float32(v*d.zoom+d.offset[1]) }
	w, h := float32((b.right-b.left)*d.zoom), float32((b.bottom-b.top)*d.zoom)
	left, top := x(b.left), y(b.top)
	cx, cy := left+w/2, top+h/2
	angle := float32(math.Mod(e.Angle, 2*math.Pi) * 180 / math.Pi)
	radius := float32(math.Hypot(float64(w), float64(h)))/2 + 16
	if cx+radius < r.X || cy+radius < r.Y || cx-radius > r.X+r.W || cy-radius > r.Y+r.H {
		return
	}
	opacity := float32(min(1, e.Opacity/100))
	stroke := drawingColor(e.StrokeColor, mygoui.RGB(30, 30, 30)).Alpha(opacity)
	fill := drawingColor(e.BackgroundColor, mygoui.Transparent).Alpha(opacity)
	if dark {
		stroke, fill = darkDrawingColor(stroke), darkDrawingColor(fill)
	}
	lineWidth := float32(min(128, max(.5, e.StrokeWidth*d.zoom)))
	paintShape := func() {
		if e.Type == drawing.Text {
			paintDrawingText(p, e, left, top, w, d.zoom, stroke)
			return
		}
		var path mygoui.Path
		switch e.Type {
		case drawing.Rectangle:
			path.MoveTo(left, top).LineTo(left+w, top).LineTo(left+w, top+h).LineTo(left, top+h).Close()
		case drawing.Diamond:
			path.MoveTo(cx, top).LineTo(left+w, cy).LineTo(cx, top+h).LineTo(left, cy).Close()
		case drawing.Ellipse:
			ellipseDrawingPath(&path, cx, cy, w/2, h/2)
		case drawing.Arrow, drawing.Line, drawing.FreeDraw:
			point := e.Points[0]
			px, py := x(e.X+point[0]), y(e.Y+point[1])
			if w == 0 && h == 0 {
				p.FillPath(path.Circle(px, py, lineWidth/2), stroke)
				return
			}
			path.MoveTo(px, py)
			traceDrawingLine(&path, e, x, y)
			if len(e.Points) >= 2 {
				if e.StartArrowhead != "" {
					drawArrowhead(&path, e.Points[1], e.Points[0], e, x, y, lineWidth)
				}
				if e.EndArrowhead != "" {
					n := len(e.Points)
					drawArrowhead(&path, e.Points[n-2], e.Points[n-1], e, x, y, lineWidth)
				}
			}
			fill = mygoui.Transparent
		}
		if fill.A != 0 {
			p.FillPath(&path, fill)
		}
		switch e.StrokeStyle {
		case "dashed":
			path.Dash(0, 5*lineWidth, 4*lineWidth)
		case "dotted":
			path.Dash(0, lineWidth, 3*lineWidth)
		}
		p.StrokePath(&path, lineWidth, stroke)
	}
	p.Transform(mygoui.Rotation(angle), cx, cy, func() {
		paintShape()
		if selected {
			p.StrokeDashed(mygoui.Rect{X: left - 4, Y: top - 4, W: w + 8, H: h + 8}, accent, 2, 1)
		}
	})
}

func traceDrawingLine(path *mygoui.Path, element drawing.Element, x, y func(float64) float32) {
	for i, point := range element.Points[1:] {
		px, py := x(element.X+point[0]), y(element.Y+point[1])
		if element.Roundness == nil || element.Roundness.Type != 2 || i+2 == len(element.Points) {
			path.LineTo(px, py)
			continue
		}
		prev, next := element.Points[i], element.Points[i+2]
		inX, inY := x(element.X+prev[0])-px, y(element.Y+prev[1])-py
		outX, outY := x(element.X+next[0])-px, y(element.Y+next[1])-py
		inLen := float32(math.Hypot(float64(inX), float64(inY)))
		outLen := float32(math.Hypot(float64(outX), float64(outY)))
		if inLen == 0 || outLen == 0 {
			path.LineTo(px, py)
			continue
		}
		radius := min(inLen, outLen) / 4
		inX, inY = inX*radius/inLen, inY*radius/inLen
		outX, outY = outX*radius/outLen, outY*radius/outLen
		path.LineTo(px+inX, py+inY).
			CubeTo(px+inX/3, py+inY/3, px+outX/3, py+outY/3, px+outX, py+outY)
	}
}

func ellipseDrawingPath(path *mygoui.Path, x, y, rx, ry float32) {
	const k = 0.5522847498
	path.MoveTo(x+rx, y).
		CubeTo(x+rx, y+ry*k, x+rx*k, y+ry, x, y+ry).
		CubeTo(x-rx*k, y+ry, x-rx, y+ry*k, x-rx, y).
		CubeTo(x-rx, y-ry*k, x-rx*k, y-ry, x, y-ry).
		CubeTo(x+rx*k, y-ry, x+rx, y-ry*k, x+rx, y).Close()
}

func drawArrowhead(path *mygoui.Path, start, end drawing.Point, e drawing.Element, x, y func(float64) float32, width float32) {
	length := math.Hypot(end[0]-start[0], end[1]-start[1])
	if length == 0 {
		return
	}
	dx, dy := float32((end[0]-start[0])/length), float32((end[1]-start[1])/length)
	tipX, tipY := x(e.X+end[0]), y(e.Y+end[1])
	size := min(max(width*4, 6), float32(math.Hypot(float64(tipX-x(e.X+start[0])), float64(tipY-y(e.Y+start[1])))))
	baseX, baseY := tipX-dx*size, tipY-dy*size
	path.MoveTo(baseX-dy*size/2, baseY+dx*size/2).
		LineTo(tipX, tipY).LineTo(baseX+dy*size/2, baseY-dx*size/2)
}

func drawingColor(value string, fallback mygoui.Color) mygoui.Color {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "transparent") {
		return mygoui.Transparent
	}
	hex := strings.TrimPrefix(value, "#")
	switch len(hex) {
	case 3, 4, 6, 8:
		if _, err := strconv.ParseUint(hex, 16, 32); err == nil {
			return mygoui.Hex(value)
		}
	}
	return fallback
}

// The drawing WebView uses invert(93%) hue-rotate(180deg) in dark mode (EV-UI-040).
func darkDrawingColor(color mygoui.Color) mygoui.Color {
	if color.A == 0 {
		return color
	}
	r, g, b := 237.15-.86*float64(color.R), 237.15-.86*float64(color.G), 237.15-.86*float64(color.B)
	luminance := .213*r + .715*g + .072*b
	channel := func(value float64) uint8 {
		return uint8(math.Round(min(255, max(0, 2*luminance-value))))
	}
	return mygoui.RGBA(channel(r), channel(g), channel(b), float32(color.A)/255)
}
