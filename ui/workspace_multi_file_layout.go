package ui

import (
	"fmt"
	"math"
	"slices"

	mygoui "github.com/egoist/mygo/ui"
)

type multiFileArrangement uint8

const (
	multiFileGrid multiFileArrangement = iota
	multiFileColumns
	multiFileRows
)

type workspaceMultiLayout struct {
	arrangement multiFileArrangement
	sizes       [3]multiFileLayoutSizes
	members     []*workspaceTab
	canvas      mygoui.Handle
	drag        *multiFileLayoutDrag
}

type multiFileLayoutSizes struct {
	columns, rows map[*workspaceTab]float32
}

// A track uses the first tab in that column or row as its identity.
type multiFileLayoutTracks struct {
	tabs    []*workspaceTab
	weights map[*workspaceTab]float32
	edges   []float32
	total   float32
}

type multiFileLayoutDivider struct {
	arrangement   multiFileArrangement
	before, after *workspaceTab
	resizesRows   bool
}

type multiFileLayoutDrag struct {
	divider                   multiFileLayoutDivider
	weights                   map[*workspaceTab]float32
	press, extent, total      float32
	originalBefore, pairTotal float32
	trackCount                int
}

func (layout *workspaceMultiLayout) draw(c *mygoui.Context, tabs []*workspaceTab, tile func(*mygoui.Context, *workspaceTab)) {
	mode := layout.arrangement
	if mode > multiFileRows {
		mode = multiFileGrid
	}
	if !slices.Equal(layout.members, tabs) {
		layout.updateMembers(tabs)
	}
	if layout.drag != nil && layout.drag.divider.arrangement != mode {
		layout.cancelDrag()
	}
	if len(tabs) == 0 {
		return
	}

	columns := len(tabs)
	switch mode {
	case multiFileGrid:
		columns = int(math.Ceil(math.Sqrt(float64(len(tabs)))))
	case multiFileRows:
		columns = 1
	}
	rowTabs := make([]*workspaceTab, 0, (len(tabs)+columns-1)/columns)
	for i := 0; i < len(tabs); i += columns {
		rowTabs = append(rowTabs, tabs[i])
	}
	sizes := &layout.sizes[mode]
	if sizes.columns == nil {
		sizes.columns, sizes.rows = make(map[*workspaceTab]float32), make(map[*workspaceTab]float32)
	}
	x, y := multiFileTracks(tabs[:columns], sizes.columns), multiFileTracks(rowTabs, sizes.rows)

	mygoui.Box(c.Key("multi-file layout")).Grow(1).AlignSelf(mygoui.Stretch).MinWidth(0).MinHeight(0).
		Clip().Bind(&layout.canvas).Children(func() {
		for i, tab := range tabs {
			col, row := i%columns, i/columns
			end := col + 1
			if i == len(tabs)-1 {
				end = columns
			}
			// The tile remains under the same parent as its track changes.
			mygoui.Box(c.Key(tab)).Absolute().LeftPercent(100 * x.edges[col]).TopPercent(100 * y.edges[row]).
				WidthPercent(100 * (x.edges[end] - x.edges[col])).HeightPercent(100 * (y.edges[row+1] - y.edges[row])).
				Clip().Children(func() { tile(c, tab) })
		}

		for i := 0; i+1 < len(x.tabs); i++ {
			height := float32(1)
			if remainder := len(tabs) % columns; remainder != 0 && remainder <= i+1 {
				height = y.edges[len(y.tabs)-1]
			}
			layout.drawDivider(c, mode, x, i, false, height)
		}
		for i := 0; i+1 < len(y.tabs); i++ {
			layout.drawDivider(c, mode, y, i, true, 1)
		}
	})
}

func (layout *workspaceMultiLayout) updateMembers(tabs []*workspaceTab) {
	layout.cancelDrag()
	layout.members = slices.Clone(tabs)
	live := make(map[*workspaceTab]bool, len(tabs))
	for _, tab := range tabs {
		live[tab] = true
	}
	for _, sizes := range layout.sizes {
		for _, weights := range [...]map[*workspaceTab]float32{sizes.columns, sizes.rows} {
			for tab := range weights {
				if !live[tab] {
					delete(weights, tab)
				}
			}
		}
	}
}

func multiFileTracks(tabs []*workspaceTab, weights map[*workspaceTab]float32) multiFileLayoutTracks {
	edges := make([]float32, len(tabs)+1)
	for i, tab := range tabs {
		edges[i+1] = edges[i] + multiFileWeight(weights, tab)
	}
	total := edges[len(tabs)]
	for i := 1; i < len(edges)-1; i++ {
		edges[i] /= total
	}
	edges[len(tabs)] = 1
	return multiFileLayoutTracks{tabs: tabs, weights: weights, edges: edges, total: total}
}

func multiFileWeight(weights map[*workspaceTab]float32, tab *workspaceTab) float32 {
	if weight := weights[tab]; weight > 0 {
		return weight
	}
	return 1
}

func (layout *workspaceMultiLayout) drawDivider(c *mygoui.Context, mode multiFileArrangement, tracks multiFileLayoutTracks,
	index int, rows bool, length float32) {
	key := multiFileLayoutDivider{mode, tracks.tabs[index], tracks.tabs[index+1], rows}
	before := tracks.edges[index]
	center := tracks.edges[index+1]
	after := tracks.edges[index+2]
	maxHit := 100 * min(center-before, after-center)
	axis := "columns"
	if rows {
		axis = "rows"
	}
	divider := mygoui.Box(c.Key(key)).Absolute().Role(mygoui.RoleSplitter).
		Label(fmt.Sprintf("Resize %s %d and %d", axis, index+1, index+2)).
		Focusable().KeepFocus().Range(float64(100*before), float64(100*after), float64(100*center))
	handle := mygoui.Local(divider, "splitter handle", func() mygoui.Handle { return mygoui.Handle{} })
	divider.Bind(handle)
	if rows {
		divider.TopPercent(100*center).Left(0).Right(0).Height(splitDividerHitWidth).MaxHeightPercent(maxHit).
			TranslatePercent(0, -50).Cursor(mygoui.CursorResizeNS).Orientation(mygoui.Horizontal)
	} else {
		divider.LeftPercent(100*center).Top(0).HeightPercent(100*length).Width(splitDividerHitWidth).
			MaxWidthPercent(maxHit).TranslatePercent(-50, 0).Cursor(mygoui.CursorResizeEW).Orientation(mygoui.Vertical)
	}

	newDrag := func(press float32) *multiFileLayoutDrag {
		room := layout.canvas.Bounds(c)
		extent := room.W
		if rows {
			extent = room.H
		}
		first := multiFileWeight(tracks.weights, key.before)
		return &multiFileLayoutDrag{
			divider: key, weights: tracks.weights, press: press, extent: extent, total: tracks.total,
			originalBefore: first, pairTotal: first + multiFileWeight(tracks.weights, key.after), trackCount: len(tracks.tabs),
		}
	}
	shrink := func() { newDrag(0).move(-10) }
	grow := func() { newDrag(0).move(10) }
	// MyGo dispatches accessibility increment/decrement as Right/Left.
	divider.OnShortcut(0, mygoui.KeyLeft, shrink).OnShortcut(0, mygoui.KeyRight, grow)
	if rows {
		divider.OnShortcut(0, mygoui.KeyUp, shrink).OnShortcut(0, mygoui.KeyDown, grow)
	}
	divider.
		HandleInput(func(event mygoui.InputEvent) bool {
			bounds := handle.Bounds(c)
			position := bounds.X + event.X
			if rows {
				position = bounds.Y + event.Y
			}
			switch event.Kind {
			case mygoui.InputPointerDown:
				if event.Button != 0 || layout.drag != nil {
					return false
				}
				layout.drag = newDrag(position)
			case mygoui.InputPointerMove, mygoui.InputPointerUp:
				drag := layout.drag
				if drag == nil || drag.divider != key {
					return false
				}
				if event.Canceled {
					layout.cancelDrag()
					return true
				}
				drag.move(position - drag.press)
				if event.Kind == mygoui.InputPointerUp {
					layout.drag = nil
				}
			default:
				return false
			}
			return true
		})

	ink := c.Theme().Text.Alpha(.15)
	if layout.drag != nil && layout.drag.divider == key {
		ink = c.Theme().Text.Alpha(.30)
	}
	if divider.FocusVisible() {
		ink = c.Theme().Accent
	}
	divider.Draw(func(p *mygoui.Painter, r mygoui.Rect) {
		if rows {
			r.Y, r.H = r.Y+max(0, (r.H-1)/2), min(1, r.H)
		} else {
			r.X, r.W = r.X+max(0, (r.W-1)/2), min(1, r.W)
		}
		p.Fill(r, ink, 0)
	})
}

func (drag *multiFileLayoutDrag) move(delta float32) {
	if drag.extent <= 0 {
		return
	}
	if delta == 0 {
		drag.set(drag.originalBefore)
		return
	}
	// On small canvases the minimum falls with the equal track size.
	// Resizing the window preserves the resulting proportions.
	const minTrackPoints = 96
	weightPerPoint := drag.total / drag.extent
	minimum := min(minTrackPoints*weightPerPoint, drag.total/float32(2*drag.trackCount), drag.pairTotal/2)
	weight := min(max(drag.originalBefore+delta*weightPerPoint, minimum), drag.pairTotal-minimum)
	drag.set(weight)
}

func (drag *multiFileLayoutDrag) set(before float32) {
	drag.weights[drag.divider.before], drag.weights[drag.divider.after] = before, drag.pairTotal-before
}

func (layout *workspaceMultiLayout) cancelDrag() {
	if drag := layout.drag; drag != nil {
		drag.set(drag.originalBefore)
		layout.drag = nil
	}
}
