package ui

import (
	"fmt"
	"html"
	"slices"
	"strings"

	"cyberwriter/internal/core/exportdoc"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/richtext"
)

func (app *workspaceWindow) sourceEditing() bool {
	mode := app.displayedMode()
	return !app.modalOpen && app.code != nil && !app.code.ReadOnly && (mode == sourceMode || mode == splitMode)
}

// EV-UI-102: a command can insert into the active selection without moving focus.
func (app *workspaceWindow) insertMarkdownAtSelection(markdown string) bool {
	switch app.displayedMode() {
	case sourceMode, splitMode:
		if !app.sourceEditing() {
			return false
		}
		selection := app.code.Selection()
		start, end := selection.Start(), selection.End()
		if start != end {
			app.code.Select(start, end)
			app.code.Insert(markdown)
		} else {
			app.draft.Edit(editor.Edit{Start: start, End: end, Text: markdown})
			caret := start + len(markdown)
			app.code.Select(caret, caret)
		}
		return true
	case wysiwygMode:
		if !app.wys.editable(app) || !app.wys.session.InsertMarkdown(markdown) {
			return false
		}
		app.wys.sync(app)
		return true
	case previewMode:
		// The native terminal acknowledges the command while the editor is absent.
		return true
	}
	return false
}

func (app *workspaceWindow) prefixLines(prefix string) {
	if !app.sourceEditing() {
		return
	}
	source := app.draft.String()
	var starts []int
	for _, selected := range app.code.Selections() {
		start := strings.LastIndexByte(source[:selected.Start()], '\n') + 1
		for offset := start; ; {
			starts = append(starts, offset)
			lineEnd := strings.IndexByte(source[offset:], '\n')
			if lineEnd < 0 {
				break
			}
			offset += lineEnd + 1
			if offset >= selected.End() {
				break
			}
		}
	}
	slices.Sort(starts)
	edits := make([]editor.Edit, 0, len(starts))
	for _, start := range slices.Compact(starts) {
		end := start
		text := prefix
		if strings.HasPrefix(source[start:], prefix) {
			end += len(prefix)
			text = ""
		} else if strings.HasPrefix(prefix, "#") {
			// Replace a leading ATX heading marker when changing its level.
			for end < len(source) && end-start < 6 && source[end] == '#' {
				end++
			}
			if end > start && end < len(source) && source[end] == ' ' {
				end++
			} else {
				end = start
			}
		}
		edits = append(edits, editor.Edit{Start: start, End: end, Text: text})
	}
	app.draft.Edit(edits...)
	app.focusSource = true
}

func (app *workspaceWindow) applyBlock(prefix string, rich func(*richtext.Document)) {
	if app.mode == wysiwygMode && app.wys.editable(app) {
		rich(app.wys.session.Document)
		app.wys.sync(app)
		app.wys.focus = true
		return
	}
	app.prefixLines(prefix)
}

func (app *workspaceWindow) insertSourceBlock(leading, fallback, trailing string) {
	if !app.sourceEditing() {
		return
	}
	selections := app.code.Selections()
	snapshot := app.draft.Snapshot()
	edits := make([]editor.Edit, 0, len(selections))
	for _, selection := range selections {
		start, end := selection.Start(), selection.End()
		body := snapshot.Slice(start, end)
		if body == "" {
			body = fallback
		} else if fallback != "" {
			body = fallback + "\n\n" + body
		}
		text := leading + body + trailing
		if start > 0 {
			text = "\n\n" + text
		}
		if end < snapshot.Len() {
			text += "\n\n"
		}
		edits = append(edits, editor.Edit{Start: start, End: end, Text: text})
	}
	app.draft.Edit(edits...)
	app.focusSource = true
}

func (app *workspaceWindow) insertImage() {
	if app.sourceEditing() {
		description := escapeLinkLabel.Replace(app.code.SelectedText())
		caret := 2
		if description == "" {
			description = "alt text"
		} else {
			caret = len(description) + 4
		}
		app.insertSourceSnippetAt("!["+description+"](image-url)", caret)
		return
	}
	if app.displayedMode() == wysiwygMode && app.wys.editable(app) {
		app.wys.flush(app)
		if !app.wys.session.InsertImage("image-url", true) {
			return
		}
		app.wys.sync(app)
		app.wys.focus = true
	}
}

func (app *workspaceWindow) insertFootnote() {
	if !app.sourceEditing() {
		return
	}
	source := app.draft.String()
	for i := 1; ; i++ {
		ref := fmt.Sprintf("[^%d]", i)
		if strings.Contains(source, ref) {
			continue
		}
		end := app.code.Selection().End()
		marker := editor.Edit{Start: end, End: end, Text: ref}
		definition := "\n\n" + ref + ": Footnote text\n"
		if end == len(source) {
			marker.Text += definition
			app.draft.Edit(marker)
		} else {
			app.draft.Edit(marker, editor.Edit{Start: len(source), End: len(source), Text: definition})
		}
		app.focusSource = true
		return
	}
}

type blockCommand struct {
	label  string
	action func()
	source bool
}

const (
	insertTableLabel    = "Table"
	insertContentsLabel = "Table of Contents"
)

func (app *workspaceWindow) insertionCommands() []blockCommand {
	return []blockCommand{
		{"Bullet List", app.bulletList, false},
		{"Numbered List", app.numberedList, false},
		{"Task List", func() { app.applyBlock("- [ ] ", func(d *richtext.Document) { d.ToggleList(richtext.Tasks) }) }, false},
		{"Blockquote", func() { app.applyBlock("> ", (*richtext.Document).ToggleQuote) }, false},
		{"Code Block", app.insertCodeBlock, false},
		{"Horizontal Rule", func() { app.insertSourceBlock("---\n", "", "") }, true},
		{"Page Break", app.insertPageBreak, false},
		{insertTableLabel, app.insertTable, true},
		{"Footnote", app.insertFootnote, true},
		{insertContentsLabel, app.insertContents, false},
	}
}

func (app *workspaceWindow) insertPageBreak() {
	if app.displayedMode() == wysiwygMode && app.wys.editable(app) {
		app.wys.session.InsertPageBreak()
		app.wys.sync(app)
		app.wys.focus = true
		return
	}
	if app.sourceEditing() {
		app.insertMarkdownAtSelection("\n" + exportdoc.PageBreakMarker + "\n")
		app.focusSource = true
	}
}

func (app *workspaceWindow) insertContents() {
	mode := app.displayedMode()
	if !app.sourceEditing() && (mode != wysiwygMode || !app.wys.editable(app)) {
		return
	}
	app.wys.sync(app)
	if app.insertMarkdownAtSelection(workspace.MarkdownContents(app.workingText())) {
		app.focusSource = mode != wysiwygMode
		app.wys.focus = mode == wysiwygMode
	}
}

// EV-UI-144: the Insert command and Add menu share the fixed example.
const (
	exampleTableHeader = "| Header 1 | Header 2 | Header 3 |\n"
	exampleTableBody   = "\n| Cell 1   | Cell 2   | Cell 3   |\n| Cell 4   | Cell 5   | Cell 6   |"
)

func (app *workspaceWindow) insertTable() {
	if app.displayedMode() == wysiwygMode {
		const markdown = exampleTableHeader + "| -------- | -------- | -------- |" + exampleTableBody
		if !app.wys.insertVisualTable(app, markdown, true) {
			app.wys.openTableTemplate(app, markdown)
		}
		return
	}
	app.insertSourceTable(exampleTableHeader+"|----------|----------|----------|"+exampleTableBody, 2)
}

func (app *workspaceWindow) insertSourceTable(markdown string, caretOffset int) {
	if !app.sourceEditing() {
		return
	}
	selection := app.code.Selection()
	start := selection.Start()
	app.draft.Edit(editor.Edit{Start: start, End: selection.End(), Text: markdown})
	caret := start + caretOffset
	app.code.Select(caret, caret)
	app.focusSource = true
}

func (app *workspaceWindow) bulletList() {
	app.applyBlock("- ", func(d *richtext.Document) { d.ToggleList(richtext.Bullets) })
}

func (app *workspaceWindow) numberedList() {
	app.applyBlock("1. ", func(d *richtext.Document) { d.ToggleList(richtext.Numbers) })
}

func (app *workspaceWindow) insertCodeBlock() {
	if app.mode == wysiwygMode && app.wys.editable(app) {
		app.wys.session.Document.ToggleCodeBlock()
		app.wys.sync(app)
		app.wys.focus = true
		return
	}
	if app.sourceEditing() {
		fence := "```"
		for strings.Contains(app.code.SelectedText(), fence) {
			fence += "`"
		}
		app.insertSourceBlock(fence+"\n", "", "\n"+fence+"\n")
	}
}

func (app *workspaceWindow) heading(level int) {
	app.applyBlock(strings.Repeat("#", level)+" ", func(d *richtext.Document) { d.SetHeading(level) })
}

func (app *workspaceWindow) headingMenu(m *mygoui.Menu) {
	for level := 1; level <= 6; level++ {
		if m.Item(fmt.Sprintf("Heading %d", level)).Chosen() {
			app.heading(level)
		}
	}
}

func (app *workspaceWindow) insertMenu(m *mygoui.Menu) {
	source := app.sourceEditing()
	structure := source || (app.mode == wysiwygMode && app.wys.editable(app))
	m.Submenu("Headings", app.headingMenu).Disabled(!structure)
	m.Submenu("Links & Media", app.linksAndMediaMenu).Disabled(!structure)
	m.Separator()
	for _, item := range app.insertionCommands() {
		enabled := source || (structure && !item.source)
		if item.label == insertTableLabel {
			enabled = source || app.wys.canInsertTable(app)
		}
		if m.Item(item.label).Disabled(!enabled).Chosen() {
			item.action()
		}
	}
	m.Separator()
	m.Submenu("Diagrams", app.diagramMenu).Disabled(!structure)
	m.Submenu("Math", app.mathMenu).Disabled(!structure)
	if structure {
		m.Submenu("Live Data", app.liveDataMenu)
	}
	m.Submenu("Charts", app.chartMenu).Disabled(!structure)
}

func (app *workspaceWindow) nativeInsertMenu() []*mygo.MenuItem {
	items := make([]*mygo.MenuItem, 0, 16)
	var headings []*mygo.MenuItem
	for level := 1; level <= 6; level++ {
		headings = append(headings, &mygo.MenuItem{
			Label: fmt.Sprintf("Heading %d", level), Accelerator: headingShortcut(level),
			Click: app.menuCommand(func() { app.heading(level) }),
		})
	}
	items = append(items, &mygo.MenuItem{Label: "Headings", Submenu: headings})
	items = append(items, &mygo.MenuItem{Label: "Links & Media", Submenu: []*mygo.MenuItem{
		{Label: "Link", Click: app.menuCommand(app.insertLinkTemplate)},
		{Label: "Image", Accelerator: "CmdOrCtrl+Shift+K", Click: app.menuCommand(app.insertImage)},
		{Label: "HTML Image", Click: app.menuCommand(func() { app.insertHTMLImage(false) })},
		{Label: "HTML Image (Sized)", Click: app.menuCommand(func() { app.insertHTMLImage(true) })},
	}})
	for _, entry := range formattingCommands {
		if entry.format == "" {
			continue
		}
		items = append(items, &mygo.MenuItem{Label: entry.label, Click: app.menuCommand(func() {
			if app.mode == wysiwygMode {
				app.wys.format(app, entry.format)
			} else {
				app.applyFormatting(entry.format)
			}
		})})
	}
	for _, item := range app.insertionCommands() {
		entry := &mygo.MenuItem{Label: item.label, Click: app.menuCommand(item.action)}
		if item.label == insertContentsLabel {
			entry.Accelerator = "CmdOrCtrl+Shift+T"
		}
		items = append(items, entry)
	}
	items = append(items, app.nativeDiagramMenu())
	return items
}

func (app *workspaceWindow) insertLinkTemplate() {
	if app.displayedMode() == wysiwygMode && app.wys.editable(app) {
		app.wys.format(app, linkFormat)
	} else if app.sourceEditing() {
		app.applyFormatting(linkFormat)
	}
}

func (app *workspaceWindow) linksAndMediaMenu(m *mygoui.Menu) {
	if m.Item("Link").Chosen() {
		app.insertLinkTemplate()
	}
	if m.Item("Image").Shortcut(mygoui.Cmd|mygoui.Shift, mygoui.KeyK).Chosen() {
		app.insertImage()
	}
	if m.Item("HTML Image").Disabled(!app.sourceEditing()).Chosen() {
		app.insertHTMLImage(false)
	}
	if m.Item("HTML Image (Sized)").Disabled(!app.sourceEditing()).Chosen() {
		app.insertHTMLImage(true)
	}
}

func (app *workspaceWindow) insertHTMLImage(sized bool) {
	if !app.sourceEditing() {
		return
	}
	source := app.code.SelectedText()
	if source == "" {
		source = "image.png"
	}
	size := ""
	if sized {
		size = ` width="300" height="200"`
	}
	app.insertSourceSnippetAt(`<img src="`+html.EscapeString(source)+`"`+size+` alt="description">`, len(`<img src="`))
}
