package ui

import (
	"sort"
	"strings"
	"time"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

const queryPaddingX, queryPaddingY = 15, 13
const queryMargin = 12

type queryCardKey struct {
	offset  int
	outline bool
}

type workspaceQueryEdit struct {
	query  *wysiwyg.QueryBlock
	offset int
	source string
	focus  bool
	height float32
	resize *fencedSourceResize
	grip   mygoui.Handle
}

type visualQueryResult struct {
	hasSnapshot bool
	result      vaultindex.QueryResult
	err         error
	width       float32
	font        mygoui.Font
	edges       []float32
}

func (w *workspaceWYSIWYG) finishQueryEdit(app *workspaceWindow, commit, focus bool) {
	edit := w.queryEdit
	if edit.query == nil {
		return
	}
	w.queryEdit = workspaceQueryEdit{}
	delete(w.queryResults, edit.query)
	if commit && w.session != nil && w.session.Current(app.draft) &&
		w.session.ChangeQuery(edit.offset, edit.query, edit.source) {
		w.session.Sync(app.draft)
	}
	w.focus = focus
}

func (w *workspaceWYSIWYG) retainQueryResults() {
	doc := w.session.Document
	version := doc.Version()
	if w.queryVersion == version {
		return
	}
	w.queryVersion = version
	if len(w.queryResults) == 0 {
		return
	}
	text := doc.Text()
	active := make(map[*wysiwyg.QueryBlock]*visualQueryResult, len(w.queryResults))
	for offset := range text.Paragraphs() {
		if query, ok := w.session.QueryAt(offset); ok && w.queryResults[query] != nil {
			active[query] = w.queryResults[query]
		}
	}
	w.queryResults = active
}

func (w *workspaceWYSIWYG) queryResult(query *wysiwyg.QueryBlock, index *vaultindex.Index) *visualQueryResult {
	result := w.queryResults[query]
	// An open card keeps its snapshot. Adopt the first index publication
	// if the note opened before the initial scan had finished.
	if result == nil || !result.hasSnapshot && index != nil {
		rows, err := index.Query(query.Source)
		result = &visualQueryResult{hasSnapshot: index != nil, result: rows, err: err}
		if w.queryResults == nil {
			w.queryResults = make(map[*wysiwyg.QueryBlock]*visualQueryResult)
		}
		w.queryResults[query] = result
	}
	return result
}

// EV-UI-158: the source field replaces the result stage. The surrounding
// note keeps a single selectable attachment, regardless of result size.
func (w *workspaceWYSIWYG) drawQueryCard(c *mygoui.Context, app *workspaceWindow, box richtext.ParagraphBox, query *wysiwyg.QueryBlock, view visualImageFrame) {
	docTheme := app.resolvePreviewTheme(app.previewThemeSelection())
	canvas, ink, link := mygoui.Hex(docTheme.BackgroundColor), mygoui.Hex(docTheme.BodyTextColor), mygoui.Hex(docTheme.LinkColor)
	gray := mygoui.RGB(127, 127, 127)
	family, _, _ := strings.Cut(docTheme.BodyFontFamily, ",")
	family = strings.Trim(family, " '\"")
	loadBundledFont(family)
	font := mygoui.Font{Family: family + ", sans-serif", Size: view.font.Size, LineHeight: 1.5}

	width := view.viewport.W - 2*view.padding
	stageWidth := max(1, width-2*queryPaddingX)
	text := w.session.Document.Text()
	previous := text.ParagraphStyleAt(max(0, box.Start-1))
	top := box.Top + queryMargin - 1
	if box.Start > 0 && previous.Heading == 0 && previous.List == richtext.NoList && !previous.CodeBlock {
		top -= view.font.Size * .75
	}
	card := mygoui.Box(c.Key(queryCardKey{offset: box.Start})).Absolute().Left(view.padding).Width(width).Top(top).
		Background(canvas.Mix(gray, .04)).Border(1, gray.Alpha(.2)).Radius(8).Clip().
		Role(mygoui.RoleGroup).Label(wysiwyg.QueryBlockLabel).
		HandleInput(func(mygoui.InputEvent) bool { return false })

	editing := w.queryEdit.query == query && w.queryEdit.offset == box.Start
	var height float32
	if editing {
		rows := min(30, max(3, strings.Count(query.Source, "\n")+2))
		inputHeight := float32(float64(rows)*1.4+1.2) * 12
		if w.queryEdit.height > 0 {
			inputHeight = w.queryEdit.height
		}
		hint := diagramSourceShortcutHint()
		hintFont := mygoui.Font{Family: font.Family, Size: 10, LineHeight: 1.3}
		hintHeight := float32(max(1, len(mygoui.ShapeText(hint, hintFont, stageWidth).Lines()))) * 13
		height = inputHeight + 8.296875 + hintHeight + 2*queryPaddingY
		mygoui.Box(c.Key(queryCardKey{offset: box.Start, outline: true})).Absolute().Left(view.padding-4).Width(width+8).
			Top(top-4).Height(height+8).Radius(12).Border(2, mygoui.RGBA(100, 150, 255, .4)).IgnoreInput(true).Role(mygoui.RoleNone)
		card.Children(func() { w.drawQuerySource(c, app, canvas, ink, gray, inputHeight, hintHeight, hint, font.Family) })
	} else {
		render := w.queryResult(query, app.index.data)
		if render.width != stageWidth || render.font != font {
			render.measure(stageWidth, font)
		}
		table := render.result.Columns != nil && len(render.result.Rows) > 0
		stageHeight := float32(24)
		if len(render.edges) > 0 {
			stageHeight = max(stageHeight, render.edges[len(render.edges)-1])
			if table {
				stageHeight += 1 + 2*queryMargin
			}
		}
		height = stageHeight + 2*queryPaddingY
		hovered := view.hovered && (mygoui.Rect{X: view.padding, Y: top, W: width, H: height}).Contains(view.pointer.X, view.pointer.Y)
		selected := view.start <= box.Start && box.Start < view.end
		card.AccessSelected(selected).Children(func() {
			if w.editable(app) {
				mygoui.Box(c.Key("select query")).Absolute().Left(0).Right(0).Top(0).Bottom(0).Role(mygoui.RoleNone).
					OnClick(func() {
						w.session.Document.SetSelection(box.Start, box.Start+1)
						w.focus = true
					})
			}
			switch {
			case render.err != nil:
				mygoui.Text(c, queryInvalid).Absolute().Top(queryPaddingY).Left(queryPaddingX).
					Font(font.Family).FontSize(13).FixedLineHeight(17).TextColor(mygoui.Hex("#e05252")).
					Role(mygoui.RoleAlert).IgnoreInput(true)
			case len(render.result.Rows) == 0:
				mygoui.Text(c, queryNoResults).Absolute().Top(queryPaddingY).Left(queryPaddingX).
					Font(font.Family).FontSize(13).FixedLineHeight(17).Italic().TextColor(ink.Alpha(.6)).
					Role(mygoui.RoleStatus).IgnoreInput(true)
			default:
				drawQueryResults(c, app, render, top, view.viewport.H, ink, link, gray, table)
			}
			button := mygoui.Button(c, "Edit source").Absolute().Top(7).Right(7).Size(72.6875, 21).
				Font(font.Family).FontSize(10).FontWeight(500).TextColor(ink).Padding(0).Radius(4).
				Background(canvas.Mix(ink, .14)).Border(1, ink.Alpha(.45)).KeepFocus().Disabled(!w.editable(app)).
				OnClick(func() {
					w.finishQueryEdit(app, true, false)
					w.finishDiagramEdit(app, true, false)
					w.finishAPIEdit(app, true, false)
					w.queryEdit = workspaceQueryEdit{query: query, offset: box.Start, source: query.Source, focus: true}
				})
			if button.Hovered() {
				button.Background(canvas.Mix(ink, .26))
			}
			opacity := float32(0)
			if hovered || selected || button.Focused() || c.Preferences().HighContrast {
				opacity = 1
			}
			button.Opacity(button.Animate("opacity", opacity, 150*time.Millisecond))
		})
	}
	card.Height(height)
	nextSpacing := float32(0)
	if next := box.Start + 2; next <= text.Len() {
		style := text.ParagraphStyleAt(next)
		if style.Heading == 0 && style.List == richtext.NoList && !style.CodeBlock {
			nextSpacing = view.font.Size * .75
		}
	}
	reserved := height + queryMargin + top - box.Top - view.font.Metrics().Descent - nextSpacing
	if query.Height() != reserved {
		query.ReserveHeight(reserved)
		w.queryMetricsChanged = true
		c.Invalidate()
	}
}

func (r *visualQueryResult) measure(width float32, font mygoui.Font) {
	r.width, r.font = width, font
	n := len(r.result.Rows)
	if n == 0 {
		return
	}
	table := r.result.Columns != nil
	header := 0
	if table {
		n++
		header = 1
		font.Size, font.LineHeight = 13, 17.0/13
		width = max(1, (width-1)/float32(len(r.result.Columns)+1)-25)
	}
	r.edges = make([]float32, n+1)
	for i := range n {
		var name string
		var values []string
		if table && i == 0 {
			name, values = queryFirstColumn, r.result.Columns
		} else {
			row := r.result.Rows[i-header]
			name, values = row.Name, row.Cells
		}
		height := font.Size * font.LineHeight
		if table {
			font.Weight = queryRowWeight(i)
		}
		height *= float32(max(1, len(mygoui.ShapeText(name, font, width).Lines())))
		for _, value := range values {
			height = max(height, 17*float32(len(mygoui.ShapeText(value, font, width).Lines())))
		}
		if table {
			height += 13
		}
		r.edges[i+1] = r.edges[i] + height
	}
}

func queryRowWeight(row int) int {
	if row < 2 {
		return 600
	}
	return 400
}

func drawQueryResults(c *mygoui.Context, app *workspaceWindow, render *visualQueryResult, top, viewportHeight float32,
	ink, link, gray mygoui.Color, table bool) {
	inset := float32(queryPaddingY)
	if table {
		inset += queryMargin
	}
	edges := render.edges
	rows := len(edges) - 1
	first := sort.Search(rows, func(i int) bool { return top+inset+edges[i+1] >= 0 })
	stage := mygoui.Box(c.Key("query results")).Absolute().Top(inset).Left(queryPaddingX).Width(render.width).
		Height(edges[rows]).PassThrough().Role(mygoui.RoleList).Label("Query results")
	if table {
		stage.Height(edges[rows] + 1).Role(mygoui.RoleTable).RowCount(rows).ColumnCount(len(render.result.Columns) + 1)
	}
	stage.Children(func() {
		for i := first; i < rows && top+inset+edges[i] < viewportHeight; i++ {
			if table {
				drawQueryTableRow(c, app, render, i, ink, link, gray)
				continue
			}
			row := render.result.Rows[i]
			height := edges[i+1] - edges[i]
			mygoui.Box(c.Key(row.Path)).Absolute().Top(edges[i]).FillWidth().Height(height).
				Role(mygoui.RoleListItem).PassThrough().Children(func() {
				mygoui.Box(c.Key("bullet")).Absolute().Left(-16).Top(0).Size(6, 22.5).IgnoreInput(true).Role(mygoui.RoleNone).
					Draw(func(p *mygoui.Painter, r mygoui.Rect) {
						p.Fill(mygoui.Rect{X: r.X, Y: r.Y + 9.25, W: 4.5, H: 4.5}, ink, 2.25)
					})
				drawQueryNoteLink(c, app, row, render.font, link, render.width, height)
			})
		}
	})
}

func drawQueryTableRow(c *mygoui.Context, app *workspaceWindow, render *visualQueryResult, i int, ink, link, gray mygoui.Color) {
	columns := render.result.Columns
	width := (render.width - 1) / float32(len(columns)+1)
	height := render.edges[i+1] - render.edges[i]
	font := render.font
	font.Size, font.Weight = 13, queryRowWeight(i)
	fill := gray.Alpha(.06)
	if i == 0 || i%2 == 1 {
		fill = gray.Alpha(.18)
	}
	mygoui.Row(c.Key(i)).Absolute().Top(.5 + render.edges[i]).Left(.5).Right(.5).Height(height).
		Background(fill).Role(mygoui.RoleRow).RowIndex(i + 1).PassThrough().Children(func() {
		var values []string
		var row vaultindex.QueryRow
		role := mygoui.RoleCell
		if i == 0 {
			row.Name, values, role = queryFirstColumn, columns, mygoui.RoleColumnHeader
		} else {
			row = render.result.Rows[i-1]
			values = row.Cells
		}
		for col := range len(columns) + 1 {
			value := row.Name
			if col != 0 {
				value = values[col-1]
			}
			cell := mygoui.Column(c.Key(col)).Size(width, height).Shrink(0).Padding(6, 12.5).Justify(mygoui.Center).
				Role(role).ColumnIndex(col + 1).RowIndex(i + 1).PassThrough()
			cell.Children(func() {
				if col == 0 && i != 0 {
					drawQueryNoteLink(c, app, row, font, link, max(1, width-25), height-12)
				} else {
					mygoui.Text(c, value).FillWidth().Wrap().Font(font.Family).FontSize(13).FontWeight(font.Weight).
						FixedLineHeight(17).TextColor(ink).IgnoreInput(true)
				}
			})
			cell.Draw(func(p *mygoui.Painter, r mygoui.Rect) {
				p.Line(r.X-.5, r.Y, r.X+r.W+.5, r.Y, 1, gray.Alpha(.4))
				p.Line(r.X, r.Y, r.X, r.Y+r.H, 1, gray.Alpha(.4))
				if col == len(columns) {
					p.Line(r.X+r.W, r.Y, r.X+r.W, r.Y+r.H, 1, gray.Alpha(.4))
				}
				if i == len(render.edges)-2 {
					p.Line(r.X-.5, r.Y+r.H, r.X+r.W+.5, r.Y+r.H, 1, gray.Alpha(.4))
				}
			})
		}
	})
}

func drawQueryNoteLink(c *mygoui.Context, app *workspaceWindow, row vaultindex.QueryRow, font mygoui.Font, ink mygoui.Color, width, height float32) {
	line := float32(17)
	if font.Size != 13 {
		line = font.Size * font.LineHeight
	}
	font.LineHeight = line / font.Size
	layout := mygoui.ShapeText(row.Name, font, width)
	nameWidth, _ := layout.Size()
	link := mygoui.Link(c, row.Name, "").Width(min(width, nameWidth+1)).MaxHeight(height).Wrap().
		Font(font.Family).FontSize(font.Size).FontWeight(font.Weight).FixedLineHeight(line).
		Underline().UnderlineOffset(2).TextColor(ink).Disabled(app.modalOpen).
		OnClick(func() { app.openNoteAt(row.Path, true, "") })
	border := mygoui.RGBA(37, 99, 235, .5)
	hover := link.Hovered()
	lines := layout.Lines()
	link.Draw(func(p *mygoui.Painter, r mygoui.Rect) {
		for _, line := range lines {
			end := r.X + min(r.W, line.Rect.W)
			y := r.Y + line.Rect.Y + line.Rect.H + .5
			if hover {
				p.Line(r.X, y, end, y, 1, border)
				continue
			}
			for x := r.X; x < end; x += 6 {
				p.Line(x, y, min(end, x+3), y, 1, border)
			}
		}
	})
	if hover {
		link.Background(mygoui.RGBA(37, 99, 235, .08))
	}
}

func (w *workspaceWYSIWYG) drawQuerySource(c *mygoui.Context, app *workspaceWindow, canvas, ink, gray mygoui.Color,
	inputHeight, hintHeight float32, hint, font string) {
	edit := &w.queryEdit
	mygoui.Column(c).Absolute().Top(queryPaddingY).Left(queryPaddingX - 1).Right(queryPaddingX - 1).Gap(8.296875).Children(func() {
		input := fencedSourceInput(c, &edit.source, &edit.focus, "Query source", inputHeight, func(commit, focus bool) {
			w.finishQueryEdit(app, commit, focus)
		}).FixedLineHeight(18).Padding(10).Radius(6).FocusRing(false).TextColor(ink).
			Background(canvas.Mix(gray, .08)).Border(1, gray.Alpha(.35))
		if input.Focused() {
			input.Background(canvas.Mix(gray, .04)).BorderColor(mygoui.RGBA(100, 150, 255, .6))
		}
		mygoui.Text(c, hint).FillWidth().Height(hintHeight).Font(font).FontSize(10).Wrap().
			FixedLineHeight(13).TextColor(gray.Alpha(.7)).IgnoreInput(true)
	})
	grip := mygoui.Box(c.Key("query source resize")).Absolute().Right(queryPaddingX).Top(queryPaddingY+inputHeight-12).
		Size(12, 12).KeepFocus().Cursor(mygoui.CursorResizeNS).Role(mygoui.RoleSplitter).Label("Resize query source").Bind(&edit.grip)
	drawFencedSourceGrip(c, grip, &edit.grip, &edit.resize, &edit.height, inputHeight, gray)
}
