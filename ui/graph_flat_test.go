// spec: EV-UI-192 / EV-UI-193 / EV-UI-198 / EV-UI-202
package ui

import (
	"math"
	"testing"
	"time"

	"cyberwriter/internal/core/vaultindex"
)

func TestGraphFlatCoordinates(t *testing.T) {
	graph := &vaultindex.Graph{
		Notes: []vaultindex.GraphNote{
			{Path: "Notes/Aardvark.md", Tags: []string{"alpha"}},
			{Path: "Notes/Untagged.md"},
			{Path: "Notes/alpha Detail.md", Tags: []string{"alpha", "delta"}},
		},
		Edges: []vaultindex.GraphEdge{{From: 0, To: 1}},
		Tags:  []vaultindex.GraphTag{{Name: "alpha", Count: 2}, {Name: "delta", Count: 1}},
	}
	for _, layout := range []struct {
		name      string
		clustered bool
		want      []graphPosition
	}{
		{"even", false, []graphPosition{{x: .40824829}, {x: .07980840, y: .90937558}, {x: -.52139853, y: -.47764377}}},
		{"clustered", true, []graphPosition{{x: .53953656}, {x: -.38314811, y: -.35099505}, {x: -.00592660, y: -.28025382}}},
	} {
		t.Run(layout.name, func(t *testing.T) {
			points, clusters := graphFlatLayout(graph.Notes, graphScanOrder(graph.Notes), layout.clustered)
			for i, want := range layout.want {
				got := points[i]
				if math.Hypot(got.x-want.x, got.y-want.y) > 1e-7 || got.z != want.z {
					t.Errorf("%s = %+v, want %+v", graph.Notes[i].Path, got, want)
				}
			}
			if layout.clustered && (len(clusters) != 1 || clusters[0].label != "#alpha") {
				t.Errorf("cluster labels = %+v, want only #alpha", clusters)
			}
		})
	}
}

func TestGraphCanvasFollowsConnectedOrder(t *testing.T) {
	graph := &vaultindex.Graph{
		Notes: []vaultindex.GraphNote{
			{Path: "A.md"}, {Path: "B.md"}, {Path: "C.md"},
			{Path: "D.md"}, {Path: "E.md"}, {Path: "F.md"},
		},
		Edges: []vaultindex.GraphEdge{
			{From: 4, To: 0}, {From: 0, To: 2}, {From: 4, To: 5}, {From: 2, To: 3},
		},
	}
	wantOrder := []int{0, 2, 4, 3, 5, 1}
	options := graphCanvasOptions{Shape: graphSphere}
	var host workspaceGraph
	canvas := &host.sidebar
	canvas.rebuild(graph, host.layoutFor(graph), options)
	for slot, id := range wantOrder {
		if want := graphFlatSpiralPoint(slot, len(wantOrder), 1); canvas.flat[id] != want {
			t.Errorf("2D slot %d should be %s", slot, graph.Notes[id].Path)
		}
		if want := graphSpherePoint(slot, len(wantOrder)); canvas.spatial[id] != want {
			t.Errorf("3D slot %d should be %s", slot, graph.Notes[id].Path)
		}
	}
	start := time.Now()
	for _, globe := range []bool{false, true} {
		canvas.tour = graphTour{started: start, step: -1}
		options.Globe = globe
		for slot, id := range wantOrder {
			_, got := canvas.tourFrame(start.Add(time.Duration(slot)*graphTourInterval), graphPose{}, options)
			if got != id {
				t.Errorf("Globe=%t tour slot %d = %s, want %s", globe, slot, graph.Notes[got].Path, graph.Notes[id].Path)
			}
		}
	}
}
