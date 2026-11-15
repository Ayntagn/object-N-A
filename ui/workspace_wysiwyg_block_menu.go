package ui

import (
	"slices"
	"strings"

	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

const blockMenuLabel = "Insert block"

// EV-UI-147: the four tabs scroll one list. The filtered list contains
// only the groups with a matching item.
var visualBlockGroups = [...]string{"Text", "List", "Advanced", "cyberWriter"}

type visualBlockItem struct {
	label, icon string
	group       int
	style       richtext.ParagraphStyle
	enabled     bool
}

var visualBlockItems = [...]visualBlockItem{
	{label: "Text", enabled: true},
	{label: "Heading 1", style: richtext.ParagraphStyle{Heading: 1}, enabled: true},
	{label: "Heading 2", style: richtext.ParagraphStyle{Heading: 2}, enabled: true},
	{label: "Heading 3", style: richtext.ParagraphStyle{Heading: 3}, enabled: true},
	{label: "Heading 4", style: richtext.ParagraphStyle{Heading: 4}, enabled: true},
	{label: "Heading 5", style: richtext.ParagraphStyle{Heading: 5}, enabled: true},
	{label: "Heading 6", style: richtext.ParagraphStyle{Heading: 6}, enabled: true},
	{label: "Quote", icon: "quote.opening", style: richtext.ParagraphStyle{Quote: true}, enabled: true},
	{label: "Divider", icon: "minus", enabled: true},
	{label: "Bullet List", icon: "list.bullet", group: 1, style: richtext.ParagraphStyle{List: richtext.Bullets}, enabled: true},
	{label: "Ordered List", icon: "list.number", group: 1, style: richtext.ParagraphStyle{List: richtext.Numbers}, enabled: true},
	{label: "Task List", icon: "checkmark.square", group: 1, style: richtext.ParagraphStyle{List: richtext.Tasks}, enabled: true},
	{label: "Image", icon: "photo", group: 2, enabled: true},
	{label: "Code", icon: "chevron.left.forwardslash.chevron.right", group: 2, style: richtext.ParagraphStyle{CodeBlock: true}, enabled: true},
	{label: insertTableLabel, group: 2, enabled: true},
	{label: "Math", icon: "function", group: 2, enabled: true},
	{label: "Callout", icon: "exclamationmark.bubble", group: 3, enabled: true},
	{label: apiRequestLabel, icon: "globe", group: 3, enabled: true},
	{label: wysiwyg.QueryBlockLabel, icon: "magnifyingglass", group: 3, enabled: true},
	{label: insertContentsLabel, icon: "list.bullet.indent", group: 3, enabled: true},
	{label: "Page Break", icon: "arrow.down.to.line", group: 3, enabled: true},
	{label: "Mermaid Diagram", icon: "arrow.triangle.branch", group: 3, enabled: true},
	{label: "Vega-Lite Chart", icon: "chart.bar", group: 3, enabled: true},
}

type visualBlockMenu struct {
	open, forced bool
	start        int
	selected     int
	reveal       bool
	resetScroll  bool
	query        string
	scroll       mygoui.ScrollState
	submenu      string
	input        visualBlockInput
}

type visualBlockInput struct {
	document *richtext.Document
	version  uint64
	start    int
	text     string
	valid    bool
}

func (w *workspaceWYSIWYG) closeBlockControls() {
	w.blockMenu = visualBlockMenu{}
	w.blockGutter = visualBlockGutter{}
}

func (w *workspaceWYSIWYG) openGutterBlockMenu() {
	_, caret := w.session.Document.Selection()
	w.blockMenu = visualBlockMenu{open: true, forced: true, start: caret}
	w.focus = true
}

func blockMenuText(text richtext.Text, anchor, caret int) (int, string, bool) {
	start, end := text.ParagraphRange(caret)
	style := text.ParagraphStyleAt(start)
	if anchor != caret || caret != end || style.List != richtext.NoList || style.Indent != 0 || style.CodeBlock || style.Quote || end-start > 500 {
		return start, "", false
	}
	value := text.Slice(start, end).String()
	return start, value, !strings.ContainsRune(value, richtext.ObjectReplacement)
}

func (w *workspaceWYSIWYG) watchBlockMenu(edit mygoui.Element, doc *richtext.Document, text richtext.Text, anchor, caret int) {
	menu := &w.blockMenu
	previous := menu.input
	start, value, valid := blockMenuText(text, anchor, caret)
	menu.input = visualBlockInput{doc, doc.Version(), start, value, valid}
	// OnChange sees committed text from the native input client as well
	// as keyboard input. Loading a note or painting a preedit does not
	// deliver this notification.
	edit.OnChange(func() {
		if !edit.Focused() || previous.document != doc || previous.version == doc.Version() || menu.submenu != "" {
			return
		}
		a, z := doc.Selection()
		start, value, valid := blockMenuText(doc.Text(), a, z)
		if !valid || !strings.HasPrefix(value, "/") || previous.valid && previous.start == start && previous.text == value {
			return
		}
		if start != menu.start {
			menu.selected, menu.forced = 0, false
		}
		menu.open, menu.start = true, start
	})
}

type visualBlockGroup struct {
	index, first, end int
}

func (w *workspaceWYSIWYG) drawBlockMenu(c *mygoui.Context, app *workspaceWindow, edit mygoui.Element, padding float32,
	viewport mygoui.Rect, text richtext.Text, anchor, caret int, boxes []richtext.ParagraphBox, theme *richtext.Theme, editorFont mygoui.Font) bool {
	menu := &w.blockMenu
	if !menu.open {
		return false
	}
	if (!edit.Focused() && !w.focus) || viewport.W <= 0 || viewport.H <= 0 {
		menu.open, menu.forced, menu.submenu = false, false, ""
		return false
	}
	colors := app.blockMenuColors()
	loadBundledFont(colors.font)
	colors.font += ", sans-serif"
	if menu.submenu != "" {
		w.drawBlockSubmenu(c, app, viewport, colors)
		return true
	}
	start, value, valid := blockMenuText(text, anchor, caret)
	if !valid || start != menu.start || !menu.forced && !strings.HasPrefix(value, "/") {
		menu.open, menu.forced = false, false
		return false
	}
	query := strings.ToLower(strings.TrimPrefix(value, "/"))
	if query != menu.query {
		menu.query, menu.resetScroll = query, true
	}
	var items []visualBlockItem
	var groups []visualBlockGroup
	for _, item := range visualBlockItems {
		if !strings.Contains(strings.ToLower(item.label), query) {
			continue
		}
		if len(groups) == 0 || groups[len(groups)-1].index != item.group {
			groups = append(groups, visualBlockGroup{index: item.group, first: len(items)})
		}
		items = append(items, item)
		groups[len(groups)-1].end = len(items)
	}
	if len(items) == 0 {
		menu.open, menu.forced, menu.selected = false, false, 0
		return false
	}
	if menu.selected >= len(items) {
		menu.selected = 0
	}
	index := slices.IndexFunc(boxes, func(box richtext.ParagraphBox) bool { return box.Start == start })
	if index < 0 || boxes[index].Top+boxes[index].Height <= 0 || boxes[index].Top >= viewport.H {
		return false
	}

	selectedGroup := slices.IndexFunc(groups, func(g visualBlockGroup) bool { return menu.selected < g.end })
	selectRow := func(row int) { menu.selected, menu.reveal = row, true }
	choose := func(item visualBlockItem) { w.runVisualBlockItem(app, item) }
	edit.OnShortcut(0, mygoui.KeyUp, func() { selectRow(max(0, menu.selected-1)) }).
		OnShortcut(0, mygoui.KeyDown, func() { selectRow(min(len(items)-1, menu.selected+1)) }).
		OnShortcut(0, mygoui.KeyLeft, func() {
			row := menu.selected
			if selectedGroup > 0 {
				row = groups[selectedGroup-1].end - 1
			}
			selectRow(row)
		}).OnShortcut(0, mygoui.KeyRight, func() {
		row := menu.selected
		if selectedGroup+1 < len(groups) {
			row = groups[selectedGroup+1].first
		}
		selectRow(row)
	}).OnShortcut(0, mygoui.KeyEnter, func() { choose(items[menu.selected]) }).
		OnShortcut(0, mygoui.KeyTab, func() {
			menu.open, menu.forced = false, false
			w.session.Document.InsertText("    ")
			w.sync(app)
		})

	const navHeight, groupHeight, rowHeight, listPadding = 61, 48, 52, 12
	font := mygoui.Font{Family: colors.font, Size: 14, Weight: 600}
	width := max(float32(244), blockMenuTabsWidth(groups, font))
	width = min(width, viewport.W)
	listHeight := float32(len(groups)*groupHeight + (len(groups) - 1) + len(items)*rowHeight + listPadding)
	listHeight = min(listHeight, 420, max(20, viewport.H-navHeight-20))
	box := boxes[index]
	x, bottom := blockMenuCaret(theme, editorFont, box, text, start, caret, viewport.W-2*padding)
	position := mygoui.Box(c.Key("block menu anchor")).Absolute().Left(max(0, min(padding+x, viewport.W-width))).
		Top(max(0, min(box.Top+bottom, viewport.H-1))).Size(1, 1).PassThrough().Role(mygoui.RoleNone)
	if menu.resetScroll {
		menu.scroll.Y = 0
	}
	panel := mygoui.PopoverBase(c.Key(blockMenuLabel), position, &menu.open, func(panel mygoui.Element) {
		panel.Role(mygoui.RoleMenu).Label(blockMenuLabel).Width(width).Radius(12).Background(colors.surface).
			Margin(9, 0, 0).KeepFocus().Shadow(0, 2, 6, 0, mygoui.RGBA(0, 0, 0, .12))
		mygoui.Column(c).FillWidth().Children(func() {
			mygoui.Row(c.Key("block groups")).FillWidth().Height(navHeight-1).Gap(10).Padding(20, 22, 8).
				Role(mygoui.RoleTabList).Label("Block categories").Children(func() {
				for i, group := range groups {
					name := visualBlockGroups[group.index]
					tab := mygoui.ButtonBase(c.Key(name)).Height(32).MinWidth(0).Padding(0, 10).Radius(8).
						Role(mygoui.RoleTab).Label(name).AccessSelected(i == selectedGroup).KeepFocus().TabStop(false).
						Cursor(mygoui.CursorPointer).OnClick(func() { selectRow(group.first) })
					if i == selectedGroup {
						tab.Background(colors.accent.Alpha(.2))
					} else if tab.Hovered() {
						tab.Background(colors.hover)
					}
					tab.Children(func() { blockMenuTextLabel(c, name, colors.ink, font.Family) })
				}
			})
			mygoui.Box(c).Height(1).FillWidth().Margin(0, 12).Background(colors.separator).Role(mygoui.RoleNone)
			mygoui.Scroll(c.Key("block choices")).FillWidth().Height(listHeight).Padding(0, 12, listPadding).
				TrackScroll(&menu.scroll).Label("Block commands").KeepFocus().Children(func() {
				for i, group := range groups {
					name := visualBlockGroups[group.index]
					if i != 0 {
						mygoui.Box(c.Key("rule "+name)).Height(1).FillWidth().Margin(0, 10).Background(colors.separator).Role(mygoui.RoleNone)
					}
					mygoui.Row(c.Key("heading "+name)).FillWidth().Height(groupHeight).Shrink(0).Padding(0, 10).AlignItems(mygoui.Center).
						Role(mygoui.RoleHeading).Label(name).Children(func() { blockMenuTextLabel(c, strings.ToUpper(name), colors.ink.Alpha(.6), font.Family) })
					for j := group.first; j < group.end; j++ {
						item := items[j]
						row := w.visualBlockMenuRow(c, item, j, colors, choose)
						if j == menu.selected {
							edit.ActiveDescendant(row)
							if menu.reveal || menu.resetScroll && j > 0 {
								align := mygoui.ScrollNearest
								if menu.reveal {
									align = mygoui.ScrollStart
								}
								row.ScrollIntoView(mygoui.ScrollIntoViewOptions{Block: align})
							}
						}
					}
				}
			})
		})
	})
	edit.HasPopup(mygoui.PopupMenu).Expanded(menu.open).Controls(panel)
	menu.reveal, menu.resetScroll = false, false
	if panel.Dismissed() {
		menu.forced = false
	}
	return menu.open
}

func blockMenuTabsWidth(groups []visualBlockGroup, font mygoui.Font) float32 {
	width := float32(44 + (len(groups)-1)*10)
	for _, group := range groups {
		label, _ := mygoui.ShapeText(visualBlockGroups[group.index], font, 0).Size()
		width += label + 20
	}
	return width
}

func blockMenuTextLabel(c *mygoui.Context, label string, color mygoui.Color, family string) {
	mygoui.Text(c, label).SingleLine().Font(family).FontSize(14).FontWeight(600).FixedLineHeight(20).
		TextColor(color).Role(mygoui.RoleNone).IgnoreInput(true)
}

type visualBlockColors struct {
	surface, ink, accent, hover, separator mygoui.Color
	font                                   string
}

func (app *workspaceWindow) blockMenuColors() visualBlockColors {
	theme := app.resolvePreviewTheme(app.previewThemeSelection())
	family, _, _ := strings.Cut(theme.BodyFontFamily, ",")
	family = strings.Trim(family, " '\"")
	ink := mygoui.Hex(theme.BodyTextColor)
	return visualBlockColors{surface: mygoui.Hex(theme.BackgroundColor), ink: ink, accent: mygoui.Hex(theme.LinkColor),
		hover: mygoui.Hex(theme.TableBorderColor).Alpha(.3), separator: ink.Alpha(.096), font: family}
}

func blockMenuCaret(theme *richtext.Theme, font mygoui.Font, box richtext.ParagraphBox,
	text richtext.Text, start, caret int, width float32) (float32, float32) {
	if start == caret {
		return 0, box.Height
	}
	// The public RichText view exposes the paragraph box. Shape the live
	// query using the editor font to anchor to its end, including wrapping.
	style := text.ParagraphStyleAt(start)
	if style.Heading > 0 {
		heading := theme.Headings[min(style.Heading, 6)-1]
		font.Size *= heading.Size
		font.Weight = heading.Weight
		if font.Weight == 0 {
			font.Weight = 700
		}
	}
	value := text.Slice(start, caret).String()
	layout := mygoui.ShapeText(value, font, max(1, width))
	end := layout.CaretAt(mygoui.TextCaretPosition{Index: mygoui.UTF16Len(value), Affinity: mygoui.TextDownstream})
	return end.X, end.Y + end.H
}

func (w *workspaceWYSIWYG) visualBlockMenuRow(c *mygoui.Context, item visualBlockItem, index int, colors visualBlockColors,
	choose func(visualBlockItem)) mygoui.Element {
	menu := &w.blockMenu
	row := mygoui.ButtonBase(c.Key(item.label)).FillWidth().Height(52).Shrink(0).Padding(0, 10).Gap(16).Radius(8).
		AlignItems(mygoui.Center).Justify(mygoui.Start).Label(item.label).Role(mygoui.RoleMenuItem).
		KeepFocus().TabStop(false).AccessSelected(index == menu.selected).Disabled(!item.enabled)
	if item.enabled {
		row.Cursor(mygoui.CursorPointer).OnClick(func() { choose(item) })
		row.HandleInput(func(event mygoui.InputEvent) bool {
			if event.Kind == mygoui.InputPointerMove && event.Button < 0 && index != menu.selected {
				menu.selected = index
				return true
			}
			return false
		})
	}
	if row.Pressed() {
		row.Background(colors.accent.Alpha(.2))
	} else if index == menu.selected {
		row.Background(colors.hover)
	}
	ink := colors.ink
	if !item.enabled {
		ink = ink.Alpha(.55)
	}
	row.Children(func() {
		blockMenuIcon(c, item, colors.ink.Alpha(.48), colors.font)
		blockMenuTextLabel(c, item.label, ink, colors.font)
		if item.label == "Mermaid Diagram" || item.label == "Vega-Lite Chart" {
			blockMenuTextLabel(c, "▸", ink, colors.font)
		}
	})
	return row
}

func blockMenuIcon(c *mygoui.Context, item visualBlockItem, ink mygoui.Color, family string) {
	box := mygoui.Box(c).Size(24, 24).Center().PassThrough().Role(mygoui.RoleNone)
	if item.label == insertTableLabel {
		box.Draw(func(p *mygoui.Painter, r mygoui.Rect) {
			x, y := r.X+3, r.Y+4
			p.Stroke(mygoui.Rect{X: x, Y: y, W: 18, H: 16}, ink, 1.5, 2)
			p.Fill(mygoui.Rect{X: x + 1, Y: y + 4, W: 16, H: 2}, ink, 0)
			p.Fill(mygoui.Rect{X: x + 5.5, Y: y + 6, W: 1.5, H: 9}, ink, 0)
			p.Fill(mygoui.Rect{X: x + 11, Y: y + 6, W: 1.5, H: 9}, ink, 0)
		})
		return
	}
	if item.icon != "" {
		box.Children(func() { chromeIconColor(c, item.icon, 24, ink).IgnoreInput(true) })
		return
	}
	box.Draw(func(p *mygoui.Painter, rect mygoui.Rect) {
		glyph, size := "T", float32(21)
		if item.style.Heading > 0 {
			glyph, size = string(rune('0'+item.style.Heading)), 14
			p.Stroke(mygoui.Rect{X: rect.X + 4, Y: rect.Y + 4, W: 16, H: 16}, ink, 1, 2)
		}
		layout := mygoui.ShapeText(glyph, mygoui.Font{Family: family, Size: size, Weight: 700}, 0)
		width, height := layout.Size()
		p.TextLayout(layout, rect.X+(rect.W-width)/2, rect.Y+(rect.H-height)/2, ink)
	})
}

func (w *workspaceWYSIWYG) runVisualBlockItem(app *workspaceWindow, item visualBlockItem) {
	if !item.enabled || !w.editable(app) || !w.blockMenu.open {
		return
	}
	menu := &w.blockMenu
	doc := w.session.Document
	a, z := doc.Selection()
	start, value, valid := blockMenuText(doc.Text(), a, z)
	if !valid || start != menu.start || !menu.forced && !strings.HasPrefix(value, "/") {
		return
	}
	menu.open, menu.forced, menu.selected, menu.resetScroll = false, false, 0, true
	doc.SetSelection(start, z)
	switch item.label {
	case "Image":
		w.session.InsertImage("", false)
	case insertTableLabel:
		w.openNewTable(app)
		return
	case "Divider":
		w.session.InsertDivider()
	case "Page Break":
		app.insertPageBreak()
	case "Callout":
		w.session.InsertCallout("note", "Note", "Content")
	case wysiwyg.QueryBlockLabel:
		app.insertQueryBlock()
		return
	case apiRequestLabel:
		app.insertAPIRequest()
		return
	case "Math":
		w.insertVisualDisplayMath(app, "")
		return
	case insertContentsLabel:
		app.insertContents()
	case "Mermaid Diagram", "Vega-Lite Chart":
		if w.session.FormatBlockMenuSelection(richtext.ParagraphStyle{}) {
			menu.open, menu.submenu = true, item.label
		}
	default:
		w.session.FormatBlockMenuSelection(item.style)
	}
	w.sync(app)
	w.focus = true
}

func (w *workspaceWYSIWYG) drawBlockSubmenu(c *mygoui.Context, app *workspaceWindow, viewport mygoui.Rect, colors visualBlockColors) {
	menu := &w.blockMenu
	const submenuWidth, submenuRow, submenuTitle = 244, 33, 24
	var labels []string
	mermaid := menu.submenu == "Mermaid Diagram"
	if mermaid {
		for _, diagram := range diagramCommands {
			labels = append(labels, diagram.title)
		}
	} else {
		for _, chart := range chartCommands {
			labels = append(labels, chart.title)
		}
	}
	width := min(float32(submenuWidth), viewport.W)
	anchor := mygoui.Box(c.Key("diagram submenu anchor")).Absolute().Left(max(0, min(24, viewport.W-width))).
		Top(min(84, viewport.H-1)).Size(1, 1).PassThrough().Role(mygoui.RoleNone)
	mygoui.Overlay(c.Key("block submenu backdrop"), func() {
		mygoui.Box(c).Absolute().Top(0).Bottom(0).Left(0).Right(0).KeepFocus().Role(mygoui.RoleNone).
			HandleInput(func(event mygoui.InputEvent) bool {
				if event.Kind != mygoui.InputPointerDown {
					return false
				}
				menu.open, menu.submenu = false, ""
				return true
			})
	})
	panel := mygoui.PopoverBase(c.Key("diagram submenu"), anchor, &menu.open, func(panel mygoui.Element) {
		panel.Role(mygoui.RoleMenu).Label(menu.submenu).Width(width).Radius(12).Background(colors.surface).
			Margin(5, 0, 0).Border(1, colors.ink.Alpha(.18)).KeepFocus().Shadow(0, 10, 34, 0, mygoui.RGBA(0, 0, 0, .28))
		height := min(float32(len(labels)*submenuRow+submenuTitle+12), viewport.H*.62+12)
		mygoui.Scroll(c).FillWidth().Height(max(24, height)).Padding(6).Scrollbars(false).Label(menu.submenu + " choices").Children(func() {
			mygoui.Text(c, strings.ToUpper(menu.submenu)).Height(submenuTitle).Padding(5, 10, 7).FontSize(10).
				FontWeight(700).TextColor(colors.ink.Alpha(.45)).Role(mygoui.RoleNone)
			for i, label := range labels {
				choose := func() {
					if !menu.open {
						return
					}
					menu.open, menu.submenu = false, ""
					if mermaid {
						app.insertDiagram(diagramCommands[i])
					} else {
						app.insertChart(chartCommands[i])
					}
				}
				row := mygoui.ButtonBase(c.Key(label)).FillWidth().Height(submenuRow).Shrink(0).Radius(7).Padding(0, 10).Gap(9).
					AlignItems(mygoui.Center).Justify(mygoui.Start).Role(mygoui.RoleMenuItem).Label(label).KeepFocus().TabStop(false).
					OnClick(choose).Cursor(mygoui.CursorPointer).HandleInput(func(event mygoui.InputEvent) bool {
					if event.Kind != mygoui.InputPointerDown || event.Button != 0 {
						return false
					}
					choose()
					return true
				})
				if row.Hovered() {
					row.Background(colors.accent.Alpha(.14))
				}
				row.Children(func() {
					icon := "chart.bar"
					if mermaid {
						icon = "arrow.triangle.branch"
					}
					chromeIconColor(c, icon, 16, colors.ink.Alpha(.75)).IgnoreInput(true)
					mygoui.Text(c, label).SingleLine().FontSize(14).TextColor(colors.ink).Role(mygoui.RoleNone).IgnoreInput(true)
				})
			}
		})
	})
	if panel.Dismissed() {
		menu.submenu = ""
	}
}
