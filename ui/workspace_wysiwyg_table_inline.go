package ui

import (
	"fmt"
	"strings"

	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

type visualTableFocus struct {
	id               string
	offset, row, col int
}

type tableCellSelection uint8

const (
	selectCellStart tableCellSelection = iota
	selectCellText
	selectCellEnd
)

type tableFocusRequest struct {
	visualTableFocus
	selection tableCellSelection
}

type visualTableCell struct {
	visualTableFocus
	table    *wysiwyg.Table
	document *richtext.Document
	version  uint64
	source   string
	align    parser.Align
	indented bool
}

type visualTableRows struct {
	table       *wysiwyg.Table
	columnWidth float32
	font        mygoui.Font
	heights     []float32
	boundaries  []float32
}

const (
	wysTableBorder = float32(1)
	wysTableCellX  = float32(12.5)
	wysTableCellY  = float32(10)
	wysTableMargin = float32(12)
)

func visualCellText(source string) richtext.Text {
	if source == "<br />" {
		return richtext.Text{}
	}
	// Reuse the table inline parser, including pipe escapes in code spans
	// and punctuation that would start a block in ordinary Markdown.
	const header = "Cell\n"
	text := richtext.ParseMarkdown("| Cell |\n| --- |\n| " + source + " |")
	if !strings.HasPrefix(text.String(), header) {
		return richtext.NewText(source)
	}
	return text.Slice(len(header), text.Len())
}

func visualCellMarkdown(text richtext.Text) string {
	parts := make([]string, 0, text.ParagraphCount())
	for _, paragraph := range text.Paragraphs() {
		paragraph.Style = richtext.ParagraphStyle{}
		parts = append(parts, richtext.TextOf(paragraph).Markdown())
	}
	if value := strings.Join(parts, "<br />"); value != "" {
		return value
	}
	return "<br />"
}

func (w *workspaceWYSIWYG) syncVisualTableCell(app *workspaceWindow, cell *visualTableCell) {
	if !w.editable(app) || cell.document.Version() == cell.version {
		return
	}
	table, ok := w.session.TableAt(cell.offset)
	if !ok || table.ID() != cell.id || table.Cell(cell.row, cell.col) != cell.source {
		return
	}
	text := cell.document.Text()
	if cell.indented {
		raw := text.String()
		leading := len(raw) - len(strings.TrimLeft(raw, " "))
		trailing := len(raw) - len(strings.TrimRight(raw, " "))
		text = text.Slice(leading, max(leading, text.Len()-trailing))
	}
	value := visualCellMarkdown(text)
	updated, err := w.session.ChangeTableCell(cell.offset, table, cell.row, cell.col, value)
	if err != nil {
		app.problem = err.Error()
		return
	}
	cell.version = cell.document.Version()
	cell.source, cell.table = updated.Cell(cell.row, cell.col), updated
	w.session.Sync(app.draft)
}

func (w *workspaceWYSIWYG) tableCellKey(app *workspaceWindow, cell *visualTableCell, direction int) {
	w.syncVisualTableCell(app, cell)
	columns := cell.table.ColumnCount()
	row, col := cell.row, cell.col
	selection := w.tableSelection
	if selection.id == cell.id {
		row, col = selection.last(cell.table)
		w.tableSelection = visualTableSelection{}
	}
	next := row*columns + col + direction
	if next >= cell.table.RowCount()*columns {
		// The bundled editor's final-cell Tab falls through to indentation.
		// Spaces at the cell boundary are presentation state in a GFM table.
		if row == cell.row && col == cell.col {
			cell.indented = true
			cell.document.InsertText("    ")
			w.syncVisualTableCell(app, cell)
		} else {
			w.focusTableCell(cell.table, cell.offset, row, col, selectCellEnd)
		}
		return
	}
	if next >= 0 {
		w.focusTableCell(cell.table, cell.offset, next/columns, next%columns, selectCellText)
	} else if selection.id == cell.id {
		w.focusTableCell(cell.table, cell.offset, row, col, selectCellStart)
	}
}

func (w *workspaceWYSIWYG) drawVisualTable(c *mygoui.Context, app *workspaceWindow, padding, viewportWidth float32,
	box richtext.ParagraphBox, table *wysiwyg.Table, pointer mygoui.Point, pointerOver bool, previousHover visualTableHandles) {
	if viewportWidth <= 2*padding {
		return
	}
	// EV-UI-142: collapsed 1 px rules, equal columns, 12 px horizontal
	// padding, and 10 px vertical padding including the cell's paragraph.
	width := viewportWidth - 2*padding
	rows, columns := table.RowCount(), table.ColumnCount()
	columnWidth := (width - wysTableBorder) / float32(columns)
	docTheme := app.resolvePreviewTheme(app.previewThemeSelection())
	family, _, _ := strings.Cut(docTheme.BodyFontFamily, ",")
	family = strings.Trim(family, " '\"")
	loadBundledFont(family)
	font := mygoui.Font{Family: family + ", sans-serif", Size: float32(w.fontSize()), LineHeight: 1.5, Antialiased: true}
	ink := mygoui.Hex(docTheme.BodyTextColor)
	accent := mygoui.Hex(docTheme.LinkColor)
	if app.blackout {
		// The Crepe chrome accent is separate from the document link color.
		accent = mygoui.Hex("#a9c7ff")
	}
	selection := accent.Alpha(.08)
	grid := mygoui.RGB(127, 127, 127)
	cellTheme := &richtext.Theme{Text: ink, Link: mygoui.Hex(docTheme.LinkColor), Selection: selection, Caret: ink,
		Padding: [2]float32{wysTableCellX, wysTableCellY}}
	// MyGo's ancestor input handler would otherwise take a press before
	// a cell sees it. This layer leaves presses and wheel events to the
	// cell or the scrolling rich-text host respectively.
	group := mygoui.Column(c.Key(table.ID()).Key(box.Index)).Absolute().Left(padding).Right(padding).Top(box.Top + wysTableMargin).
		Padding(.5).Role(mygoui.RoleTable).Label("Markdown table").RowCount(rows).ColumnCount(columns).
		HandleInput(func(mygoui.InputEvent) bool { return false })
	layout := mygoui.Local(group, "table rows", func() visualTableRows { return visualTableRows{} })
	if layout.table != table || layout.columnWidth != columnWidth || layout.font != font {
		*layout = visualTableRows{table: table, columnWidth: columnWidth, font: font,
			heights: make([]float32, rows), boundaries: make([]float32, rows+1)}
		layout.boundaries[0] = .5
		for row := range rows {
			lines := 1
			measureFont := font
			if row == 0 {
				measureFont.Weight = 600
			}
			for col := range columns {
				text := visualCellText(table.Cell(row, col)).String()
				layout := mygoui.ShapeText(text, measureFont, max(1, columnWidth-2*wysTableCellX))
				lines = max(lines, len(layout.Lines()))
			}
			layout.heights[row] = float32(lines)*font.Size*font.LineHeight + 2*wysTableCellY + wysTableBorder
			layout.boundaries[row+1] = layout.boundaries[row] + layout.heights[row]
		}
	}
	rowHeights := layout.heights
	height := layout.boundaries[rows] + .5
	group.Height(height)
	geometry := visualTableGeometry{frame: mygoui.Rect{X: padding, Y: box.Top + wysTableMargin, W: width, H: height},
		rows: layout.boundaries, columnWidth: columnWidth, columns: columns}
	visibleHandles := w.updateTableHover(table, geometry, pointer, pointerOver, previousHover)
	if reserved := height + 2*wysTableMargin; table.Height() != reserved {
		table.ReserveHeight(reserved)
		w.tableLayout = !w.tableLayout
		c.Invalidate()
	}
	group.Children(func() {
		for row, rowHeight := range rowHeights {
			label, role, fill := fmt.Sprintf("Row %d", row), mygoui.RoleCell, mygoui.Transparent
			cellFont := font
			switch {
			case row == 0:
				label, role, fill = "Header", mygoui.RoleColumnHeader, mygoui.Hex(docTheme.TableHeaderBackground)
				cellFont.Weight = 600
			case row%2 == 1:
				fill = grid.Alpha(.06)
			}
			rowSelected := w.tableSelection.matches(table.ID(), tableRows, row)
			mygoui.Row(c.Key(row)).Height(rowHeight).Shrink(0).FillWidth().Background(fill).
				Role(mygoui.RoleRow).Label(label).RowIndex(row + 1).AccessSelected(rowSelected).Children(func() {
				for col := range columns {
					selected := rowSelected || w.tableSelection.matches(table.ID(), tableColumns, col)
					mygoui.Column(c.Key(col)).Width(columnWidth).Height(rowHeight).Shrink(0).
						Role(role).RowIndex(row + 1).ColumnIndex(col + 1).AccessSelected(selected).Children(func() {
						if selected {
							mygoui.Box(c.Key("cell selection")).Absolute().Left(0).Right(0).Top(0).Bottom(0).
								Background(selection).IgnoreInput(true).Role(mygoui.RoleNone)
						}
						w.drawVisualTableCell(c, app, table, box.Start, row, col, label, cellFont, cellTheme)
					})
				}
			})
		}
		mygoui.Box(c.Key("table overlay")).Absolute().Left(0).Right(0).Top(0).Bottom(0).
			Role(mygoui.RoleNone).IgnoreInput(true).Draw(func(p *mygoui.Painter, rect mygoui.Rect) {
			paintTableGrid(p, rect, geometry, grid.Alpha(.4))
			w.paintTableControlLine(p, rect, table.ID(), geometry, visibleHandles, pointer, accent)
			paintTableDragPreview(p, rect, geometry, w.tableDrag, table, font, ink, accent)
		})
		w.drawVisualTableControls(c, app, table, box.Start, geometry, visibleHandles)
	})
}

func paintTableGrid(p *mygoui.Painter, rect mygoui.Rect, geometry visualTableGeometry, color mygoui.Color) {
	p.Stroke(mygoui.Rect{X: rect.X + .5, Y: rect.Y + .5, W: rect.W - 1, H: rect.H - 1}, color, 0, wysTableBorder)
	for _, edge := range geometry.rows[1 : len(geometry.rows)-1] {
		p.Fill(mygoui.Rect{X: rect.X, Y: rect.Y + edge - .5, W: rect.W, H: wysTableBorder}, color, 0)
	}
	for col := 1; col < geometry.columns; col++ {
		x := rect.X + geometry.columnWidth*float32(col)
		p.Fill(mygoui.Rect{X: x, Y: rect.Y, W: wysTableBorder, H: rect.H}, color, 0)
	}
}

func (w *workspaceWYSIWYG) drawVisualTableCell(c *mygoui.Context, app *workspaceWindow, table *wysiwyg.Table,
	offset, row, col int, label string, font mygoui.Font, theme *richtext.Theme) {
	name := fmt.Sprintf("%s, column %d", label, col+1)
	host := mygoui.Box(c).Absolute().Top(0).Bottom(0).Left(0).Right(0).PassThrough()
	cell := mygoui.Local(host, "cell", func() visualTableCell { return visualTableCell{} })
	cell.visualTableFocus = visualTableFocus{table.ID(), offset, row, col}
	cell.table = table
	source := table.Cell(row, col)
	if align := table.Align(col); cell.document == nil || source != cell.source || align != cell.align {
		text := visualCellText(source)
		textAlign := mygoui.Start
		switch align {
		case parser.AlignCenter:
			textAlign = mygoui.Center
		case parser.AlignRight:
			textAlign = mygoui.End
		}
		text = text.FormatParagraphs(0, text.Len(), func(style *richtext.ParagraphStyle) { style.Align = textAlign })
		cell.document = richtext.NewDocument(text)
		cell.source, cell.version, cell.align = source, cell.document.Version(), align
		cell.indented = false
	}
	// Preserve the whole-cell hit area while keeping the line at the
	// measured paragraph inset. RichText takes care of selection and IME.
	host.Children(func() {
		w.drawTableField(c, app, cell, name, font, theme)
	})
}

func (w *workspaceWYSIWYG) drawTableField(c *mygoui.Context, app *workspaceWindow, cell *visualTableCell,
	name string, font mygoui.Font, theme *richtext.Theme) {
	field := richtext.Editor(c.Key("cell editor"), cell.document, richtext.EditorOptions{
		Label: name, Font: font, Theme: theme, NoMarkdown: true, ReadOnly: app.modalOpen,
		OnLink: func(url string) { w.followLink(app, url) },
		OnLinkCommand: func() {
			w.focusedCell = cell
			w.format(app, linkFormat)
		},
	}).
		Fill().MinHeight(0).MinWidth(0).Radius(0).Border(0, mygoui.Transparent).Background(mygoui.Transparent).
		OnChange(func() { w.syncVisualTableCell(app, cell) }).ObserveInput(func(ev mygoui.InputEvent) {
		// Native Edit-menu actions arrive as commands. The main document
		// owns the history, including edits to other cells and paragraphs.
		if ev.Kind == mygoui.InputCommand && (ev.Text == "undo" || ev.Text == "redo") && w.editable(app) {
			// Clear the cell history so the command has no second effect
			// when it reaches this embedded editor's default handler.
			anchor, caret := cell.document.Selection()
			cell.document.SetText(cell.document.Text())
			cell.document.SetSelection(anchor, caret)
			cell.version = cell.document.Version()
			w.undoVisualTable(app, ev.Text == "redo")
		}
	})
	field.ContextMenu(func(m *mygoui.Menu) {
		m.EditItems()
		w.visualTableContextMenu(m, app, cell)
	})
	if cell.row != 0 || cell.col != 0 || w.tableSelection.id == cell.id {
		field.OnShortcut(mygoui.Shift, mygoui.KeyTab, func() { w.tableCellKey(app, cell, -1) })
	}
	field.OnShortcut(0, mygoui.KeyTab, func() { w.tableCellKey(app, cell, 1) }).
		OnShortcut(mygoui.Cmd, mygoui.KeyBackquote, func() { w.format(app, inlineCodeFormat) }).
		OnShortcut(mygoui.Cmd, mygoui.KeyZ, func() { w.undoVisualTable(app, false) }).
		OnShortcut(mygoui.Cmd|mygoui.Shift, mygoui.KeyZ, func() { w.undoVisualTable(app, true) }).
		OnShortcut(0, mygoui.KeyEnter, func() { w.exitVisualTable(app, cell) })
	if w.tableSelection.id == cell.id {
		field.OnShortcut(0, mygoui.KeyRight, func() { w.collapseTableSelection(cell.table, cell.offset) })
		if w.tableMenu.id == cell.id {
			field.OnShortcut(0, mygoui.KeyEscape, func() { w.tableMenu = visualTableSelection{} })
		}
	}
	if request := w.tableFocus; request != nil && request.visualTableFocus == cell.visualTableFocus && !app.modalOpen {
		end := cell.document.Text().Len()
		switch request.selection {
		case selectCellText:
			cell.document.SetSelection(0, end)
		case selectCellEnd:
			cell.document.SetSelection(end, end)
		default:
			cell.document.SetSelection(0, 0)
		}
		field.Focus()
		w.tableFocus = nil
	}
	if field.Focused() {
		w.focusedCell = cell
	}
}

func (w *workspaceWYSIWYG) focusTableCell(table *wysiwyg.Table, offset, row, col int, selection tableCellSelection) {
	w.tableFocus = &tableFocusRequest{
		visualTableFocus: visualTableFocus{id: table.ID(), offset: offset, row: row, col: col},
		selection:        selection,
	}
	w.focus = false
}

func (w *workspaceWYSIWYG) exitVisualTable(app *workspaceWindow, cell *visualTableCell) {
	w.syncVisualTableCell(app, cell)
	w.closeTableControls()
	w.session.ExitTable(cell.offset)
	w.session.Sync(app.draft)
	w.focus = true
}

func (w *workspaceWYSIWYG) undoVisualTable(app *workspaceWindow, redo bool) {
	if !w.editable(app) {
		return
	}
	w.closeTableControls()
	if redo {
		w.session.Document.Redo()
	} else {
		w.session.Document.Undo()
	}
	if cell := w.focusedCell; cell != nil {
		if table, ok := w.session.TableAt(cell.offset); !ok || table.ID() != cell.id || cell.row >= table.RowCount() || cell.col >= table.ColumnCount() {
			// Undoing a table insertion removes the focused cell editor.
			// Keep subsequent history and typing in the body editor.
			w.focus = true
		}
	}
	w.session.Sync(app.draft)
}
