package ui

import (
	"fmt"

	"cyberwriter/ui/wysiwyg"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func (w *workspaceWYSIWYG) canInsertTable(app *workspaceWindow) bool {
	return app.mode == wysiwygMode && app.draft != nil && !app.modalOpen
}

func (w *workspaceWYSIWYG) openNewTable(app *workspaceWindow) {
	if !w.canInsertTable(app) {
		return
	}
	w.sync(app)
	session := w.session
	if session == nil || !session.Current(app.draft) {
		session = wysiwyg.Open(app.draft)
		w.session = session
	}
	if session.Document != nil {
		if offset, table := session.InsertTable(); table != nil {
			w.closeTableControls()
			w.focusTableCell(table, offset, 0, 0, selectCellStart)
			w.sync(app)
		}
		return
	}
	openTableEditor(app, session.NewTable(), "Insert Table at End")
}

func (w *workspaceWYSIWYG) openTableTemplate(app *workspaceWindow, markdown string) {
	if !w.canInsertTable(app) || w.session == nil || !w.session.Current(app.draft) {
		return
	}
	template := wysiwyg.Open(editor.NewBuffer(markdown)).EditTable(0)
	if template == nil {
		return
	}
	openTableEditorWithRows(app, w.session.NewTable(), "Insert Table at End", template.Rows())
}

func (w *workspaceWYSIWYG) insertVisualTable(app *workspaceWindow, markdown string, focusLast bool) bool {
	if !w.editable(app) {
		return false
	}
	w.flush(app)
	offset, table := w.session.InsertMarkdownTable(markdown)
	if table == nil {
		return false
	}
	w.closeTableControls()
	if focusLast {
		w.focusTableCell(table, offset, table.RowCount()-1, table.ColumnCount()-1, selectCellEnd)
	} else {
		w.focusTableCell(table, offset, 0, 0, selectCellStart)
	}
	w.sync(app)
	return true
}

func (w *workspaceWYSIWYG) tablePicker(c *mygoui.Context, app *workspaceWindow) {
	count := w.session.TableCount()
	open := func(index int) {
		if w.canInsertTable(app) && w.session.Current(app.draft) {
			if table := w.session.EditTable(index); table != nil {
				openTableEditor(app, table, "Edit Table")
			}
		}
	}
	switch count {
	case 0:
	case 1:
		mygoui.Button(c, "Edit Table").Disabled(app.modalOpen).OnClick(func() { open(0) })
	default:
		mygoui.MenuButton(c, "Edit Table", func(menu *mygoui.Menu) {
			for i := range count {
				if menu.Item(fmt.Sprintf("Table %d", i+1)).Chosen() {
					open(i)
				}
			}
		}).Disabled(app.modalOpen)
	}
}

type workspaceTableEditor struct {
	app     *workspaceWindow
	win     *mygo.Window
	edit    *wysiwyg.TableEdit
	title   string
	cells   [][]*mygoui.TextBuffer
	focus   *mygoui.TextBuffer
	save    bool
	problem string
}

func openTableEditor(app *workspaceWindow, edit *wysiwyg.TableEdit, title string) {
	openTableEditorWithRows(app, edit, title, edit.Rows())
}

func openTableEditorWithRows(app *workspaceWindow, edit *wysiwyg.TableEdit, title string, rows [][]string) {
	form := &workspaceTableEditor{app: app, edit: edit, title: title}
	for _, row := range rows {
		cells := make([]*mygoui.TextBuffer, len(row))
		for i, value := range row {
			cells[i] = mygoui.NewTextBuffer(value)
		}
		form.cells = append(form.cells, cells)
	}
	form.focus = form.cells[0][0]
	tab := app.workspaceTab
	app.modalOpen = true
	app.tableEditor = form
	form.win = mygo.NewWindow(mygo.WindowOptions{
		Title: title, Width: 620, Height: 410, MinWidth: 460, MinHeight: 280,
		Parent: app.win, Modal: true, DisableMinimize: true, DisableMaximize: true,
		Content: mygoui.View(form.draw),
	})
	form.win.OnClosed(func() {
		app.modalOpen = false
		app.tableEditor = nil
		app.win.Update(func() {
			if form.save && app.workspaceTab == tab {
				app.save()
			}
		})
	})
	app.win.Invalidate()
}

func (app *workspaceWindow) saveFromMenu(_ *mygo.MenuItem, _ *mygo.Window) {
	app.win.Update(func() {
		if app.tableEditor != nil {
			app.tableEditor.apply(true)
		} else if !app.modalOpen {
			app.save()
		}
	})
}

func (f *workspaceTableEditor) apply(save bool) {
	if f.win.IsDestroyed() || f.app.tableEditor != f {
		return
	}
	rows := make([][]string, len(f.cells))
	for r, cells := range f.cells {
		rows[r] = make([]string, len(cells))
		for c, cell := range cells {
			rows[r][c] = cell.String()
		}
	}
	if err := f.edit.Apply(f.app.draft, rows); err != nil {
		f.problem = err.Error()
		return
	}
	f.save = save
	f.win.Close()
}

func (f *workspaceTableEditor) addRow() {
	row := make([]*mygoui.TextBuffer, len(f.cells[0]))
	for i := range row {
		row[i] = mygoui.NewTextBuffer("")
	}
	f.cells = append(f.cells, row)
	f.focus = row[0]
}

func (f *workspaceTableEditor) addColumn() {
	for i, row := range f.cells {
		f.cells[i] = append(row, mygoui.NewTextBuffer(""))
	}
	f.focus = f.cells[0][len(f.cells[0])-1]
}

// EV-EDITOR-004 records table structure and the move to the next cell
// with Tab. Text inputs use the same row-major focus order in this form.
func (f *workspaceTableEditor) draw(c *mygoui.Context) {
	f.app.applyAppearance(c)
	c.OnWindowShortcut(0, mygoui.KeyEscape, func() { f.win.Close() })
	c.OnWindowShortcut(mygoui.Cmd, mygoui.KeyS, func() { f.apply(true) })
	mygoui.Column(c).Fill().Padding(16).Gap(10).Background(c.Theme().Background).Children(func() {
		mygoui.Text(c, f.title).Role(mygoui.RoleHeading).FontSize(18).Bold()
		f.drawCells(c)
		if f.problem != "" {
			mygoui.Text(c, f.problem).Role(mygoui.RoleAlert).TextColor(c.Theme().Danger).Wrap()
		}
		mygoui.Row(c).Role(mygoui.RoleToolbar).Label("Table editing commands").FillWidth().Gap(6).Wrap().Children(func() {
			mygoui.Button(c, "Add Row").OnClick(f.addRow)
			mygoui.Button(c, "Add Column").OnClick(f.addColumn)
			mygoui.Spacer(c)
			mygoui.Button(c, "Cancel").OnClick(func() { f.win.Close() })
			mygoui.Button(c, "Apply").OnClick(func() { f.apply(false) })
			mygoui.Button(c, "Apply and Save").OnClick(func() { f.apply(true) })
		})
	})
}

func (f *workspaceTableEditor) drawCells(c *mygoui.Context) {
	width, _ := c.Size()
	columns := len(f.cells[0])
	const gap = 6
	cellWidth := max(136, (width-46-gap*float32(columns-1))/float32(columns))
	mygoui.ScrollBoth(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
		mygoui.Column(c).Role(mygoui.RoleTable).Label("Table cell Markdown").Padding(4).Gap(gap).
			Width((cellWidth+gap)*float32(columns) + 8 - gap).Children(func() {
			for r, cells := range f.cells {
				rowLabel := fmt.Sprintf("Row %d", r)
				role := mygoui.RoleCell
				if r == 0 {
					rowLabel = "Header"
					role = mygoui.RoleColumnHeader
				}
				mygoui.Row(c.Key(cells[0])).Role(mygoui.RoleRow).Label(rowLabel).Gap(gap).Children(func() {
					for col, cell := range cells {
						mygoui.Column(c.Key(cell)).Role(role).Width(cellWidth).Shrink(0).Children(func() {
							input := mygoui.TextInputBuffer(c, cell).FillWidth().Label(
								fmt.Sprintf("%s, column %d", rowLabel, col+1))
							if r == 0 {
								input.Bold().Placeholder(fmt.Sprintf("Column %d", col+1))
							}
							switch f.edit.Align(col) {
							case parser.AlignCenter:
								input.TextAlign(mygoui.Center)
							case parser.AlignRight:
								input.TextAlign(mygoui.End)
							}
							if f.focus == cell {
								input.Focus().ScrollIntoView()
								f.focus = nil
							}
						})
					}
				})
			}
		})
	})
}
