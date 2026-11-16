package ui

import (
	"log"
	"net/url"
	"strings"

	"cyberwriter/internal/core/workspace"
	"cyberwriter/ui/wysiwyg"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/richtext"
)

type workspaceWYSIWYG struct {
	session             *wysiwyg.Session
	size                int
	pageWidth           bool
	focus               bool
	propertiesOpen      bool
	linkOpen            bool
	linkFocus           bool
	linkURL             string
	linkProblem         string
	linkCell            *visualTableCell
	find                workspaceWYSFind
	diagramEdit         workspaceDiagramEdit
	diagramLayout       bool
	queryEdit           workspaceQueryEdit
	apiEdit             workspaceAPIEdit
	queryResults        map[*wysiwyg.QueryBlock]*visualQueryResult
	queryVersion        uint64
	queryLayout         bool
	queryMetricsChanged bool
	math                visualMathState
	calloutLayout       bool
	images              visualImageCache
	imageResize         visualImageResize
	imageCaption        func()
	imageLayout         bool
	imageMetricsChanged bool
	tableLayout         bool
	tableFocus          *tableFocusRequest
	focusedCell         *visualTableCell
	tableSelection      visualTableSelection
	tableMenu           visualTableSelection
	tableHover          visualTableHandles
	tableDrag           visualTableDrag
	blockMenu           visualBlockMenu
	blockGutter         visualBlockGutter
}

func (w *workspaceWYSIWYG) reset() {
	*w = workspaceWYSIWYG{size: w.size, pageWidth: w.pageWidth}
}

func (w *workspaceWYSIWYG) sync(app *workspaceWindow) {
	if w.session != nil {
		if source := w.math.activeSource; source != nil {
			w.updateDisplayMath(source)
		}
		w.session.Sync(app.draft)
	}
}

func (w *workspaceWYSIWYG) flush(app *workspaceWindow) {
	if w.focusedCell != nil {
		w.syncVisualTableCell(app, w.focusedCell)
	}
	if w.imageCaption != nil {
		w.imageCaption()
	}
	w.finishDiagramEdit(app, true, false)
	w.finishQueryEdit(app, true, false)
	w.finishAPIEdit(app, true, false)
	w.sync(app)
}

func (w *workspaceWYSIWYG) editable(app *workspaceWindow) bool {
	return !app.modalOpen && w.session != nil && w.session.Document != nil && w.session.Current(app.draft)
}

func (w *workspaceWYSIWYG) openSession(app *workspaceWindow) {
	if w.session == nil || !w.session.Current(app.draft) {
		w.reset()
		w.session = wysiwyg.Open(app.draft)
		if w.session.Document != nil {
			w.session.Document.SetSelection(0, 0)
			w.focus = true
		}
	}
}

func (w *workspaceWYSIWYG) draw(c *mygoui.Context, app *workspaceWindow) {
	if app.draft == nil {
		return
	}
	w.openSession(app)
	if !app.focusMode {
		app.wysToolbar(c)
	}
	if w.find.open && w.session.Document != nil {
		w.drawFind(c, app)
	}
	if w.linkOpen {
		w.linkEditor(c, app)
	}

	// EV-UI-021: the host is inset 48 points and the document a further
	// 120 points. Reduce the insets when a narrow window would hide text.
	const pageMaxWidth = 916 + 2*48
	width, _ := c.Size()
	width -= 10 // The WYS pane has six and four point window gutters.
	vault, outline := app.visibleSidebars()
	if vault {
		width -= vaultPanelWidth
	}
	if outline {
		width -= outlinePanelWidth
	}
	if w.pageWidth {
		width = min(pageMaxWidth, width)
	}
	// EV-UI-142 retains both insets at a 620 point viewport, leaving 284
	// points for the document. Allow that content width at narrower sizes.
	padding := min(168, max(24, (width-284)/2))
	mygoui.Column(c.Key("wysiwyg-document")).Grow(1).MinHeight(0).FillWidth().
		AlignItems(mygoui.Center).Label("cyberWriter WYSIWYG").Children(func() {
		page := mygoui.Column(c.Key(w.session)).Grow(1).MinHeight(0).FillWidth()
		if w.pageWidth {
			page.MaxWidth(pageMaxWidth)
		}
		page.Children(func() {
			if properties := strings.TrimSpace(w.session.Properties); properties != "" {
				mygoui.Collapsible(c, "Document properties", &w.propertiesOpen, func() {
					mygoui.Scroll(c).MaxHeight(112).FillWidth().Children(func() {
						mygoui.Text(c, properties).Font("monospace").FontSize(12)
					})
				}).Margin(4, padding, 0)
			}
			if w.session.Document == nil {
				w.readOnly(c, app, padding)
				return
			}
			w.editor(c, app, padding)
		})
	})
}

// EV-UI-021 measured the body at 15 points. Heading multipliers come
// from the document styling declarations in EV-UI-011.
var wysHeadingSizes = [...]float32{2.05, 1.65, 1.4, 1.2, 1.05, 0.95}

func (w *workspaceWYSIWYG) editor(c *mygoui.Context, app *workspaceWindow, padding float32) {
	verticalPadding := float32(24)
	if app.blackout {
		// EV-UI-036 measures glyphs relative to the WYSIWYG host frame.
		verticalPadding = 72
	}
	if w.diagramLayout {
		// The RichText paragraph cache keys on the layout theme, including
		// padding. Refresh attachment metrics without adding a document edit.
		verticalPadding += 1.0 / 256
	}
	if w.tableLayout {
		verticalPadding += 1.0 / 512
	}
	if w.calloutLayout {
		verticalPadding += 1.0 / 1024
	}
	if w.imageLayout {
		verticalPadding += 1.0 / 2048
	}
	if w.queryLayout {
		verticalPadding += 1.0 / 4096
	}
	verticalPadding += float32(w.math.layoutRevision) / 65536
	theme := &richtext.Theme{
		ParagraphSpacing: 0.75, Padding: [2]float32{padding, verticalPadding},
		QuoteBar: mygoui.RGBA(127, 127, 127, 0),
	}
	for level, size := range wysHeadingSizes {
		theme.Headings[level].Size = size
		if app.blackout {
			theme.Headings[level].Weight = 400
		}
	}
	viewport := mygoui.Box(c.Key("visual viewport")).Grow(1).MinHeight(0).FillWidth().Clip()
	renderer := mygoui.Local(viewport, "formula renderer", func() workspaceMathRender { return workspaceMathRender{} })
	viewport.Children(func() {
		w.visualSurface(c, app, padding, theme, renderer)
	})
}

func (w *workspaceWYSIWYG) visualSurface(c *mygoui.Context, app *workspaceWindow, padding float32, theme *richtext.Theme, renderer *workspaceMathRender) {
	document := w.session.Document
	streaming := false
	if request := app.ai.request; request != nil && request.visualWriteback != nil {
		if preview := request.visualWriteback.previewFor(app, document); preview != nil {
			document, streaming = preview, true
		}
	}
	w.focusedCell = nil
	w.math.activeSource = nil
	w.imageCaption = nil
	w.imageMetricsChanged = false
	w.queryMetricsChanged = false
	w.retainQueryResults()
	if q := w.queryEdit; q.query != nil {
		if current, ok := w.session.QueryAt(q.offset); !ok || current != q.query {
			w.queryEdit = workspaceQueryEdit{}
		}
	}
	if a := w.apiEdit; a.block != nil {
		if current, ok := w.session.APIAt(a.offset); !ok || current != a.block {
			w.apiEdit = workspaceAPIEdit{}
		}
	}
	if colors := app.blockMenuColors(); colors != w.math.appearance {
		w.math.appearance = colors
		w.math.relayout(c)
	}
	font := mygoui.Font{Family: c.Theme().Font, Size: float32(w.fontSize())}
	edit := richtext.Editor(c.Key("visual-editor"), document, richtext.EditorOptions{
		Label: "WYSIWYG editor", Font: font,
		Theme: theme, ReadOnly: app.modalOpen || streaming,
		OnLink:        func(target string) { w.followLink(app, target) },
		OnLinkCommand: func() { w.format(app, linkFormat) },
		TagStyle:      w.visualTagStyle,
	}).Grow(1).MinHeight(0).FillWidth().Radius(0).Border(0, c.Theme().Border).
		Background(mygoui.Transparent).OnShortcut(mygoui.Cmd, mygoui.KeyBackquote, func() { w.format(app, inlineCodeFormat) })
	edit.ContextMenu(func(m *mygoui.Menu) {
		m.EditItems()
		app.snippetCaptureMenu(m)
	})
	if w.focus && !app.modalOpen {
		edit.Focus()
		w.focus = false
	}
	if streaming {
		return
	}
	text := document.Text()
	anchor, caret := document.Selection()
	viewport := edit.Bounds()
	viewportWidth := viewport.W
	boxes := document.VisibleParagraphs()
	w.drawCallouts(c, viewport, padding, text, boxes)
	w.drawFindMarks(c, app, text, boxes, viewportWidth, theme, font)
	x, y, over := edit.PointerPosition()
	pointer := mygoui.Point{X: x, Y: y}
	images := visualImageFrame{viewport: viewport, padding: padding, pointer: pointer, hovered: over,
		start: min(anchor, caret), end: max(anchor, caret), font: font}
	w.watchVisualInput(app, edit, document)
	if w.session.TableCount() > 0 {
		// A text-field AX node hides its descendants. The editor remains
		// the scroll handler for the accessible cell editors it contains.
		if anchor == caret && edit.Focused() {
			// At a table boundary, move across the block instead of joining
			// it into an ordinary paragraph. A range can still delete it.
			if _, ok := w.session.TableAt(caret - 2); ok && text.Slice(caret-1, caret).String() == "\n" {
				edit.OnShortcut(0, mygoui.KeyBackspace, func() { document.SetSelection(caret-2, caret-2) })
			}
			if _, ok := w.session.TableAt(caret + 1); ok && text.Slice(caret, caret+1).String() == "\n" {
				edit.OnShortcut(0, mygoui.KeyDelete, func() { document.SetSelection(caret+2, caret+2) })
			}
		}
	}
	if w.session.TableCount() > 0 || w.session.HasQueries() || w.session.HasAPIBlocks() {
		// Wheel events over results scroll the editor, and a group keeps
		// the query actions visible in the accessibility tree.
		edit.Role(mygoui.RoleGroup).Children(func() {
			previousHover := w.tableHover
			w.tableHover = visualTableHandles{}
			for _, box := range boxes {
				if table, ok := w.session.TableAt(box.Start); ok {
					w.drawVisualTable(c, app, padding, viewportWidth, box, table, pointer, over, previousHover)
				}
				if query, ok := w.session.QueryAt(box.Start); ok && viewportWidth > 2*padding {
					w.drawQueryCard(c, app, box, query, images)
				}
				if block, ok := w.session.APIAt(box.Start); ok && viewportWidth > 2*padding {
					w.drawAPICard(c, app, box, block, images)
				}
			}
		})
	}
	for _, box := range boxes {
		if viewportWidth > 2*padding && w.drawVisualImages(c, app, text, box, images) {
			continue
		}
		if formula, ok := wysiwyg.FormulaAt(text, box.Start); ok && formula.Block {
			w.drawDisplayFormula(c, app, renderer, padding, viewportWidth, box, formula)
			continue
		}
		if diagram, ok := w.session.DiagramAt(box.Start); ok {
			top := box.Top + diagramMargin
			hovered := over && x >= padding && x < viewportWidth-padding &&
				y >= top && y < box.Top+box.Height-diagramMargin
			selected := min(anchor, caret) <= box.Start && box.Start < max(anchor, caret)
			w.drawDiagram(c, app, padding, box, diagram, hovered || selected)
			continue
		}
		if box.Top < 0 {
			continue
		}
		if w.session.IsPageBreak(text, box.Start) {
			mygoui.Box(c.Key(box.Start)).Absolute().Left(padding).Right(padding).Top(box.Top).Height(box.Height).
				Role(mygoui.RoleSeparator).Label(wysiwyg.PageBreakLabel).IgnoreInput(true)
		}
		if w.session.IsDivider(text, box.Start) {
			mygoui.Box(c.Key(box.Start).Key("divider")).Absolute().Left(padding).Right(padding).Top(box.Top).Height(box.Height).
				Role(mygoui.RoleSeparator).Label(wysiwyg.DividerLabel).IgnoreInput(true)
		}
		// The native page-break insertion can leave an empty leading heading.
		if box.Start == 0 && text.Len() >= 2 && text.Slice(0, 1).String() == "\n" && w.session.IsPageBreak(text, 1) {
			mygoui.Text(c.Key("empty heading placeholder"), "Please enter...").Absolute().Left(padding).Top(box.Top).
				FontSize(float32(w.fontSize()) * wysHeadingSizes[0]).TextColor(c.Theme().TextMuted).Role(mygoui.RoleNone).IgnoreInput(true)
		}
	}
	if w.imageMetricsChanged {
		w.imageLayout = !w.imageLayout
	}
	if w.queryMetricsChanged {
		w.queryLayout = !w.queryLayout
	}
	w.drawInlineFormulas(c, app, edit, renderer, font, viewport, padding, text, boxes, anchor, caret)
	if w.editable(app) && !w.blockMenu.open && w.session.CanCompleteTypedCodeFence() {
		edit.OnShortcut(0, mygoui.KeyEnter, func() { w.completeVisualCodeFence(app) })
	}
	if w.editable(app) && !w.blockMenu.open && w.session.CanCalloutEnter() {
		edit.OnShortcut(0, mygoui.KeyEnter, func() {
			if w.session.CalloutEnter() {
				w.sync(app)
			}
		})
	}
	if w.editable(app) && !w.blockMenu.open && w.session.CanCalloutBackspace() {
		edit.OnShortcut(0, mygoui.KeyBackspace, func() {
			w.session.CalloutBackspace()
			w.sync(app)
		})
	}
	if !w.editable(app) || w.linkOpen || w.diagramEdit.diagram != nil || w.queryEdit.query != nil || w.apiEdit.block != nil ||
		app.quickSwitcher.open || app.commandPalette.open {
		w.closeBlockControls()
		return
	}
	w.watchBlockMenu(edit, document, text, anchor, caret)
	if w.blockGutter.drag != nil {
		edit.OnShortcut(0, mygoui.KeyEscape, func() { w.cancelBlockGutterDrag() })
	}
	if !w.drawBlockMenu(c, app, edit, padding, viewport, text, anchor, caret, boxes, theme, font) {
		w.drawBlockGutter(c, app, padding, viewport, text, boxes, pointer, over)
	}
}

func (w *workspaceWYSIWYG) completeVisualCodeFence(app *workspaceWindow) {
	if !w.editable(app) {
		return
	}
	formula, ok := w.session.CompleteTypedCodeFence()
	if !ok {
		return
	}
	if formula != nil {
		w.math.focusBlock = formula.ID
		w.math.newFence, w.math.newFenceVersion = formula, w.session.Document.Version()
	}
	w.sync(app)
}

func (w *workspaceWYSIWYG) readOnly(c *mygoui.Context, app *workspaceWindow, padding float32) {
	mygoui.Row(c).FillWidth().Shrink(0).AlignItems(mygoui.Center).Gap(8).Padding(6, 12).
		Background(c.Theme().Surface).Children(func() {
		status := "This Markdown uses features that require source editing."
		if w.session.TableCount() > 0 {
			status = "Markdown preview"
		}
		mygoui.Text(c, status).
			Role(mygoui.RoleStatus).FontSize(12).Grow(1).MinWidth(0).Wrap()
		w.tablePicker(c, app)
		mygoui.Button(c, "Edit Source").Disabled(app.modalOpen).OnClick(func() { app.setMode(sourceMode) })
	})
	app.previewPane.init(app)
	if app.previewPane.ready || app.previewPane.awaitingDOM {
		mygoui.Column(c).Grow(1).MinHeight(0).FillWidth().Label("WYSIWYG read-only preview").
			Children(func() { app.previewPane.draw(c, app) })
		return
	}
	theme := &markdown.Theme{FontSize: float32(w.fontSize())}
	for level, size := range wysHeadingSizes {
		theme.Headings[level].Size = size
	}
	mygoui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Padding(24, padding).
		Label("WYSIWYG read-only preview").Children(func() {
		markdown.View(c, w.session.Body, &markdown.Options{
			Theme: theme, Extensions: wysiwyg.Extensions, OnLink: openVisualLink,
		})
	})
}

func (w *workspaceWYSIWYG) fontSize() int {
	if w.size == 0 {
		return 15
	}
	return w.size
}

func (w *workspaceWYSIWYG) changeFontSize(delta int) {
	w.size = min(32, max(9, w.fontSize()+delta))
}

func (w *workspaceWYSIWYG) togglePageWidth() {
	w.pageWidth = !w.pageWidth
}

func (w *workspaceWYSIWYG) format(app *workspaceWindow, format markdownFormat) bool {
	if !w.editable(app) {
		return false
	}
	doc := w.session.Document
	if cell := w.focusedCell; cell != nil {
		doc = cell.document
	}
	switch format {
	case boldFormat:
		doc.ToggleBold()
	case italicFormat:
		doc.ToggleItalic()
	case strikethroughFormat:
		doc.ToggleStrikethrough()
	case inlineCodeFormat:
		doc.ToggleCode()
	case highlightFormat:
		if !wysiwyg.ToggleHighlight(doc) {
			return false
		}
	case linkFormat:
		w.linkCell = w.focusedCell
		w.linkURL = doc.Formatting().Link
		w.linkOpen, w.linkFocus, w.focus = true, true, false
		w.linkProblem = ""
		return true
	default:
		return false
	}
	if cell := w.focusedCell; cell != nil {
		w.syncVisualTableCell(app, cell)
		return true
	}
	w.sync(app)
	w.focus = true
	return true
}

func (w *workspaceWYSIWYG) linkEditor(c *mygoui.Context, app *workspaceWindow) {
	closeEditor := func() {
		w.linkOpen, w.focus = false, true
		if w.linkCell != nil {
			w.tableFocus = &tableFocusRequest{visualTableFocus: w.linkCell.visualTableFocus, selection: selectCellText}
			w.focus = false
		}
		w.linkCell = nil
	}
	apply := func() {
		if !w.editable(app) {
			return
		}
		target := strings.TrimSpace(w.linkURL)
		dest, err := url.Parse(target)
		if err != nil || dest.Host != "" && dest.Scheme == "" ||
			dest.Scheme != "" && previewExternalURL(target) == "" {
			w.linkProblem = "Enter a web address or a relative Markdown link."
			return
		}
		if cell := w.linkCell; cell != nil {
			cell.document.SetLink(target)
			w.syncVisualTableCell(app, cell)
		} else {
			w.session.Document.SetLink(target)
			w.sync(app)
		}
		closeEditor()
	}
	mygoui.Row(c.Key("visual-link")).FillWidth().Shrink(0).Padding(6, 8).Gap(6).
		AlignItems(mygoui.Center).Label("Edit link").Children(func() {
		input := mygoui.TextInput(c, &w.linkURL).Label("Link destination").Placeholder("Link destination").
			Grow(1).MinWidth(0).OnSubmit(apply).OnShortcut(0, mygoui.KeyEscape, closeEditor)
		if w.linkFocus {
			input.Focus()
			w.linkFocus = false
		}
		mygoui.Button(c, "Apply").OnClick(apply)
		mygoui.Button(c, "Cancel").OnClick(closeEditor)
	})
	if w.linkProblem != "" {
		mygoui.Text(c, w.linkProblem).Role(mygoui.RoleAlert).TextColor(c.Theme().Danger).Padding(0, 8, 6)
	}
}

func (w *workspaceWYSIWYG) followLink(app *workspaceWindow, address string) {
	if fragment, ok := strings.CutPrefix(address, "#"); ok && w.session.Document != nil {
		target, err := url.PathUnescape(fragment)
		if err != nil {
			return
		}
		doc := w.session.Document
		if target == "" {
			doc.SetSelection(0, 0)
			return
		}
		for offset, paragraph := range doc.Text().Paragraphs() {
			if paragraph.Style.Heading == 0 {
				continue
			}
			var title strings.Builder
			for _, run := range paragraph.Runs {
				title.WriteString(run.Text)
			}
			if workspace.HeadingAnchor(title.String()) == target {
				doc.SetSelection(offset, offset)
				break
			}
		}
		return
	}
	if target, ok := w.session.WikilinkTarget(address); ok {
		origin := app.previewOrigin()
		if origin.index != nil {
			if relative, resolved := origin.index.Resolve(origin.note, target); resolved {
				_, section, _ := strings.Cut(target, "#")
				app.openNoteAt(relative, true, markdown.Slug(section))
			}
		}
		return
	}
	openVisualLink(address)
}

func openVisualLink(target string) {
	if address := previewExternalURL(target); address != "" {
		go func() {
			if err := mygo.Shell.OpenExternal(address); err != nil {
				log.Printf("Open WYSIWYG link: %v", err)
			}
		}()
	}
}
