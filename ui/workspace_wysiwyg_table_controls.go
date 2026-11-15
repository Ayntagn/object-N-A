package ui

import (
	"fmt"
	"slices"

	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type tableAxis uint8

const (
	tableRows tableAxis = iota
	tableColumns
)

type visualTableSelection struct {
	id    string
	axis  tableAxis
	index int
}

func (s visualTableSelection) matches(id string, axis tableAxis, index int) bool {
	return s.id == id && s.axis == axis && s.index == index
}

func (s visualTableSelection) first() (row, col int) {
	if s.axis == tableRows {
		return s.index, 0
	}
	return 0, s.index
}

func (s visualTableSelection) last(table *wysiwyg.Table) (row, col int) {
	if s.axis == tableRows {
		return s.index, table.ColumnCount() - 1
	}
	return table.RowCount() - 1, s.index
}

type visualTableGeometry struct {
	frame       mygoui.Rect
	rows        []float32 // Interior edges relative to the table's top.
	columnWidth float32
	columns     int
}

func (g visualTableGeometry) edge(axis tableAxis, index int) float32 {
	if axis == tableRows {
		return g.rows[index]
	}
	return .5 + g.columnWidth*float32(index)
}

func (g visualTableGeometry) cellAt(x, y float32) (row, col int, ok bool) {
	if x < 0 || x >= g.frame.W || y < 0 || y >= g.frame.H {
		return 0, 0, false
	}
	next, edge := slices.BinarySearch(g.rows[1:], y)
	if edge {
		next++
	}
	row = min(next, len(g.rows)-2)
	col = min(max(0, int((x-.5)/g.columnWidth)), g.columns-1)
	return row, col, true
}

type visualTableControl struct {
	axis   tableAxis
	index  int
	adding bool
}

// EV-UI-145 measured the controls at cell centers and shared grid lines.
func (h visualTableControl) frame(g visualTableGeometry) mygoui.Rect {
	center := g.edge(h.axis, h.index)
	if !h.adding {
		center = (center + g.edge(h.axis, h.index+1)) / 2
	}
	if h.axis == tableRows {
		x := float32(-7.5)
		if h.adding {
			x = -8
		}
		return mygoui.Rect{X: x, Y: center - 14, W: 16, H: 28}
	}
	y := float32(-7.5)
	if h.adding {
		y = -8
	}
	return mygoui.Rect{X: center - 14, Y: y, W: 28, H: 16}
}

type visualTableHandles struct {
	id       string
	controls [2]visualTableControl
	count    int
}

func (h visualTableHandles) contains(g visualTableGeometry, x, y float32) bool {
	for _, handle := range h.controls[:h.count] {
		if g.valid(handle) && handle.frame(g).Contains(x, y) {
			return true
		}
	}
	return false
}

func (w *workspaceWYSIWYG) updateTableHover(table *wysiwyg.Table, g visualTableGeometry, pointer mygoui.Point,
	over bool, previous visualTableHandles) visualTableHandles {
	if !over || w.tableDrag.id != "" || w.tableMenu.id != "" {
		return visualTableHandles{}
	}
	x, y := pointer.X-g.frame.X, pointer.Y-g.frame.Y
	if previous.id == table.ID() && previous.contains(g, x, y) {
		w.tableHover = previous
		return previous
	}
	row, col, ok := g.cellAt(x, y)
	if !ok {
		return visualTableHandles{}
	}
	const edgeZone = 8
	columnEdge, rowEdge := -1, -1
	switch {
	case x-g.edge(tableColumns, col) < edgeZone:
		columnEdge = col
	case g.edge(tableColumns, col+1)-x < edgeZone:
		columnEdge = col + 1
	}
	if row > 0 {
		switch {
		case y-g.edge(tableRows, row) < edgeZone:
			rowEdge = row
		case g.edge(tableRows, row+1)-y < edgeZone:
			rowEdge = row + 1
		}
	}
	handles := visualTableHandles{id: table.ID()}
	if columnEdge >= 0 {
		handles.controls[handles.count] = visualTableControl{axis: tableColumns, index: columnEdge, adding: true}
		handles.count++
	}
	if rowEdge >= 0 {
		handles.controls[handles.count] = visualTableControl{axis: tableRows, index: rowEdge, adding: true}
		handles.count++
	}
	if handles.count == 0 {
		handles.controls = [2]visualTableControl{{axis: tableRows, index: row}, {axis: tableColumns, index: col}}
		handles.count = len(handles.controls)
	}
	w.tableHover = handles
	return handles
}

func (g visualTableGeometry) valid(handle visualTableControl) bool {
	count := g.columns
	if handle.axis == tableRows {
		count = len(g.rows) - 1
	}
	return handle.index >= 0 && (handle.index < count || handle.adding && handle.index == count)
}

type visualTableDrag struct {
	visualTableSelection
	start  mygoui.Point // Press in handle-local coordinates.
	point  mygoui.Point // Current pointer relative to the table.
	moved  bool
	target int
}

func (w *workspaceWYSIWYG) drawVisualTableControls(c *mygoui.Context, app *workspaceWindow, table *wysiwyg.Table,
	offset int, geometry visualTableGeometry, hovered visualTableHandles) {
	if app.modalOpen {
		return
	}
	for _, handle := range hovered.controls[:hovered.count] {
		w.drawTableControl(c, app, table, offset, geometry, handle)
	}
	active := w.tableMenu
	if w.tableDrag.id != "" {
		active = w.tableDrag.visualTableSelection
	}
	handle := visualTableControl{axis: active.axis, index: active.index}
	if active.id == table.ID() && geometry.valid(handle) && !slices.Contains(hovered.controls[:hovered.count], handle) {
		w.drawTableControl(c, app, table, offset, geometry, handle)
	}
}

func tableHandleLabel(handle visualTableControl, table *wysiwyg.Table) string {
	if !handle.adding {
		if handle.axis == tableColumns {
			return fmt.Sprintf("Column %d options", handle.index+1)
		}
		if handle.index == 0 {
			return "Select header row"
		}
		return fmt.Sprintf("Row %d options", handle.index)
	}
	if handle.axis == tableColumns {
		if handle.index == table.ColumnCount() {
			return "Add column at end"
		}
		return fmt.Sprintf("Add column before column %d", handle.index+1)
	}
	if handle.index == table.RowCount() {
		return "Add row at end"
	}
	return fmt.Sprintf("Add row before row %d", handle.index)
}

var tableControlBackground = mygoui.RGB(247, 247, 247)
var tableControlInk = mygoui.RGB(168, 168, 168)

func (w *workspaceWYSIWYG) drawTableControl(c *mygoui.Context, app *workspaceWindow, table *wysiwyg.Table,
	offset int, geometry visualTableGeometry, handle visualTableControl) {
	selection := visualTableSelection{table.ID(), handle.axis, handle.index}
	frame := handle.frame(geometry)
	name := tableHandleLabel(handle, table)
	button := mygoui.ButtonBase(c.Key(handle)).Absolute().Left(frame.X).Top(frame.Y).Size(frame.W, frame.H).
		Label(name).KeepFocus().TabStop(false).Radius(8).Background(tableControlBackground).
		Shadow(0, 1, 3, 1, mygoui.RGBA(0, 0, 0, .15))
	if handle.adding {
		action := wysiwyg.AddTableRow
		if handle.axis == tableColumns {
			action = wysiwyg.AddTableColumn
		}
		button.Cursor(mygoui.CursorPointer).OnClick(func() {
			w.changeVisualTable(app, offset, selection, action, 0, parser.AlignNone)
		}).Children(func() { chromeIconColor(c, "plus", 16, tableControlInk) })
		return
	}
	button.Cursor(mygoui.CursorGrab).DrawOver(drawTableHandleGrip)
	button.OnShortcut(mygoui.Cmd, mygoui.KeyZ, func() { w.undoVisualTable(app, false) }).
		OnShortcut(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ, func() { w.undoVisualTable(app, true) })
	if w.tableDrag.visualTableSelection == selection && w.tableDrag.moved {
		button.Cursor(mygoui.CursorGrabbing)
	}
	button.OnClick(func() { w.openVisualTableMenu(app, offset, selection) })
	button.HandleInput(func(ev mygoui.InputEvent) bool {
		return w.tableHandleInput(app, offset, geometry, frame, selection, ev)
	})
	if selection.axis == tableRows && selection.index == 0 {
		return
	}
	open := w.tableMenu == selection
	button.HasPopup(mygoui.PopupMenu).Expanded(open)
	if open {
		w.drawVisualTableMenu(c, app, button, offset, selection, name, &open)
	}
}

type visualTableMenuItem struct {
	label     string
	action    wysiwyg.TableAction
	alignment parser.Align
}

var tableColumnMenuItems = [...]visualTableMenuItem{
	{"Align Left", wysiwyg.AlignTableColumn, parser.AlignLeft},
	{"Align Center", wysiwyg.AlignTableColumn, parser.AlignCenter},
	{"Align Right", wysiwyg.AlignTableColumn, parser.AlignRight},
	{"Delete Column", wysiwyg.DeleteTableColumn, parser.AlignNone},
}

var tableDeleteRow = visualTableMenuItem{"Delete Row", wysiwyg.DeleteTableRow, parser.AlignNone}

func (w *workspaceWYSIWYG) drawVisualTableMenu(c *mygoui.Context, app *workspaceWindow, trigger mygoui.Element,
	offset int, selection visualTableSelection, name string, open *bool) {
	items := tableColumnMenuItems[:]
	if selection.axis == tableRows {
		items = []visualTableMenuItem{tableDeleteRow}
	}
	panel := mygoui.PopoverBase(c.Key("table menu"), trigger, open, func(panel mygoui.Element) {
		panel.AttachTo(trigger, mygoui.AnchorTop, mygoui.AnchorBottom).Margin(0, 0, 8).
			Role(mygoui.RoleMenu).Label(name).Radius(8).Background(tableControlBackground).
			Shadow(0, 1, 3, 1, mygoui.RGBA(0, 0, 0, .15))
		mygoui.Row(c).Padding(6).Gap(12).Children(func() {
			for _, item := range items {
				button := mygoui.ButtonBase(c.Key(item.label)).Size(32, 32).Role(mygoui.RoleMenuItem).Label(item.label).
					KeepFocus().Radius(4).OnClick(func() {
					w.changeVisualTable(app, offset, selection, item.action, 0, item.alignment)
				}).DrawOver(func(p *mygoui.Painter, r mygoui.Rect) { drawTableMenuIcon(p, r, item.alignment) })
				if button.Hovered() {
					button.Background(mygoui.RGB(232, 232, 232))
				}
			}
		})
	})
	if panel.Dismissed() && w.tableMenu == selection {
		w.tableMenu = visualTableSelection{}
	}
}

func drawTableMenuIcon(p *mygoui.Painter, r mygoui.Rect, alignment parser.Align) {
	x, y := r.X+(r.W-18)/2, r.Y+(r.H-18)/2
	if alignment == parser.AlignNone {
		p.Stroke(mygoui.Rect{X: x + 3.5, Y: y + 3.5, W: 11, H: 13}, tableControlInk, 1, 1.3)
		p.Fill(mygoui.Rect{X: x + 2, Y: y + 2, W: 14, H: 1.3}, tableControlInk, .65)
		p.Fill(mygoui.Rect{X: x + 6.5, Y: y + .5, W: 5, H: 1.5}, tableControlInk, .5)
		for _, shift := range [...]float32{7, 10} {
			p.Fill(mygoui.Rect{X: x + shift, Y: y + 6, W: 1.15, H: 8}, tableControlInk, .55)
		}
		return
	}
	for i, length := range [...]float32{17, 11, 17, 11, 17} {
		var shift float32
		switch alignment {
		case parser.AlignCenter:
			shift = (17 - length) / 2
		case parser.AlignRight:
			shift = 17 - length
		}
		p.Fill(mygoui.Rect{X: x + shift + .5, Y: y + float32(i)*3.9 + .6, W: length, H: 1.3}, tableControlInk, .65)
	}
}

func (w *workspaceWYSIWYG) closeTableControls() {
	w.tableSelection = visualTableSelection{}
	w.tableMenu = visualTableSelection{}
	w.tableHover = visualTableHandles{}
	w.tableDrag = visualTableDrag{}
	w.tableFocus = nil
}

func (w *workspaceWYSIWYG) openVisualTableMenu(app *workspaceWindow, offset int, selection visualTableSelection) {
	if !w.editable(app) {
		return
	}
	table, ok := w.session.TableAt(offset)
	if !ok || table.ID() != selection.id {
		return
	}
	if selection.axis == tableColumns || selection.index != 0 {
		w.tableMenu = selection
	} else {
		w.tableMenu = visualTableSelection{}
	}
	w.selectTableAxis(table, offset, selection)
}

func (w *workspaceWYSIWYG) selectTableAxis(table *wysiwyg.Table, offset int, selection visualTableSelection) {
	w.tableSelection = selection
	row, col := selection.first()
	w.focusTableCell(table, offset, row, col, selectCellStart)
}

func (w *workspaceWYSIWYG) collapseTableSelection(table *wysiwyg.Table, offset int) {
	row, col := w.tableSelection.first()
	w.closeTableControls()
	w.focusTableCell(table, offset, row, col, selectCellStart)
}

func (w *workspaceWYSIWYG) changeVisualTable(app *workspaceWindow, offset int, selection visualTableSelection,
	action wysiwyg.TableAction, destination int, alignment parser.Align) {
	if !w.editable(app) {
		return
	}
	if w.focusedCell != nil {
		w.syncVisualTableCell(app, w.focusedCell)
	}
	current, ok := w.session.TableAt(offset)
	if !ok || current.ID() != selection.id {
		return
	}
	table, err := w.session.ChangeTableStructure(offset, current, action, selection.index, destination, alignment)
	if err != nil {
		app.problem = err.Error()
		return
	}
	w.closeTableControls()
	w.sync(app)
	if table == nil || action == wysiwyg.DeleteTableRow && current.RowCount() == 2 {
		w.focus = true
		return
	}
	switch action {
	case wysiwyg.DeleteTableRow:
		w.focusTableCell(table, offset, min(selection.index, table.RowCount()-1), 0, selectCellStart)
	case wysiwyg.DeleteTableColumn:
		w.focusTableCell(table, offset, 0, min(selection.index, table.ColumnCount()-1), selectCellStart)
	case wysiwyg.MoveTableRow, wysiwyg.MoveTableColumn:
		selection.index = destination
		w.selectTableAxis(table, offset, selection)
	default:
		w.selectTableAxis(table, offset, selection)
	}
}

func (w *workspaceWYSIWYG) visualTableContextMenu(m *mygoui.Menu, app *workspaceWindow, cell *visualTableCell) {
	if !w.editable(app) {
		return
	}
	m.Separator()
	apply := func(item visualTableMenuItem, axis tableAxis, index, destination int) {
		w.changeVisualTable(app, cell.offset, visualTableSelection{cell.id, axis, index}, item.action, destination, item.alignment)
	}
	m.Submenu("Row", func(menu *mygoui.Menu) {
		if menu.Item("Add Row Above").Disabled(cell.row == 0).Chosen() {
			apply(visualTableMenuItem{action: wysiwyg.AddTableRow}, tableRows, cell.row, 0)
		}
		if menu.Item("Add Row Below").Chosen() {
			apply(visualTableMenuItem{action: wysiwyg.AddTableRow}, tableRows, cell.row+1, 0)
		}
		if menu.Item("Move Row Up").Disabled(cell.row == 0).Chosen() {
			apply(visualTableMenuItem{action: wysiwyg.MoveTableRow}, tableRows, cell.row, cell.row-1)
		}
		if menu.Item("Move Row Down").Disabled(cell.row+1 >= cell.table.RowCount()).Chosen() {
			apply(visualTableMenuItem{action: wysiwyg.MoveTableRow}, tableRows, cell.row, cell.row+1)
		}
		if menu.Item(tableDeleteRow.label).Disabled(cell.row == 0).Chosen() {
			apply(tableDeleteRow, tableRows, cell.row, 0)
		}
	})
	m.Submenu("Column", func(menu *mygoui.Menu) {
		if menu.Item("Add Column Before").Chosen() {
			apply(visualTableMenuItem{action: wysiwyg.AddTableColumn}, tableColumns, cell.col, 0)
		}
		if menu.Item("Add Column After").Chosen() {
			apply(visualTableMenuItem{action: wysiwyg.AddTableColumn}, tableColumns, cell.col+1, 0)
		}
		if menu.Item("Move Column Left").Disabled(cell.col == 0).Chosen() {
			apply(visualTableMenuItem{action: wysiwyg.MoveTableColumn}, tableColumns, cell.col, cell.col-1)
		}
		if menu.Item("Move Column Right").Disabled(cell.col+1 >= cell.table.ColumnCount()).Chosen() {
			apply(visualTableMenuItem{action: wysiwyg.MoveTableColumn}, tableColumns, cell.col, cell.col+1)
		}
		for _, item := range tableColumnMenuItems {
			if menu.Item(item.label).Chosen() {
				apply(item, tableColumns, cell.col, 0)
			}
		}
	})
}

func drawTableHandleGrip(p *mygoui.Painter, r mygoui.Rect) {
	x, y := r.X+(r.W-10)/2, r.Y+r.H/2
	for _, shift := range [...]float32{-2, 0, 2} {
		p.Fill(mygoui.Rect{X: x, Y: y + shift, W: 10, H: .9}, tableControlInk, .45)
	}
}

func (w *workspaceWYSIWYG) tableHandleInput(app *workspaceWindow, offset int, geometry visualTableGeometry,
	frame mygoui.Rect, selection visualTableSelection, event mygoui.InputEvent) bool {
	point := mygoui.Point{X: event.X, Y: event.Y}
	drag := &w.tableDrag
	switch event.Kind {
	case mygoui.InputPointerDown:
		if event.Button != 0 || drag.id != "" {
			return false
		}
		if w.tableMenu != selection {
			w.tableMenu = visualTableSelection{}
		}
		*drag = visualTableDrag{visualTableSelection: selection, start: point, target: selection.index}
		return true
	case mygoui.InputPointerMove, mygoui.InputPointerUp:
		if drag.visualTableSelection != selection {
			return false
		}
		dx, dy := point.X-drag.start.X, point.Y-drag.start.Y
		drag.moved = drag.moved || dx*dx+dy*dy >= 16
		drag.point = mygoui.Point{X: frame.X + point.X, Y: frame.Y + point.Y}
		row, col, inTable := geometry.cellAt(drag.point.X, drag.point.Y)
		drag.target = -1
		if inTable {
			drag.target = row
			if selection.axis == tableColumns {
				drag.target = col
			}
		}
		if event.Kind == mygoui.InputPointerMove {
			return true
		}
		gesture := *drag
		*drag = visualTableDrag{}
		if event.Canceled || event.Button != 0 {
			return true
		}
		// A native release can land outside the handle's clipped hit path.
		// Opening is idempotent if MyGo also delivers the click.
		if !gesture.moved && (mygoui.Rect{W: frame.W, H: frame.H}).Contains(point.X, point.Y) {
			w.openVisualTableMenu(app, offset, selection)
		}
		if gesture.moved && gesture.target >= 0 && gesture.target != selection.index {
			action := wysiwyg.MoveTableRow
			if selection.axis == tableColumns {
				action = wysiwyg.MoveTableColumn
			}
			w.changeVisualTable(app, offset, selection, action, gesture.target, parser.AlignNone)
		}
		return true
	}
	return false
}

func paintTableDragPreview(p *mygoui.Painter, rect mygoui.Rect, g visualTableGeometry, drag visualTableDrag,
	table *wysiwyg.Table, font mygoui.Font, ink, accent mygoui.Color) {
	if !drag.moved || drag.id != table.ID() || drag.target < 0 {
		return
	}
	firstRow, firstCol := drag.first()
	lastRow, lastCol := drag.last(table)
	preview := mygoui.Rect{X: rect.X, Y: rect.Y, W: rect.W - 1, H: rect.H - 1}
	if drag.axis == tableRows {
		preview.H = g.rows[firstRow+1] - g.rows[firstRow]
		preview.Y += min(rect.H-preview.H, max(0, drag.point.Y-preview.H/2))
	} else {
		preview.W = g.columnWidth
		preview.X += min(rect.W-preview.W, max(0, drag.point.X-preview.W/2))
	}
	p.Fill(preview, tableControlBackground.Alpha(.4), 0)
	p.Stroke(preview, accent.Alpha(.4), 0, 1)
	p.Clip(preview, 0, func() {
		for row := firstRow; row <= lastRow; row++ {
			cellFont := font
			if row == 0 {
				cellFont.Weight = 600
			}
			y := preview.Y + g.rows[row] - g.rows[firstRow] + wysTableCellY
			for col := firstCol; col <= lastCol; col++ {
				x := preview.X + float32(col-firstCol)*g.columnWidth + wysTableCellX
				maxWidth := max(1, g.columnWidth-2*wysTableCellX)
				layout := mygoui.ShapeText(visualCellText(table.Cell(row, col)).String(), cellFont, maxWidth)
				textWidth, _ := layout.Size()
				switch table.Align(col) {
				case parser.AlignCenter:
					x += max(0, maxWidth-textWidth) / 2
				case parser.AlignRight:
					x += max(0, maxWidth-textWidth)
				}
				p.TextLayout(layout, x, y, ink.Alpha(.4))
			}
		}
	})
}

func (w *workspaceWYSIWYG) paintTableControlLine(p *mygoui.Painter, rect mygoui.Rect, id string, g visualTableGeometry,
	hovered visualTableHandles, pointer mygoui.Point, color mygoui.Color) {
	if drag := w.tableDrag; drag.id == id && drag.moved && drag.target >= 0 && drag.target != drag.index {
		boundary := drag.target
		if drag.target > drag.index {
			boundary++
		}
		paintTableEdge(p, rect, drag.axis, g.edge(drag.axis, boundary), color)
	}
	for _, handle := range hovered.controls[:hovered.count] {
		if !handle.adding {
			continue
		}
		x, y := pointer.X-g.frame.X, pointer.Y-g.frame.Y
		offset := g.edge(handle.axis, handle.index)
		line := mygoui.Rect{X: 0, Y: offset, W: rect.W, H: 1}
		if handle.axis == tableColumns {
			line = mygoui.Rect{X: offset, Y: 0, W: 1, H: rect.H}
		}
		if line.Contains(x, y) {
			paintTableEdge(p, rect, handle.axis, offset, color)
		}
	}
}

func paintTableEdge(p *mygoui.Painter, rect mygoui.Rect, axis tableAxis, offset float32, color mygoui.Color) {
	if axis == tableColumns {
		p.Fill(mygoui.Rect{X: rect.X + offset, Y: rect.Y, W: 1, H: rect.H}, color, 0)
		return
	}
	p.Fill(mygoui.Rect{X: rect.X, Y: rect.Y + offset, W: rect.W, H: 1}, color, 0)
}
