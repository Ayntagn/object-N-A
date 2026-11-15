package ui

import (
	"fmt"

	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-144 measured the picker content, excluding the native popover chrome.
const (
	tableGridRows       = 6
	tableGridColumns    = 8
	tableGridCellSize   = 18
	tableGridCellGap    = 3
	tableGridPadding    = 12
	tableGridCaptionGap = 8
	tableGridCaptionH   = 12
	tableGridWidth      = tableGridColumns*tableGridCellSize + (tableGridColumns-1)*tableGridCellGap
	tableGridHeight     = tableGridRows*tableGridCellSize + (tableGridRows-1)*tableGridCellGap
)

type workspaceTableGrid struct {
	open, focus               bool
	row, column               int
	pointerRow, pointerColumn int
}

func (app *workspaceWindow) tableGridButton(c *mygoui.Context, symbol string, enabled bool) mygoui.Element {
	button := chromeButton(c, symbol, insertTableLabel, paneAction(enabled, func() {
		app.tableGrid = workspaceTableGrid{open: !app.tableGrid.open, focus: true}
	})).KeepFocus().Tooltip("Insert table - sweep to size").HasPopup(mygoui.PopupGrid)
	app.drawTableGrid(c, button, enabled)
	return button.Expanded(app.tableGrid.open)
}

func (app *workspaceWindow) drawTableGrid(c *mygoui.Context, anchor mygoui.Element, enabled bool) {
	picker := &app.tableGrid
	if !enabled {
		*picker = workspaceTableGrid{}
		return
	}
	if !picker.open {
		return
	}
	var grid mygoui.Element
	theme := c.Theme()
	mygoui.PopoverBase(c, anchor, &picker.open, func(panel mygoui.Element) {
		panel.Label("Insert Table").AttachTo(anchor, mygoui.AnchorBottom, mygoui.AnchorTop).
			Margin(4, 0, 0).Padding(tableGridPadding).Radius(theme.Radius+2).
			Background(theme.Surface).Shadow(0, 6, 20, 0, mygoui.RGBA(0, 0, 0, .18))
		mygoui.Column(c).Width(tableGridWidth).Gap(tableGridCaptionGap).Children(func() {
			grid = app.tableDimensions(c)
		})
	})
	anchor.Controls(grid)
	if picker.open && picker.focus {
		// The overlay remembers the opener before moving focus into the grid.
		grid.Focus()
		picker.focus = false
	} else if grid.FocusLeft() {
		picker.open = false
	}
}

func (app *workspaceWindow) tableDimensions(c *mygoui.Context) mygoui.Element {
	picker := &app.tableGrid
	grid := mygoui.Column(c).Size(tableGridWidth, tableGridHeight).Gap(tableGridCellGap).
		Role(mygoui.RoleGrid).Label("Table dimensions").RowCount(tableGridRows).ColumnCount(tableGridColumns).TabStop(false)
	// Retain the last hovered cell on exit, without letting a stationary
	// pointer overwrite a keyboard choice made in the same cell.
	x, y, over := grid.PointerPosition()
	pointerRow, pointerColumn := 0, 0
	if over && x >= 0 && y >= 0 && x < tableGridWidth && y < tableGridHeight {
		const pitch = tableGridCellSize + tableGridCellGap
		column, row := int(x)/pitch, int(y)/pitch
		if x-float32(column*pitch) < tableGridCellSize && y-float32(row*pitch) < tableGridCellSize {
			pointerRow, pointerColumn = row+1, column+1
		}
	}
	if pointerRow != 0 && (pointerRow != picker.pointerRow || pointerColumn != picker.pointerColumn) {
		picker.row, picker.column = pointerRow, pointerColumn
	}
	picker.pointerRow, picker.pointerColumn = pointerRow, pointerColumn

	activeRow, activeCol := max(1, picker.row), max(1, picker.column)
	move := func(dr, dc int) {
		picker.row = max(1, min(tableGridRows, activeRow+dr))
		picker.column = max(1, min(tableGridColumns, activeCol+dc))
	}
	grid.OnShortcut(0, mygoui.KeyUp, func() { move(-1, 0) }).
		OnShortcut(0, mygoui.KeyDown, func() { move(1, 0) }).
		OnShortcut(0, mygoui.KeyLeft, func() { move(0, -1) }).
		OnShortcut(0, mygoui.KeyRight, func() { move(0, 1) }).
		OnShortcut(0, mygoui.KeyEnter, func() { app.insertGridTable(activeRow, activeCol) }).
		OnShortcut(0, mygoui.KeySpace, func() { app.insertGridTable(activeRow, activeCol) })

	fill, border, accent, ink := app.tableGridColors(c)
	grid.Children(func() {
		for row := 1; row <= tableGridRows; row++ {
			mygoui.Row(c.Key(row)).Gap(tableGridCellGap).Role(mygoui.RoleRow).RowIndex(row).Children(func() {
				for col := 1; col <= tableGridColumns; col++ {
					cell := mygoui.Box(c.Key(col)).Size(tableGridCellSize, tableGridCellSize).Radius(2).
						Role(mygoui.RoleCell).RowIndex(row).ColumnIndex(col).Label(fmt.Sprintf("%d × %d", row, col)).
						KeepFocus().Cursor(mygoui.CursorPointer).OnClick(func() { app.insertGridTable(row, col) })
					if row <= picker.row && col <= picker.column {
						cell.Background(accent.Alpha(.55)).Border(1, accent)
					} else {
						cell.Background(fill).Border(1, border)
					}
					cell.AccessSelected(row == picker.row && col == picker.column)
					if row == activeRow && col == activeCol {
						grid.ActiveDescendant(cell)
					}
				}
			})
		}
	})
	caption := "Sweep to size, click to insert"
	if picker.row != 0 && picker.column != 0 {
		caption = fmt.Sprintf("%d × %d  (first row = header)", picker.row, picker.column)
	}
	mygoui.Text(c, caption).Role(mygoui.RoleStatus).FillWidth().Height(tableGridCaptionH).
		FontSize(10).FixedLineHeight(tableGridCaptionH).TextAlign(mygoui.Center).TextColor(ink).SingleLine()
	return grid
}

func (app *workspaceWindow) tableGridColors(c *mygoui.Context) (fill, border, accent, ink mygoui.Color) {
	border, accent = mygoui.Hex("#45474b").Alpha(.4), blackoutTitleAccent
	mode := app.appAppearance()
	switch mode {
	case appearanceBlackout:
		return mygoui.Hex("#18181c"), border, accent, mygoui.Hex("#c4c6d0")
	case appearanceCyber:
		return mygoui.Hex("#262a31"), border, accent, mygoui.Hex("#c4c6d0")
	case appearancePLATO:
		return mygoui.Hex("#2a1809"), mygoui.Hex("#4a3319").Alpha(.4), mygoui.Hex("#ffc46b"), mygoui.Hex("#c79a5c")
	}
	if mode == appearanceDark || mode != appearanceLight && c.Theme().Dark {
		return mygoui.RGB(46, 46, 48), border, accent, mygoui.RGB(199, 199, 199)
	}
	return mygoui.RGB(235, 235, 237), mygoui.RGB(89, 89, 89).Alpha(.4), mygoui.Hex("#3a5ea3"), mygoui.RGB(82, 82, 82)
}

func (app *workspaceWindow) insertGridTable(rows, columns int) {
	app.tableGrid = workspaceTableGrid{}
	markdown := wysiwyg.TableGridMarkdown(rows, columns)
	if app.displayedMode() == wysiwygMode {
		if !app.wys.insertVisualTable(app, markdown, false) {
			app.wys.openTableTemplate(app, markdown)
		}
		return
	}
	app.insertSourceTable(markdown, len(markdown))
}
