// spec: EV-UI-097
package ui

import (
	"math"
	"slices"
	"testing"
)

func TestGraphShapeCoordinates(t *testing.T) {
	const count = 16
	for _, sample := range []struct {
		shape graphShape
		first graphPosition
	}{
		{graphAuto, graphPosition{-.72, 0, -.72}},
		{graphSphere, graphPosition{.34798527, .9375, 0}},
		{graphSaddle, graphPosition{-.72, 0, -.72}},
		{graphTesseract, graphPosition{-.03269644, -.12542119, -.03710643}},
		{graphTorusKnot, graphPosition{.88100513, .15298311, .36492427}},
		{graphHelix, graphPosition{.5, -.7, 0}},
		{graphCube, graphPosition{-.6, -.6, -.6}},
		{graphVortex, graphPosition{.17097094, -.5625, .11423913}},
		{graphRose, graphPosition{.15147197, .03012965, .03397668}},
	} {
		positions := graphShapeSlots(sample.shape, count)
		if len(positions) != count {
			t.Fatalf("%s: %d positions, want %d", sample.shape, len(positions), count)
		}
		got := positions[0]
		if math.Abs(got.x-sample.first.x) > 1e-6 || math.Abs(got.y-sample.first.y) > 1e-6 || math.Abs(got.z-sample.first.z) > 1e-6 {
			t.Errorf("%s first point = %+v, want %+v", sample.shape, got, sample.first)
		}
	}
	if !slices.Equal(graphShapeSlots(graphAuto, 150), graphShapeSlots(graphSaddle, 150)) ||
		!slices.Equal(graphShapeSlots(graphAuto, 151), graphShapeSlots(graphSphere, 151)) {
		t.Error("Auto must switch to Sphere at 151 notes")
	}
}
