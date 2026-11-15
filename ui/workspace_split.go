package ui

import mygoui "github.com/egoist/mygo/ui"

const (
	splitDefaultRatio    = 0.5
	splitDividerHitWidth = 9
	splitMinSourceWidth  = 400
	splitMinPreviewWidth = 420
)

type workspaceSplitState struct {
	ratio  float32 // Zero is the initial equal division.
	handle mygoui.Handle
	drag   workspaceSplitDrag
}

type workspaceSplitDrag struct {
	x, y, ratio float32
	held, moved bool
}

func clampSplitSource(room, source float32) float32 {
	return max(splitMinSourceWidth, min(source, room-splitMinPreviewWidth))
}

// EV-UI-232: the movable nine-point handle and the one-point rule share a
// center. The preview stays mounted while its bounds change.
func (app *workspaceWindow) drawSplitDivider(c *mygoui.Context, document mygoui.Element, center, windowWidth float32) {
	split := &app.split
	room := windowWidth - splitDividerHitWidth
	if center <= 0 || room <= 0 {
		split.drag = workspaceSplitDrag{}
		return
	}
	mygoui.Box(c.Key("document split handle")).Absolute().Top(0).Bottom(0).
		Left(center - splitDividerHitWidth/2.0).Width(splitDividerHitWidth).
		Role(mygoui.RoleSplitter).Label("Resize source and preview").KeepFocus().Cursor(mygoui.CursorResizeEW).
		Bind(&split.handle).HandleInput(func(event mygoui.InputEvent) bool {
		bounds := split.handle.Bounds(c)
		return split.resize(event, room, bounds.X+event.X, bounds.Y+event.Y)
	})
	ink, top := c.Theme().Text, float32(0)
	switch app.appAppearance() {
	case appearanceBlackout:
		ink = mygoui.Hex("#e3e2e6")
		if !c.Preferences().HighContrast {
			top = 4
		}
	case appearancePLATO:
		ink = mygoui.Hex("#e8b36a")
	}
	opacity := float32(0.15)
	if split.drag.moved {
		opacity = 0.30
	}
	ink = ink.Alpha(opacity)
	document.DrawOver(func(p *mygoui.Painter, r mygoui.Rect) {
		x := roundPaneHalfPoint(r.X + center - 0.5)
		p.Fill(mygoui.Rect{X: x, Y: r.Y + top, W: 1, H: max(0, r.H-top)}, ink, 0)
	})
}

func (s *workspaceSplitState) resize(event mygoui.InputEvent, room, x, y float32) bool {
	switch event.Kind {
	case mygoui.InputPointerDown:
		if event.Button != 0 || s.drag.held {
			return false
		}
		ratio := s.ratio
		if ratio == 0 {
			ratio = splitDefaultRatio
		}
		s.drag = workspaceSplitDrag{x: x, y: y, ratio: ratio, held: true}
	case mygoui.InputPointerMove, mygoui.InputPointerUp:
		if !s.drag.held {
			return false
		}
		dx, dy := x-s.drag.x, y-s.drag.y
		if !event.Canceled && (s.drag.moved || dx*dx+dy*dy >= 1) {
			s.drag.moved = true
			s.ratio = clampSplitSource(room, room*s.drag.ratio+dx) / room
		}
		if event.Kind == mygoui.InputPointerUp {
			s.drag = workspaceSplitDrag{}
		}
	default:
		return false
	}
	return true
}
