package ui

import (
	"fmt"
	"strings"
	"time"

	"cyberwriter/internal/core/liveapi"
	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

type workspaceAPIEdit struct {
	block  *wysiwyg.APIBlock
	offset int
	source string
	focus  bool
	height float32
	resize *fencedSourceResize
	grip   mygoui.Handle
}

type apiCardKey struct {
	offset  int
	outline bool
}

func (w *workspaceWYSIWYG) finishAPIEdit(app *workspaceWindow, commit, focus bool) {
	edit := w.apiEdit
	if edit.block == nil {
		return
	}
	w.apiEdit = workspaceAPIEdit{}
	if commit && w.session != nil && w.session.Current(app.draft) &&
		w.session.ChangeAPI(edit.offset, edit.block, edit.source) {
		w.session.Sync(app.draft)
	}
	w.focus = focus
}

func (w *workspaceWYSIWYG) drawAPICard(c *mygoui.Context, app *workspaceWindow, box richtext.ParagraphBox, block *wysiwyg.APIBlock, view visualImageFrame) {
	width := view.viewport.W - 2*view.padding
	stageWidth := max(1, width-2*queryPaddingX)
	text := w.session.Document.Text()
	previous := text.ParagraphStyleAt(max(0, box.Start-1))
	top := box.Top + queryMargin - 1
	if box.Start > 0 && previous.Heading == 0 && previous.List == richtext.NoList && !previous.CodeBlock {
		top -= view.font.Size * .75
	}
	theme := app.resolvePreviewTheme(app.previewThemeSelection())
	family, _, _ := strings.Cut(theme.BodyFontFamily, ",")
	family = strings.Trim(family, " '\"")
	loadBundledFont(family)
	font := family + ", sans-serif"
	canvas, ink := mygoui.Hex(theme.BackgroundColor), mygoui.Hex(theme.BodyTextColor)
	gray := mygoui.RGB(127, 127, 127)
	card := mygoui.Box(c.Key(apiCardKey{offset: box.Start})).Absolute().Left(view.padding).Top(top).Width(width).
		Background(canvas.Mix(gray, .04)).Border(1, gray.Alpha(.2)).Radius(8).Clip().
		Role(mygoui.RoleGroup).Label("API Request").
		HandleInput(func(mygoui.InputEvent) bool { return false })
	if block.Result {
		card.Label("API Result")
	}
	selected := view.start <= box.Start && box.Start < view.end
	editing := w.apiEdit.block == block && w.apiEdit.offset == box.Start
	height := float32(50)
	if editing {
		rows := min(30, max(3, strings.Count(block.Source, "\n")+2))
		inputHeight := float32(float64(rows)*1.4+1.2) * 12
		if w.apiEdit.height > 0 {
			inputHeight = w.apiEdit.height
		}
		hint := diagramSourceShortcutHint()
		hintHeight := float32(max(1, len(mygoui.ShapeText(hint, mygoui.Font{Family: font, Size: 10, LineHeight: 1.3}, stageWidth).Lines()))) * 13
		height = inputHeight + 8.296875 + hintHeight + 2*queryPaddingY
		mygoui.Box(c.Key(apiCardKey{offset: box.Start, outline: true})).Absolute().Left(view.padding-4).Width(width+8).
			Top(top-4).Height(height+8).Radius(12).Border(2, mygoui.RGBA(100, 150, 255, .4)).IgnoreInput(true).Role(mygoui.RoleNone)
		card.Children(func() { w.drawAPISource(c, app, canvas, ink, gray, inputHeight, hintHeight, font, hint) })
	} else {
		frame := mygoui.Rect{X: view.padding, Y: top, W: width, H: height}
		card.AccessSelected(selected).Children(func() {
			if w.editable(app) {
				mygoui.Box(c.Key("select api block")).Absolute().Left(0).Right(0).Top(0).Bottom(0).Role(mygoui.RoleNone).
					OnClick(func() {
						w.session.Document.SetSelection(box.Start, box.Start+1)
						w.focus = true
					})
			}
			if block.Result {
				presentation := mygoui.Local(card, "API response", func() apiResultPresentation { return apiResultPresentation{} })
				if presentation.block != block {
					result := liveapi.ParseResult(block.Source)
					options := w.apiPreviousOptions(box.Start)
					if options.Format == liveapi.FormatRaw {
						result.Format = liveapi.FormatRaw
					}
					*presentation = newAPIResultPresentation(block, result, options)
				}
				height = w.drawAPIResult(c, presentation, stageWidth, theme, font)
			} else {
				height = drawAPIRequestHeader(c, app, block, box.Start, stageWidth, font, ink)
			}
			frame.H = height
			reveal := selected || view.hovered && frame.Contains(view.pointer.X, view.pointer.Y)
			right := float32(75)
			if block.Result {
				right = 7
			}
			button := mygoui.Button(c, "Edit source").Absolute().Top(6).Right(right-1).Size(72.6875, 21).
				Font(font).FontSize(10).FontWeight(500).TextColor(ink).Padding(0).Radius(4).
				Background(canvas.Mix(ink, .14)).Border(1, ink.Alpha(.45)).KeepFocus().Disabled(!w.editable(app)).
				OnClick(func() {
					w.finishAPIEdit(app, true, false)
					w.finishQueryEdit(app, true, false)
					w.finishDiagramEdit(app, true, false)
					w.apiEdit = workspaceAPIEdit{block: block, offset: box.Start, source: block.Source, focus: true}
				})
			opacity := float32(0)
			if reveal || button.Hovered() || button.Focused() || c.Preferences().HighContrast {
				opacity = 1
			}
			if button.Hovered() {
				button.Background(canvas.Mix(ink, .26))
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
	reserve := height + queryMargin + top - box.Top - view.font.Metrics().Descent - nextSpacing
	if block.Height() != reserve {
		block.ReserveHeight(reserve)
		w.queryMetricsChanged = true
		c.Invalidate()
	}
}

func (w *workspaceWYSIWYG) drawAPISource(c *mygoui.Context, app *workspaceWindow, canvas, ink, gray mygoui.Color,
	inputHeight, hintHeight float32, font, hint string) {
	edit := &w.apiEdit
	label := "API request source"
	if edit.block.Result {
		label = "API result source"
	}
	mygoui.Column(c).Absolute().Top(queryPaddingY - 1).Left(queryPaddingX - 1).Right(queryPaddingX - 1).Gap(8.296875).Children(func() {
		input := fencedSourceInput(c, &edit.source, &edit.focus, label, inputHeight, func(commit, focus bool) {
			w.finishAPIEdit(app, commit, focus)
		}).FixedLineHeight(18).Padding(10).Radius(6).FocusRing(false).TextColor(ink).
			Background(canvas.Mix(gray, .08)).Border(1, gray.Alpha(.35))
		if input.Focused() {
			input.Background(canvas.Mix(gray, .04)).BorderColor(mygoui.RGBA(100, 150, 255, .6))
		}
		mygoui.Text(c, hint).FillWidth().Height(hintHeight).Font(font).FontSize(10).Wrap().
			FixedLineHeight(13).TextColor(gray.Alpha(.7)).IgnoreInput(true)
	})
	grip := mygoui.Box(c.Key("api source resize")).Absolute().Right(queryPaddingX).Top(queryPaddingY+inputHeight-13).
		Size(12, 12).KeepFocus().Cursor(mygoui.CursorResizeNS).Role(mygoui.RoleSplitter).Label("Resize API source").Bind(&edit.grip)
	drawFencedSourceGrip(c, grip, &edit.grip, &edit.resize, &edit.height, inputHeight, gray)
}

func apiPillColor(method string) mygoui.Color {
	switch method {
	case "GET":
		return mygoui.Hex("#4a9eff")
	case "POST":
		return mygoui.Hex("#3fb950")
	case "PUT", "PATCH":
		return mygoui.Hex("#d29922")
	case "DELETE":
		return mygoui.Hex("#f85149")
	}
	return mygoui.RGB(127, 127, 127)
}

type apiHeaderCell struct {
	label     string
	size      float32
	weight    int
	ink, fill mygoui.Color
	frame     mygoui.Rect
	run       bool
}

func drawAPIRequestHeader(c *mygoui.Context, app *workspaceWindow, block *wysiwyg.APIBlock, offset int, width float32, family string, ink mygoui.Color) float32 {
	source := strings.TrimSpace(block.Source)
	line, extra, _ := strings.Cut(source, "\n")
	parts := strings.Fields(line)
	method, address := "API", ""
	if len(parts) > 1 {
		method, address = strings.ToUpper(parts[0]), parts[1]
	}
	methodW, _ := mygoui.ShapeText(method, mygoui.Font{Family: family, Size: 11, Weight: 700}, 0).Size()
	urlLayout := mygoui.ShapeText(address, mygoui.Font{Family: family, Size: 12.5, LineHeight: 1.28}, width)
	urlWidth, _ := urlLayout.Size()
	urlHeight := max(16, float32(len(urlLayout.Lines()))*16)
	cells := []apiHeaderCell{
		{label: method, size: 11, weight: 700, ink: mygoui.RGB(255, 255, 255), fill: apiPillColor(method),
			frame: mygoui.Rect{W: methodW + 16, H: 18}},
	}
	if address != "" {
		cells = append(cells, apiHeaderCell{label: address, size: 12.5, ink: ink.Alpha(.9), frame: mygoui.Rect{W: urlWidth, H: urlHeight}})
	}
	if extra != "" {
		lines := 1 + strings.Count(extra, "\n")
		more := fmt.Sprintf("+%d lines", lines)
		if lines == 1 {
			more = "+1 line"
		}
		w, _ := mygoui.ShapeText(more, mygoui.Font{Family: family, Size: 10}, 0).Size()
		cells = append(cells, apiHeaderCell{label: more, size: 10, ink: ink.Alpha(.5), frame: mygoui.Rect{W: w, H: 13}})
	}
	busy := app.visualAPIRequestRunning(offset, block)
	buttonLabel := "Run"
	if busy {
		buttonLabel = "…"
	}
	cells = append(cells, apiHeaderCell{label: buttonLabel, run: true, frame: mygoui.Rect{W: 44.5, H: 20}})
	x, y, rowHeight := float32(0), float32(0), float32(0)
	rowStart := 0
	placeRow := func(end int) {
		for i := rowStart; i < end; i++ {
			cells[i].frame.Y = y + (rowHeight-cells[i].frame.H)/2
		}
	}
	for i := range cells {
		cell := &cells[i]
		if x > 0 && x+cell.frame.W > width+.01 {
			placeRow(i)
			y += rowHeight + 8
			rowStart, x, rowHeight = i, 0, 0
		}
		cell.frame.X = x
		if cell.run {
			cell.frame.X = max(0, width-cell.frame.W)
		}
		x += cell.frame.W + 8
		rowHeight = max(rowHeight, cell.frame.H)
	}
	placeRow(len(cells))
	for _, cell := range cells {
		r := cell.frame
		if cell.run {
			button := mygoui.Button(c, cell.label).Absolute().Top(queryPaddingY-1+r.Y).Left(queryPaddingX-1+r.X).Size(r.W, r.H).
				Label("Run").Font(family).FontSize(11).FontWeight(600).Padding(0).Radius(5).
				Background(mygoui.Transparent).Border(0, mygoui.Transparent).TextColor(ink).
				KeepFocus().Disabled(app.modalOpen || busy).OnClick(func() { app.runVisualAPIRequest(offset, block) })
			if busy {
				button.Opacity(.5)
			}
			continue
		}
		label := mygoui.Text(c, cell.label).Absolute().Top(queryPaddingY-1+r.Y).Left(queryPaddingX-1+r.X).Size(r.W, r.H).
			Font(family).FontSize(cell.size).FontWeight(cell.weight).TextColor(cell.ink).Wrap().IgnoreInput(true)
		if cell.fill != mygoui.Transparent {
			label.Background(cell.fill).Radius(4).TextAlign(mygoui.Center).FixedLineHeight(18)
		} else if cell.size == 12.5 {
			label.FixedLineHeight(16)
		}
	}
	return max(24, y+rowHeight) + 2*queryPaddingY
}

type apiResultPresentation struct {
	block          *wysiwyg.APIBlock
	result         liveapi.Result
	table          liveapi.Table
	hasTable, open bool
	rawOpen        bool
}

func newAPIResultPresentation(block *wysiwyg.APIBlock, result liveapi.Result, options liveapi.Options) apiResultPresentation {
	table, hasTable := result.Table()
	return apiResultPresentation{block: block, result: result, table: table, hasTable: hasTable,
		open: !options.Collapsed && len(strings.TrimSpace(result.Body)) <= 1000}
}

func (w *workspaceWYSIWYG) apiPreviousOptions(offset int) liveapi.Options {
	if prior, ok := w.session.APIAt(offset - 2); ok && !prior.Result {
		options, _ := liveapi.ParseOptions(prior.Info)
		// WYS receives only result source from the original bridge. Retain
		// raw formatting when the adjoining request explicitly uses it.
		return liveapi.Options{Format: options.Format}
	}
	return liveapi.Options{}
}

func (w *workspaceWYSIWYG) drawAPIResult(c *mygoui.Context, p *apiResultPresentation, width float32, theme documentPreviewTheme, font string) float32 {
	gray := mygoui.RGB(127, 127, 127)
	ink := mygoui.Hex(theme.BodyTextColor)
	canvas := mygoui.Hex(theme.BackgroundColor)
	code := mygoui.Hex(theme.CodeBackgroundColor)
	codeInk := mygoui.Hex(theme.CodeTextColor)
	body := p.result.Body
	if body == "" {
		body = "No content"
	}
	mono := mygoui.Font{Family: "monospace", Size: 11.5, LineHeight: 1.5}
	innerW := max(1, width-26)
	var rawHeight float32
	if p.open && (!p.hasTable || p.rawOpen) {
		if preview, shortened := apiTextPreview(body); shortened {
			body = preview + "\n\nSee the result source for the full response."
		}
		rawHeight = min(360, float32(max(1, len(mygoui.ShapeText(body, mono, max(1, innerW-20)).Lines())))*17.25+16)
	}
	rowHeight := float32(29)
	tableHeight := float32(len(p.table.Rows)+1)*rowHeight + 1
	tableHeight = min(7*rowHeight+1, tableHeight)
	contentHeight := float32(20)
	if p.open {
		contentHeight += 7 + rawHeight + 9 + 24
		if p.hasTable {
			contentHeight = 20 + 7 + tableHeight + 7 + 20 + 10 + 24
			if p.rawOpen {
				contentHeight += rawHeight + 6
			}
		}
	}
	height := contentHeight + 26
	panel := mygoui.Column(c.Key("api result details")).Absolute().Top(queryPaddingY+7).Left(queryPaddingX-1).Width(width).
		Height(height).Padding(12).Gap(7).Background(canvas.Mix(gray, .08)).Border(1, gray.Alpha(.24)).Radius(8).
		Role(mygoui.RoleGroup).Label("Response").PassThrough()
	panel.Children(func() {
		title := "Response"
		if p.result.StatusCode != 0 {
			title += fmt.Sprintf(" · %d", p.result.StatusCode)
		}
		button := mygoui.ButtonBase(c.Key("API response disclosure")).Label(title).Role(mygoui.RoleDisclosure).Expanded(p.open).
			FillWidth().Height(20).Shrink(0).Justify(mygoui.Start).AlignItems(mygoui.Center).Gap(5).
			OnClick(func() { p.open = !p.open })
		button.Children(func() {
			glyph := "▸"
			if p.open {
				glyph = "▾"
			}
			mygoui.Text(c, glyph+" "+title).Font(font).FontSize(14).FontWeight(600).TextColor(ink).Role(mygoui.RoleNone)
			status := mygoui.Hex("#3fb950")
			if p.result.StatusCode >= 400 {
				status = mygoui.Hex("#f85149")
			}
			mygoui.Box(c).Size(7, 7).Radius(4).Background(status).Role(mygoui.RoleNone)
		})
		if !p.open {
			return
		}
		if p.hasTable {
			drawAPITable(c, p.table, innerW, tableHeight, rowHeight, font, ink, gray)
			mygoui.ButtonBase(c.Key("raw response")).Label("Raw response").Role(mygoui.RoleDisclosure).Expanded(p.rawOpen).
				FillWidth().Height(20).Shrink(0).Justify(mygoui.Start).
				OnClick(func() { p.rawOpen = !p.rawOpen }).Children(func() {
				glyph := "▸"
				if p.rawOpen {
					glyph = "▾"
				}
				mygoui.Text(c, glyph+" Raw response").Font(font).FontSize(12).TextColor(ink.Alpha(.7)).Role(mygoui.RoleNone)
			})
		}
		if !p.hasTable || p.rawOpen {
			mygoui.Scroll(c.Key("api response body")).FillWidth().Height(rawHeight).Shrink(0).Padding(8, 10).
				Background(code).Radius(4).Label("Response body").Children(func() {
				mygoui.Text(c, body).FillWidth().Font(mono.Family).FontSize(mono.Size).
					FixedLineHeight(17.25).TextColor(codeInk).Wrap().Selectable()
			})
		}
		metadata := mygoui.Text(c, p.result.Metadata).FillWidth().Height(24).Shrink(0).Padding(5, 0, 0).
			BorderWidth(1, 0, 0, 0).BorderColor(gray.Alpha(.13)).Font(font).FontSize(12).TextColor(ink.Alpha(.65))
		if !p.hasTable {
			metadata.Margin(2, 0, 0)
		}
	})
	return max(24, height+7+28) + 2*queryPaddingY
}

func drawAPITable(c *mygoui.Context, table liveapi.Table, width, height, rowHeight float32, font string, ink, gray mygoui.Color) {
	columns := len(table.Columns)
	if columns == 0 {
		return
	}
	columnWidth := max(85, (width-1)/float32(columns))
	visible := min(7*rowHeight+1, height)
	mygoui.Scroll(c.Key("api table")).FillWidth().Height(visible).Shrink(0).
		Role(mygoui.RoleTable).Label("API response table").ColumnCount(columns).RowCount(len(table.Rows) + 1).Children(func() {
		for i := range len(table.Rows) + 1 {
			values, role, weight := table.Columns, mygoui.RoleColumnHeader, 600
			if i > 0 {
				values, role, weight = table.Rows[i-1], mygoui.RoleCell, 400
			}
			row := mygoui.Row(c.Key(i)).Width(columnWidth * float32(columns)).Height(rowHeight).
				Role(mygoui.RoleRow).RowIndex(i + 1).Shrink(0)
			if i == 0 {
				row.Background(gray.Alpha(.15))
			}
			row.Children(func() {
				for j, value := range values {
					mygoui.Text(c.Key(j), value).Width(columnWidth).Height(rowHeight).Shrink(0).Padding(6, 10).
						Font(font).FontSize(13).FontWeight(weight).TextColor(ink).SingleLine().Ellipsis("…").
						Role(role).RowIndex(i+1).ColumnIndex(j+1).Border(1, gray.Alpha(.25))
				}
			})
		}
	})
}
