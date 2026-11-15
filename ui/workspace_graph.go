package ui

import (
	"fmt"
	"slices"
	"sync"

	"cyberwriter/internal/core/vaultindex"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

const graphPanelHeight = 208
const graphWindowButtonHeight = 20.5

type workspaceGraph struct {
	win                     *mygo.Window
	sidebar, floating       graphCanvas
	open                    bool
	hideLines               bool
	labels                  [2]*bool
	sidebarInfo, windowInfo graphInfoPopover
	tag, windowTag          string
	highlight               map[string]bool
	layoutSource            *vaultindex.Graph
	layout                  vaultindex.GraphLayout
}

func (app *workspaceWindow) graphData() *vaultindex.Graph {
	if app.index.data == nil {
		return nil
	}
	return app.index.data.Graph()
}

func (g *workspaceGraph) layoutFor(data *vaultindex.Graph) vaultindex.GraphLayout {
	if g.layoutSource != data {
		g.layoutSource = data
		if data == nil {
			g.layout = vaultindex.GraphLayout{}
		} else {
			g.layout = data.Layout(graphScanOrder(data.Notes))
		}
	}
	return g.layout
}

func (app *workspaceWindow) toggleGraph() {
	app.graph.open = !app.graph.open
	if !app.graph.open {
		app.graph.sidebar.cancelTour()
	}
	app.openFilesSidebar()
}

func (app *workspaceWindow) graphSection(c *mygoui.Context) {
	g := &app.graph
	showStats := app.preferences.GraphShowStats
	headerFill := c.Theme().Surface
	if app.blackout {
		headerFill = mygoui.RGB(15, 15, 18)
	}
	graph := mygoui.Column(c.Key("vault-graph")).FillWidth().Shrink(0).Padding(0, 4)
	var tags []measuredVaultTag
	tagWidth := float32(vaultTagContentWidth)
	if g.open && showStats {
		graph.Width(vaultTagSectionWidth).Left(4).Padding(0)
		if data := app.graphData(); data != nil {
			top := slices.Clone(data.Tags)
			slices.SortStableFunc(top, func(a, b vaultindex.GraphTag) int { return b.Count - a.Count })
			tags = app.measureVaultTags(c, top[:min(len(top), 8)])
			tagWidth = vaultTagFlowWidth(tags)
			// The natural-width Stats card expands around the sidebar's center.
			graph.Left(4 + (vaultTagContentWidth-tagWidth)/2)
		}
	}
	graph.Children(func() {
		mygoui.Row(c).FillWidth().Height(26).AlignItems(mygoui.Center).Padding(0, 12).Gap(4).
			Background(headerFill).Children(func() {
			chevron := "chevron.right"
			if g.open {
				chevron = "chevron.down"
			}
			disclosureWidth := float32(101)
			if g.open && !showStats {
				shapeWidth, _ := sidebarGraphShapeSize(app.preferences.GraphShape)
				disclosureWidth = 122 - shapeWidth
			}
			toggle := mygoui.ButtonBase(c).Height(26).Width(disclosureWidth).Gap(6).Justify(mygoui.Start).
				Label("Graph").Expanded(g.open).Disabled(app.modalOpen).OnClick(app.toggleGraph)
			if g.open && showStats {
				toggle.Grow(1)
			}
			toggle.Children(func() {
				chromeIconColor(c, chevron, 9, c.Theme().TextMuted)
				chromeIconColor(c, "point.3.connected.trianglepath.dotted", 12, app.graphAccent(c))
				mygoui.Text(c, "Graph").FontSize(12).Bold()
			})
			if !g.open {
				return
			}
			if !showStats {
				app.graphToggleButton(c, "play.fill", "Play", false, g.sidebar.touring(), g.sidebar.toggleTour).Size(19, 20)
				app.graphDisplayButtons(c, false)
				chromeButtonIcon(c, "arrow.up.left.and.arrow.down.right", "Open Graph in Window", app.showGraphWindow, 12, c.Theme().TextMuted).
					Size(22.5, 22.5).MarginY(.25).Disabled(app.modalOpen)
			}
			statsIcon := "chart.bar"
			if showStats {
				statsIcon = "chart.bar.fill"
			}
			app.graphToggleButton(c, statsIcon, "Vault Stats", false, showStats, func() {
				app.preferences.GraphShowStats = !showStats
				if !showStats {
					g.sidebar.cancelTour()
				}
				app.preferencesChanged()
			}).Size(25.5, 22.5).MarginY(.25)
			app.graphInfoButton(c, "Info", app.win, &g.sidebarInfo, 13).Size(23, 23)
		})
		if !g.open {
			return
		}
		if showStats {
			mygoui.Column(c.Key("stats panel")).FillWidth().Height(graphPanelHeight).Shrink(0).
				Padding(4, 4, 4).Children(func() { app.graphStats(c, app.graphData(), tags, tagWidth) })
			return
		}
		mygoui.Column(c).FillWidth().Height(graphPanelHeight).Shrink(0).Padding(4, 3, 0).Children(func() {
			data := app.graphData()
			g.sidebar.view(c, data, g.layoutFor(data), app.graphOptions(false), app.openGraphNote)
		})
	})
}

// EV-UI-192 / EV-UI-093. The selected shape changes the compact trigger's size.
func sidebarGraphShapeSize(shape graphShape) (width, height float32) {
	switch shape {
	case graphTesseract:
		return 22, 17
	case graphTorusKnot:
		return 21, 15
	case graphRose:
		return 23, 16
	default:
		return 21, 17
	}
}

var graphShapeMenuIcons = sync.OnceValue(func() map[graphShape]*mygo.MenuIcon {
	icons := make(map[graphShape]*mygo.MenuIcon, len(graphShapeChoices))
	for _, shape := range graphShapeChoices {
		var icon *mygo.MenuIcon
		if data := nativeMenuSymbol(shape.symbol); len(data) != 0 {
			icon, _ = mygo.NewMenuIcon(data, true)
		}
		if icon == nil && chromeIconFallbacks[shape.symbol] != nil {
			icon, _ = mygo.NewMenuIconSVG(chromeIconFallbacks[shape.symbol], true)
		}
		icons[shape.id] = icon
	}
	return icons
})

func (app *workspaceWindow) graphDisplayButtons(c *mygoui.Context, floating bool) {
	g := &app.graph
	clusterWidth, linesWidth, modeWidth := float32(23.5), float32(23.5), float32(25)
	clusterHeight, linesHeight, modeHeight := float32(23), float32(22), float32(25)
	clusterIcon, clusterHelp := "circle.hexagongrid.fill", "Even placement"
	if !app.preferences.GraphClustered {
		clusterIcon, clusterHelp = "circle.hexagongrid", "Cluster by tag"
	}
	linesIcon, linesHelp := "point.3.connected.trianglepath.dotted", "Hide connecting lines"
	if g.hideLines {
		linesIcon, linesHelp = "circle.grid.3x3", "Show connecting lines"
		linesHeight = 23
	}
	if floating {
		clusterWidth, linesWidth, modeWidth = 34.5, 35.5, 33
		clusterHeight, linesHeight, modeHeight = graphWindowButtonHeight, graphWindowButtonHeight, graphWindowButtonHeight
		if g.hideLines {
			linesWidth = 34.5
		}
		if app.preferences.Graph3D {
			modeWidth = 37
		}
	}
	app.graphToggleButton(c, clusterIcon, "Cluster", floating, app.preferences.GraphClustered, func() {
		app.preferences.GraphClustered = !app.preferences.GraphClustered
		app.writePreferences()
	}, clusterHelp).Size(clusterWidth, clusterHeight)
	app.graphToggleButton(c, linesIcon, "Lines", floating, !g.hideLines, func() {
		g.hideLines = !g.hideLines
	}, linesHelp).Size(linesWidth, linesHeight)
	if floating {
		mode, shown := app.floatingGraphLabels()
		labelHelp := "Show all labels"
		if shown {
			labelHelp = "Show only the active label"
		}
		app.graphToggleButton(c, "tag.fill", "Labels", true, shown, func() {
			shown = !shown
			g.labels[mode] = &shown
		}, labelHelp).Size(36.5, graphWindowButtonHeight)
	}
	label, icon := "Show 3D globe (experimental)", "globe"
	if app.preferences.Graph3D {
		label, icon = "Flat view", "circle.grid.cross"
	}
	app.graphToggleButton(c, icon, label, floating, app.preferences.Graph3D, func() {
		app.preferences.Graph3D = !app.preferences.Graph3D
		app.writePreferences()
	}).Size(modeWidth, modeHeight)
	current := app.preferences.GraphShape.choice()
	shapeWidth, shapeHeight := sidebarGraphShapeSize(current.id)
	iconSize, shapeInk := shapeHeight, c.Theme().Text
	if floating {
		shapeWidth, shapeHeight, iconSize = current.windowWidth, graphWindowButtonHeight, 22
		shapeInk = app.graphAccent(c)
	} else if app.blackout {
		shapeInk = mygoui.RGB(224, 224, 224)
	}
	chromeControl(c.Key("graph shape"), "Graph shape", app.modalOpen, "Graph shape: "+current.name).
		Size(shapeWidth, shapeHeight).Menu(func(m *mygoui.Menu) {
		icons := graphShapeMenuIcons()
		for _, shape := range graphShapeChoices {
			if m.Item(shape.name).Icon(icons[shape.id]).Checked(shape.id == current.id).Chosen() {
				app.preferences.GraphShape, app.preferences.Graph3D = shape.id, true
				app.writePreferences()
				app.invalidateGraphViews()
			}
		}
	}).Children(func() {
		app.graphToolbarSymbol(c, current.symbol, iconSize, shapeInk, floating && current.id == graphAuto)
	})
}

func (app *workspaceWindow) floatingGraphLabels() (mode int, shown bool) {
	if app.preferences.Graph3D {
		mode = 1
	}
	if override := app.graph.labels[mode]; override != nil {
		return mode, *override
	}
	data := app.graphData()
	return mode, data != nil && len(data.Notes) < 51
}

func (app *workspaceWindow) graphAccent(c *mygoui.Context) mygoui.Color {
	if app.blackout {
		return blackoutSourceHead
	}
	return c.Theme().Accent
}

func (app *workspaceWindow) graphToggleButton(c *mygoui.Context, icon, label string, floating, selected bool, action func(), help ...string) mygoui.Element {
	color, size := c.Theme().TextMuted, float32(13)
	if selected {
		color = app.graphAccent(c)
	}
	if floating {
		size = 22
		color = app.graphWindowInk(c)
	}
	if app.blackout && (label == "Play" || label == "Tour graph nodes") {
		color = mygoui.RGB(184, 132, 184)
		if selected {
			color = app.graphAccent(c)
		}
	}
	button := chromeControl(c.Key(label), label, app.modalOpen, help...).OnClick(func() {
		action()
		app.invalidateGraphViews()
	}).AccessSelected(selected)
	return button.Children(func() {
		app.graphToolbarSymbol(c, icon, size, color, floating)
	})
}

func (app *workspaceWindow) graphToolbarSymbol(c *mygoui.Context, name string, size float32, ink mygoui.Color, floating bool) {
	fallback := chromeIconFallbacks[name]
	if name == "circle.grid.3x3" {
		fallback = chromeIconFallbacks["circle.grid.cross"]
	}
	var fittedSize, offsetY float32
	if floating && app.blackout && !app.preferences.Graph3D {
		// EV-UI-192 / EV-MYGO-088. Fit the glyph independently of the AX hit box.
		switch name {
		case "xmark.circle.fill":
			fittedSize, ink = 35.5, mygoui.RGB(80, 84, 93)
		case "circle.hexagongrid", "point.3.connected.trianglepath.dotted", "globe", "sparkles":
			fittedSize, ink = 35.5, mygoui.RGB(111, 159, 187)
			if name == "sparkles" {
				offsetY = -1.25
			}
		}
	}
	icon := sfsymbols.Symbol(c, name).Size(size).Natural().Color(ink).Fallback(fallback).Role(mygoui.RoleNone)
	if fittedSize != 0 {
		icon.Size(fittedSize, fittedSize).Absolute().LeftPercent(50).TopPercent(50).
			TranslatePercent(-50, -50).Translate(0, offsetY).IgnoreInput(true)
	}
}

func (app *workspaceWindow) graphWindowInk(c *mygoui.Context) mygoui.Color {
	if app.blackout {
		return mygoui.RGB(120, 155, 181)
	}
	return c.Theme().TextMuted
}

func (app *workspaceWindow) graphOptions(floating bool) graphCanvasOptions {
	g := &app.graph
	_, labels := app.floatingGraphLabels()
	tag := g.tag
	if floating {
		tag = g.windowTag
	}
	return graphCanvasOptions{
		Globe: app.preferences.Graph3D, Lines: !g.hideLines, Cluster: app.preferences.GraphClustered,
		Labels: floating && labels, Shape: app.preferences.GraphShape, Compact: !floating,
		ColorByTag: app.preferences.GraphColorNodesByTag,
		ActivePath: app.currentVaultPath(), Tag: tag, Highlight: g.highlight,
	}
}

func (app *workspaceWindow) setGraphTag(tag string) {
	app.graph.tag, app.graph.windowTag = tag, tag
	app.invalidateGraphWindow()
}

func (app *workspaceWindow) openGraphNote(relative string) {
	app.openDocument(relative)
	app.invalidateGraphViews()
}

func (app *workspaceWindow) invalidateGraphViews() {
	if app.win != nil && !app.win.IsDestroyed() {
		app.win.Invalidate()
	}
	app.invalidateGraphWindow()
}

func (app *workspaceWindow) invalidateGraphWindow() {
	if win := app.graph.win; win != nil && !win.IsDestroyed() {
		win.Invalidate()
	}
}

func (app *workspaceWindow) closeGraphWindow() {
	if win := app.graph.win; win != nil && !win.IsDestroyed() {
		win.Close()
	}
}

func (app *workspaceWindow) showGraphWindow() {
	if app.win == nil || app.win.IsDestroyed() || app.vault == nil || app.modalOpen {
		return
	}
	g := &app.graph
	if g.win != nil && !g.win.IsDestroyed() {
		g.win.Focus()
		return
	}
	g.floating = graphCanvas{}
	g.windowTag = g.tag
	win := mygo.NewWindow(mygo.WindowOptions{
		Width: 1100, Height: 760, UseContentSize: true, Parent: app.win, AlwaysOnTop: true,
		DisableFullScreen: true, TitleBarStyle: mygo.TitleBarHidden, HideWindowButtons: true, Hidden: true,
		Content: mygoui.View(app.graphWindowView),
	})
	g.win = win
	win.SetTitle("")
	win.OnClosed(func() {
		if app.graph.win == win {
			app.graph.win = nil
			app.graph.windowInfo.inline = false
		}
	})
	win.Show()
}

func (app *workspaceWindow) graphWindowView(c *mygoui.Context) {
	app.applyAppearance(c)
	g := &app.graph
	mygoui.Column(c).Fill().Children(func() {
		data := app.graphData()
		g.floating.view(c, data, g.layoutFor(data), app.graphOptions(true), app.openGraphNote)
		mygoui.Row(c.Key("graph toolbar")).Absolute().Top(0).Left(0).Right(0).Height(42.5).
			AlignItems(mygoui.Center).Padding(0, 9).Gap(8).DragWindow().Children(func() {
			chromeControl(c, "Close graph window", false).OnClick(app.closeGraphWindow).
				Size(33, graphWindowButtonHeight).Children(func() {
				app.graphToolbarSymbol(c, "xmark.circle.fill", 22, c.Theme().TextMuted, true)
			})
			app.graphInfoButton(c, "About the graph", g.win, &g.windowInfo, 22).Size(33, graphWindowButtonHeight)
			mygoui.Spacer(c)
			app.graphDisplayButtons(c, true)
			app.graphToggleButton(c, "play.fill", "Tour graph nodes", true, g.floating.touring(), g.floating.toggleTour).
				Size(24.5, graphWindowButtonHeight)
			chromeButtonIcon(c, "arrow.counterclockwise", "Reset graph layout (or double-click)", g.floating.resetCamera, 22, app.graphWindowInk(c)).
				Size(29.5, graphWindowButtonHeight)
			if app.preferences.Graph3D {
				label, icon := "Pause rotation", "pause.fill"
				width := float32(23.5)
				if g.floating.paused {
					label, icon = "Resume rotation", "play.fill"
					width = 29.5
				}
				app.graphToggleButton(c, icon, label, true, !g.floating.paused, func() {
					g.floating.paused = !g.floating.paused
				}).Size(width, graphWindowButtonHeight)
			}
		})
	})
}

func (app *workspaceWindow) graphStats(c *mygoui.Context, data *vaultindex.Graph, tags []measuredVaultTag, tagWidth float32) {
	if data == nil {
		mygoui.Text(c, "Indexing Vault…").Role(mygoui.RoleStatus).Padding(12)
		return
	}
	panel, cell, buttonFill := c.Theme().Surface, c.Theme().Background, c.Theme().SurfaceHover
	if app.blackout {
		panel, cell, buttonFill = mygoui.RGB(12, 12, 15), mygoui.RGB(20, 20, 23), mygoui.RGB(30, 30, 33)
	}
	mygoui.Column(c.Key("vault-stats")).Width(tagWidth + 16).MaxHeight(200).Shrink(0).
		AlignItems(mygoui.Center).Padding(8).Gap(8).
		Background(panel).Radius(6).Children(func() {
		mygoui.Row(c).FillWidth().Height(13).Shrink(0).AlignItems(mygoui.Center).Gap(8).Children(func() {
			chromeIconColor(c, "chart.bar", 12, app.graphAccent(c))
			mygoui.Text(c, "Vault Stats").FontSize(11).FontWeight(600).Role(mygoui.RoleHeading)
		})
		cardHeight, numberSize := float32(64), float32(16)
		if len(tags) != 0 && graphStatsTagHeight(tags, tagWidth) > 84 {
			// The stats cells compress when the tag rows use the proposed height.
			cardHeight, numberSize = 56, 11
		}
		mygoui.Row(c).FillWidth().Height(cardHeight).Shrink(0).Gap(8).Children(func() {
			for _, stat := range []struct {
				label, icon string
				value       int
			}{
				{"Notes", "doc.text", len(data.Notes)}, {"Tags", "number", len(data.Tags)},
				{"Links", "link", data.Links}, {"Linked", "point.3.connected.trianglepath.dotted", data.Linked},
			} {
				mygoui.Column(c.Key(stat.label)).Grow(1).Basis(0).MinWidth(0).AlignItems(mygoui.Center).
					Padding(6, 0).Gap(3).Background(cell).Radius(5).Children(func() {
					chromeIconColor(c, stat.icon, 11, app.graphAccent(c))
					mygoui.Text(c, fmt.Sprint(stat.value)).FontSize(numberSize).FontWeight(600).SingleLine()
					mygoui.Text(c, stat.label).FontSize(10).TextColor(c.Theme().TextMuted).SingleLine()
				})
			}
		})
		if len(tags) > 0 {
			app.vaultTagFlow(c, tags, tagWidth, false)
		}
		mygoui.ButtonBase(c).Label("Open Graph in Window").FillWidth().Height(19).Radius(4).Gap(6).
			Background(buttonFill).Disabled(app.modalOpen).OnClick(app.showGraphWindow).Children(func() {
			chromeIconColor(c, "arrow.up.left.and.arrow.down.right", 10, c.Theme().TextMuted)
			mygoui.Text(c, "Open Graph in Window").FontSize(11).FontWeight(600)
		})
	})
}

func graphStatsTagHeight(tags []measuredVaultTag, width float32) float32 {
	var rowWidth float32
	rows := 1
	for _, tag := range tags {
		next := rowWidth + tag.width + vaultTagGap
		if rowWidth > 0 && next > width {
			rows++
			next = tag.width + vaultTagGap
		}
		rowWidth = next
	}
	return float32(rows*(vaultTagPillHeight+vaultTagGap) - vaultTagGap)
}
