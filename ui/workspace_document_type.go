package ui

import (
	"path"
	"slices"
	"strings"
	"time"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/go-text/typesetting/segmenter"
)

type documentType uint8

const (
	documentUndetermined documentType = iota
	documentMarkdown
	documentHTML
	documentCode
	documentPlainText
)

var documentTypeChoices = [...]documentType{documentMarkdown, documentHTML, documentCode, documentPlainText}
var documentTypeLabels = [...]string{"Markdown", "Markdown", "HTML", "Code", "Plain Text"}
var documentTypeIcons = [...]string{"text.document", "text.document", "globe", "chevron.left.forwardslash.chevron.right", "doc.text"}

type workspaceDocumentType struct {
	kind, suggested documentType
	overridden      bool
	filename        string
	version         uint64
	suggestionDue   time.Time
}

func (app *workspaceWindow) previewDocumentType() documentType {
	state := &app.documentType
	if state.kind == documentUndetermined || state.filename != app.document.Path {
		source := app.workingText()
		if !state.overridden {
			state.kind = detectDocumentType(app.document.Path, source)
		}
		state.filename = app.document.Path
		state.version = app.textVersion
		state.suggested, state.suggestionDue = documentUndetermined, time.Time{}
	}
	return state.kind
}

func (app *workspaceWindow) observeDocumentType(c *mygoui.Context) {
	kind := app.previewDocumentType()
	source := app.workingText()
	state := &app.documentType
	if state.version != app.textVersion {
		state.version = app.textVersion
		state.suggestionDue = c.Now().Add(300 * time.Millisecond)
	}
	if state.suggestionDue.IsZero() {
		return
	}
	if remaining := state.suggestionDue.Sub(c.Now()); remaining > 0 {
		c.After(remaining)
		return
	}
	state.suggestionDue = time.Time{}
	state.suggested = documentUndetermined
	if suggested := detectDocumentBody(source); suggested != kind && suggested != documentPlainText {
		state.suggested = suggested
	}
}

func (app *workspaceWindow) selectDocumentType(kind documentType) {
	source := app.workingText()
	overridden := kind != documentUndetermined
	if !overridden {
		kind = detectDocumentType(app.document.Path, source)
	}
	app.documentType = workspaceDocumentType{
		kind: kind, overridden: overridden, filename: app.document.Path, version: app.textVersion,
	}
	if app.pdfPreview.enabled {
		app.pdfPreview.enabled = false
		app.pdfPreview.returnTab = nil
		app.pdfPreview.viewer.clear()
	}
	app.previewPane.refresh()
	if kind != documentHTML && (app.problem == htmlPDFExportProblem || app.problem == htmlDocumentExportProblem) {
		app.problem = ""
	}
}

func (app *workspaceWindow) documentTypeMenu(m *mygoui.Menu) {
	current := app.previewDocumentType()
	for _, kind := range documentTypeChoices {
		if m.Item(documentTypeLabels[kind]).Checked(current == kind).Chosen() {
			app.selectDocumentType(kind)
		}
	}
	m.Separator()
	if m.Item("Auto-detect").Chosen() {
		app.selectDocumentType(documentUndetermined)
	}
	if suggested := app.documentType.suggested; suggested != documentUndetermined {
		m.Separator()
		if m.Item("Switch to " + documentTypeLabels[suggested]).Chosen() {
			app.selectDocumentType(suggested)
		}
	}
}

// EV-UI-056 records the extension priority and the bounded content scan.
func detectDocumentType(filename, source string) documentType {
	ext := strings.ToLower(path.Ext(filename))
	if ext == ".html" || ext == ".htm" {
		return documentHTML
	}
	if filename == "" && strings.TrimSpace(source) == "" {
		return documentMarkdown
	}
	if detected := detectDocumentBody(source); detected != documentPlainText {
		return detected
	}
	switch ext {
	case ".md", ".markdown", ".mdown", ".mkd", ".mkdn":
		return documentMarkdown
	case ".xhtml":
		return documentHTML
	case "", ".txt", ".text", ".log":
		return documentPlainText
	default:
		return documentCode
	}
}

var documentHTMLMarkers = []string{
	"<div", "<svg", "<table", "<section", "<article", "<nav", "<header", "<footer",
	"<main", "<form", "<canvas", "<style", "<span style",
}
var documentMarkdownMarkers = []string{
	"# ", "## ", "```", "- [ ]", "- [x]", "![", "[pagebreak]", "---\n", "[[", "**", "__",
}

func detectDocumentBody(source string) documentType {
	units := 0
	for offset, r := range source {
		units++
		if r > 0xffff {
			units++
		}
		if units > 2048 {
			source = source[:offset]
			break
		}
	}
	source = strings.ToLower(strings.TrimSpace(source))
	header := documentCharacterPrefix(source, 500)
	if slices.ContainsFunc([]string{"<!", "<html", "<head", "<body"}, func(marker string) bool {
		return strings.HasPrefix(header, marker)
	}) {
		return documentHTML
	}
	html := documentCharacterPrefix(source, 2000)
	if slices.ContainsFunc(documentHTMLMarkers, func(marker string) bool { return strings.HasPrefix(html, marker) }) {
		return documentHTML
	}
	md := documentCharacterPrefix(source, 1000)
	if slices.ContainsFunc(documentMarkdownMarkers, func(marker string) bool { return strings.Contains(md, marker) }) ||
		strings.Contains(md, "[") && strings.Contains(md, "](") {
		return documentMarkdown
	}
	if slices.ContainsFunc(documentHTMLMarkers, func(marker string) bool { return strings.Contains(html, marker) }) {
		return documentHTML
	}
	return documentPlainText
}

func documentCharacterPrefix(source string, limit int) string {
	if len(source) <= limit {
		return source
	}
	var segments segmenter.Segmenter
	_ = segments.InitWithString(source)
	graphemes := segments.GraphemeIterator()
	for i := 0; graphemes.Next(); i++ {
		if i == limit {
			return source[:graphemes.Grapheme().OffsetInBytes]
		}
	}
	return source
}
