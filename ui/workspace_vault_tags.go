package ui

import (
	"math"
	"slices"
	"strconv"

	"cyberwriter/internal/core/vaultindex"
	mygoui "github.com/egoist/mygo/ui"
)

const (
	vaultTagSectionWidth = vaultPanelWidth - 8
	vaultTagRowWidth     = vaultTagSectionWidth - 2*vaultTagInset - vaultTagGap
	vaultTagInset        = 12
	vaultTagGap          = 4
	vaultTagPillGap      = 3
	vaultTagPillPad      = 6
	vaultTagPillHeight   = 18
	vaultTagRowHeight    = vaultTagPillHeight + vaultTagGap
	vaultTagViewportPad  = 8
	vaultTagMaxRows      = 11
	vaultTagHeaderHeight = 28 // Divider and disclosure button.
)

func (app *workspaceWindow) vaultTags(c *mygoui.Context) {
	data := app.graphData()
	if data == nil || len(data.Tags) == 0 {
		return
	}
	expanded := app.preferences.VaultTagsExpanded
	colors := app.vaultSectionColors(c)
	colors.accent = app.vaultMarkdownIconInk(c, colors)
	minHeight := float32(vaultTagHeaderHeight)
	if expanded {
		minHeight += vaultTagPillHeight + 2*vaultTagViewportPad - vaultTagGap
	}
	section := mygoui.Column(c.Key("vault tags")).Width(vaultTagSectionWidth).Left(4).MinHeight(minHeight)
	section.Children(func() {
		mygoui.Box(c).FillWidth().Height(1).Shrink(0).Background(colors.border.Alpha(.2))
		vaultSectionHeading(c, "Tags", strconv.Itoa(len(data.Tags)), "number", expanded, app.modalOpen, colors, func() {
			app.preferences.VaultTagsExpanded = !expanded
			app.preferencesChanged()
		})
		if expanded {
			measured := measureVaultTagLayout(c, section, data, 0)
			app.vaultTagList(c, measured.rows)
		}
	})
}

type measuredVaultTag struct {
	tag         vaultindex.GraphTag
	name, count string
	width       float32
	elided      bool
}

type vaultTagLayout struct {
	source *vaultindex.Graph
	locale string
	tags   []measuredVaultTag
	rows   [][]measuredVaultTag
}

func measureVaultTagLayout(c *mygoui.Context, owner mygoui.Element, data *vaultindex.Graph, limit int) *vaultTagLayout {
	layout := mygoui.Local(owner, "tag layout", func() vaultTagLayout { return vaultTagLayout{} })
	if layout.source != data || layout.locale != c.Locale() {
		tags := data.Tags
		if limit > 0 {
			tags = slices.Clone(tags)
			slices.SortStableFunc(tags, func(a, b vaultindex.GraphTag) int { return b.Count - a.Count })
			tags = tags[:min(len(tags), limit)]
		}
		measured := measureVaultTags(c, tags)
		*layout = vaultTagLayout{source: data, locale: c.Locale(), tags: measured, rows: vaultTagRows(measured)}
	}
	return layout
}

func measureVaultTags(c *mygoui.Context, tags []vaultindex.GraphTag) []measuredVaultTag {
	measured := make([]measuredVaultTag, 0, len(tags))
	for _, tag := range tags {
		name, count := "#"+tag.Name, strconv.Itoa(tag.Count)
		nameWidth, _ := c.MeasureText(0, mygoui.Span{Text: name, Font: "monospace", Size: 11, Weight: 500})
		countWidth, _ := c.MeasureText(0, mygoui.Span{Text: count, Font: "monospace", Size: 10, Weight: 600})
		width := nameWidth + countWidth + 2*vaultTagPillPad + vaultTagPillGap
		// Allow half-point layout while avoiding premature wrap at the pixel boundary.
		width = float32(math.Ceil(float64(width)*2-0.001)) / 2
		measured = append(measured, measuredVaultTag{tag: tag, name: name, count: count,
			width: min(width, vaultTagRowWidth), elided: width > vaultTagRowWidth})
	}
	return measured
}

func vaultTagRows(tags []measuredVaultTag) [][]measuredVaultTag {
	var rows [][]measuredVaultTag
	var start int
	var used float32
	for i, tag := range tags {
		if used > 0 && used+vaultTagGap+tag.width > vaultTagRowWidth {
			rows = append(rows, tags[start:i])
			start, used = i, 0
		}
		if used > 0 {
			used += vaultTagGap
		}
		used += tag.width
	}
	if start < len(tags) {
		rows = append(rows, tags[start:])
	}
	return rows
}

// EV-MYGO-260. A large set uses only a fraction of the sidebar and builds
// the chip rows MyGo can display in the scroll viewport.
func (app *workspaceWindow) vaultTagList(c *mygoui.Context, rows [][]measuredVaultTag) {
	_, height := c.Size()
	rowsHeight := min(len(rows), vaultTagMaxRows)*vaultTagRowHeight - vaultTagGap
	viewportHeight := min(float32(rowsHeight+2*vaultTagViewportPad), height/4)
	mygoui.List(c.Key("tags viewport"), nil, len(rows)).FillWidth().
		Height(viewportHeight).MinHeight(0).Padding(vaultTagViewportPad, vaultTagInset, vaultTagViewportPad-vaultTagGap).
		Role(mygoui.RoleScroll).Label("Vault tags").ItemKey(func(i int) any { return rows[i][0].tag.Name }).
		Rows(func(row mygoui.ListRow) {
			c := row.Context
			mygoui.Row(c).Size(vaultTagRowWidth, vaultTagRowHeight).Gap(vaultTagGap).AlignItems(mygoui.Start).
				Children(func() { app.drawVaultTagPills(c, rows[row.Index], true) })
		})
}

// EV-UI-190/191. Only the Files tags run a filter action.
func (app *workspaceWindow) drawVaultTagPills(c *mygoui.Context, tags []measuredVaultTag, filter bool) {
	ink, fill, hover, contrast := app.vaultTagColors(c)
	for _, pill := range tags {
		tag := pill.tag
		selected := app.graph.tag == tag.Name
		tagInk, tagFill := ink, fill
		if app.preferences.TagsColoredByHash {
			color := graphTagPalette[graphTagSlot(tag.Name)].color()
			tagInk, tagFill = color, color.Alpha(.12)
			if !c.Theme().Dark {
				tagInk, tagFill = mygoui.RGB(0, 0, 0).Mix(color, .45), color.Alpha(.32)
			}
			if selected {
				tagInk, tagFill = contrast, color
				if !c.Theme().Dark {
					tagInk = mygoui.RGB(20, 20, 20)
				}
			}
		} else if selected {
			tagInk, tagFill = contrast, hover
		}
		button := mygoui.ButtonBase(c.Key(tag.Name)).Size(pill.width, vaultTagPillHeight).Shrink(0).
			Padding(0, vaultTagPillPad).Gap(vaultTagPillGap).Label(pill.name + ", " + pill.count).Radius(6).
			Background(tagFill).AccessSelected(selected).Disabled(app.modalOpen).KeepFocus()
		if pill.elided {
			button.Tooltip(pill.name)
		}
		if !app.preferences.TagsColoredByHash && !selected && button.Hovered() {
			button.DrawOver(func(p *mygoui.Painter, frame mygoui.Rect) {
				p.Stroke(frame, hover.Alpha(.6), 6, 1)
			})
		}
		if filter {
			button.OnClick(func() {
				if app.graph.tag == tag.Name {
					app.setGraphTag("")
				} else {
					app.setGraphTag(tag.Name)
				}
			})
		}
		button.Children(func() {
			mygoui.Text(c, pill.name).Font("monospace").FontSize(11).FontWeight(500).
				SingleLine().MinWidth(0).Ellipsis("…").TextColor(tagInk).Role(mygoui.RoleNone)
			mygoui.Text(c, pill.count).Font("monospace").FontSize(10).FontWeight(600).
				SingleLine().Shrink(0).TextColor(tagInk.Alpha(.6)).Role(mygoui.RoleNone)
		})
	}
}

func (app *workspaceWindow) vaultTagColors(c *mygoui.Context) (ink, fill, accent, contrast mygoui.Color) {
	colors := app.vaultSectionColors(c)
	ink, accent = colors.secondary.Alpha(.85), app.vaultMarkdownIconInk(c, colors)
	contrast = c.Theme().Background
	switch app.appAppearance() {
	case appearanceBlackout:
		fill, contrast = mygoui.Hex("#202024"), mygoui.RGB(0, 0, 0)
	case appearanceCyber:
		fill, contrast = mygoui.Hex("#31353c"), mygoui.Hex("#10141a")
	case appearancePLATO:
		fill, contrast = mygoui.Hex("#2c1e0d"), mygoui.Hex("#171008")
	default:
		fill = mygoui.RGB(56, 56, 59)
		if !c.Theme().Dark {
			fill, accent = mygoui.RGB(224, 224, 230), mygoui.Hex("#b83b9e")
		}
	}
	return
}

func (app *workspaceWindow) vaultTagFilter(c *mygoui.Context) {
	tag := app.graph.tag
	if tag == "" {
		return
	}
	ink, _, accent, _ := app.vaultTagColors(c)
	if app.preferences.TagsColoredByHash {
		ink = graphTagPalette[graphTagSlot(tag)].color()
		if !c.Theme().Dark {
			ink = mygoui.RGB(0, 0, 0).Mix(ink, .45)
		}
	} else {
		ink = accent
	}
	mygoui.Row(c.Key("vault-tag-filter")).Height(14).FillWidth().Gap(4).AlignItems(mygoui.Center).Children(func() {
		mygoui.Box(c).Size(6, 6).Shrink(0).Radius(3).Background(ink)
		mygoui.Text(c, "#"+tag).Font("monospace").FontSize(11).FontWeight(600).TextColor(ink).
			SingleLine().MinWidth(0).Ellipsis("…")
		mygoui.Spacer(c)
		chromeButtonIcon(c, "xmark.circle.fill", "Clear tag filter", func() {
			app.setGraphTag("")
		}, 11, app.vaultSectionColors(c).secondary.Alpha(.5)).Size(11, 11).KeepFocus().Disabled(app.modalOpen)
	})
}
