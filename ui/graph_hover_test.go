// spec: Knowledge Graph hover-only display. Original behavior: EV-UI-205 / EV-UI-211 / EV-UI-228.
package ui

import (
	"fmt"
	"slices"
	"testing"

	"cyberwriter/internal/core/vaultindex"
	mygoui "github.com/egoist/mygo/ui"
)

func TestGraphHoverShowsOnlyDirectConnections(t *testing.T) {
	graph := &vaultindex.Graph{
		Notes: []vaultindex.GraphNote{
			{Path: "Active.md"}, {Path: "Active Neighbor.md"},
			{Path: "Folder/Hover.md", Tags: []string{"alpha", "topic"}},
			{Path: "Neighbor.md"}, {Path: "Other Neighbor.md"}, {Path: "Second Hop.md"}, {Path: "Search.md"},
		},
		Edges: []vaultindex.GraphEdge{{From: 0, To: 1}, {From: 3, To: 2}, {From: 2, To: 4}, {From: 3, To: 4}, {From: 4, To: 5}},
	}
	var host workspaceGraph
	canvas := &host.floating
	options := graphCanvasOptions{ActivePath: "Active.md", Lines: true, Highlight: map[string]bool{"Search.md": true}}
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		theme := *mygoui.DarkTheme()
		theme.Background = mygoui.RGB(0, 0, 0)
		c.SetTheme(&theme)
		canvas.view(c, graph, host.layoutFor(graph), options, nil)
	}, 520, 380)
	assertEdges := func(wanted ...int) {
		t.Helper()
		withLines := tester.Image()
		options.Lines = false
		tester.Frame()
		withoutLines := tester.Image()
		for i, edge := range graph.Edges {
			// Only the Lines toggle changes between frames. Probe the edge's midpoint.
			a, b := canvas.projected[edge.From].point, canvas.projected[edge.To].point
			x, y := int((a.X+b.X)/2+0.5), int((a.Y+b.Y)/2+0.5)
			visible := false
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					visible = visible || withLines.RGBAAt(x+dx, y+dy) != withoutLines.RGBAAt(x+dx, y+dy)
				}
			}
			if visible != slices.Contains(wanted, i) {
				t.Errorf("hover %d: edge %d→%d visible = %v", canvas.hover, edge.From, edge.To, visible)
			}
		}
		options.Lines = true
		tester.Frame()
	}
	searchBrightness := func() uint8 {
		point := canvas.projected[6].point
		ink := tester.Image().RGBAAt(int(point.X+0.5), int(point.Y+0.5))
		return max(ink.R, ink.G, ink.B)
	}
	initial := make([]float32, len(graph.Notes))
	for i, projected := range canvas.projected {
		initial[i] = projected.radius
	}
	idleSearch := searchBrightness()
	assertEdges()
	point := canvas.projected[2].point
	tester.Move(point.X+canvas.projected[2].radius+3, point.Y)
	if canvas.hover != 2 {
		t.Errorf("entering the note's hit margin selected note %d, want 2", canvas.hover)
	}
	tester.Move(point.X, point.Y)
	if canvas.hover != 2 || canvas.projected[2].radius <= initial[2] || canvas.projected[3].radius <= initial[3] {
		t.Error("hover should enlarge the target and its directly linked note")
	}
	if canvas.projected[0].radius >= initial[0] || canvas.projected[1].radius >= initial[1] {
		t.Error("hover should replace the active note's node emphasis")
	}
	if !canvas.projected[6].matched || searchBrightness() >= idleSearch || canvas.projected[6].radius >= initial[6] {
		t.Error("hover should dim the separate search result while retaining its match")
	}
	if title, tags := canvas.hoverLabel(); title != "Hover" || tags != "#alpha  #topic" {
		t.Errorf("hover label = (%q, %q), want the filename and both tags", title, tags)
	}
	assertEdges(1, 2)
	options.Lines = false
	tester.Frame()
	if canvas.hover != 2 || canvas.projected[3].radius <= initial[3] {
		t.Error("hiding lines should preserve connected node emphasis")
	}
	options.Lines = true
	point = canvas.projected[3].point
	tester.Move(point.X, point.Y)
	if title, tags := canvas.hoverLabel(); canvas.hover != 3 || title != "Neighbor" || tags != "" {
		t.Errorf("moving to the neighbor selected note %d with label (%q, %q)", canvas.hover, title, tags)
	}
	assertEdges(1, 3)
	tester.Move(-5, -5)
	if canvas.hover != -1 {
		t.Errorf("leaving kept hover on note %d", canvas.hover)
	}
	if searchBrightness() != idleSearch {
		t.Error("leaving should restore the search result's brightness")
	}
	assertEdges()
	for i, expected := range initial {
		if canvas.projected[i].radius != expected {
			t.Errorf("leaving should restore note %d's initial radius", i)
		}
	}
	options.Compact = true
	tester.SetSize(336, 200)
	point = canvas.projected[2].point
	tester.Move(point.X+8, point.Y)
	if canvas.hover != 2 {
		t.Errorf("small note's 8-point margin selected note %d, want 2", canvas.hover)
	}
	tester.Move(point.X+12, point.Y)
	if canvas.hover != -1 {
		t.Errorf("leaving the small note's margin kept hover on note %d", canvas.hover)
	}
}

func TestGraphHoverAndClickUseNearestProjectedCenter(t *testing.T) {
	graph := &vaultindex.Graph{Notes: make([]vaultindex.GraphNote, 18)}
	for i := range graph.Notes {
		graph.Notes[i].Path = fmt.Sprintf("Note %02d.md", i+1)
	}
	var host workspaceGraph
	canvas := &host.floating
	canvas.paused = true
	options := graphCanvasOptions{Globe: true, Shape: graphSaddle}
	var opened string
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		canvas.view(c, graph, host.layoutFor(graph), options, func(path string) { opened = path })
	}, 1100, 760)
	// In the paused saddle, Note 13 overlaps the exact center of Note 08.
	point := canvas.projected[7].point
	tester.Move(point.X, point.Y)
	if title, _ := canvas.hoverLabel(); title != "Note 08" {
		t.Errorf("hover title = %q, want Note 08 at its projected center", title)
	}
	tester.ClickAt(point.X, point.Y)
	if opened != "Note 08.md" {
		t.Errorf("click opened %q, want the hovered note", opened)
	}
}

func TestGraphHoverAndClickUseThePresentedNodeRadius(t *testing.T) {
	graph := &vaultindex.Graph{Notes: []vaultindex.GraphNote{{Path: "Large.md"}}}
	var host workspaceGraph
	canvas := &host.sidebar
	options := graphCanvasOptions{Compact: true, ActivePath: "Large.md"}
	var opened string
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		canvas.view(c, graph, host.layoutFor(graph), options, func(path string) { opened = path })
	}, 336, 200)
	center := canvas.projected[0].point
	tester.Scroll(center.X, center.Y, 0, -360)
	tester.Move(-5, -5)
	node := canvas.projected[0]
	normalReach := max(9, node.baseRadius+4)
	if node.radius <= normalReach {
		t.Fatalf("zoomed fixture does not expose the enlarged disk beyond the normal hit reach: radius %v, reach %v", node.radius, normalReach)
	}
	rim := node.point.X + (normalReach+node.radius)/2
	tester.Move(rim, node.point.Y)
	if canvas.hover != 0 {
		t.Error("entering the enlarged active node at its visible rim should hover it")
	}

	options.ActivePath = ""
	tester.Move(-5, -5)
	tester.Move(rim, node.point.Y)
	if canvas.hover != -1 {
		t.Error("the same pointer position should miss the non-emphasized node")
	}
	tester.Move(node.point.X, node.point.Y)
	tester.Move(rim, node.point.Y)
	if canvas.hover != 0 {
		t.Error("moving from the center to the visible rim should retain the hover")
	}
	tester.ClickAt(rim, node.point.Y)
	if opened != "Large.md" {
		t.Errorf("click at the hovered rim opened %q, want Large.md", opened)
	}
	tester.Move(node.point.X+node.radius+5, node.point.Y)
	if canvas.hover != -1 {
		t.Error("moving beyond the enlarged hit radius should clear the hover")
	}
}
