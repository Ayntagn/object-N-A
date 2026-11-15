package ui

import (
	"math"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-207. The native popover adds its own chrome around this content.
const graphInfoWidth, graphInfoHeight = 330, 458

type graphInfoPopover struct {
	flyout *mygo.Window
	inline bool
}

func (app *workspaceWindow) graphInfoButton(c *mygoui.Context, label string, parent *mygo.Window, info *graphInfoPopover, size float32) mygoui.Element {
	button := chromeControl(c, label, false).Children(func() {
		chromeIconColor(c, "info.circle", size, c.Theme().TextMuted)
	})
	if parent == nil {
		// Views rendered without a platform window can still show the same content.
		button.OnClick(func() { info.inline = !info.inline })
		mygoui.PopoverBase(c, button, &info.inline, func(panel mygoui.Element) {
			panel.Size(graphInfoWidth, graphInfoHeight).AttachTo(button, mygoui.AnchorBottom, mygoui.AnchorTop)
			app.graphInfoSurface(c, app.graphInfoContent(c))
		})
		return button
	}
	if button.Clicked() && !parent.IsDestroyed() {
		if info.flyout == nil {
			app.showGraphInfo(parent, button.Bounds(), info)
		} else {
			info.flyout.Close()
		}
	}
	return button
}

func (app *workspaceWindow) showGraphInfo(parent *mygo.Window, anchor mygoui.Rect, info *graphInfoPopover) {
	floatingOnMac := runtime.GOOS == "darwin" && parent == app.graph.win
	if floatingOnMac {
		// EV-UI-207 / EV-MYGO-097. Match the floating About popover's attachment point.
		anchor.X += 5
		anchor.Y -= 1
	}
	flyout := mygo.NewFlyout(mygo.FlyoutOptions{
		Parent: parent,
		Anchor: mygo.Rectangle{
			X: int(math.Round(float64(anchor.X))), Y: int(math.Round(float64(anchor.Y))),
			Width: int(math.Round(float64(anchor.W))), Height: int(math.Round(float64(anchor.H))),
		},
		Placement: mygo.PlacementBottom,
		Width:     graphInfoWidth, Height: graphInfoHeight,
		Popover: true, Focusable: true, Shadow: true,
		Content: mygoui.View(func(c *mygoui.Context) {
			if floatingOnMac {
				// EV-UI-213. Escape leaves the floating Graph help open on macOS.
				c.OnWindowShortcut(0, mygoui.KeyEscape, func() {})
			}
			app.applyAppearance(c)
			c.Root().Background(mygoui.Transparent)
			content := app.graphInfoContent(c)
			if !c.Vibrancy() {
				app.graphInfoSurface(c, content)
			}
		}),
	})
	info.flyout = flyout
	flyout.OnClosed(func() {
		if info.flyout == flyout {
			info.flyout = nil
		}
	})
}

func (app *workspaceWindow) graphInfoSurface(c *mygoui.Context, panel mygoui.Element) {
	fill := c.Theme().Surface
	if app.blackout {
		fill = mygoui.RGB(34, 34, 36)
	}
	border := c.Theme().Border
	panel.Background(fill).Radius(16).DrawOver(func(p *mygoui.Painter, r mygoui.Rect) {
		p.Stroke(r, border, 16, 1)
	})
}

var graphInfoRows = [...]struct {
	symbol, label, body string
}{
	{"circle.grid.cross", "Circles Arranged In A Cross Pattern", "Each **node is a note**, and **the lines are [[backlinks]]** - notes that link to one another. Connected notes pull together, so clusters reveal related ideas."},
	{"magnifyingglass", "Search", "**Search** the vault (top of the sidebar) and the graph lights up matching notes - **brighter and larger means more relevant** - while the rest dim. Names, tags, content, and related concepts all count."},
	{"tag", "Tag", "**Click a tag** in the Tags list to light up every note carrying it and dim the rest - a quick visual filter."},
	{"circle.hexagongrid", "Circle With Circles In A Hexagonal Grid", "**Cluster** groups notes by tag into regions, so the graph reads like a map - each lobe is a tag."},
	{"globe", "Globe", "The **globe** button toggles between the flat **2D** graph and a rotating **3D** globe."},
	{"play.fill", "Play", "**Play** auto-tours the notes, focusing each in turn. **Expand** opens a larger floating window in whichever view you're using."},
	{"rotate.3d", "Three-Dimensional Rotation Arrows", "**In 3D:** drag to spin, scroll to zoom, hover a note to read it, and **click empty space to pause or resume** the spin."},
	{"hand.tap", "Hand Tapping", "**In 2D:** drag notes around, scroll to zoom, and **click the empty canvas to reset & jiggle** the graph (it's fun)."},
}

func (app *workspaceWindow) graphInfoContent(c *mygoui.Context) mygoui.Element {
	titleInk, bodyInk, iconInk := c.Theme().Text, c.Theme().TextMuted, app.graphAccent(c)
	if app.blackout {
		titleInk, bodyInk, iconInk = mygoui.Hex("#E3E2E6"), mygoui.Hex("#C4C6D0"), mygoui.Hex("#FFAAF7")
	}
	return mygoui.Column(c).Fill().Padding(16, 16.25).Gap(11).AlignItems(mygoui.Start).Children(func() {
		mygoui.Text(c, "Knowledge Graph").AlignSelf(mygoui.Start).
			FontSize(13).FixedLineHeight(16).Bold().TextColor(titleInk)
		for _, row := range graphInfoRows {
			mygoui.Row(c.Key(row.symbol)).FillWidth().Gap(9).AlignItems(mygoui.Start).Children(func() {
				mygoui.Box(c).Width(16).Height(14).Shrink(0).Children(func() {
					fallback := chromeIconFallbacks[row.symbol]
					switch row.symbol {
					case "rotate.3d":
						fallback = chromeIconFallbacks["arrow.counterclockwise"]
					case "hand.tap":
						fallback = chromeIconFallbacks["circle.circle"]
					}
					// EV-UI-207 / EV-MYGO-097. Keep the slot independent of the drawn glyph.
					sfsymbols.Symbol(c, row.symbol).Size(15).Natural().Color(iconInk).Fallback(fallback).
						Label(row.label).Attach(mygoui.AnchorCenter, mygoui.AnchorCenter)
				})
				runs := strings.Split(row.body, "**")
				spans := make([]mygoui.Span, 0, len(runs))
				for i, run := range runs {
					weight := 400
					if i%2 != 0 {
						weight = 700
					}
					spans = append(spans, mygoui.Span{Text: run, Weight: weight})
				}
				// Match the original line counts with a metrically compatible body face.
				mygoui.RichText(c, spans...).Grow(1).MinWidth(0).Wrap().Font("Arial").FontSize(12).FixedLineHeight(14).TextColor(bodyInk)
			})
		}
	})
}
