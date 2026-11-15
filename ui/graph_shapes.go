package ui

import (
	"cmp"
	"math"
	"math/bits"
	"slices"

	"cyberwriter/internal/core/vaultindex"
)

type graphShape string

const (
	graphAuto      graphShape = "auto"
	graphSphere    graphShape = "sphere"
	graphSaddle    graphShape = "saddle"
	graphTesseract graphShape = "tesseract"
	graphTorusKnot graphShape = "torusKnot"
	graphHelix     graphShape = "helix"
	graphCube      graphShape = "cube"
	graphVortex    graphShape = "vortex"
	graphRose      graphShape = "rose"
)

type graphShapeChoice struct {
	id           graphShape
	name, symbol string
	windowWidth  float32
}

var graphShapeChoices = [...]graphShapeChoice{
	{graphAuto, "Auto", "sparkles", 30.5},
	{graphSphere, "Sphere", "globe", 33},
	{graphSaddle, "Saddle", "water.waves", 35},
	{graphTesseract, "Tesseract", "cube.transparent", 33.5},
	{graphTorusKnot, "Torus Knot", "circle.circle", 33},
	{graphHelix, "Double Helix", "alternatingcurrent", 35.5},
	{graphCube, "Cube Lattice", "cube", 33.5},
	{graphVortex, "Vortex", "hurricane", 23.5},
	{graphRose, "Rose", "camera.macro", 36},
}

func (shape graphShape) choice() graphShapeChoice {
	for _, choice := range graphShapeChoices {
		if choice.id == shape {
			return choice
		}
	}
	return graphShapeChoices[0]
}

func (shape graphShape) effective(count int) graphShape {
	id := shape.choice().id
	if id != graphAuto {
		return id
	}
	if count < 151 {
		return graphSaddle
	}
	return graphSphere
}

// EV-UI-097: coordinates are in units of the scene's globeRadius.
func graphShapeSlots(shape graphShape, count int) []graphPosition {
	points := make([]graphPosition, count)
	switch shape.effective(count) {
	case graphSphere:
		for i := range points {
			points[i] = graphSpherePoint(i, count)
		}
	case graphSaddle:
		columns := max(1, int(math.Ceil(math.Sqrt(float64(count)))))
		rows := (count + columns - 1) / columns
		for i := range points {
			x, z := graphGridAxis(i%columns, columns), graphGridAxis(i/columns, rows)
			points[i] = graphPosition{0.72 * x, 0.72 * 0.7 * (x*x - z*z), 0.72 * z}
		}
	case graphTesseract:
		graphTesseractSlots(points)
	case graphTorusKnot:
		for i := range points {
			t := (float64(i) + 0.5) / float64(count)
			sinA, cosA := math.Sincos(4 * math.Pi * t)
			sinB, cosB := math.Sincos(6 * math.Pi * t)
			ring := (1 + 0.38*cosB) / 1.38
			points[i] = graphPosition{ring * cosA, 0.38 * sinB / 1.38, ring * sinA}
		}
	case graphHelix:
		pairs := max(1, count/2)
		for i := range points {
			t := float64(i/2) / float64(pairs)
			angle := 5*math.Pi*t + math.Pi*float64(i%2)
			sin, cos := math.Sincos(angle)
			points[i] = graphPosition{0.5 * cos, 1.4 * (t - 0.5), 0.5 * sin}
		}
	case graphCube:
		side := max(1, int(math.Ceil(math.Cbrt(float64(count)))))
		for i := range points {
			points[i] = graphPosition{
				0.6 * graphGridAxis(i%side, side),
				0.6 * graphGridAxis((i/side)%side, side),
				0.6 * graphGridAxis(i/(side*side), side),
			}
		}
	case graphVortex:
		for i := range points {
			t := (float64(i) + 0.5) / float64(count)
			sin, cos := math.Sincos(6 * math.Pi * t)
			radius := 0.18 + 0.82*t
			points[i] = graphPosition{radius * cos, 1.2 * (t - 0.5), radius * sin}
		}
	case graphRose:
		turns := min(8, max(1, float64(count)/60))
		for i := range points {
			t := (float64(i) + 0.5) / float64(count)
			angle := 2 * math.Pi * turns * t
			sin, cos := math.Sincos(angle)
			radial := (0.35 + 0.65*t) * math.Sin(2*angle+1.2*t)
			radius := math.Abs(radial)
			points[i] = graphPosition{radius * cos, radius * sin, 0.22 * radial}
		}
	}
	return points
}

func graphGridAxis(index, count int) float64 {
	if count < 2 {
		return 0
	}
	return 2*float64(index)/float64(count-1) - 1
}

func graphTesseractSlots(points []graphPosition) {
	var corners [16][4]float64
	var edges [32][2]int
	nextEdge := 0
	for i := range corners {
		for axis := range corners[i] {
			corners[i][axis] = 2*float64((i>>axis)&1) - 1
		}
		for j := i + 1; j < len(corners); j++ {
			if bits.OnesCount(uint(i^j)) == 1 {
				edges[nextEdge] = [2]int{i, j}
				nextEdge++
			}
		}
	}
	layers := (max(0, len(points)-len(corners)) + len(edges) - 1) / len(edges)
	sinX, cosX := math.Sincos(0.6)
	sinZ, cosZ := math.Sincos(0.45)
	for i := range points {
		var point [4]float64
		if i < len(corners) {
			point = corners[i]
		} else {
			extra := i - len(corners)
			edge := edges[extra%len(edges)]
			t := float64(extra/len(edges)+1) / float64(layers+1)
			a, b := corners[edge[0]], corners[edge[1]]
			for axis := range point {
				point[axis] = a[axis] + (b[axis]-a[axis])*t
			}
		}
		x, y, z, w := point[0], point[1], point[2], point[3]
		x, w = x*cosX-w*sinX, x*sinX+w*cosX
		z, w = z*cosZ-w*sinZ, z*sinZ+w*cosZ
		perspective := 0.5 / (2.3 - w)
		points[i] = graphPosition{x * perspective, y * perspective, z * perspective}
	}
}

const graphUntagged = "\x01untagged"

type graphNoteGroup struct {
	tag  string
	rank int
	ids  []int
}

func graphNoteGroups(notes []vaultindex.GraphNote, order []int) []graphNoteGroup {
	byName := make(map[string]int)
	var groups []graphNoteGroup
	for _, i := range order {
		note := notes[i]
		name := graphUntagged
		if len(note.Tags) != 0 {
			name = slices.Min(note.Tags)
		}
		slot, ok := byName[name]
		if !ok {
			slot = len(groups)
			byName[name] = slot
			groups = append(groups, graphNoteGroup{tag: name, rank: graphTagSlot(name)})
		}
		groups[slot].ids = append(groups[slot].ids, i)
	}
	slices.SortStableFunc(groups, func(a, b graphNoteGroup) int { return cmp.Compare(a.rank, b.rank) })
	return groups
}

func graphSpatialLayout(notes []vaultindex.GraphNote, order []int, shape graphShape, clustered bool) []graphPosition {
	count := len(notes)
	points := make([]graphPosition, count)
	if !clustered || count == 0 {
		for slot, point := range graphShapeSlots(shape, count) {
			points[order[slot]] = point
		}
		return points
	}
	groups := graphNoteGroups(notes, order)
	if shape.effective(count) == graphSphere {
		for i, group := range groups {
			center := graphSpherePoint(i, len(groups))
			rotation := graphAlignZ(center)
			size := float64(len(group.ids))
			cap := 0.85 * math.Acos(max(-1, 1-2*size/float64(count)))
			for index, id := range group.ids {
				z := 1.0
				if len(group.ids) > 1 {
					z -= (1 - math.Cos(cap)) * (float64(index) + 0.5) / size
				}
				ring := math.Sqrt(max(0, 1-z*z))
				sin, cos := math.Sincos(float64(index) * graphGoldenAngle)
				points[id] = rotation.apply(graphPosition{ring * cos, ring * sin, z})
			}
		}
		return points
	}
	slots := graphShapeSlots(shape, count)
	offset := 0
	for _, group := range groups {
		for _, id := range group.ids {
			points[id] = slots[offset]
			offset++
		}
	}
	return points
}
