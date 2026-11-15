package ui

import (
	"math"
	"strconv"

	"cyberwriter/internal/core/vaultindex"
	mygoui "github.com/egoist/mygo/ui"
)

const (
	vaultTagSectionWidth = vaultPanelWidth - 8
	vaultTagContentWidth = vaultTagSectionWidth - 2*vaultTagInset
	vaultTagInset        = 12
	vaultTagGap          = 4
	vaultTagPillGap      = 3
	vaultTagPillPad      = 6
	vaultTagPillHeight   = 18
)

func (app *workspaceWindow) vaultTags(c *mygoui.Context) {
	data := app.graphData()
	if data == nil || len(data.Tags) == 0 {
		return
	}
	expanded := app.preferences.VaultTagsExpanded
	var tags []measuredVaultTag
	width := float32(vaultTagContentWidth)
	colors := app.vaultSectionColors(c)
	colors.accent = app.vaultMarkdownIconInk(c, colors)
	section := mygoui.Column(c.Key("vault tags")).Width(vaultTagSectionWidth).Left(4).Shrink(0)
	if expanded {
		tags = app.measureVaultTags(c, data.Tags)
		width = vaultTagFlowWidth(tags)
		section.Left(4 + (vaultTagContentWidth-width)/2)
	}
	section.Children(func() {
		mygoui.Box(c).FillWidth().Height(1).Shrink(0).Background(colors.border.Alpha(.2))
		vaultSectionHeading(c, "Tags", strconv.Itoa(len(data.Tags)), "number", expanded, app.modalOpen, colors, func() {
			app.preferences.VaultTagsExpanded = !expanded
			app.preferencesChanged()
		})
		if expanded {
			mygoui.Column(c).FillWidth().Shrink(0).Padding(8, vaultTagInset).Children(func() {
				app.vaultTagFlow(c, tags, width, true)
			})
		}
	})
}

type measuredVaultTag struct {
	tag         vaultindex.GraphTag
	name, count string
	width       float32
}

func (app *workspaceWindow) measureVaultTags(c *mygoui.Context, tags []vaultindex.GraphTag) []measuredVaultTag {
	measured := make([]measuredVaultTag, 0, len(tags))
	for _, tag := range tags {
		name, count := "#"+tag.Name, strconv.Itoa(tag.Count)
		nameWidth, _ := c.MeasureText(0, mygoui.Span{Text: name, Font: "monospace", Size: 11, Weight: 500})
		countWidth, _ := c.MeasureText(0, mygoui.Span{Text: count, Font: "monospace", Size: 10, Weight: 600})
		width := nameWidth + countWidth + 2*vaultTagPillPad + vaultTagPillGap
		// Allow half-point layout while avoiding premature wrap at the pixel boundary.
		width = float32(math.Ceil(float64(width)*2-0.001)) / 2
		measured = append(measured, measuredVaultTag{tag: tag, name: name, count: count, width: width})
	}
	return measured
}

// The proposed width stays at 316 pt until a single natural-width tag
// requires a wider row. Swift's FlowLayout includes the final 4 pt advance.
func vaultTagFlowWidth(tags []measuredVaultTag) float32 {
	width := float32(vaultTagContentWidth)
	for _, tag := range tags {
		if tag.width > vaultTagContentWidth {
			width = max(width, tag.width+vaultTagGap)
		}
	}
	return width
}

// EV-UI-190/191. Only the Files tags run a filter action.
func (app *workspaceWindow) vaultTagFlow(c *mygoui.Context, tags []measuredVaultTag, width float32, filter bool) {
	ink, fill, hover, contrast := app.vaultTagColors(c)
	mygoui.Row(c.Key("tag flow")).Width(width - vaultTagGap).AlignSelf(mygoui.Start).
		Shrink(0).Wrap().Gap(vaultTagGap).Children(func() {
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
			button := mygoui.ButtonBase(c.Key(tag.Name)).Size(pill.width, vaultTagPillHeight).
				Padding(0, vaultTagPillPad).Gap(vaultTagPillGap).Label(pill.name + ", " + pill.count).Radius(6).
				Background(tagFill).AccessSelected(selected).Disabled(app.modalOpen).KeepFocus()
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
					SingleLine().Shrink(0).TextColor(tagInk).Role(mygoui.RoleNone)
				mygoui.Text(c, pill.count).Font("monospace").FontSize(10).FontWeight(600).
					SingleLine().Shrink(0).TextColor(tagInk.Alpha(.6)).Role(mygoui.RoleNone)
			})
		}
	})
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
