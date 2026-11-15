package ui

import (
	"math"
	"slices"

	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

const (
	blockGutterButton = float32(32)
	blockGutterGap    = float32(2)
	blockGutterInset  = float32(16)
)

type visualBlockGutter struct {
	hovered *visualGutterBlock
	drag    *visualBlockDrag
}

type visualGutterBlock struct {
	box     richtext.ParagraphBox
	after   int // Start of the last paragraph in the block.
	movable bool
	grouped bool
}

type visualBlockDrag struct {
	block    visualGutterBlock
	document *richtext.Document
	version  uint64
	press    mygoui.Point
	point    mygoui.Point
	moved    bool
}

// drawBlockGutter overlays the measured 66-by-32-point control pair in the
// viewport. Keep the target while the pointer crosses the margin to it.
func (w *workspaceWYSIWYG) drawBlockGutter(c *mygoui.Context, app *workspaceWindow, padding float32,
	viewport mygoui.Rect, text richtext.Text, boxes []richtext.ParagraphBox, pointer mygoui.Point, over bool) {
	gutter := &w.blockGutter
	document := w.session.Document
	if padding < 4 {
		w.cancelBlockGutterDrag()
		gutter.hovered = nil
		return
	}
	if gutter.drag != nil && (gutter.drag.document != document || gutter.drag.version != document.Version()) {
		w.cancelBlockGutterDrag()
	}
	if viewport.W <= 2*padding || viewport.H <= 0 {
		w.cancelBlockGutterDrag()
		gutter.hovered = nil
		return
	}
	if drag := gutter.drag; drag != nil {
		// Keep the captured handle in the view until release, even if the
		// source scrolls out of the set of visible paragraphs.
		for _, box := range boxes {
			if box.Start == drag.block.box.Start {
				drag.block.box = box
				break
			}
		}
		if drag.moved {
			w.drawBlockDrop(c, padding, viewport, text, boxes, drag, app.blockMenuColors().ink)
		}
		w.drawBlockGutterButtons(c, app, padding, viewport, text, boxes, drag.block)
		return
	}

	if !over {
		gutter.hovered = nil
		return
	}
	if target := gutter.hovered; target != nil && pointer.X < padding {
		// Retain the target across the space between the text and buttons.
		// Recalculate from the current layout when scrolled or resized.
		gutter.hovered = nil
		for i, box := range boxes {
			if box.Start != target.box.Start {
				continue
			}
			block, ok := visualGutterBlockAt(text, boxes, i)
			if ok {
				frame := w.blockGutterFrame(text, block, padding)
				frame.W = padding - frame.X
				if frame.Contains(pointer.X, pointer.Y) {
					gutter.hovered = &block
				}
			}
			break
		}
	} else {
		previous := gutter.hovered
		gutter.hovered = nil
		for i, box := range boxes {
			if pointer.Y < box.Top || pointer.Y >= box.Top+box.Height {
				continue
			}
			if block, ok := visualGutterBlockAt(text, boxes, i); ok && pointer.X >= w.blockGutterFrame(text, block, padding).X {
				gutter.hovered = &block
			}
			break
		}
		if gutter.hovered == nil && previous != nil && slices.Contains(boxes, previous.box) {
			// Empty space between adjacent lines does not move the handle
			// while crossing from the block into its gutter.
			frame := w.blockGutterFrame(text, *previous, padding)
			frame.W = viewport.W - frame.X
			if frame.Contains(pointer.X, pointer.Y) {
				gutter.hovered = previous
			}
		}
	}
	if target := gutter.hovered; target != nil {
		w.drawBlockGutterButtons(c, app, padding, viewport, text, boxes, *target)
	}
}

func visualGutterBlockAt(text richtext.Text, boxes []richtext.ParagraphBox, index int) (visualGutterBlock, bool) {
	first, last := index, index
	style := func(i int) richtext.ParagraphStyle { return text.ParagraphStyleAt(boxes[i].Start) }
	for first > 0 && wysiwyg.SameBlockGroup(style(first-1), style(first)) {
		first--
	}
	for last+1 < len(boxes) && wysiwyg.SameBlockGroup(style(last), style(last+1)) {
		last++
	}
	start := boxes[first].Start
	_, end := text.ParagraphRange(boxes[last].Start)
	// Resolve only complete groups. A partially visible nested block may
	// continue beyond the available layout or accept a different PM drop.
	if start > 0 && wysiwyg.SameBlockGroup(text.ParagraphStyleAt(start-1), style(first)) ||
		end < text.Len() && wysiwyg.SameBlockGroup(style(last), text.ParagraphStyleAt(end+1)) {
		return visualGutterBlock{}, false
	}
	block := visualGutterBlock{box: boxes[first], after: boxes[last].Start, grouped: first != last}
	paragraph := style(first)
	block.movable = canMoveGutterBlock(text, start, end, paragraph)
	return block, true
}

func canMoveGutterBlock(text richtext.Text, start, end int, paragraph richtext.ParagraphStyle) bool {
	movable := start < end && paragraph.List == richtext.NoList &&
		paragraph.Indent == 0 && !paragraph.Quote && !paragraph.CodeBlock
	if movable {
		for _, run := range text.Runs(start, end) {
			if run.Attributes.Attachment != nil {
				return false
			}
		}
	}
	return movable
}

func (w *workspaceWYSIWYG) blockGutterFrame(text richtext.Text, block visualGutterBlock, padding float32) mygoui.Rect {
	box := block.box
	style := text.ParagraphStyleAt(box.Start)
	// Keep the buttons out of the text when the replica reduces its inset.
	inset := blockGutterInset
	if padding < 2*blockGutterButton+blockGutterGap+blockGutterInset {
		inset = min(blockGutterInset, padding/6)
	}
	button := min(blockGutterButton, (padding-inset-blockGutterGap)/2)
	width := 2*button + blockGutterGap
	top := box.Top
	if _, ok := w.session.TableAt(box.Start); ok {
		top += wysTableMargin
	} else if _, ok := w.session.DiagramAt(box.Start); ok {
		top += diagramMargin
	} else if formula, ok := wysiwyg.FormulaAt(text, box.Start); ok && formula.Block {
		top += visualMathMargin
	} else if !block.grouped && text.AttributesAt(box.Start).Attachment == nil &&
		(style.Heading > 0 || box.Height <= blockGutterButton) {
		top += (box.Height - button) / 2
	}
	return mygoui.Rect{X: padding - inset - width, Y: top, W: width, H: button}
}

func (w *workspaceWYSIWYG) drawBlockGutterButtons(c *mygoui.Context, app *workspaceWindow, padding float32,
	viewport mygoui.Rect, text richtext.Text, boxes []richtext.ParagraphBox, block visualGutterBlock) {
	frame := w.blockGutterFrame(text, block, padding)
	colors := app.blockMenuColors()
	ink := colors.ink.Alpha(.48)
	add := mygoui.ButtonBase(c.Key("add block")).Absolute().Left(frame.X).Top(frame.Y).
		Size(frame.H, frame.H).Radius(4).Label("Add block").HasPopup(mygoui.PopupMenu).
		KeepFocus().Cursor(mygoui.CursorPointer).OnClick(func() { w.addGutterBlock(app, block.after) }).
		DrawOver(func(p *mygoui.Painter, r mygoui.Rect) { drawBlockGutterPlus(p, r, ink) })
	if add.Pressed() {
		add.Background(colors.accent.Alpha(.2))
	} else if add.Hovered() {
		add.Background(colors.hover)
	}
	frame.X += frame.H + blockGutterGap
	grip := mygoui.ButtonBase(c.Key("move block")).Absolute().Left(frame.X).Top(frame.Y).
		Size(frame.H, frame.H).Radius(4).Label("Move block").KeepFocus().
		Disabled(!block.movable).Cursor(mygoui.CursorGrab).
		DrawOver(func(p *mygoui.Painter, r mygoui.Rect) { drawBlockGutterGrip(p, r, ink) })
	if block.movable {
		// Pointer input selects on press. A keyboard or AX press uses
		// the button's ordinary activation path.
		grip.OnClick(func() { w.selectGutterBlock(app, block) }).
			HandleInput(func(event mygoui.InputEvent) bool {
				return w.blockGutterInput(app, frame, viewport, padding, boxes, block, event)
			})
	}
	if w.blockGutter.drag != nil {
		grip.Cursor(mygoui.CursorGrabbing)
	}
	if grip.Hovered() && block.movable {
		grip.Background(colors.hover)
	}
}

func drawBlockGutterPlus(p *mygoui.Painter, r mygoui.Rect, ink mygoui.Color) {
	x, y := r.X+r.W/2, r.Y+r.H/2
	scale := r.W / blockGutterButton
	// Join the two strokes before applying the theme's translucent ink.
	// Paint their overlap once so the center matches the stroke ends.
	p.Fill(mygoui.Rect{X: x - 7*scale, Y: y - scale, W: 14 * scale, H: 2 * scale}, ink, scale)
	p.Fill(mygoui.Rect{X: x - scale, Y: y - 7*scale, W: 2 * scale, H: 6 * scale}, ink, scale)
	p.Fill(mygoui.Rect{X: x - scale, Y: y + scale, W: 2 * scale, H: 6 * scale}, ink, scale)
}

func drawBlockGutterGrip(p *mygoui.Painter, r mygoui.Rect, ink mygoui.Color) {
	x, y := r.X+r.W/2, r.Y+r.H/2
	scale := r.W / blockGutterButton
	for _, dx := range [...]float32{-3, 3} {
		for _, dy := range [...]float32{-6, 0, 6} {
			p.Fill(mygoui.Rect{X: x + (dx-2)*scale, Y: y + (dy-2)*scale, W: 4 * scale, H: 4 * scale}, ink, 2*scale)
		}
	}
}

func (w *workspaceWYSIWYG) addGutterBlock(app *workspaceWindow, after int) {
	if !w.editable(app) {
		return
	}
	w.flush(app)
	if w.session.InsertBlockParagraph(after) {
		w.blockGutter.hovered = nil
		w.closeTableControls()
		w.openGutterBlockMenu()
		w.sync(app)
	}
}

func (w *workspaceWYSIWYG) selectGutterBlock(app *workspaceWindow, block visualGutterBlock) bool {
	if !w.editable(app) || !block.movable {
		return false
	}
	w.flush(app)
	document := w.session.Document
	text := document.Text()
	start, end := text.ParagraphRange(block.box.Start)
	if start != block.box.Start || !canMoveGutterBlock(text, start, end, text.ParagraphStyleAt(start)) {
		return false
	}
	w.closeTableControls()
	document.SetSelection(start, end)
	w.focus = true
	return true
}

func (w *workspaceWYSIWYG) cancelBlockGutterDrag() bool {
	if w.blockGutter.drag == nil {
		return false
	}
	w.blockGutter.drag = nil
	w.blockGutter.hovered = nil
	return true
}

func (w *workspaceWYSIWYG) blockGutterInput(app *workspaceWindow, frame, viewport mygoui.Rect, padding float32,
	boxes []richtext.ParagraphBox, block visualGutterBlock, event mygoui.InputEvent) bool {
	point := mygoui.Point{X: frame.X + event.X, Y: frame.Y + event.Y}
	gutter := &w.blockGutter
	switch event.Kind {
	case mygoui.InputPointerDown:
		if event.Button != 0 || gutter.drag != nil || !w.selectGutterBlock(app, block) {
			return false
		}
		document := w.session.Document
		gutter.drag = &visualBlockDrag{block: block, document: document, version: document.Version(), press: point, point: point}
		return true
	case mygoui.InputPointerMove, mygoui.InputPointerUp:
		drag := gutter.drag
		if drag == nil || event.Button != 0 {
			return false
		}
		dx, dy := point.X-drag.press.X, point.Y-drag.press.Y
		drag.moved = drag.moved || dx*dx+dy*dy >= 16
		drag.point = point
		if event.Kind == mygoui.InputPointerMove {
			return true
		}
		gutter.drag = nil
		if event.Canceled || !drag.moved || !w.editable(app) || drag.document != w.session.Document || drag.version != drag.document.Version() {
			return true
		}
		index, _, ok := blockDropBoundary(drag.document.Text(), boxes, padding, viewport, point)
		if ok && w.session.MoveBlock(drag.block.box.Index, index) {
			gutter.hovered = nil
			w.sync(app)
		}
		return true
	}
	return false
}

func blockDropBoundary(text richtext.Text, boxes []richtext.ParagraphBox, padding float32, viewport mygoui.Rect, point mygoui.Point) (int, float32, bool) {
	if len(boxes) == 0 || point.X < padding || point.X >= viewport.W-padding || point.Y < 0 || point.Y >= viewport.H {
		return 0, 0, false
	}
	best := float32(math.Inf(1))
	index, position, found := 0, float32(0), false
	consider := func(boundary int, y float32) {
		if distance := float32(math.Abs(float64(point.Y - y))); y >= 0 && y <= viewport.H && distance < best {
			best, index, position, found = distance, boundary, y, true
		}
	}
	for _, box := range boxes {
		style := text.ParagraphStyleAt(box.Start)
		if box.Start == 0 || !wysiwyg.SameBlockGroup(text.ParagraphStyleAt(box.Start-1), style) {
			consider(box.Index, box.Top)
		}
		_, end := text.ParagraphRange(box.Start)
		if end == text.Len() || !wysiwyg.SameBlockGroup(style, text.ParagraphStyleAt(end+1)) {
			consider(box.Index+1, box.Top+box.Height)
		}
	}
	return index, position, found
}

func (w *workspaceWYSIWYG) drawBlockDrop(c *mygoui.Context, padding float32, viewport mygoui.Rect,
	text richtext.Text, boxes []richtext.ParagraphBox, drag *visualBlockDrag, ink mygoui.Color) {
	index, y, ok := blockDropBoundary(text, boxes, padding, viewport, drag.point)
	if !ok || index == drag.block.box.Index || index == drag.block.box.Index+1 {
		return
	}
	point := drag.point
	start, end := text.ParagraphRange(drag.block.box.Start)
	value := text.Slice(start, end).String()
	font := mygoui.Font{Size: float32(w.fontSize())}
	if heading := text.ParagraphStyleAt(start).Heading; heading > 0 && heading <= len(wysHeadingSizes) {
		font.Size *= wysHeadingSizes[heading-1]
	}
	width := viewport.W - 2*padding
	y = float32(math.Round(float64(y)))
	preview := mygoui.ShapeText(value, font, width)
	mygoui.Box(c.Key("block drop feedback")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		IgnoreInput(true).Role(mygoui.RoleNone).DrawOver(func(p *mygoui.Painter, r mygoui.Rect) {
		p.Clip(r, 0, func() {
			p.TextLayout(preview, r.X+point.X, r.Y+point.Y, ink.Alpha(.55))
			p.Fill(mygoui.Rect{X: r.X + padding, Y: r.Y + y - 2, W: width, H: 4}, ink.Alpha(.12), 0)
		})
	})
}
