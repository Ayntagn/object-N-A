package ui

import (
	"fmt"
	"html"
	"sync"

	"cyberwriter/ui/wysiwyg"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

const queryTemplateSource = "TABLE role\nFROM #person\nSORT role ASC"
const queryTemplate = "```query\n" + queryTemplateSource + "\n```"
const queryMenuSymbol = "tablecells.badge.ellipsis"
const queryFirstColumn = "File"
const queryNoResults = "No matching notes"
const queryInvalid = "⚠︎ Invalid query"

func (app *workspaceWindow) insertQueryBlock() {
	if app.sourceEditing() {
		app.insertSourceSnippetAt(queryTemplate, len("```query\n"))
		return
	}
	if app.displayedMode() == wysiwygMode && app.wys.editable(app) && app.wys.session.InsertQuery(queryTemplateSource) {
		app.wys.sync(app)
		app.wys.focus = true
	}
}

func (app *workspaceWindow) liveDataMenu(m *mygoui.Menu) {
	item := m.Item(wysiwyg.QueryBlockLabel).Icon(queryMenuIcon())
	if item.Chosen() {
		app.insertQueryBlock()
	}
	if m.Item(apiRequestLabel).Chosen() {
		app.insertAPIRequest()
	}
}

var queryMenuIcon = sync.OnceValue(func() *mygo.MenuIcon {
	if data := nativeMenuSymbol(queryMenuSymbol); len(data) != 0 {
		if icon, err := mygo.NewMenuIcon(data, true); err == nil {
			return icon
		}
	}
	icon, _ := mygo.NewMenuIconSVG(chromeIconFallbacks[queryMenuSymbol], true)
	return icon
})

// EV-UI-159: Markdown Preview and WYSIWYG use the same saved Vault
// snapshot. The Preview link uses the normal in-app note navigation.
func (m *previewMarkup) writeQueryBlock(source, lineAttribute string) {
	result, err := m.queryIndex.Query(source)
	switch {
	case err != nil:
		fmt.Fprintf(&m.out, `<div class="query-block query-error"%s>%s</div>`, lineAttribute, queryInvalid)
	case len(result.Rows) == 0:
		fmt.Fprintf(&m.out, `<div class="query-block query-empty"%s><em>%s</em></div>`, lineAttribute, queryNoResults)
	case result.Columns == nil:
		fmt.Fprintf(&m.out, `<ul class="query-block query-list"%s>`, lineAttribute)
		for _, row := range result.Rows {
			m.out.WriteString("<li>")
			m.writeQueryLink(row.Path, row.Name)
			m.out.WriteString("</li>")
		}
		m.out.WriteString("</ul>")
	default:
		fmt.Fprintf(&m.out, `<table class="query-block query-table"%s><thead><tr><th>%s</th>`, lineAttribute, queryFirstColumn)
		for _, field := range result.Columns {
			fmt.Fprintf(&m.out, "<th>%s</th>", html.EscapeString(field))
		}
		m.out.WriteString("</tr></thead><tbody>")
		for _, row := range result.Rows {
			m.out.WriteString("<tr><td>")
			m.writeQueryLink(row.Path, row.Name)
			m.out.WriteString("</td>")
			for _, cell := range row.Cells {
				fmt.Fprintf(&m.out, "<td>%s</td>", html.EscapeString(cell))
			}
			m.out.WriteString("</tr>")
		}
		m.out.WriteString("</tbody></table>")
	}
}

func (m *previewMarkup) writeQueryLink(path, name string) {
	fmt.Fprintf(&m.out, `<a class="wikilink" href="%s">%s</a>`, html.EscapeString(previewNoteURL("", path, "")), html.EscapeString(name))
}
