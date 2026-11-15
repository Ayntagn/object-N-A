package ui

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"cyberwriter/internal/core/drawing"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

const (
	drawingSelection drawing.Kind = "selection"
	drawingHand      drawing.Kind = "hand"
	drawingEraser    drawing.Kind = "eraser"
)

var drawingTools = []struct {
	kind  drawing.Kind
	title string
	label string
	key   mygoui.Key
	digit mygoui.Key
}{
	{drawingHand, "Hand", "Hand (panning tool)", mygoui.KeyH, mygoui.KeyUnknown},
	{drawingSelection, "Select", "Selection", mygoui.KeyV, mygoui.Key1},
	{drawing.Rectangle, "Rectangle", "Rectangle", mygoui.KeyR, mygoui.Key2},
	{drawing.Diamond, "Diamond", "Diamond", mygoui.KeyD, mygoui.Key3},
	{drawing.Ellipse, "Ellipse", "Ellipse", mygoui.KeyO, mygoui.Key4},
	{drawing.Arrow, "Arrow", "Arrow", mygoui.KeyA, mygoui.Key5},
	{drawing.Line, "Line", "Line", mygoui.KeyL, mygoui.Key6},
	{drawing.FreeDraw, "Draw", "Draw", mygoui.KeyP, mygoui.Key7},
	{drawing.Text, "Text", "Text", mygoui.KeyT, mygoui.Key8},
	{drawingEraser, "Eraser", "Eraser", mygoui.KeyE, mygoui.Key0},
}

type workspaceDrawing struct {
	storage   *workspace.Vault
	file      workspace.Drawing
	scene     *drawing.Scene
	undo      []*drawing.Scene
	redo      []*drawing.Scene
	tool      drawing.Kind
	keepTool  bool
	selected  int
	zoom      float64
	offset    drawing.Point
	viewport  drawing.Point
	viewReady bool
	linkedMap bool
	gesture   drawingGesture
	textEdit  *drawingTextDraft
}

type drawingGesture struct {
	active       bool
	button       int
	tool         drawing.Kind
	startView    drawing.Point
	offsetBefore drawing.Point
	points       []drawing.Point
	erased       map[int]struct{}
}

func newWorkspaceDrawing(storage *workspace.Vault, file workspace.Drawing) *workspaceDrawing {
	return &workspaceDrawing{
		storage: storage, file: file, scene: file.Scene, tool: drawingSelection,
		zoom: 1, selected: -1,
		linkedMap: slices.ContainsFunc(file.Scene.Elements(), func(element drawing.Element) bool { return element.WikiCard() != "" }),
	}
}

func loadWorkspaceDrawing(storage *workspace.Vault, relativePath string) (*workspaceDrawing, error) {
	file, err := storage.ReadDrawing(relativePath)
	if err != nil {
		return nil, err
	}
	return newWorkspaceDrawing(storage, file), nil
}

func (app *workspaceWindow) newDrawing() {
	if app.vault == nil || app.modalOpen {
		return
	}
	if app.multiFile.active {
		app.problem = multiFileMarkdownOnly
		return
	}
	file, err := app.vault.CreateDrawing()
	if err != nil {
		app.problem = fmt.Sprintf("Could not create a drawing. %v", err)
		return
	}
	app.openNewDrawing(file)
}

func (app *workspaceWindow) openNewDrawing(file workspace.Drawing) {
	if app.modalOpen {
		tab := app.blankTab()
		app.tabs = append(app.tabs, tab)
		app.setTabDrawing(tab, newWorkspaceDrawing(app.vault, file))
		app.refreshVault()
		return
	}
	tab := app.availableTab()
	app.setTabDrawing(tab, newWorkspaceDrawing(app.vault, file))
	app.refreshVault()
	app.activateTab(tab)
}

func (app *workspaceWindow) openDrawing(relativePath string, newTab bool) {
	if app.vault == nil || app.modalOpen {
		return
	}
	if tab := app.findVaultTab(relativePath); tab != nil {
		app.activateTab(tab)
		return
	}
	load := func() {
		draw, err := loadWorkspaceDrawing(app.vault, relativePath)
		if err != nil {
			app.problem = fmt.Sprintf("Could not open %q. %v", relativePath, err)
			return
		}
		tab := app.workspaceTab
		if newTab {
			tab = app.availableTab()
		}
		app.releaseTabStorage(tab)
		app.setTabDrawing(tab, draw)
		app.activateTab(tab)
	}
	if newTab {
		load()
	} else {
		app.beforeLeavingTabs([]*workspaceTab{app.workspaceTab}, load)
	}
}

func (app *workspaceWindow) setTabDrawing(tab *workspaceTab, draw *workspaceDrawing) {
	tab.wys.reset()
	tab.document = workspace.Document{Path: draw.file.Path}
	tab.drawing = draw
	tab.draft, tab.code = nil, nil
	tab.text, tab.textVersion = "", 0
	tab.metrics = workspace.TextMetrics{}
	tab.outlineCache = outlineCache{}
	tab.previewAnchor = ""
}

func (d *workspaceDrawing) dirty() bool { return d.scene != d.file.Scene }

func (d *workspaceDrawing) save() error {
	if !d.dirty() {
		return nil
	}
	file, err := d.storage.SaveDrawing(d.file, d.scene)
	if err == nil {
		d.file = file
	}
	return err
}

func (app *workspaceWindow) drawingPane(c *mygoui.Context) {
	d := app.drawing
	if !app.modalOpen && !app.commandPalette.open {
		d.shortcuts(c)
	}
	accent := c.Theme().Accent
	dark := d.linkedMap && (c.Theme().Dark || app.blackout)
	mygoui.Box(c.Key(d)).Grow(1).MinWidth(0).MinHeight(0).FillWidth().Clip().Children(func() {
		cursor := mygoui.CursorCrosshair
		switch {
		case d.gesture.active && d.gesture.tool == drawingHand:
			cursor = mygoui.CursorGrabbing
		case d.tool == drawingHand:
			cursor = mygoui.CursorGrab
		case d.tool == drawingSelection:
			cursor = mygoui.CursorDefault
		case d.tool == drawing.Text:
			cursor = mygoui.CursorText
		}
		canvas := mygoui.Box(c).Fill().Focusable().AutoFocus().Role(mygoui.RoleGroup).
			Label("Drawing canvas").Cursor(cursor).Disabled(app.modalOpen && d.textEdit == nil).
			HandleInput(func(event mygoui.InputEvent) bool {
				handled := d.input(event)
				if d.textEdit != nil {
					app.modalOpen = true
				}
				return handled
			}).Draw(func(p *mygoui.Painter, r mygoui.Rect) {
			d.paint(p, r, accent, dark)
		})
		bounds := canvas.Bounds()
		if bounds.W > 0 && bounds.H > 0 {
			d.viewport = drawing.Point{float64(bounds.W), float64(bounds.H)}
		} else {
			width, height := c.Size()
			if app.vaultPanelVisible() {
				width -= vaultPanelWidth
			}
			chrome := float32(titleBarHeight)
			if len(app.tabs) > 1 {
				chrome += tabBarHeight + tabBarGap
			}
			d.viewport = drawing.Point{float64(width), float64(max(1, height-chrome))}
		}
		if !d.viewReady {
			d.fitInitialView()
			d.viewReady = true
		}
		if app.vault == d.storage && !d.gesture.active && d.tool == drawingSelection {
			app.drawingWikiLinks(c, d)
		}
		d.toolbar(c, app.modalOpen)
	})
	if d.textEdit != nil {
		app.drawingTextEditor(c, d)
	}
}

func (app *workspaceWindow) drawingWikiLinks(c *mygoui.Context, d *workspaceDrawing) {
	for _, card := range d.scene.Elements() {
		link := card.WikiCard()
		if link == "" {
			continue
		}
		left := (card.X+card.Width)*d.zoom + d.offset[0] - 1
		top := card.Y*d.zoom + d.offset[1] - 16
		if left < 0 || top < 0 || left+18 > d.viewport[0] || top+18 > d.viewport[1] {
			continue
		}
		label := "Open " + link[2:len(link)-2]
		button := mygoui.ButtonBase(c.Key(card.ID)).Role(mygoui.RoleLink).Label(label).Tooltip(label).
			Absolute().Left(float32(left)).Top(float32(top)).Size(18, 18).Center().Disabled(app.modalOpen)
		button.OnClick(func() { app.openDrawingWikiLink(d.file.Path, link, button.ClickModifiers()) }).Children(func() {
			chromeIconColor(c, "arrow.up.forward.app", 12, mygoui.Hex("#4095fa"))
		})
	}
}

func (app *workspaceWindow) openDrawingWikiLink(source, link string, mods mygoui.Modifiers) {
	if app.index.data != nil {
		if target, ok := app.index.data.Resolve(source, link); ok {
			app.openVaultFileByPreference(target, mods, true)
			return
		}
	}
	app.problem = fmt.Sprintf("Could not resolve %s", link)
}

func (d *workspaceDrawing) toolbar(c *mygoui.Context, disabled bool) {
	theme := c.Theme()
	mygoui.Row(c).Absolute().Left(8).Right(8).Top(12).Justify(mygoui.Center).PassThrough().Children(func() {
		mygoui.Row(c).Role(mygoui.RoleToolbar).Label("Drawing tools").
			MaxWidth(float32(max(1, d.viewport[0]-16))).AlignItems(mygoui.Start).
			Gap(4).Padding(4, 8).Background(theme.Surface).Border(1, theme.Border).Radius(theme.Radius).
			Children(func() {
				mygoui.Checkbox(c, &d.keepTool, "Lock").Label("Keep selected tool active after drawing").
					Tooltip("Keep selected tool active after drawing (Q)").Shrink(0).Disabled(disabled)
				mygoui.RadioGroup(c, func() {
					for _, tool := range drawingTools {
						button := mygoui.RadioBase(c.Key(tool.kind), &d.tool, tool.kind).
							Label(tool.label).Tooltip(tool.label).Disabled(disabled).
							Center().Padding(5, 7).Radius(theme.Radius).Shrink(0).
							OnChange(func() { d.selected = -1 })
						ink := theme.Text
						if d.tool == tool.kind {
							button.Background(theme.SurfacePressed)
							ink = theme.Accent
						} else if button.Hovered() {
							button.Background(theme.SurfaceHover)
						}
						button.Children(func() {
							mygoui.Text(c, tool.title).FontSize(12).SingleLine().TextColor(ink)
						})
					}
				}).Row().Wrap().MinWidth(0).Shrink(1).Gap(1).Label("Shapes")
			})
	})
	mygoui.Toolbar(c, func() {
		center := drawing.Point{d.viewport[0] / 2, d.viewport[1] / 2}
		mygoui.Button(c, "−").Label("Zoom out").Disabled(disabled || d.zoom <= .1).
			OnClick(func() { d.setZoom(d.zoom/1.25, center) })
		mygoui.Button(c, fmt.Sprintf("%.0f%%", d.zoom*100)).Label("Reset zoom").Disabled(disabled).
			OnClick(func() { d.setZoom(1, center) })
		mygoui.Button(c, "+").Label("Zoom in").Disabled(disabled || d.zoom >= 8).
			OnClick(func() { d.setZoom(d.zoom*1.25, center) })
		mygoui.Button(c, "Undo").Disabled(disabled || len(d.undo) == 0).OnClick(d.undoEdit)
		mygoui.Button(c, "Redo").Disabled(disabled || len(d.redo) == 0).OnClick(d.redoEdit)
	}).Label("Canvas actions").Absolute().Left(12).Bottom(12).Gap(4).Padding(4).
		Background(theme.Surface).Border(1, theme.Border).Radius(theme.Radius)
}

func (d *workspaceDrawing) shortcuts(c *mygoui.Context) {
	for _, tool := range drawingTools {
		selectTool := func() {
			d.cancelGesture()
			d.selected = -1
			d.tool = tool.kind
		}
		c.OnWindowShortcut(0, tool.key, selectTool)
		if tool.digit != mygoui.KeyUnknown {
			c.OnWindowShortcut(0, tool.digit, selectTool)
		}
	}
	c.OnWindowShortcut(0, mygoui.KeyEscape, func() {
		d.cancelGesture()
		d.tool, d.selected = drawingSelection, -1
	})
	c.OnWindowShortcut(0, mygoui.KeyQ, func() { d.keepTool = !d.keepTool })
	c.OnWindowShortcut(0, mygoui.KeyDelete, d.deleteSelection)
	c.OnWindowShortcut(0, mygoui.KeyBackspace, d.deleteSelection)
	c.OnWindowShortcut(mygoui.Cmd, mygoui.KeyZ, d.undoEdit)
	c.OnWindowShortcut(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ, d.redoEdit)
}

func (d *workspaceDrawing) input(event mygoui.InputEvent) bool {
	view := drawing.Point{float64(event.X), float64(event.Y)}
	switch event.Kind {
	case mygoui.InputKeyDown:
		return d.textKey(event)
	case mygoui.InputCommand:
		switch event.Text {
		case "undo":
			d.undoEdit()
		case "redo":
			d.redoEdit()
		case "delete":
			d.deleteSelection()
		default:
			return false
		}
	case mygoui.InputPointerDown:
		if event.Button != 0 && event.Button != 2 || d.gesture.active {
			return false
		}
		d.beginGesture(view, event.Button, event.Clicks)
	case mygoui.InputPointerMove:
		if !d.gesture.active || event.Button != d.gesture.button {
			return false
		}
		d.trackGesture(view)
	case mygoui.InputPointerUp:
		if !d.gesture.active || event.Button != d.gesture.button {
			return false
		}
		if event.Canceled {
			d.cancelGesture()
		} else {
			d.trackGesture(view)
			d.commitGesture()
		}
	case mygoui.InputZoom:
		if d.gesture.active {
			return false
		}
		d.setZoom(d.zoom*float64(event.Scale), view)
	case mygoui.InputScroll:
		if d.gesture.active {
			return false
		}
		if event.Mods&(mygoui.Ctrl|mygoui.Cmd) != 0 {
			d.setZoom(d.zoom*math.Exp(-float64(event.DY)/180), view)
		} else {
			d.offset[0] -= float64(event.DX)
			d.offset[1] -= float64(event.DY)
		}
	default:
		return false
	}
	return true
}

func (d *workspaceDrawing) beginGesture(view drawing.Point, button, clicks int) {
	tool := d.tool
	if button == 2 {
		tool = drawingHand
	}
	start := d.scenePoint(view)
	d.gesture = drawingGesture{
		active: true, button: button, tool: tool, startView: view, offsetBefore: d.offset,
		points: []drawing.Point{start, start},
	}
	switch tool {
	case drawingSelection:
		d.selected = d.hit(start, true)
		if clicks == 2 && d.selected >= 0 && d.scene.Elements()[d.selected].Type == drawing.Text {
			d.gesture.tool = drawing.Text
		}
	case drawingEraser:
		d.gesture.erased = make(map[int]struct{})
		d.trackGesture(view)
	case drawing.FreeDraw:
		d.gesture.points = d.gesture.points[:1]
	}
}

func (d *workspaceDrawing) trackGesture(view drawing.Point) {
	g := &d.gesture
	if g.tool == drawingHand {
		d.offset = drawing.Point{
			g.offsetBefore[0] + view[0] - g.startView[0],
			g.offsetBefore[1] + view[1] - g.startView[1],
		}
		return
	}
	point := d.scenePoint(view)
	switch g.tool {
	case drawingEraser:
		if hit := d.hit(point, false); hit >= 0 {
			g.erased[hit] = struct{}{}
		}
	case drawing.FreeDraw:
		if point != g.points[len(g.points)-1] {
			g.points = append(g.points, point)
		}
	default:
		g.points[1] = point
	}
}

func (d *workspaceDrawing) cancelGesture() {
	if d.gesture.active && d.gesture.tool == drawingHand {
		d.offset = d.gesture.offsetBefore
	}
	d.gesture = drawingGesture{}
}

func (d *workspaceDrawing) commitGesture() {
	g := d.gesture
	d.gesture = drawingGesture{}
	scene := d.scene
	switch g.tool {
	case drawingHand:
		return
	case drawing.Text:
		d.editTextAt(g.points[0])
		return
	case drawingSelection:
		delta := g.delta()
		if d.selected >= 0 && math.Hypot(delta[0], delta[1])*d.zoom >= 2 {
			d.commit(scene.Move(d.selected, delta[0], delta[1]))
		}
		return
	case drawingEraser:
		scene = scene.Erase(slices.Collect(maps.Keys(g.erased)))
		d.selected = -1
	default:
		element := drawing.Shape(g.tool, g.points)
		switch g.tool {
		case drawing.Rectangle, drawing.Diamond, drawing.Ellipse:
			if min(element.Width, element.Height)*d.zoom < 2 {
				return
			}
		case drawing.Line, drawing.Arrow:
			if math.Hypot(element.Width, element.Height)*d.zoom < 2 {
				return
			}
		}
		d.selected = len(scene.Elements())
		scene = scene.Add(element)
	}
	d.commit(scene)
	if !d.keepTool {
		d.tool = drawingSelection
	}
}

func (g drawingGesture) delta() drawing.Point {
	first, last := g.points[0], g.points[len(g.points)-1]
	return drawing.Point{last[0] - first[0], last[1] - first[1]}
}

func (d *workspaceDrawing) commit(scene *drawing.Scene) {
	if scene == d.scene {
		return
	}
	// A bounded undo list keeps large imported scenes usable over long edits.
	if len(d.undo) == 100 {
		copy(d.undo, d.undo[1:])
		d.undo = d.undo[:99]
	}
	d.undo = append(d.undo, d.scene)
	d.scene, d.redo = scene, nil
}

func (d *workspaceDrawing) undoEdit() {
	if len(d.undo) == 0 {
		return
	}
	d.cancelGesture()
	d.redo = append(d.redo, d.scene)
	last := len(d.undo) - 1
	d.scene, d.undo[last] = d.undo[last], nil
	d.undo, d.selected = d.undo[:last], -1
}

func (d *workspaceDrawing) redoEdit() {
	if len(d.redo) == 0 {
		return
	}
	d.cancelGesture()
	d.undo = append(d.undo, d.scene)
	last := len(d.redo) - 1
	d.scene, d.redo[last] = d.redo[last], nil
	d.redo, d.selected = d.redo[:last], -1
}

func (d *workspaceDrawing) deleteSelection() {
	if d.selected < 0 || d.selected >= len(d.scene.Elements()) || !d.scene.Elements()[d.selected].Editable() {
		return
	}
	d.cancelGesture()
	d.commit(d.scene.Erase([]int{d.selected}))
	d.selected = -1
}

func (d *workspaceDrawing) setZoom(zoom float64, view drawing.Point) {
	if math.IsNaN(zoom) {
		return
	}
	anchor := d.scenePoint(view)
	d.zoom = min(8, max(.1, zoom))
	d.offset = drawing.Point{view[0] - anchor[0]*d.zoom, view[1] - anchor[1]*d.zoom}
}

func (d *workspaceDrawing) scenePoint(view drawing.Point) drawing.Point {
	return drawing.Point{(view[0] - d.offset[0]) / d.zoom, (view[1] - d.offset[1]) / d.zoom}
}
