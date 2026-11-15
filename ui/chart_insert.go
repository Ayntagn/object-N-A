package ui

import (
	"sync"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type chartCommand struct {
	title, symbol, source string
}

const chartFenceStart = "```vega-lite\n"
const chartCaret = len(chartFenceStart)

// EV-UI-147/151: order, marks, encodings, interaction, and source caret.
// The inline data and sample fields are authored for this editor.
var chartCommands = [...]chartCommand{
	{"Bar Chart", "chart.bar", `{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {"values": [
    {"day": "Mon", "checkouts": 19},
    {"day": "Tue", "checkouts": 26},
    {"day": "Wed", "checkouts": 21},
    {"day": "Thu", "checkouts": 32},
    {"day": "Fri", "checkouts": 29}
  ]},
  "mark": {"type": "bar", "cursor": "pointer", "cornerRadiusEnd": 4},
  "params": [{"name": "hover", "select": {"type": "point", "on": "pointerover", "clear": "pointerout"}}],
  "encoding": {
    "x": {"field": "day", "type": "nominal", "sort": null, "axis": {"labelAngle": 0, "title": null}},
    "y": {"field": "checkouts", "type": "quantitative", "title": "Checkouts"},
    "color": {"field": "day", "type": "nominal", "legend": null},
    "opacity": {"condition": {"param": "hover", "value": 1}, "value": 0.55},
    "tooltip": [{"field": "day", "type": "nominal"}, {"field": "checkouts", "type": "quantitative"}]
  }
}`},
	{"Line Chart", "chart.xyaxis.line", `{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {"values": [
    {"week": "W1", "room": "Gallery", "visits": 46},
    {"week": "W1", "room": "Studio", "visits": 24},
    {"week": "W2", "room": "Gallery", "visits": 53},
    {"week": "W2", "room": "Studio", "visits": 30},
    {"week": "W3", "room": "Gallery", "visits": 49},
    {"week": "W3", "room": "Studio", "visits": 37},
    {"week": "W4", "room": "Gallery", "visits": 62},
    {"week": "W4", "room": "Studio", "visits": 31},
    {"week": "W5", "room": "Gallery", "visits": 68},
    {"week": "W5", "room": "Studio", "visits": 43}
  ]},
  "mark": {"type": "line", "point": true, "cursor": "pointer"},
  "params": [{"name": "hover", "select": {"type": "point", "fields": ["room"], "on": "pointerover", "clear": "pointerout"}}],
  "encoding": {
    "x": {"field": "week", "type": "ordinal", "sort": null, "axis": {"title": null}},
    "y": {"field": "visits", "type": "quantitative", "title": "Visits"},
    "color": {"field": "room", "type": "nominal", "title": "Room"},
    "opacity": {"condition": {"param": "hover", "value": 1}, "value": 0.3},
    "tooltip": [
      {"field": "room", "type": "nominal"},
      {"field": "week", "type": "ordinal"},
      {"field": "visits", "type": "quantitative"}
    ]
  }
}`},
	{"Area Chart", "chart.line.uptrend.xyaxis", `{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {"values": [
    {"day": "Mon", "desk": "Archive", "requests": 18},
    {"day": "Tue", "desk": "Archive", "requests": 27},
    {"day": "Wed", "desk": "Archive", "requests": 22},
    {"day": "Thu", "desk": "Archive", "requests": 38},
    {"day": "Fri", "desk": "Archive", "requests": 34}
  ]},
  "mark": "area",
  "encoding": {
    "x": {"field": "day", "type": "ordinal", "sort": null},
    "y": {"field": "requests", "type": "quantitative"}
  }
}`},
	{"Scatter Plot", "chart.dots.scatter", `{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {"values": [
    {"light": 11, "growth": 21, "leaves": 8, "variety": "Basil"},
    {"light": 15, "growth": 28, "leaves": 10, "variety": "Basil"},
    {"light": 19, "growth": 30, "leaves": 14, "variety": "Basil"},
    {"light": 23, "growth": 35, "leaves": 12, "variety": "Basil"},
    {"light": 10, "growth": 18, "leaves": 5, "variety": "Mint"},
    {"light": 14, "growth": 25, "leaves": 9, "variety": "Mint"},
    {"light": 20, "growth": 26, "leaves": 11, "variety": "Mint"},
    {"light": 25, "growth": 32, "leaves": 16, "variety": "Mint"}
  ]},
  "mark": {"type": "circle", "cursor": "pointer"},
  "params": [{"name": "hover", "select": {"type": "point", "on": "pointerover", "clear": "pointerout"}}],
  "encoding": {
    "x": {"field": "light", "type": "quantitative", "scale": {"zero": false}},
    "y": {"field": "growth", "type": "quantitative", "scale": {"zero": false}},
    "color": {"field": "variety", "type": "nominal", "title": "Variety"},
    "size": {"field": "leaves", "type": "quantitative", "title": "Leaves"},
    "opacity": {"condition": {"param": "hover", "value": 1}, "value": 0.5},
    "tooltip": [
      {"field": "variety", "type": "nominal"},
      {"field": "light", "type": "quantitative"},
      {"field": "growth", "type": "quantitative"},
      {"field": "leaves", "type": "quantitative"}
    ]
  }
}`},
	{"Pie Chart", "chart.pie", `{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {"values": [
    {"day": "Mon", "desk": "Library", "checkouts": 15},
    {"day": "Tue", "desk": "Library", "checkouts": 22},
    {"day": "Wed", "desk": "Library", "checkouts": 31},
    {"day": "Thu", "desk": "Library", "checkouts": 17},
    {"day": "Fri", "desk": "Library", "checkouts": 28}
  ]},
  "mark": "arc",
  "encoding": {
    "theta": {"field": "checkouts", "type": "quantitative"},
    "color": {"field": "day", "type": "nominal"}
  }
}`},
	{"Donut Chart", "chart.pie.fill", `{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {"values": [
    {"format": "Film", "loans": 31},
    {"format": "Book", "loans": 57},
    {"format": "Audio", "loans": 24},
    {"format": "Magazine", "loans": 16}
  ]},
  "mark": {"type": "arc", "innerRadius": 60, "cursor": "pointer"},
  "params": [{"name": "hover", "select": {"type": "point", "on": "pointerover", "clear": "pointerout"}}],
  "encoding": {
    "theta": {"field": "loans", "type": "quantitative"},
    "color": {"field": "format", "type": "nominal", "title": "Format"},
    "opacity": {"condition": {"param": "hover", "value": 1}, "value": 0.6},
    "tooltip": [{"field": "format", "type": "nominal"}, {"field": "loans", "type": "quantitative"}]
  }
}`},
	{"Stacked Bar", "chart.bar.fill", `{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {"values": [
    {"day": "Mon", "desk": "Main", "checkouts": 15},
    {"day": "Tue", "desk": "Main", "checkouts": 19},
    {"day": "Mon", "desk": "East", "checkouts": 8},
    {"day": "Tue", "desk": "East", "checkouts": 14},
    {"day": "Wed", "desk": "Main", "checkouts": 23}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "desk", "type": "nominal"},
    "y": {"field": "checkouts", "type": "quantitative", "aggregate": "sum"},
    "color": {"field": "day", "type": "nominal"}
  }
}`},
	{"Grouped Bar", "chart.bar.xaxis", `{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {"values": [
    {"day": "Mon", "desk": "Main", "checkouts": 17},
    {"day": "Mon", "desk": "East", "checkouts": 12},
    {"day": "Tue", "desk": "Main", "checkouts": 25},
    {"day": "Tue", "desk": "East", "checkouts": 15},
    {"day": "Wed", "desk": "Main", "checkouts": 19},
    {"day": "Wed", "desk": "East", "checkouts": 22},
    {"day": "Thu", "desk": "Main", "checkouts": 31},
    {"day": "Thu", "desk": "East", "checkouts": 18}
  ]},
  "mark": {"type": "bar", "cursor": "pointer"},
  "params": [{"name": "hover", "select": {"type": "point", "fields": ["desk"], "on": "pointerover", "clear": "pointerout"}}],
  "encoding": {
    "x": {"field": "day", "type": "nominal", "sort": null, "axis": {"labelAngle": 0, "title": null}},
    "y": {"field": "checkouts", "type": "quantitative", "title": "Checkouts"},
    "xOffset": {"field": "desk"},
    "color": {"field": "desk", "type": "nominal", "title": "Desk"},
    "opacity": {"condition": {"param": "hover", "value": 1}, "value": 0.5},
    "tooltip": [
      {"field": "day", "type": "nominal"},
      {"field": "desk", "type": "nominal"},
      {"field": "checkouts", "type": "quantitative"}
    ]
  }
}`},
	{"Heatmap", "square.grid.3x3.fill", `{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {"values": [
    {"day": "Mon", "room": "Gallery", "visits": 24},
    {"day": "Mon", "room": "Studio", "visits": 14},
    {"day": "Tue", "room": "Gallery", "visits": 35},
    {"day": "Tue", "room": "Studio", "visits": 29},
    {"day": "Wed", "room": "Gallery", "visits": 41}
  ]},
  "mark": "rect",
  "encoding": {
    "x": {"field": "day", "type": "nominal", "sort": null},
    "y": {"field": "room", "type": "nominal"},
    "color": {"field": "visits", "type": "quantitative"}
  }
}`},
	{"Histogram", "chart.bar", `{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {"values": [
    {"day": "Mon", "room": "Gallery", "visits": 12},
    {"day": "Tue", "room": "Gallery", "visits": 31},
    {"day": "Wed", "room": "Studio", "visits": 16},
    {"day": "Thu", "room": "Studio", "visits": 25},
    {"day": "Fri", "room": "Gallery", "visits": 38}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "visits", "type": "quantitative", "bin": true},
    "y": {"aggregate": "count"}
  }
}`},
	{"Box Plot", "rectangle.split.3x1", `{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {"values": [
    {"day": "Mon", "room": "Gallery", "visits": 24},
    {"day": "Tue", "room": "Gallery", "visits": 37},
    {"day": "Wed", "room": "Gallery", "visits": 48},
    {"day": "Mon", "room": "Studio", "visits": 17},
    {"day": "Tue", "room": "Studio", "visits": 29}
  ]},
  "mark": {"type": "boxplot", "extent": "min-max"},
  "encoding": {
    "x": {"field": "room", "type": "nominal"},
    "y": {"field": "visits", "type": "quantitative"}
  }
}`},
	{"Bullet Chart", "gauge.with.dots.needle.33percent", `{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {"values": [
    {"room": "Gallery", "bands": [30, 60, 90], "visits": 54, "goal": 70},
    {"room": "Studio", "bands": [20, 45, 75], "visits": 38, "goal": 50}
  ]},
  "facet": {"row": {"field": "room", "type": "nominal", "header": {"labelAngle": 0, "title": null}}},
  "spec": {"layer": [
    {"mark": {"type": "bar", "color": "#e0e6eb"},
     "encoding": {"x": {"field": "bands[2]", "type": "quantitative", "title": null}}},
    {"mark": {"type": "bar", "color": "#bdcbd7"},
     "encoding": {"x": {"field": "bands[1]", "type": "quantitative"}}},
    {"mark": {"type": "bar", "color": "#91a4b5"},
     "encoding": {"x": {"field": "bands[0]", "type": "quantitative"}}},
    {"mark": {"type": "bar", "color": "#386e91", "size": 8},
     "encoding": {"x": {"field": "visits", "type": "quantitative"}}},
    {"mark": {"type": "tick", "color": "#1d2730", "thickness": 2},
     "encoding": {"x": {"field": "goal", "type": "quantitative"}}}
  ]}
}`},
}

func (chart chartCommand) markdown() string {
	return chartFenceStart + chart.source + "\n```"
}

func (app *workspaceWindow) insertChart(chart chartCommand) {
	if app.sourceEditing() {
		app.insertSourceSnippetAt(chart.markdown(), chartCaret)
		return
	}
	if app.displayedMode() == wysiwygMode && app.wys.editable(app) && app.wys.session.InsertVegaChart(chart.source) {
		app.wys.sync(app)
		app.wys.focus = true
	}
}

func chartSymbol(title string) string {
	for _, chart := range chartCommands {
		if chart.title == title {
			return chart.symbol
		}
	}
	return ""
}

var chartMenuIcons = sync.OnceValue(func() map[string]*mygo.MenuIcon {
	icons := make(map[string]*mygo.MenuIcon, len(chartCommands))
	for _, chart := range chartCommands {
		if _, cached := icons[chart.symbol]; cached {
			continue
		}
		if data := nativeMenuSymbol(chart.symbol); len(data) != 0 {
			icons[chart.symbol], _ = mygo.NewMenuIcon(data, true)
		}
		if svg := chromeIconFallbacks[chart.symbol]; icons[chart.symbol] == nil && svg != nil {
			icons[chart.symbol], _ = mygo.NewMenuIconSVG(svg, true)
		}
	}
	return icons
})

func (app *workspaceWindow) chartMenu(menu *mygoui.Menu) {
	icons := chartMenuIcons()
	for _, chart := range chartCommands {
		if menu.Item(chart.title).Icon(icons[chart.symbol]).Chosen() {
			app.insertChart(chart)
		}
	}
}
