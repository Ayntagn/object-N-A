package drawing

import (
	"math"
	"slices"
	"strings"
)

const ExcalifontFamily = 5

type MapCard struct {
	Label, Color string
	Bounds       MapBounds
}

type MapBounds struct {
	X, Y, Width, Height, TextWidth float64
}

type MapArrow struct {
	From, To   int
	Points     []Point
	Reciprocal bool
}

type MapLegendItem struct {
	Label, Color string
	Width        float64
}

// NewVaultMap assembles the linked Excalidraw v2 scene from flowchart geometry.
// EV-UI-080 measures the bindings, text and legend in the generated file.
func NewVaultMap(cards []MapCard, arrows []MapArrow, legend []MapLegendItem) *Scene {
	scene := NewScene()
	count := len(cards)
	scene.elements = make([]Element, 2*count, 2*count+len(arrows)+2*len(legend))
	left, top := math.Inf(1), math.Inf(1)
	for i, item := range cards {
		b := item.Bounds
		card := identifyElement(newElement(Rectangle, b.X, b.Y, b.Width, b.Height))
		card.Link = "[[" + item.Label + "]]"
		card.Roughness = 1
		if item.Color != "" {
			card.BackgroundColor = item.Color
		}
		label := mapText(Point{b.X + (b.Width-b.TextWidth)/2, b.Y + (b.Height-25)/2}, item.Label, b.TextWidth, 20)
		label.ContainerID = card.ID
		label.TextAlign, label.VerticalAlign = "center", "middle"
		card.BoundElements = []BoundElement{{ID: label.ID, Type: Text}}
		scene.elements[i], scene.elements[i+count] = card, label
		left, top = min(left, b.X), min(top, b.Y)
	}
	for _, edge := range arrows {
		from, to := &scene.elements[edge.From], &scene.elements[edge.To]
		points := slices.Clone(edge.Points)
		last := len(points) - 1
		gap := 4.0
		if edge.Reciprocal {
			gap = 1
		}
		points[0] = toward(points[0], points[1], 1)
		points[last] = toward(points[last], points[last-1], gap)
		arrow := identifyElement(Shape(Arrow, points))
		if edge.Reciprocal {
			arrow.EndArrowhead = ""
		}
		arrow.Roughness, arrow.Roundness = 1, &Roundness{Type: 2}
		arrow.StartBinding = &Binding{ElementID: from.ID, Gap: 1}
		arrow.EndBinding = &Binding{ElementID: to.ID, Gap: gap}
		ref := BoundElement{ID: arrow.ID, Type: Arrow}
		from.BoundElements = append(from.BoundElements, ref)
		to.BoundElements = append(to.BoundElements, ref)
		scene.elements = append(scene.elements, arrow)
	}
	if count == 0 {
		return scene
	}
	y := top - 36 - 34*float64(len(legend))
	for _, item := range legend {
		swatch := identifyElement(newElement(Rectangle, left, y, 22, 22))
		swatch.BackgroundColor, swatch.Roughness = item.Color, 1
		label := mapText(Point{left + 32, y + 2}, item.Label, item.Width, 16)
		scene.elements = append(scene.elements, swatch, label)
		y += 34
	}
	return scene
}

func mapText(position Point, label string, width, size float64) Element {
	e := identifyElement(NewText(position, label, width, size*1.25))
	e.FontFamily, e.FontSize, e.LineHeight = ExcalifontFamily, size, 1.25
	return e
}

func toward(from, to Point, length float64) Point {
	dx, dy := to[0]-from[0], to[1]-from[1]
	if distance := math.Hypot(dx, dy); distance > 0 {
		factor := min(length/distance, .4)
		return Point{from[0] + dx*factor, from[1] + dy*factor}
	}
	return from
}

// WikiCard returns the Wikilink on a visible rectangular card.
func (e Element) WikiCard() string {
	if e.Type != Rectangle || e.IsDeleted || len(e.BoundElements) == 0 || e.Opacity <= 0 {
		return ""
	}
	if strings.HasPrefix(e.Link, "[[") && strings.HasSuffix(e.Link, "]]") && len(e.Link) > 4 {
		return e.Link
	}
	return ""
}

func (e Element) MovableCard() bool {
	return e.WikiCard() != "" && e.ID != "" && !e.Locked && !e.grouped && e.Angle == 0 && e.Width > 0 && e.Height > 0
}

// Move translates a single element or a Wiki card with its text and arrows.
// The whole operation is refused if a related element uses an unsupported binding.
func (s *Scene) Move(index int, dx, dy float64) *Scene {
	card := s.elements[index]
	if card.Editable() {
		return s.Replace(index, card.Move(dx, dy))
	}
	if !card.MovableCard() {
		return s
	}
	next := *s
	next.elements = slices.Clone(s.elements)
	moved := card.Move(dx, dy)
	next.elements[index] = moved
	refs := make(map[string]Kind, len(card.BoundElements))
	for _, ref := range card.BoundElements {
		refs[ref.ID] = ref.Type
	}
	for i, element := range s.elements {
		if i == index || element.IsDeleted {
			continue
		}
		start, end := boundTo(element.StartBinding, card.ID), boundTo(element.EndBinding, card.ID)
		text := element.ContainerID == card.ID
		kind, listed := refs[element.ID]
		if !listed && !start && !end && !text {
			continue
		}
		if element.Locked || element.grouped || element.Angle != 0 || kind != element.Type {
			return s
		}
		switch {
		case text && element.Type == Text:
			next.elements[i] = element.Move(dx, dy)
		case (start || end) && element.Type == Arrow && len(element.Points) >= 2 && !element.Elbowed:
			next.elements[i] = moveMapArrow(element, moved, start, end)
		default:
			return s
		}
		delete(refs, element.ID)
	}
	if len(refs) != 0 {
		return s
	}
	return &next
}

func boundTo(binding *Binding, id string) bool {
	return binding != nil && binding.ElementID == id
}

func moveMapArrow(arrow, card Element, start, end bool) Element {
	points := make([]Point, len(arrow.Points))
	for i, point := range arrow.Points {
		points[i] = Point{arrow.X + point[0], arrow.Y + point[1]}
	}
	last := len(points) - 1
	if start {
		points[0] = mapCardAnchor(card, points[1], arrow.StartBinding.Gap)
	}
	if end {
		points[last] = mapCardAnchor(card, points[last-1], arrow.EndBinding.Gap)
	}
	path := Shape(Arrow, points)
	return arrow.edited(map[string]any{
		"x": path.X, "y": path.Y, "width": path.Width, "height": path.Height, "points": path.Points,
	})
}

func mapCardAnchor(card Element, neighbor Point, gap float64) Point {
	center := Point{card.X + card.Width/2, card.Y + card.Height/2}
	dx, dy := neighbor[0]-center[0], neighbor[1]-center[1]
	scale := max(math.Abs(dx)/(card.Width/2), math.Abs(dy)/(card.Height/2))
	if scale == 0 {
		return Point{center[0] + card.Width/2 + gap, center[1]}
	}
	factor := 1/scale + gap/math.Hypot(dx, dy)
	return Point{center[0] + factor*dx, center[1] + factor*dy}
}
