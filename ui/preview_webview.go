package ui

import (
	"fmt"
	"html"
	"log"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"cyberwriter/internal/core/mermaid"
	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	"cyberwriter/ui/wysiwyg"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const previewChangeEvent = "cyberwriter:preview-change"

var previewChanged = mygo.NewEvent[previewFrame](previewChangeEvent)

type previewFrame struct {
	Revision      uint64  `json:"revision"`
	Document      uint64  `json:"document"`
	Title         string  `json:"title"`
	Label         string  `json:"label,omitempty"`
	HTML          string  `json:"html"`
	Theme         string  `json:"theme"`
	ThemeFonts    string  `json:"themeFonts,omitempty"`
	CustomCSS     string  `json:"customCSS,omitempty"`
	LineSpacing   float64 `json:"lineSpacing"`
	ScrollPastEnd bool    `json:"scrollPastEnd"`
	Anchor        string  `json:"anchor,omitempty"`
}

type workspacePreview struct {
	view             *previewViewport
	html             workspaceHTMLPreview
	ready            bool
	refreshPending   bool
	mermaidRequested bool
	vegaRequested    bool
	chartFiles       *previewChartFiles
	zoom             int
	theme            string
	customCSS        string
	lineSpacing      float64
	scrollPastEnd    bool
	numberFigures    bool
	sourceScroll     sourceScrollPosition
	tab              *workspaceTab
	draft            *editor.Buffer
	path, source     string
	kind             documentType
	parsed           *parser.Document
	queryIndex       *vaultindex.Index
	queryVault       *workspace.Vault
	document         uint64
	revision         uint64
	apiEnabled       bool
	apiVersion       uint64
	apiDocument      uint64
}

func (p *workspacePreview) init(app *workspaceWindow) {
	win := app.win
	if win == nil || win.IsDestroyed() || !mygo.WebViewAvailable() {
		return
	}
	view, err := newPreviewViewport(win, mygo.WebViewOptions{
		Transparent: true,
		Page: mygo.PageOptions{
			Untrusted:     true,
			PreloadScript: previewScript(),
		},
	})
	if err != nil {
		log.Printf("Markdown Preview WebView: %v", err)
		return
	}
	p.view = view
	page := view.Page()
	markUnready := func() {
		if p.view == view && p.ready {
			p.ready = false
			win.Invalidate()
		}
	}
	page.OnDidNavigate(func(string) { markUnready() })
	page.OnRenderProcessGone(func(string) { markUnready() })
	page.OnDOMReady(func() {
		if p.view != view {
			return
		}
		p.ready = true
		p.zoom = 0
		p.refreshPending = true
		p.mermaidRequested = false
		p.vegaRequested = false
		win.Invalidate()
	})
	page.OnDidFinishLoad(func() {
		if p.view == view {
			p.zoom = 0
			win.Invalidate()
		}
	})
	page.OnWillNavigate(func(e *mygo.NavigateEvent) {
		e.PreventDefault()
		if !e.UserInitiated || p.handleAPINavigation(app, view, e.URL) || p.handleSnippetNavigation(app, view, e.URL) {
			return
		}
		if relative, anchor, currentTab, ok := previewNoteNavigation(e.URL); ok {
			vault := app.vault
			tab := p.tab
			win.Update(func() {
				if !currentTab || app.workspaceTab == tab && app.vault == vault && app.document.Path == workspace.VaultHealthFile {
					app.openNoteAt(relative, !currentTab, anchor)
				}
			})
			return
		}
		openPreviewExternalLink(e.URL)
	})
	page.LoadHTML(previewPage, "")
	win.Invalidate()
}

func (p *workspacePreview) refresh() {
	p.refreshPending = true
	p.html.tab = nil
	p.mermaidRequested = false
	p.vegaRequested = false
	if p.view != nil && !p.view.IsDestroyed() && !p.ready {
		p.view.Page().LoadHTML(previewPage, "")
	}
}

// EV-UI-060: Return to preview recreates the document's preview host.
func (p *workspacePreview) returnToDocument(app *workspaceWindow) {
	view := p.view
	p.html.close()
	*p = workspacePreview{}
	if view != nil && !view.IsDestroyed() {
		view.Destroy()
	}
	p.init(app)
}

func (p *workspacePreview) draw(c *mygoui.Context, app *workspaceWindow) {
	text := app.workingText()
	spacing := previewLineSpacing(text, app.preferences.Export.spacing())
	theme := app.resolvePreviewTheme(app.previewThemeSelection())
	zoom := normalizedPreviewZoom(app.previewZoom)
	kind := app.previewDocumentType()
	label := documentTypeLabels[kind] + " Preview"
	filename := app.document.Path
	if app.inspection {
		filename = ""
	}
	badge := sourceFileBadge(filename, documentTypeLabels[kind])
	if kind == documentHTML {
		p.chartFiles = nil
		p.html.draw(c, app, text, zoom)
		return
	}
	p.html.close()
	if p.view == nil || p.view.IsDestroyed() || !p.ready {
		if kind != documentMarkdown {
			drawSourceFilePreview(c, label, text, badge, zoom)
			return
		}
		mygoui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Label("Markdown Preview").Padding(24).
			Background(mygoui.Hex(theme.BackgroundColor)).Children(func() {
			markdown.View(c, text, &markdown.Options{
				Extensions: parser.GFM,
				Theme:      theme.markdownTheme(zoom, spacing),
			})
		})
		return
	}

	if p.zoom != zoom {
		p.zoom = zoom
		p.view.setZoom(zoom)
	}
	var customCSS string
	if kind == documentMarkdown {
		customCSS = app.activeCustomStylesheet()
	}
	switched := p.tab != app.workspaceTab || p.draft != app.draft || p.path != app.document.Path || p.kind != kind
	if switched {
		p.tab, p.draft, p.path, p.kind = app.workspaceTab, app.draft, app.document.Path, kind
		p.apiDocument = 0
		if kind == documentMarkdown {
			p.parsed = parser.New(webPreviewExtensions)
		}
		p.document++
	}
	// A background index publication can refresh resolved links in this
	// pane. Queries take a new snapshot on a document render or explicit reload.
	if switched || p.source != text || p.queryIndex == nil || p.queryVault != app.vault {
		p.queryVault, p.queryIndex = app.vault, app.index.data
	}
	apiEnabled := kind == documentMarkdown && app.previewAPIEnabled()
	if switched || p.refreshPending || p.source != text || p.theme != theme.ID || p.customCSS != customCSS ||
		p.lineSpacing != spacing || p.scrollPastEnd != app.preferences.ScrollPastEnd ||
		p.numberFigures != app.preferences.NumberFigures || app.previewAnchor != "" ||
		p.apiEnabled != apiEnabled || (apiEnabled && p.apiVersion != app.textVersion) {
		var themeFonts string
		if p.theme != theme.ID || p.refreshPending {
			themeFonts = previewThemeFonts(theme.ID)
		}
		p.refreshPending = false
		p.source, p.theme, p.customCSS = text, theme.ID, customCSS
		p.lineSpacing = spacing
		p.scrollPastEnd = app.preferences.ScrollPastEnd
		p.numberFigures = app.preferences.NumberFigures
		p.apiEnabled, p.apiVersion = apiEnabled, app.textVersion
		p.chartFiles = nil
		var body string
		title := app.title()
		if kind == documentMarkdown {
			resources := app.previewOrigin().resources()
			if resources != nil && app.storage == nil && p.path == workspace.VaultHealthFile && vaultindex.IsGeneratedHealthReport(text) {
				resources.resolveReportNote = vaultindex.HealthReportNoteResolver(app.entries)
			}
			figures := figureRender{numbered: p.numberFigures}
			markup := previewMarkup{doc: p.parsed, resources: resources, relative: p.path, charts: true,
				figures: &figures, liveQueries: true, queryIndex: p.queryIndex, liveAPIFences: true,
				snippets: loadPreviewSnippets(text, theme.dark())}
			if apiEnabled {
				if p.apiDocument == 0 {
					p.apiDocument = newPreviewAPIDocument()
				}
				markup.apiActions = &previewAPIActions{version: p.apiVersion, document: p.apiDocument}
			}
			body, title = markup.render(text, title, true)
			if resources != nil {
				p.chartFiles = resources.chartFiles
			}
		} else if table, ok := renderDelimitedPreview(text, filename, theme.dark()); ok {
			body = table
		} else {
			body = sourceFilePreview(text, badge, theme.dark())
		}
		p.revision++
		frame := previewFrame{Revision: p.revision, Document: p.document, Title: title, Label: label, HTML: body,
			Theme: p.theme, ThemeFonts: themeFonts, CustomCSS: customCSS, LineSpacing: spacing,
			ScrollPastEnd: p.scrollPastEnd, Anchor: app.previewAnchor}
		if err := previewChanged.EmitPage(p.view.Page(), frame); err != nil {
			p.refreshPending = true
			log.Printf("Update Markdown Preview: %v", err)
		} else {
			app.previewAnchor = ""
			if !p.mermaidRequested && strings.Contains(body, mermaidBlockStart) {
				p.mermaidRequested = true
				go loadPreviewAsset(p.view.Page(), "Mermaid", mermaidLoader)
			}
			if !p.vegaRequested && strings.Contains(body, vegaBlockMarker) {
				p.vegaRequested = true
				go loadPreviewAsset(p.view.Page(), "Vega", vegaLoader)
			}
		}
	}
	mygoui.WebView(c, p.view).Grow(1).MinWidth(0).MinHeight(0).Label(label)
}

func previewExternalURL(dest string) string {
	if !mygoui.SafeToOpen(dest) {
		return ""
	}
	target, err := url.Parse(dest)
	if err != nil {
		return ""
	}
	switch target.Scheme {
	case "http", "https":
		if target.Hostname() != "" && target.User == nil {
			return target.String()
		}
	case "mailto":
		if target.Opaque != "" {
			return target.String()
		}
	}
	return ""
}

const previewExtensions = parser.GFM | parser.ExtAlerts | parser.ExtFootnotes
const webPreviewExtensions = previewExtensions | parser.ExtMath | parser.ExtDollarMath

func renderPreview(doc *parser.Document, text, fallbackTitle string) (body, title string) {
	return renderPreviewResources(doc, text, fallbackTitle, nil, "", false)
}

func renderPreviewResources(doc *parser.Document, text, fallbackTitle string, resources *previewResources, relative string, charts bool) (body, title string) {
	return renderPreviewDocument(doc, text, fallbackTitle, resources, relative, charts, false, nil)
}

func renderPreviewDocument(doc *parser.Document, text, fallbackTitle string, resources *previewResources, relative string, charts, trackLines bool, figures *figureRender) (body, title string) {
	markup := previewMarkup{doc: doc, resources: resources, relative: relative, charts: charts, figures: figures}
	return markup.render(text, fallbackTitle, trackLines)
}

func (m *previewMarkup) render(text, fallbackTitle string, trackLines bool) (body, title string) {
	prefix, _, source := workspace.SplitFrontmatter(text)
	setPreviewDocument(m.doc, source)
	if m.apiActions != nil {
		m.apiPrefix = len(prefix)
		if len(m.doc.Source()) != len(source) {
			m.apiOriginalOffsets = previewAPIRequestOffsets(m.doc, source)
		}
	}
	m.anchors = make(map[string]int)
	if trackLines {
		m.lineStarts = previewLineStarts(m.doc.Source(), strings.ContainsRune(prefix, '\n'))
	}
	m.blocks(0)
	m.writeSnippetHoverTemplates()
	title = m.title
	if title == "" {
		title = fallbackTitle
	}
	return m.out.String(), title
}

// previewMarkup renders parsed Markdown for previews and standalone exports.
type previewMarkup struct {
	doc                *parser.Document
	out                strings.Builder
	anchors            map[string]int
	title              string
	resources          *previewResources
	relative           string
	embedded           bool
	inLink             bool
	charts             bool
	figures            *figureRender
	lineStarts         []int
	standalone         standaloneHTMLMode
	htmlBlocks         bool
	codeTheme          *editor.Theme
	liveQueries        bool
	queryIndex         *vaultindex.Index
	wikiFigures        []deferredWikiFigure
	snippets           *previewSnippetLibrary
	liveAPIFences      bool
	apiActions         *previewAPIActions
	apiPrefix          int
	apiOriginalOffsets map[int]int
}

func previewLineStarts(source string, strippedFrontmatter bool) []int {
	start := 0
	if strippedFrontmatter {
		// The frozen renderer starts line numbering after leading newlines in the body.
		start = len(source) - len(strings.TrimLeft(source, "\r\n\v\f\u0085\u2028\u2029"))
	}
	starts := []int{start}
	for i := start; i < len(source); i++ {
		if source[i] == '\n' || source[i] == '\r' && (i+1 == len(source) || source[i+1] != '\n') {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func (m *previewMarkup) lineAttribute(block *parser.Block) string {
	line := sort.Search(len(m.lineStarts), func(i int) bool { return m.lineStarts[i] > block.Start })
	if line == 0 {
		return ""
	}
	return ` data-source-line="` + strconv.Itoa(line) + `"`
}

func (m *previewMarkup) blocks(parent int) {
	var previous int
	for i := range m.doc.Children(parent) {
		block := m.doc.Block(i)
		prior := previous
		previous = i
		switch block.Kind {
		case parser.Paragraph:
			if parent > 0 {
				item := m.doc.Block(parent)
				if item.Kind == parser.Item && m.doc.Block(item.Parent).Tight {
					m.inlineBlock("", i)
					continue
				}
			}
			m.inlineBlock("p", i)
		case parser.Heading:
			m.heading(i, block)
		case parser.ThematicBreak:
			fmt.Fprintf(&m.out, "<hr%s>", m.lineAttribute(block))
		case parser.CodeBlock:
			source := m.doc.Lines(i)
			anchor := m.lineAttribute(block)
			if m.liveQueries && wysiwyg.IsQueryFence(block) {
				m.writeQueryBlock(strings.TrimSuffix(source, "\n"), anchor)
				continue
			}
			if (m.liveAPIFences || m.embedded) && m.writeLiveAPIFence(i, prior, source, anchor) {
				continue
			}
			if m.charts && block.Fenced && !block.Open {
				if background, ok := mermaid.Background(block.Info()); ok {
					fmt.Fprintf(&m.out, mermaidBlockStart+`%s"%s>`, html.EscapeString(background), anchor)
					if m.standalone != htmlInApp {
						m.out.WriteString(html.EscapeString(strings.TrimSuffix(source, "\n")))
					} else {
						fmt.Fprintf(&m.out, "<code>%s</code>", html.EscapeString(source))
					}
					m.out.WriteString("</pre>")
					continue
				}
				if mode := wysiwyg.VegaMode(block.Info()); mode != "" {
					fmt.Fprintf(&m.out, `<div class="%s"%s`+vegaBlockMarker+`<code class="vega-source">%s</code></div>`,
						mode, anchor, html.EscapeString(m.preparedVega(source, mode)))
					continue
				}
			}
			if m.codeTheme != nil && block.Fenced && !block.Open && m.writeHighlightedFence(block.Language(), anchor, source) {
				continue
			}
			fmt.Fprintf(&m.out, "<pre%s><code>%s</code></pre>", anchor, html.EscapeString(source))
		case parser.MathBlock:
			if block.Open {
				source := m.doc.Source()[block.Start:block.End]
				fmt.Fprintf(&m.out, `<div class="raw-markup">%s</div>`, html.EscapeString(source))
			} else {
				tex := strings.TrimSuffix(m.doc.Lines(i), "\n")
				fmt.Fprintf(&m.out, `<div class="math-display">%s</div>`, html.EscapeString(tex))
			}
		case parser.HTMLBlock:
			source := m.doc.Lines(i)
			if m.htmlBlocks {
				if markup, ok := standaloneHTMLBlock(source); ok {
					m.out.WriteString(markup)
					continue
				}
			}
			fmt.Fprintf(&m.out, `<div class="raw-markup">%s</div>`, html.EscapeString(source))
		case parser.Quote:
			fmt.Fprintf(&m.out, "<blockquote%s>", m.lineAttribute(block))
			if block.Alert != parser.NoAlert {
				fmt.Fprintf(&m.out, `<strong class="callout-title">%s</strong>`, html.EscapeString(block.Alert.String()))
			}
			m.blocks(i)
			m.out.WriteString("</blockquote>")
		case parser.List:
			m.list(i, block)
		case parser.Item:
			if block.Task == parser.Unchecked || block.Task == parser.Checked {
				m.out.WriteString("<li>")
			} else {
				fmt.Fprintf(&m.out, "<li%s>", m.lineAttribute(block))
			}
			switch block.Task {
			case parser.Unchecked:
				m.out.WriteString(`<input type="checkbox" disabled aria-label="Incomplete"> `)
			case parser.Checked:
				m.out.WriteString(`<input type="checkbox" checked disabled aria-label="Complete"> `)
			}
			m.blocks(i)
			m.out.WriteString("</li>")
		case parser.Table:
			m.table(i, block.Align)
		case parser.FootnoteDefinition:
			fmt.Fprintf(&m.out, `<aside class="footnote" id="%s">`, html.EscapeString(footnoteID(block.Label())))
			m.blocks(i)
			m.out.WriteString("</aside>")
		}
	}
}

func (m *previewMarkup) heading(i int, block *parser.Block) {
	in := m.doc.Inlines(i)
	plain := strings.TrimSpace(parser.PlainText(in))
	if m.title == "" && block.Parent == 0 && (m.standalone == htmlInApp || block.Level == 1) {
		m.title = plain
	}
	slug := markdown.Slug(plain)
	tag := fmt.Sprintf("h%d", block.Level)
	contentsAnchor := ""
	if block.Parent == 0 {
		line, _, _ := strings.Cut(m.doc.Source()[block.Start:], "\n")
		marks := len(line) - len(strings.TrimLeft(line, "#"))
		if marks == block.Level {
			contentsAnchor = workspace.HeadingAnchor(line[marks:])
		}
	}
	if slug != "" && (!m.embedded || m.standalone != htmlInApp) {
		id := slug
		for suffix := m.anchors[slug]; m.anchors[id] > 0; suffix++ {
			id = fmt.Sprintf("%s-%d", slug, suffix)
			m.anchors[slug] = suffix + 1
		}
		m.anchors[id] = 1
		fmt.Fprintf(&m.out, `<%s id="%s"%s`, tag, html.EscapeString(id), m.lineAttribute(block))
		if contentsAnchor != "" && contentsAnchor != id {
			fmt.Fprintf(&m.out, ` data-contents-anchor="%s"`, html.EscapeString(contentsAnchor))
		}
		m.out.WriteByte('>')
	} else {
		fmt.Fprintf(&m.out, "<%s%s>", tag, m.lineAttribute(block))
	}
	m.inlines(in, 0, len(in))
	fmt.Fprintf(&m.out, "</%s>", tag)
}

func (m *previewMarkup) list(i int, block *parser.Block) {
	if block.Ordered {
		fmt.Fprintf(&m.out, `<ol start="%d"%s>`, block.Number, m.lineAttribute(block))
		m.blocks(i)
		m.out.WriteString("</ol>")
		return
	}
	contents := i > 1 && block.Parent == 0 && m.doc.Block(i-1).Kind == parser.Heading &&
		strings.EqualFold(parser.PlainText(m.doc.Inlines(i-1)), "Table of Contents")
	if contents {
		fmt.Fprintf(&m.out, `<ul class="document-contents"%s>`, m.lineAttribute(block))
	} else {
		fmt.Fprintf(&m.out, "<ul%s>", m.lineAttribute(block))
	}
	m.blocks(i)
	m.out.WriteString("</ul>")
}

func (m *previewMarkup) table(i int, align []parser.Align) {
	fmt.Fprintf(&m.out, `<div class="table-scroll"><table%s>`, m.lineAttribute(m.doc.Block(i)))
	for row := range m.doc.Children(i) {
		m.out.WriteString("<tr>")
		tag := "td"
		if m.doc.Block(row).Header {
			tag = "th"
		}
		col := 0
		for cell := range m.doc.Children(row) {
			fmt.Fprintf(&m.out, "<%s", tag)
			if col < len(align) {
				switch align[col] {
				case parser.AlignCenter:
					m.out.WriteString(` class="center"`)
				case parser.AlignRight:
					m.out.WriteString(` class="right"`)
				}
			}
			m.out.WriteByte('>')
			m.inlineBlock("", cell)
			fmt.Fprintf(&m.out, "</%s>", tag)
			col++
		}
		m.out.WriteString("</tr>")
	}
	m.out.WriteString("</table></div>")
}

func (m *previewMarkup) inlineBlock(tag string, i int) {
	in := m.doc.Inlines(i)
	if tag == "" {
		m.inlines(in, 0, len(in))
		return
	}
	anchor := m.lineAttribute(m.doc.Block(i))
	if tag == "p" && len(in) == 1 && in[0].Kind == parser.Text && strings.TrimSpace(in[0].Text) == "[pagebreak]" {
		if m.standalone != htmlInApp {
			m.out.WriteString(`<hr class="page-break">`)
		} else {
			fmt.Fprintf(&m.out, `<div class="page-break" role="separator" aria-label="Page break"%s></div>`, anchor)
		}
		return
	}
	if tag == "p" && !m.embedded && (m.writeSnippetEmbed(in, anchor) || m.resources != nil && m.embedParagraph(in, anchor)) {
		return
	}
	if tag == "p" && m.figures != nil && m.figureParagraph(in, anchor, m.doc.Block(i).Parent == 0) {
		return
	}
	fmt.Fprintf(&m.out, "<%s%s>", tag, anchor)
	m.inlines(in, 0, len(in))
	fmt.Fprintf(&m.out, "</%s>", tag)
}

func (m *previewMarkup) inlineGroup(tag string, in []parser.Inline, start, end int) {
	fmt.Fprintf(&m.out, "<%s>", tag)
	m.inlines(in, start, end)
	fmt.Fprintf(&m.out, "</%s>", tag)
}

func (m *previewMarkup) inlines(in []parser.Inline, start, end int) {
	for i := start; i < end; i = in[i].Next {
		n := in[i]
		switch n.Kind {
		case parser.Text:
			m.prose(n)
		case parser.HTML:
			m.out.WriteString(html.EscapeString(n.Text))
		case parser.SoftBreak:
			m.out.WriteByte('\n')
		case parser.LineBreak:
			m.out.WriteString("<br>")
		case parser.Code:
			fmt.Fprintf(&m.out, "<code>%s</code>", html.EscapeString(n.Text))
		case parser.Math:
			class := "math-inline"
			if n.Display {
				class = "math-display"
			}
			fmt.Fprintf(&m.out, `<span class="%s">%s</span>`, class, html.EscapeString(n.Text))
		case parser.Emphasis:
			m.inlineGroup("em", in, i+1, n.Next)
		case parser.Strong:
			m.inlineGroup("strong", in, i+1, n.Next)
		case parser.Strikethrough:
			m.inlineGroup("del", in, i+1, n.Next)
		case parser.Link:
			m.link(n, in, i+1)
		case parser.Image:
			alt := parser.PlainText(in[i+1 : n.Next])
			if src := m.markdownImageSource(n.Dest); src != "" {
				m.writeImage(src, alt, n.Title, 0)
			} else if m.figures != nil && m.figures.pdf {
				m.writeImage("data:,", alt, n.Title, 0)
			} else if alt != "" {
				fmt.Fprintf(&m.out, `<span class="image-description" role="img" aria-label="%s">%s</span>`, html.EscapeString(alt), html.EscapeString(alt))
			}
		case parser.FootnoteRef:
			fmt.Fprintf(&m.out, `<sup><a href="#%s">[%s]</a></sup>`,
				html.EscapeString(url.PathEscape(footnoteID(n.Text))), html.EscapeString(n.Text))
		}
	}
}

func (m *previewMarkup) link(n parser.Inline, in []parser.Inline, start int) {
	dest := previewExternalURL(n.Dest)
	external := dest != ""
	if strings.HasPrefix(n.Dest, "#") {
		dest = n.Dest
	} else if dest == "" && m.standalone != htmlInApp {
		dest = standaloneRelativeURL(n.Dest)
	} else if dest == "" && m.resources != nil && m.resources.index != nil {
		if target, ok := m.resources.index.ResolveMarkdown(m.relative, n.Dest); ok {
			parsed, _ := url.Parse(n.Dest)
			dest = previewNoteURL(m.relative, target, markdown.Slug(parsed.Fragment))
		}
	}
	if dest == "" {
		m.inlines(in, start, n.Next)
		return
	}
	fmt.Fprintf(&m.out, `<a href="%s"`, html.EscapeString(dest))
	if external && m.standalone == htmlPageSetup {
		m.out.WriteString(` target="_blank" rel="noopener noreferrer"`)
	}
	if n.Title != "" {
		fmt.Fprintf(&m.out, ` title="%s"`, html.EscapeString(n.Title))
	}
	m.out.WriteByte('>')
	wasInLink := m.inLink
	m.inLink = true
	m.inlines(in, start, n.Next)
	m.inLink = wasInLink
	m.out.WriteString("</a>")
}

func footnoteID(label string) string { return "footnote-" + parser.NormalizeLabel(label) }

const previewContentPolicy = "default-src 'none'; script-src '" + mermaidScriptHash + "' " + vegaScriptHashes +
	"; connect-src https: http: data:; img-src data:; font-src data:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'"

// EV-UI-014 and EV-UI-063 record the markup styles and document theme variables.
var previewStyles = `
  :root { color-scheme: light dark; }
  body { margin: 0; color: CanvasText; background: Canvas; font: 15px/var(--preview-line-height, 1.5) system-ui, sans-serif; overflow-wrap: anywhere; }
  main { margin: 0 auto; max-width: 800px; padding: 24px; }
  h1, h2, h3, h4, h5, h6 { line-height: 1.3; letter-spacing: -0.015em; }
  h1 { font-size: 2em; } h2 { font-size: 1.5em; } h3 { font-size: 1.25em; }
  h4 { font-size: 1.1em; } h5 { font-size: 1em; } h6 { font-size: 0.9em; }
  a { text-decoration-thickness: 1px; text-underline-offset: 3px; }
  code { font: 0.9em ui-monospace, monospace; padding: 2px 6px; border-radius: 4px; background: rgba(127,127,127,.14); }
  pre { overflow-x: auto; padding: 16px; border-radius: 8px; background: rgba(127,127,127,.14); }
  pre code { background: transparent; padding: 0; }
  blockquote { margin-inline: 0; border-inline-start: 3px solid rgba(127,127,127,.4); padding-inline-start: 1em; }
  .table-scroll { overflow-x: auto; }
  table { border-collapse: collapse; }
  th, td { padding: 8px 12px; text-align: left; border: 1px solid rgba(127,127,127,.35); }
  th.center, td.center { text-align: center; }
  th.right, td.right { text-align: right; }
  .raw-markup { white-space: pre-wrap; }
  .image-description { font-style: italic; }
  figure.md-figure { margin: 1.2em 0; padding: 0; text-align: center; break-inside: avoid; }
  figure.md-figure img { max-width: 100%; border-radius: 6px; }
  figure.md-figure > figcaption { margin-top: .5em; font-size: .88em; line-height: 1.4; opacity: .75; }
  figure.md-figure .figure-label { font-weight: 600; opacity: .9; }
  img { display: inline-block; max-width: 100%; height: auto; }
  .note-embed { padding-inline-start: 1em; border-inline-start: 3px solid rgba(127,127,127,.4); }
  .math-display > .katex-display { overflow-x: auto; overflow-y: hidden; }
  .math-display > .katex-display > .katex { min-width: 100%; width: max-content; }
  .footnote { font-size: 0.9em; }
  .document-contents, .document-contents ul { padding-inline-start: 24px; }
  .document-contents li, .document-contents ul { margin-block: 16px 0; }
  .query-list { padding-inline-start: 24px; }
  .query-table { width: 100%; table-layout: fixed; }
  .query-empty, .query-error { font-size: .9em; }
  .query-empty { opacity: .65; }
  .query-error { color: #e05252; }
  .page-break { margin: 1.5em 0; border-top: 1px dashed rgba(127,127,127,.4); }
` + previewMermaidCSS + previewVegaCSS + previewAPICSS + previewSnippetCSS + sourceFilePreviewCSS + delimitedPreviewCSS + previewScrollCSS +
	documentPreviewPalettes + documentThemeElements + documentPreviewLayout

var previewPage = previewHTMLShell("Markdown Preview", "", previewContentPolicy, "", 0, "", "")

// MyGo installs the preload; the document permits the pinned chart scripts.
func previewScript() string {
	return `(() => {` + previewMathScript() + previewMermaidJS + previewVegaJS + previewScrollJS + previewAPIScript + previewSnippetScript + `
  const chartSelector = diagramSelector + "," + vegaSelector;
  const chartKey = node => node.matches(diagramSelector) ? diagramKey(node) : vegaKey(node);

  function replacePreviewBody(preview, html, reuse) {
    const existing = preview.querySelectorAll(chartSelector);
    if (!reuse || !existing.length) {
      for (const node of existing) releaseVegaChart(node);
      preview.innerHTML = html;
      return;
    }
    const available = new Map();
    for (const node of existing) {
      const key = chartKey(node);
      if (!available.has(key)) available.set(key, []);
      available.get(key).push(node);
    }
    const frame = document.createElement("template");
    frame.innerHTML = html;
    for (const node of frame.content.querySelectorAll(chartSelector)) {
      const prior = available.get(chartKey(node))?.shift();
      if (prior) {
        const line = node.getAttribute("data-source-line");
        if (line === null) prior.removeAttribute("data-source-line");
        else prior.setAttribute("data-source-line", line);
        node.replaceWith(prior);
      }
    }
    for (const nodes of available.values()) for (const node of nodes) releaseVegaChart(node);
    preview.replaceChildren(frame.content);
  }

  let revision = 0;
  let documentID = 0;
  window.mygo.on("` + previewScrollEvent + `", target => {
    if (target.document === documentID && target.revision === revision) scrollPreview(target);
  });
  window.mygo.on("` + previewChangeEvent + `", frame => {
    const preview = document.querySelector("main");
    if (!preview || frame.revision <= revision) return;
    const sameDocument = frame.document === documentID;
    if (!sameDocument) resetPreviewScroll();
    const sameTheme = frame.theme === document.documentElement.dataset.previewTheme;
    const scrollY = sameDocument ? window.scrollY : 0;
    if (!sameTheme || frame.themeFonts) {
      document.documentElement.dataset.previewTheme = frame.theme;
      document.getElementById("preview-theme-fonts").textContent = frame.themeFonts || "";
    }
    const stylesheet = document.getElementById("custom-stylesheet");
    const css = frame.customCSS || "";
    const sameStylesheet = stylesheet.textContent === css;
    if (!sameStylesheet) stylesheet.textContent = css;
    setPreviewScrollPadding(frame.scrollPastEnd);
    if (frame.lineSpacing > 0) document.documentElement.style.setProperty("--preview-line-height", String(frame.lineSpacing));
    else document.documentElement.style.removeProperty("--preview-line-height");
    hideSnippetPreview();
    replacePreviewBody(preview, frame.html, sameDocument && sameTheme && sameStylesheet);
    renderMath(preview);
    renderDiagrams(preview);
    renderVegaCharts(preview);
    document.title = frame.title;
    preview.setAttribute("aria-label", frame.label || "Markdown Preview");
    window.scrollTo(0, scrollY);
    if (frame.anchor) document.getElementById(frame.anchor)?.scrollIntoView();
    revision = frame.revision;
    documentID = frame.document;
  });
  document.addEventListener("click", event => {
    const link = event.target.closest?.("a[href]");
    const fragment = link?.getAttribute("href");
    if (!fragment?.startsWith("#")) return;
    event.preventDefault();
    try {
      const id = decodeURIComponent(fragment.slice(1));
      if (id) {
        const inContents = link.closest(".document-contents");
        const match = inContents && Array.from(document.querySelectorAll("[data-contents-anchor]")).find(node => node.dataset.contentsAnchor === id);
        (match || document.getElementById(id))?.scrollIntoView();
      }
      else window.scrollTo(0, 0);
    } catch {
      // A malformed fragment leaves the current viewport in place.
    }
  });
})();`
}
