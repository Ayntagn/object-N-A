package ui

import (
	"strings"
	"sync"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type diagramCommand struct {
	title, symbol string
	caret         int
	source        string
}

// EV-UI-120/121: menu order and source selection offsets. The examples
// are written here and share the fence syntax used by preview and WYSIWYG.
var diagramCommands = [...]diagramCommand{
	{"Flowchart", "arrow.triangle.branch", 25, `graph TD
    A[Open request] --> B{Review complete?}
    B -->|Yes| C[Send response]
    B -->|No| D[Ask for details]
    C --> E[Close request]
    D --> E`},
	{"Sequence Diagram", "arrow.left.arrow.right", 27, `sequenceDiagram
    participant R as Reader
    participant L as Library
    R->>L: Find available copies
    L-->>R: Show results
    R->>L: Reserve a copy
    L-->>R: Confirm reservation`},
	{"Class Diagram", "square.stack.3d.up", 25, `classDiagram
    class Publication {
        +String title
        +describe()
    }
    class Journal {
        +int volume
    }
    Publication <|-- Journal`},
	{"State Diagram", "circle.hexagongrid", 28, `stateDiagram-v2
    [*] --> Draft
    Draft --> Review : Submit
    Review --> Draft : Request changes
    Review --> Published : Accept
    Published --> [*]`},
	{"ER Diagram", "tablecells", 22, `erDiagram
    LIBRARY ||--o{ LOAN : records
    LOAN }o--|| COPY : references
    LIBRARY {
        int id
        string name
    }
    COPY {
        int barcode
        string title
    }`},
	{"Gantt Chart", "chart.bar.xaxis", 18, `gantt
    title Exhibition preparation
    dateFormat YYYY-MM-DD
    section Planning
        Choose works :choose, 2026-02-02, 5d
        Write captions :after choose, 7d
    section Installation
        Set up room :2026-02-18, 4d`},
	{"Pie Chart", "chart.pie", 16, `pie title Reading by genre
    "Fiction" : 45
    "History" : 25
    "Science" : 18
    "Poetry" : 12`},
	{"Mind Map", "point.3.filled.connected.trianglepath.dotted", 20, `mindmap
  root((Community garden))
    Plants
      Herbs
      Vegetables
    Water
      Collection
      Irrigation
    Volunteers`},
	{"Timeline", "timeline.selection", 22, `timeline
    title Archive project
    March : Collect recordings
          : Check permissions
    April : Describe the collection
    May : Open the archive`},
	{"Git Graph", "arrow.triangle.branch", 22, `gitGraph:
    commit id:"Setup"
    branch catalog
    checkout catalog
    commit id:"Add entries"
    checkout main
    commit id:"Add README"
    merge catalog`},
	{"XY Chart", "chart.xyaxis.line", 25, `xychart-beta
    title "Library visits"
    x-axis [Mon, Tue, Wed, Thu, Fri]
    y-axis "Visitors" 0 --> 80
    bar [20, 35, 28, 50, 64]
    line [22, 30, 34, 46, 58]`},
	{"C4 Diagram", "square.3.layers.3d", 22, `C4Context
    title Museum booking
    Person(visitor, "Visitor", "Reserves a museum visit")
    System(booking, "Booking", "Manages time slots")
    System_Ext(calendar, "Calendar", "Lists opening dates")
    Rel(visitor, booking, "Books")
    Rel(booking, calendar, "Reads dates")`},
	{"Sankey Diagram", "arrow.right.arrow.left", 23, `sankey-beta

North hall,Gallery,24
North hall,Auditorium,12
South hall,Gallery,18
South hall,Workshop,16`},
	{"Quadrant Chart", "square.split.2x2", 27, `quadrantChart
    title Workshop planning
    x-axis Less preparation --> More preparation
    y-axis Small audience --> Large audience
    quadrant-1 Plan early
    quadrant-2 Run often
    quadrant-3 Try once
    quadrant-4 Refine
    Reading club: [0.25, 0.65]
    Print workshop: [0.72, 0.56]`},
	{"Kanban Board", "rectangle.split.3x1", 19, `kanban
    waiting[Waiting]
        captions[Write captions]
        labels[Check labels]
    working[In progress]
        photos[Take photos]
    ready[Ready]
        plan[Approve floor plan]`},
	{"Tree Map", "rectangle.split.2x2.fill", 25, `treemap-beta
"Collection"
    "Paintings": 42
    "Prints": 31
    "Sculpture": 19
"Archive"
    "Photographs": 34
    "Letters": 23
    "Posters": 15`},
}

func (diagram diagramCommand) markdown() string {
	return "```mermaid\n" + diagram.source + "\n```"
}

var diagramSnippetEscapes = strings.NewReplacer(`\`, `\\`, "$", `\$`)

func (app *workspaceWindow) insertDiagram(diagram diagramCommand) {
	if app.sourceEditing() {
		app.insertSourceSnippetAt(diagram.markdown(), diagram.caret)
		return
	}
	if app.displayedMode() == wysiwygMode && app.wys.editable(app) {
		if app.wys.session.InsertMarkdown(diagram.markdown()) {
			app.wys.sync(app)
			app.wys.focus = true
		}
	}
}

func (app *workspaceWindow) insertSourceSnippetAt(markdown string, caret int) {
	snippet, err := editor.ParseSnippet(diagramSnippetEscapes.Replace(markdown[:caret]) +
		"${0}" + diagramSnippetEscapes.Replace(markdown[caret:]))
	if err == nil && app.code.InsertSnippet(snippet) {
		app.focusSource = true
	}
}

func diagramSymbol(title string) string {
	for _, diagram := range diagramCommands {
		if diagram.title == title {
			return diagram.symbol
		}
	}
	return "point.3.connected.trianglepath.dotted"
}

var diagramMenuIcons = sync.OnceValue(func() map[string]*mygo.MenuIcon {
	icons := make(map[string]*mygo.MenuIcon)
	for _, diagram := range diagramCommands {
		if icons[diagram.symbol] != nil {
			continue
		}
		var icon *mygo.MenuIcon
		if data := nativeMenuSymbol(diagram.symbol); len(data) != 0 {
			icon, _ = mygo.NewMenuIcon(data, true)
		}
		if icon == nil && chromeIconFallbacks[diagram.symbol] != nil {
			icon, _ = mygo.NewMenuIconSVG(chromeIconFallbacks[diagram.symbol], true)
		}
		icons[diagram.symbol] = icon
	}
	return icons
})

func (app *workspaceWindow) diagramMenu(m *mygoui.Menu) {
	icons := diagramMenuIcons()
	for _, diagram := range diagramCommands {
		if m.Item(diagram.title).Icon(icons[diagram.symbol]).Chosen() {
			app.insertDiagram(diagram)
		}
	}
}

func (app *workspaceWindow) nativeDiagramMenu() *mygo.MenuItem {
	menu := &mygo.MenuItem{Label: "Diagrams"}
	icons := diagramMenuIcons()
	for _, diagram := range diagramCommands {
		menu.Submenu = append(menu.Submenu, &mygo.MenuItem{
			Label: diagram.title, Icon: icons[diagram.symbol],
			Click: app.menuCommand(func() { app.insertDiagram(diagram) }),
		})
	}
	return menu
}
