package ui

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"cyberwriter/internal/core/vaultindex"
	mygoui "github.com/egoist/mygo/ui"
)

type graphCluster struct {
	label  string
	center graphPosition
	notes  []int
}

// Graph's recursive Vault scan visits siblings in localized name order.
func graphScanOrder(notes []vaultindex.GraphNote) []int {
	order := make([]int, len(notes))
	segments := make([][]string, len(notes))
	for i, note := range notes {
		order[i] = i
		segments[i] = strings.Split(note.Path, "/")
	}
	withGraphNameComparer(func(compare func(string, string) int) {
		slices.SortStableFunc(order, func(a, b int) int {
			left, right := segments[a], segments[b]
			for i := range min(len(left), len(right)) {
				if left[i] == right[i] {
					continue
				}
				if relative := compare(left[i], right[i]); relative != 0 {
					return relative
				}
				// Keep different case-equivalent sibling directories together.
				return strings.Compare(left[i], right[i])
			}
			return cmp.Compare(len(left), len(right))
		})
	})
	return order
}

// EV-UI-193. Positions are in units of the scene radius, with y increasing down.
func graphFlatLayout(notes []vaultindex.GraphNote, order []int, clustered bool) ([]graphPosition, []graphCluster) {
	count := len(notes)
	points := make([]graphPosition, count)
	if !clustered {
		for i, index := range order {
			points[index] = graphFlatSpiralPoint(i, count, 1)
		}
		return points, nil
	}
	groups := graphNoteGroups(notes, order)
	clusters := make([]graphCluster, 0, len(groups))
	for i, group := range groups {
		var center graphPosition
		if len(groups) > 1 {
			center = graphFlatSpiralPoint(i, len(groups), .6)
		}
		if group.tag != graphUntagged {
			clusters = append(clusters, graphCluster{label: "#" + group.tag, center: center, notes: group.ids})
		}
		spread := .03 + .55*math.Sqrt(float64(len(group.ids))/float64(count))
		for j, index := range group.ids {
			point := center
			if len(group.ids) > 1 {
				offset := graphFlatSpiralPoint(j, len(group.ids), spread)
				point.x += offset.x
				point.y += offset.y
			}
			points[index] = point
		}
	}
	return points, clusters
}

func graphFlatSpiralPoint(index, count int, radius float64) graphPosition {
	angle := float64(index) * graphGoldenAngle
	radius *= math.Sqrt((float64(index) + .5) / float64(count))
	sin, cos := math.Sincos(angle)
	return graphPosition{x: radius * cos, y: -radius * sin}
}

func graphFlatScale(size mygoui.Point, compact bool) float64 {
	if compact {
		return float64(size.Y) / 2
	}
	return float64(size.Y) / (2 * 1.08)
}

type graphFlatProjection struct {
	origin mygoui.Point
	scale  float64
}

func (projection graphFlatProjection) point(p graphPosition) mygoui.Point {
	return mygoui.Point{
		X: projection.origin.X + float32(p.x*projection.scale),
		Y: projection.origin.Y + float32(p.y*projection.scale),
	}
}

func (v *graphCanvas) paintFlatClusters(p *mygoui.Painter, r mygoui.Rect, projection graphFlatProjection, compact, dark bool) {
	size, offset := float32(22), float32(48)
	if compact {
		size, offset = 12, 18
	}
	ink := mygoui.RGB(224, 224, 224).Alpha(.50 * .65)
	if !dark {
		ink = mygoui.RGB(77, 77, 77).Alpha(.65 * .65)
	}
	for _, cluster := range v.clusters {
		center := projection.point(cluster.center)
		caption := mygoui.Span{Text: cluster.label, Font: "Helvetica Neue", Size: size, Weight: 700, Color: ink}
		width, height := p.MeasureText(0, caption)
		x, y := center.X-width/2, center.Y-offset-height/2
		if x+width >= 0 && x <= r.W && y+height >= 0 && y <= r.H {
			p.RichText(r.X+x, r.Y+y, 0, caption)
		}
	}
}

func drawFlatNoteLabel(p *mygoui.Painter, r mygoui.Rect, note graphProjectedNote, name string, compact, dark bool) {
	label := layoutFlatNoteLabel(p, r, note, name, compact, dark)
	label.drawBackground(p)
	label.drawText(p)
}

func layoutFlatNoteLabel(p *mygoui.Painter, r mygoui.Rect, note graphProjectedNote, name string, compact, dark bool) graphNoteLabel {
	point := note.point
	if note.radius == 0 || point.X < 0 || point.X > r.W || point.Y < 0 || point.Y > r.H {
		return graphNoteLabel{}
	}
	// In 2D, the native note label's perspective-size factor still uses the
	// fixed camera distance and the point's zero world depth (EV-UI-193).
	scale, size, offset := float32(.5), float32(15), float32(14)
	weight, alpha := 500, float32(.60)
	padX, padY := float32(16), float32(8)
	if compact {
		scale, size, offset = 1.3/2.3, 13, 24
		weight, alpha = 700, 1
		padX, padY = 20, 12
	}
	ink, background := mygoui.RGB(250, 250, 250), mygoui.RGB(8, 8, 8).Alpha(.76)
	if !dark {
		ink, background = mygoui.RGB(38, 38, 38), mygoui.RGB(255, 255, 255).Alpha(.82)
	}
	caption := mygoui.Span{Text: name, Font: "Helvetica Neue", Size: size * scale, Weight: weight, Color: ink.Alpha(alpha)}
	width, height := p.MeasureText(0, caption)
	centerX, centerY := r.X+point.X, r.Y+point.Y+offset*scale
	x, y := centerX-width/2, centerY-height/2
	corner := (height + padY*scale) / 2
	if compact && !strings.ContainsRune(name, '\n') {
		// EV-UI-221. The label and its backdrop share the projected center.
		// RichText places the baseline with the theme font, even for a smaller span.
		theme := p.Theme()
		base := (mygoui.Font{Family: theme.Font, Size: theme.FontSize}).Metrics()
		font := (mygoui.Font{Family: caption.Font, Size: caption.Size, Weight: weight}).Metrics()
		const helveticaNeueCapHeight = 0.714 // fraction of the em
		y = centerY + (caption.Size*helveticaNeueCapHeight-font.Descent)/2 - base.Ascent - base.LineGap/2
		height = font.Ascent + font.Descent
		padX += 2 // SpriteKit's label frame includes one point on each side of the glyph bounds.
		corner = 7 * scale
	}
	padX, padY = padX*scale, padY*scale
	box := mygoui.Rect{X: centerX - (width+padX)/2, Y: centerY - (height+padY)/2, W: width + padX, H: height + padY}
	return graphNoteLabel{
		title: caption, position: mygoui.Point{X: x, Y: y},
		box: box, surface: background.Alpha(alpha), corner: corner,
	}
}
