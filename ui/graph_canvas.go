package ui

import (
	"cmp"
	"math"
	"path"
	"slices"
	"strings"
	"time"

	"cyberwriter/internal/core/vaultindex"
	mygoui "github.com/egoist/mygo/ui"
)

type graphCanvasOptions struct {
	Globe, Lines, Cluster, Labels bool
	Compact, ColorByTag           bool
	Shape                         graphShape
	ActivePath, Tag               string
	Highlight                     map[string]float64
}

func (o graphCanvasOptions) cameraDistance() float64 {
	if o.Compact {
		return 2.3
	}
	return 2
}

// Each host owns a canvas and reads the same immutable Vault graph.
type graphCanvas struct {
	graph     *vaultindex.Graph
	globe     bool
	clustered bool
	shape     graphShape
	flat      []graphPosition
	layout    vaultindex.GraphLayout
	spatial   []graphPosition
	shapeFrom []graphPosition
	shapeAt   time.Time
	captions  []string
	clusters  []graphCluster
	tagSlots  []int
	pins      map[string]graphPosition
	degrees   []int
	tour      graphTour

	activePath string
	activeNote int
	nearActive []bool
	nearHover  []bool
	zoom       float64
	pan        mygoui.Point
	yaw, pitch float64
	paused     bool
	emptyClick bool
	spinAt     time.Time
	jiggleAt   time.Time
	drag       graphCanvasDrag

	// Points are in the last painted canvas's local DIPs, also the units
	// of MyGo's pointer events. The hit test uses the visible projection.
	size, pointer mygoui.Point
	pointerInside bool
	scale         float64
	hover         int
	projected     []graphProjectedNote
	drawOrder     []int
	flatColors    graphFlatColors
}

type graphPosition struct{ x, y, z float64 }

type graphProjectedNote struct {
	point              mygoui.Point
	depth              float64
	baseRadius, radius float32
	matched            bool
	searchWeight       float32
}

type graphCanvasDrag struct {
	active, moved bool
	note          int
	start, last   mygoui.Point
	original      graphPosition
	hadPin        bool
	yaw, pitch    float64
}

func (v *graphCanvas) view(c *mygoui.Context, g *vaultindex.Graph, layout vaultindex.GraphLayout, options graphCanvasOptions, openNote func(string)) {
	if v.zoom == 0 {
		v.reset()
	}
	if v.graph != g || v.clustered != options.Cluster || v.shape != options.Shape {
		v.drag = graphCanvasDrag{}
		v.projected = v.projected[:0]
		v.setHover(-1)
		v.rebuild(g, layout, options)
	}
	if v.globe != options.Globe {
		v.cancelTour()
		v.globe = options.Globe
		v.drag = graphCanvasDrag{}
		v.spinAt = time.Time{}
		v.emptyClick = false
		v.setHover(-1)
		v.projected = v.projected[:0]
	}
	if v.activePath != options.ActivePath {
		v.setActive(options.ActivePath)
	}

	theme := *c.Theme()
	canvas := mygoui.Box(c.Key(v)).Grow(1).MinHeight(0).MinWidth(0).FillWidth().Clip().
		Role(mygoui.RoleGroup).Label("Knowledge Graph canvas").Background(theme.Background).
		HandleInput(func(event mygoui.InputEvent) bool {
			return v.input(event, openNote)
		})
	if !canvas.Hovered() {
		v.pointerInside = false
		v.setHover(-1)
	}
	switch {
	case v.drag.active && v.drag.moved:
		canvas.Cursor(mygoui.CursorGrabbing)
	case v.hover >= 0:
		canvas.Cursor(mygoui.CursorPointer)
	case v.globe:
		canvas.Cursor(mygoui.CursorGrab)
	}
	canvas.Draw(func(p *mygoui.Painter, r mygoui.Rect) {
		p.Clip(r, 0, func() { v.paint(p, r, options, theme) })
	})
}

func (v *graphCanvas) reset() {
	v.zoom = 1
	v.pan = mygoui.Point{}
	v.yaw, v.pitch = 0, 0
	v.emptyClick = false
	v.spinAt, v.jiggleAt = time.Time{}, time.Time{}
	v.drag = graphCanvasDrag{}
	v.pins = nil
	v.setHover(-1)
	v.projected = v.projected[:0]
}

func (v *graphCanvas) resetCamera() {
	v.stopTour()
	v.reset()
}

func (v *graphCanvas) rebuild(g *vaultindex.Graph, layout vaultindex.GraphLayout, options graphCanvasOptions) {
	changedGraph := v.graph != g
	changedCluster := v.clustered != options.Cluster
	v.graph, v.clustered, v.shape = g, options.Cluster, options.Shape
	if g == nil {
		v.flat, v.spatial, v.captions = nil, nil, nil
		v.layout = vaultindex.GraphLayout{}
		v.clusters, v.degrees = nil, nil
		v.tagSlots = nil
		v.shapeFrom, v.shapeAt = nil, time.Time{}
		v.tour = graphTour{}
		v.nearActive, v.nearHover = nil, nil
		v.setActive(options.ActivePath)
		return
	}
	if changedGraph {
		v.cancelTour()
		v.layout = layout
		v.captions = make([]string, len(g.Notes))
		v.degrees = make([]int, len(g.Notes))
		v.tagSlots = make([]int, len(g.Notes))
		for _, edge := range v.layout.Edges {
			v.degrees[edge.From]++
			v.degrees[edge.To]++
		}
		for i, note := range g.Notes {
			if len(note.Tags) > 0 {
				v.tagSlots[i] = graphTagSlot(note.Tags[0]) + 1
			}
			base := path.Base(note.Path)
			v.captions[i] = noteName(base)
			if v.captions[i] == "" {
				v.captions[i] = base
			}
		}
		v.nearActive = make([]bool, len(g.Notes))
		v.nearHover = make([]bool, len(g.Notes))
		v.setActive(options.ActivePath)
		// Retain positions dragged in this host for notes still in the Vault.
		if len(v.pins) > 0 {
			pins := make(map[string]graphPosition, len(v.pins))
			for _, note := range g.Notes {
				if point, ok := v.pins[note.Path]; ok {
					pins[note.Path] = point
				}
			}
			v.pins = pins
		}
	}
	if changedGraph || changedCluster {
		v.flat, v.clusters = graphFlatLayout(g.Notes, v.layout.Order, options.Cluster)
	}
	if !changedGraph && v.globe && options.Globe && len(v.spatial) == len(g.Notes) {
		now := time.Now()
		progress := v.shapeProgress(now)
		from := make([]graphPosition, len(v.spatial))
		for i := range from {
			from[i] = v.spatialPoint(i, progress)
		}
		v.shapeFrom, v.shapeAt = from, now
	} else {
		v.shapeFrom, v.shapeAt = nil, time.Time{}
	}
	v.spatial = graphSpatialLayout(g.Notes, v.layout.Order, options.Shape, options.Cluster)
}

func (v *graphCanvas) setActive(path string) {
	v.activePath = path
	v.activeNote = -1
	if v.graph != nil && path != "" {
		v.activeNote = slices.IndexFunc(v.graph.Notes, func(note vaultindex.GraphNote) bool { return note.Path == path })
	}
	v.markNeighbors(v.nearActive, v.activeNote)
}

func (v *graphCanvas) setHover(index int) bool {
	if v.hover == index {
		return false
	}
	v.hover = index
	v.markNeighbors(v.nearHover, index)
	return true
}

func (v *graphCanvas) markNeighbors(neighbors []bool, index int) {
	clear(neighbors)
	if index < 0 {
		return
	}
	for _, edge := range v.layout.Edges {
		switch index {
		case edge.From:
			neighbors[edge.To] = true
		case edge.To:
			neighbors[edge.From] = true
		}
	}
}

// EV-UI-205. Hover temporarily takes over the active document's emphasis.
func (v *graphCanvas) applyEmphasis() (focus int, neighbors []bool) {
	focus, neighbors = v.activeNote, v.nearActive
	if v.hover >= 0 {
		focus, neighbors = v.hover, v.nearHover
	}
	for i := range v.projected {
		point := &v.projected[i]
		weight := point.searchWeight
		if v.hover >= 0 {
			weight = 0
		}
		point.radius = point.baseRadius * graphEmphasisScale(i == focus, neighbors[i], weight)
	}
	return focus, neighbors
}

func graphEmphasisScale(focused, neighbor bool, searchWeight float32) float32 {
	switch {
	case focused:
		return 1.7
	case neighbor:
		return 1.2
	default:
		// EV-UI-187. A lexical result has weight 1. Related weights
		// retain their relative strengths in both canvases.
		return 1 + 1.4*searchWeight
	}
}

const graphGoldenAngle = math.Pi * (3 - 2.23606797749979)

func graphSpherePoint(i, count int) graphPosition {
	angle := float64(i) * graphGoldenAngle
	y := 1 - 2*(float64(i)+0.5)/float64(count)
	ring := math.Sqrt(1 - y*y)
	sin, cos := math.Sincos(angle)
	return graphPosition{ring * cos, y, ring * sin}
}

func (v *graphCanvas) shapeProgress(now time.Time) float64 {
	if v.shapeFrom == nil {
		return 1
	}
	elapsed := now.Sub(v.shapeAt)
	if elapsed >= graphShapeMove {
		v.shapeFrom, v.shapeAt = nil, time.Time{}
		return 1
	}
	return graphEase(float64(elapsed) / float64(graphShapeMove))
}

func (v *graphCanvas) spatialPoint(index int, progress float64) graphPosition {
	point := v.spatial[index]
	if progress < 1 {
		start := v.shapeFrom[index]
		point.x = start.x + (point.x-start.x)*progress
		point.y = start.y + (point.y-start.y)*progress
		point.z = start.z + (point.z-start.z)*progress
	}
	return point
}

func (v *graphCanvas) project(p *mygoui.Painter, r mygoui.Rect, options graphCanvasOptions) (int, graphFlatProjection) {
	v.size = mygoui.Point{X: r.W, Y: r.H}
	now := p.Now()
	oscillation := 1.0
	if !v.jiggleAt.IsZero() {
		phase := now.Sub(v.jiggleAt).Seconds() / 0.40
		if phase >= 0 && phase < 1 {
			oscillation += math.Sin(phase*4*math.Pi) * (1 - phase) * 0.05
			p.AnimationFrameRate(30)
		} else {
			v.jiggleAt = time.Time{}
		}
	}
	tourAnimating := v.touring() || !v.tour.stopping.IsZero()
	rotating := options.Globe && !v.paused && !v.drag.active && !tourAnimating && v.hover < 0
	if rotating {
		if !v.spinAt.IsZero() {
			elapsed := min(0.10, max(0, now.Sub(v.spinAt).Seconds()))
			v.yaw = math.Mod(v.yaw+0.105*elapsed, 2*math.Pi)
		}
		v.spinAt = now
	} else {
		v.spinAt = time.Time{}
	}

	n := len(v.graph.Notes)
	shapeProgress := v.shapeProgress(now)
	if rotating || tourAnimating || options.Globe && shapeProgress < 1 {
		fps := float32(30)
		if n > 4000 {
			fps = 15
		}
		p.AnimationFrameRate(fps)
	}
	base := graphPose{
		pan: v.pan, zoom: v.zoom, distance: options.cameraDistance() / v.zoom,
		rotation: graphEulerRotation(v.pitch, v.yaw),
	}
	pose, tourNote := v.tourFrame(now, base, options)
	v.scale = graphFlatScale(v.size, options.Compact) * pose.zoom
	projection := graphFlatProjection{
		origin: mygoui.Point{X: r.W/2 + pose.pan.X, Y: r.H/2 + pose.pan.Y},
		scale:  v.scale * oscillation,
	}
	v.projected = slices.Grow(v.projected[:0], n)[:n]
	flatRadius := max(7, 2.2*math.Sqrt(float64(n)))
	globeRadius := max(7, 2*math.Sqrt(float64(n)))
	focal := float64(r.H) * math.Sqrt(3) / 2
	filteredBySearch := len(options.Highlight) > 0
	for i, note := range v.graph.Notes {
		point := v.flat[i]
		nodeRadius := 0.18 + min(0.5, 0.045*float64(max(1, v.degrees[i])))
		depth, radius := 0.0, nodeRadius*v.scale/flatRadius
		var screen mygoui.Point
		if options.Globe {
			point = pose.rotation.apply(v.spatialPoint(i, shapeProgress))
			depth, point.y = point.z, -point.y
			distance := pose.distance - depth
			if distance <= 0.1/globeRadius || distance >= 8 {
				v.projected[i] = graphProjectedNote{}
				continue
			}
			scale := focal / distance
			screen = mygoui.Point{X: projection.origin.X + float32(point.x*scale), Y: projection.origin.Y + float32(point.y*scale)}
			radius = max(1.15, nodeRadius*scale/globeRadius)
		} else {
			if pin, ok := v.pins[note.Path]; ok {
				point = pin
			}
			screen = projection.point(point)
		}
		weight, matched := options.Highlight[note.Path]
		if !filteredBySearch && options.Tag != "" {
			matched = slices.Contains(note.Tags, options.Tag)
		}
		v.projected[i] = graphProjectedNote{
			point: screen, depth: depth, baseRadius: float32(radius), matched: matched, searchWeight: float32(weight),
		}
	}
	return tourNote, projection
}

func (v *graphCanvas) paint(p *mygoui.Painter, r mygoui.Rect, options graphCanvasOptions, theme mygoui.Theme) {
	if v.graph == nil || len(v.graph.Notes) == 0 || r.W <= 0 || r.H <= 0 {
		return
	}
	tourNote, flatProjection := v.project(p, r, options)
	if tourNote >= 0 {
		v.setHover(-1)
	}
	focus, nearFocus := v.applyEmphasis()
	// Hit the current projection with its existing emphasis before resolving
	// a new hover. This covers both moving nodes and changes in magnification.
	if tourNote < 0 && v.pointerInside && !v.drag.active && v.setHover(v.hit(v.pointer)) {
		focus, nearFocus = v.applyEmphasis()
	}
	hovering := v.hover >= 0
	normal, active := theme.Accent, theme.Accent
	idleOpacity := float32(0.55)
	blackout := theme.Dark && theme.Background == mygoui.RGB(0, 0, 0)
	if blackout {
		normal, active = mygoui.RGB(122, 216, 254), mygoui.RGB(255, 183, 253)
		idleOpacity = 0.45
		if options.Globe {
			// EV-UI-098 samples the idle and active untagged 3D nodes.
			normal, idleOpacity = mygoui.RGB(111, 211, 255), 0.47
			if focus >= 0 && len(v.graph.Notes[focus].Tags) == 0 {
				active = normal
			}
		} else if options.Compact {
			normal = mygoui.RGB(115, 218, 255)
		} else {
			normal = mygoui.RGB(117, 223, 255)
		}
	}
	if !options.Globe {
		opacity := .18
		if !theme.Dark {
			opacity = .34
		}
		v.flatColors.update(normal, theme.Background, opacity, blackout)
	}
	// The hover-only display omits the faint background edges observed in EV-UI-211.
	if options.Lines && hovering {
		ink := normal.Alpha(0.55)
		if blackout && !options.Globe {
			ink = mygoui.RGB(118, 139, 181)
		}
		for _, edge := range v.layout.Edges {
			if edge.From != v.hover && edge.To != v.hover {
				continue
			}
			a, b := v.projected[edge.From], v.projected[edge.To]
			if a.radius == 0 || b.radius == 0 || a.point.X < 0 && b.point.X < 0 || a.point.Y < 0 && b.point.Y < 0 ||
				a.point.X > r.W && b.point.X > r.W || a.point.Y > r.H && b.point.Y > r.H {
				continue
			}
			pair := [2][2]float32{
				{r.X + a.point.X, r.Y + a.point.Y},
				{r.X + b.point.X, r.Y + b.point.Y},
			}
			p.StrokePolyline(pair[:], 1, ink)
		}
	}
	if options.Globe {
		v.paintClusters(p, r, theme.TextMuted)
	}

	v.drawOrder = slices.Grow(v.drawOrder[:0], len(v.projected))[:len(v.projected)]
	for i := range v.drawOrder {
		v.drawOrder[i] = i
	}
	if options.Globe {
		slices.SortFunc(v.drawOrder, func(a, b int) int {
			return cmp.Compare(v.projected[a].depth, v.projected[b].depth)
		})
	}
	for _, i := range v.drawOrder {
		point := v.projected[i]
		x, y, radius := point.point.X, point.point.Y, point.radius
		if radius == 0 || x+radius < 0 || y+radius < 0 || x-radius > r.W || y-radius > r.H {
			continue
		}
		focused := i == focus
		alpha := idleOpacity
		if options.Globe && !blackout {
			alpha *= float32(0.70 + 0.15*(point.depth+1))
		}
		if focused || nearFocus[i] || !hovering && point.matched {
			alpha = 1
		}
		tagSlot := 0
		if options.ColorByTag {
			tagSlot = v.tagSlots[i]
		}
		ink := normal
		if focused {
			ink = active
		}
		switch {
		case !options.Globe && alpha == 1:
			ink = v.flatColors.bright[tagSlot]
		case !options.Globe:
			ink = v.flatColors.dim[tagSlot]
		case tagSlot != 0:
			ink = graphTagPalette[tagSlot-1].color().Alpha(alpha)
		case !focused:
			ink = ink.Alpha(alpha)
		}
		box := mygoui.Rect{X: r.X + x - radius, Y: r.Y + y - radius, W: 2 * radius, H: 2 * radius}
		if focused && options.Globe {
			p.Stroke(mygoui.Rect{X: box.X - 1.5, Y: box.Y - 1.5, W: box.W + 3, H: box.H + 3},
				ink.Alpha(0.40), radius+1.5, 1)
		}
		p.Fill(box, ink, radius)
		if options.Globe && options.Labels && radius >= 2.6 && i != v.activeNote && i != tourNote {
			drawGraphNoteLabel(p, r, point, v.captions[i], theme.Text.Alpha(max(0.30, alpha)), theme.Surface, false)
		}
	}
	if !options.Globe {
		v.paintFlatClusters(p, r, flatProjection, options.Compact, theme.Dark)
		if options.Labels {
			for i, point := range v.projected {
				if i != v.activeNote && i != tourNote {
					drawFlatNoteLabel(p, r, point, v.captions[i], false, theme.Dark)
				}
			}
		}
	}
	var activeForeground graphNoteLabel
	if v.activeNote >= 0 && v.activeNote != tourNote && v.activeNote != v.hover && (!options.Compact || tourNote < 0) {
		point, name := v.projected[v.activeNote], v.captions[v.activeNote]
		var label graphNoteLabel
		if options.Globe {
			label = layoutGraphNoteLabel(p, r, point, name, theme.Text, theme.Surface, true)
		} else {
			label = layoutFlatNoteLabel(p, r, point, name, options.Compact, theme.Dark)
		}
		label.drawBackground(p)
		if options.Compact && hovering {
			// EV-UI-221. The sidebar active caption's text sits above the hover backdrop.
			activeForeground = label
		} else {
			label.drawText(p)
		}
	}
	if v.hover >= 0 {
		title, tags := v.hoverLabel()
		drawGraphHoverLabel(p, r, v.projected[v.hover], title, tags, options.Compact, theme.Dark, activeForeground)
	}
	if tourNote >= 0 {
		drawGraphTourLabel(p, r, v.projected[tourNote], v.captions[tourNote], theme.Text, options.Compact)
	}
}

func (v *graphCanvas) hoverLabel() (title, tags string) {
	if v.hover < 0 {
		return "", ""
	}
	title = v.captions[v.hover]
	if values := v.graph.Notes[v.hover].Tags; len(values) > 0 {
		tags = "#" + strings.Join(values, "  #")
	}
	return title, tags
}

func drawGraphHoverLabel(p *mygoui.Painter, r mygoui.Rect, note graphProjectedNote, title, tags string, compact, dark bool, activeForeground graphNoteLabel) {
	if note.radius == 0 || note.point.X < 0 || note.point.Y < 0 || note.point.X > r.W || note.point.Y > r.H {
		activeForeground.drawText(p)
		return
	}
	nameSize := float32(15)
	if compact {
		nameSize = 13
	}
	nameInk, tagInk, surface := mygoui.RGB(250, 250, 250), mygoui.RGB(140, 217, 255), mygoui.RGBA(8, 8, 8, 0.85)
	if !dark {
		nameInk, tagInk, surface = mygoui.RGB(26, 26, 26), mygoui.RGB(51, 115, 217), mygoui.RGBA(250, 250, 250, 0.92)
	}
	name := mygoui.Span{Text: title, Font: "Helvetica Neue", Size: nameSize, Weight: 700, Color: nameInk}
	detail := mygoui.Span{Text: tags, Font: "Helvetica Neue", Size: nameSize - 3, Weight: 500, Color: tagInk}
	nameW, nameH := p.MeasureText(0, name)
	width, height := nameW+20, nameH+12
	var detailW float32
	if tags != "" {
		var detailH float32
		detailW, detailH = p.MeasureText(0, detail)
		width, height = max(width, detailW+20), height+detailH+4
	}
	centerX := r.X + note.point.X
	centerY := r.Y + note.point.Y - 26
	p.Fill(mygoui.Rect{X: centerX - width/2, Y: centerY - height/2, W: width, H: height}, surface, 7)
	activeForeground.drawText(p)
	y := centerY - height/2 + 6
	p.RichText(centerX-nameW/2, y, 0, name)
	if tags != "" {
		p.RichText(centerX-detailW/2, y+nameH+4, 0, detail)
	}
}

func (v *graphCanvas) paintClusters(p *mygoui.Painter, r mygoui.Rect, ink mygoui.Color) {
	size := max(10, min(20, r.H*0.055))
	for _, cluster := range v.clusters {
		if cluster.label == "" {
			continue
		}
		var x, y, radius float32
		visible := 0
		for _, i := range cluster.notes {
			note := v.projected[i]
			if note.radius == 0 || note.point.X < 0 || note.point.Y < 0 || note.point.X > r.W || note.point.Y > r.H {
				continue
			}
			x += note.point.X
			y += note.point.Y
			radius = max(radius, note.radius)
			visible++
		}
		if visible == 0 {
			continue
		}
		x, y = x/float32(visible), y/float32(visible)
		caption := mygoui.Span{Text: cluster.label, Size: size, Weight: 600, Color: ink.Alpha(0.44)}
		width, height := p.MeasureText(0, caption)
		x = max(4, min(r.W-width-4, x-width/2))
		gap := max(5, size*0.65)
		top := y - radius - gap - height
		if top < 3 {
			top = y + radius + gap
		}
		p.RichText(r.X+x, r.Y+top, 0, caption)
	}
}

type graphNoteLabel struct {
	title    mygoui.Span
	position mygoui.Point
	box      mygoui.Rect
	surface  mygoui.Color
	corner   float32
}

func (label graphNoteLabel) drawBackground(p *mygoui.Painter) {
	if label.box.W > 0 {
		p.Fill(label.box, label.surface, label.corner)
	}
}

func (label graphNoteLabel) drawText(p *mygoui.Painter) {
	if label.title.Text != "" {
		p.RichText(label.position.X, label.position.Y, 0, label.title)
	}
}

func drawGraphNoteLabel(p *mygoui.Painter, r mygoui.Rect, note graphProjectedNote, name string, ink, surface mygoui.Color, featured bool) {
	label := layoutGraphNoteLabel(p, r, note, name, ink, surface, featured)
	label.drawBackground(p)
	label.drawText(p)
}

func layoutGraphNoteLabel(p *mygoui.Painter, r mygoui.Rect, note graphProjectedNote, name string, ink, surface mygoui.Color, featured bool) graphNoteLabel {
	if note.radius == 0 || note.point.X < 0 || note.point.Y < 0 || note.point.X > r.W || note.point.Y > r.H {
		return graphNoteLabel{}
	}
	size := max(8, min(11, r.H*0.016))
	span := mygoui.Span{Text: name, Size: size, Color: ink}
	if featured {
		span.Weight = 600
	}
	w, h := p.MeasureText(0, span)
	x := max(4, min(r.W-w-4, note.point.X-w/2))
	y := note.point.Y + note.radius + 2
	if y+h > r.H-3 {
		y = note.point.Y - note.radius - h - 2
	}
	label := graphNoteLabel{title: span, position: mygoui.Point{X: r.X + x, Y: r.Y + y}}
	if featured {
		label.box = mygoui.Rect{X: r.X + x - 3, Y: r.Y + y - 1, W: min(w+6, r.W-2), H: h + 2}
		label.surface, label.corner = surface, 4
	}
	return label
}

func drawGraphTourLabel(p *mygoui.Painter, r mygoui.Rect, note graphProjectedNote, name string, ink mygoui.Color, compact bool) {
	if note.radius == 0 || note.point.X < 0 || note.point.Y < 0 || note.point.X > r.W || note.point.Y > r.H {
		return
	}
	size := float32(15)
	if compact {
		size = 11
	}
	span := mygoui.Span{Text: name, Size: size, Weight: 600, Color: ink}
	width, height := p.MeasureText(0, span)
	x := max(4, min(r.W-width-4, note.point.X-width/2))
	y := max(3, min(r.H-height-3, note.point.Y-26-height/2))
	p.RichText(r.X+x, r.Y+y, 0, span)
}

func (v *graphCanvas) hit(at mygoui.Point) int {
	best, distance, depth := -1, float32(math.Inf(1)), math.Inf(-1)
	for i, point := range v.projected {
		if point.radius == 0 {
			continue
		}
		dx, dy := at.X-point.point.X, at.Y-point.point.Y
		d := dx*dx + dy*dy
		// EV-UI-205. Match the presented node and its pointer margin.
		reach := max(9, point.radius+4)
		if d <= reach*reach && (d < distance || d == distance && point.depth > depth) {
			best, distance, depth = i, d, point.depth
		}
	}
	return best
}

func (v *graphCanvas) input(event mygoui.InputEvent, openNote func(string)) bool {
	if v.graph == nil || len(v.graph.Notes) == 0 {
		return false
	}
	at := mygoui.Point{X: event.X, Y: event.Y}
	switch event.Kind {
	case mygoui.InputPointerDown:
		if event.Button != 0 || v.drag.active {
			return false
		}
		v.drag = graphCanvasDrag{active: true, start: at, last: at, note: v.hit(at), yaw: v.yaw, pitch: v.pitch}
		if v.drag.note >= 0 && !v.globe {
			v.drag.original, v.drag.hadPin = v.pins[v.graph.Notes[v.drag.note].Path]
		}
		return true
	case mygoui.InputPointerMove:
		if v.drag.active {
			v.moveDrag(at)
			return true
		}
		v.pointer, v.pointerInside = at, true
		if v.touring() {
			return false
		}
		return v.setHover(v.hit(at))
	case mygoui.InputPointerUp:
		if event.Button != 0 || !v.drag.active {
			return false
		}
		if event.Canceled {
			v.cancelDrag()
			return true
		}
		v.moveDrag(at)
		gesture := v.drag
		v.drag = graphCanvasDrag{}
		v.pointer, v.pointerInside = at, true
		if gesture.moved {
			v.emptyClick = false
			return true
		}
		switch {
		case gesture.note >= 0:
			v.emptyClick = false
			if v.hit(at) == gesture.note && openNote != nil {
				openNote(v.graph.Notes[gesture.note].Path)
			}
		case v.globe && event.Clicks == 2:
			// An empty first click toggled rotation. A double-click resets
			// the camera while restoring the previous rotation state.
			if v.emptyClick {
				v.paused = !v.paused
			}
			v.resetCamera()
		case v.globe:
			v.paused = !v.paused
			v.emptyClick = true
		default:
			if !v.touring() {
				v.reset()
				v.jiggleAt = event.Time
			}
		}
		return true
	case mygoui.InputScroll:
		return !v.drag.active && v.zoomAt(at, math.Exp(-float64(event.DY)/180))
	case mygoui.InputZoom:
		return !v.drag.active && v.zoomAt(at, float64(event.Scale))
	}
	return false
}

func (v *graphCanvas) moveDrag(at mygoui.Point) {
	d := &v.drag
	if !d.moved {
		dx, dy := at.X-d.start.X, at.Y-d.start.Y
		if dx*dx+dy*dy < 9 {
			return
		}
		d.moved = true
		// Keep the presented target available through a click. A drag
		// switches to its own feedback once the pointer actually moves.
		v.setHover(-1)
		if v.touring() {
			v.stopTour()
			v.paused = v.globe
		}
	}
	dx, dy := float64(at.X-d.last.X), float64(at.Y-d.last.Y)
	d.last = at
	switch {
	case v.globe:
		v.yaw += dx * 0.012
		v.pitch = max(-math.Pi/2, min(math.Pi/2, v.pitch+dy*0.012))
	case d.note >= 0 && v.scale > 0:
		path := v.graph.Notes[d.note].Path
		point, ok := v.pins[path]
		if !ok {
			point = v.flat[d.note]
		}
		point.x += dx / v.scale
		point.y += dy / v.scale
		if v.pins == nil {
			v.pins = make(map[string]graphPosition)
		}
		v.pins[path] = point
	}
}

func (v *graphCanvas) cancelDrag() {
	d := v.drag
	v.drag = graphCanvasDrag{}
	v.emptyClick = false
	if v.globe {
		v.yaw, v.pitch = d.yaw, d.pitch
	} else if d.note >= 0 {
		path := v.graph.Notes[d.note].Path
		if d.hadPin {
			v.pins[path] = d.original
		} else {
			delete(v.pins, path)
		}
	}
}

func (v *graphCanvas) zoomAt(at mygoui.Point, factor float64) bool {
	if factor <= 0 || math.IsNaN(factor) {
		return false
	}
	next := min(6, max(0.35, v.zoom*factor))
	if next == v.zoom {
		return false
	}
	ratio := float32(next / v.zoom)
	x, y := at.X-v.size.X/2, at.Y-v.size.Y/2
	v.pan.X, v.pan.Y = x-(x-v.pan.X)*ratio, y-(y-v.pan.Y)*ratio
	v.zoom = next
	return true
}
