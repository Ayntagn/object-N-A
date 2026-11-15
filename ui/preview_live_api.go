package ui

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"cyberwriter/internal/core/liveapi"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const previewAPIScheme = "cyberwriter-api"

type previewAPILocation struct {
	offset            int
	version, document uint64
}

type previewAPIActions struct {
	version, document uint64
}

// The API location uses a per-preview document ID separate from scroll sync.
func newPreviewAPIDocument() uint64 {
	var random [8]byte
	_, _ = rand.Read(random[:])
	return binary.BigEndian.Uint64(random[:]) | 1
}

func previewAPINavigation(raw string) (previewAPILocation, bool) {
	link, err := url.Parse(raw)
	if err != nil || link.Scheme != previewAPIScheme || link.Host != "run" || link.User != nil || link.Path != "" || link.Fragment != "" {
		return previewAPILocation{}, false
	}
	query, err := url.ParseQuery(link.RawQuery)
	if err != nil || len(query) != 3 || len(query["offset"]) != 1 || len(query["version"]) != 1 || len(query["document"]) != 1 {
		return previewAPILocation{}, false
	}
	offset, err := strconv.ParseUint(query.Get("offset"), 10, strconv.IntSize-1)
	if err != nil {
		return previewAPILocation{}, false
	}
	version, err := strconv.ParseUint(query.Get("version"), 10, 64)
	if err != nil {
		return previewAPILocation{}, false
	}
	document, err := strconv.ParseUint(query.Get("document"), 10, 64)
	if err != nil || document == 0 {
		return previewAPILocation{}, false
	}
	return previewAPILocation{offset: int(offset), version: version, document: document}, true
}

func (app *workspaceWindow) previewAPIEnabled() bool {
	if app.workspaceTab == nil || app.draft == nil || app.pdfPreview.enabled {
		return false
	}
	mode := app.displayedMode()
	return mode == splitMode || mode == previewMode
}

// handleAPINavigation receives only user-initiated navigation from the preview.
func (p *workspacePreview) handleAPINavigation(app *workspaceWindow, view *previewViewport, target string) bool {
	if !strings.HasPrefix(target, previewAPIScheme+":") {
		return false
	}
	location, ok := previewAPINavigation(target)
	if !ok {
		return true
	}
	win := app.win
	win.Update(func() {
		if win.IsDestroyed() || view.IsDestroyed() || p.view != view || !p.ready || !p.apiEnabled ||
			p.apiDocument != location.document || p.apiVersion != location.version ||
			p.tab != app.workspaceTab || !app.previewAPIEnabled() || p.draft != app.draft || p.path != app.document.Path ||
			app.previewDocumentType() != documentMarkdown || app.draft.Version() != location.version {
			return
		}
		app.runPreviewAPIRequest(location.offset, location.version)
	})
	return true
}

func previewAPIRequestOptions(block *parser.Block) (liveapi.Options, bool) {
	if block.Kind != parser.CodeBlock || !block.Fenced || block.Open {
		return liveapi.Options{}, false
	}
	return liveapi.ParseOptions(block.Info())
}

// Preview can insert escapes to preserve code span boundaries next to inline
// math. In that case match the rendered request fences to their raw offsets.
func previewAPIRequestOffsets(doc *parser.Document, source string) map[int]int {
	raw := parser.New(doc.Extensions())
	raw.Set(source)
	var requests []int
	for i := range raw.Children(0) {
		if _, ok := previewAPIRequestOptions(raw.Block(i)); ok {
			requests = append(requests, i)
		}
	}
	offsets := make(map[int]int, len(requests))
	for i := range doc.Children(0) {
		block := doc.Block(i)
		if _, ok := previewAPIRequestOptions(block); !ok {
			continue
		}
		if len(requests) == 0 {
			clear(offsets)
			return offsets
		}
		j := requests[0]
		requests = requests[1:]
		original := raw.Block(j)
		if original.Info() == block.Info() && raw.Lines(j) == doc.Lines(i) {
			offsets[block.Start] = original.Start
		}
	}
	if len(requests) != 0 {
		clear(offsets)
	}
	return offsets
}

func (m *previewMarkup) apiRequestLocation(block *parser.Block) (previewAPILocation, bool) {
	if m.apiActions == nil || block.Parent != 0 {
		return previewAPILocation{}, false
	}
	start := block.Start
	if m.apiOriginalOffsets != nil {
		var ok bool
		start, ok = m.apiOriginalOffsets[start]
		if !ok {
			return previewAPILocation{}, false
		}
	}
	return previewAPILocation{offset: m.apiPrefix + start, version: m.apiActions.version, document: m.apiActions.document}, true
}

// EV-UI-164 and EV-UI-165: a saved result inherits the preceding request's
// options in the workspace preview and exports.
func (m *previewMarkup) writeLiveAPIFence(i, previous int, source, anchor string) bool {
	block := m.doc.Block(i)
	if !block.Fenced || block.Open {
		return false
	}
	source = strings.TrimSuffix(source, "\n")
	if _, ok := previewAPIRequestOptions(block); ok {
		if m.apiActions != nil || (m.embedded && m.standalone == htmlInApp && (m.figures == nil || !m.figures.pdf)) {
			m.writePreviewAPIRequest(block, source, anchor)
		}
		return true
	}
	if !strings.EqualFold(block.Language(), "api-result") {
		return false
	}
	var options liveapi.Options
	if previous != 0 {
		prior := m.doc.Block(previous)
		if inherited, ok := previewAPIRequestOptions(prior); ok && strings.TrimSpace(m.doc.Source()[prior.End:block.Start]) == "" {
			options = inherited
		}
	}
	m.writePreviewAPIResult(liveapi.ParseResult(source), options, anchor)
	return true
}

func (m *previewMarkup) writePreviewAPIRequest(block *parser.Block, source, anchor string) {
	fmt.Fprintf(&m.out, `<div class="api-block"%s>`, anchor)
	request, err := liveapi.ParseRequest(block.Info(), source)
	if err != nil {
		fmt.Fprintf(&m.out, `<span class="api-request-error">%s</span>`, html.EscapeString(err.Error()))
	} else {
		fmt.Fprintf(&m.out, `<span class="api-method">%s</span><span class="api-url">%s</span>`,
			html.EscapeString(request.Method), html.EscapeString(request.URL))
	}
	if location, ok := m.apiRequestLocation(block); ok {
		fmt.Fprintf(&m.out, `<button type="button" class="api-run-btn" data-offset="%d" data-version="%d" data-document="%d">Run</button>`,
			location.offset, location.version, location.document)
	}
	m.out.WriteString(`</div>`)
}

func (m *previewMarkup) writePreviewAPIResult(result liveapi.Result, options liveapi.Options, anchor string) {
	interactive := m.apiActions != nil
	disclose := interactive && !options.Bare
	if disclose {
		title := options.Title
		if title == "" {
			title = "Response"
		}
		open := " open"
		if options.Collapsed || utf8.RuneCountInString(strings.TrimSpace(result.Body)) > 1000 {
			open = ""
		}
		fmt.Fprintf(&m.out, `<details class="api-block-result"%s%s><summary>%s</summary>`, open, anchor, html.EscapeString(title))
	} else {
		fmt.Fprintf(&m.out, `<div class="api-block-result"%s>`, anchor)
		if options.Title != "" {
			fmt.Fprintf(&m.out, `<div class="api-result-title">%s</div>`, html.EscapeString(options.Title))
		}
	}
	if options.Format == liveapi.FormatRaw {
		result.Format = liveapi.FormatRaw
	}
	if table, ok := result.Table(); ok {
		m.writePreviewAPITable(table)
		if disclose && result.Format != liveapi.FormatAI {
			fmt.Fprintf(&m.out, `<details class="api-raw-response"><summary>Raw response</summary><pre>%s</pre></details>`, html.EscapeString(result.Body))
		}
	} else if strings.TrimSpace(result.Body) == "" {
		m.out.WriteString(`<div class="api-result-meta"><em>No content</em></div>`)
	} else {
		fmt.Fprintf(&m.out, `<pre class="api-result-body">%s</pre>`, html.EscapeString(result.Body))
	}
	if disclose {
		if result.Metadata != "" {
			fmt.Fprintf(&m.out, `<div class="api-result-meta">%s</div>`, html.EscapeString(strings.Replace(result.Metadata, "format:ai", "AI", 1)))
		}
		m.out.WriteString(`</details>`)
		return
	}
	if result.Timestamp != "" {
		fmt.Fprintf(&m.out, `<div class="api-result-meta">fetched %s</div>`, html.EscapeString(result.Timestamp))
	}
	if result.StatusCode >= 400 {
		fmt.Fprintf(&m.out, `<div class="api-result-meta">request failed (%d)</div>`, result.StatusCode)
	}
	m.out.WriteString(`</div>`)
}

func (m *previewMarkup) writePreviewAPITable(table liveapi.Table) {
	m.out.WriteString(`<div class="table-scroll api-result-table"><table><thead><tr>`)
	for _, column := range table.Columns {
		fmt.Fprintf(&m.out, `<th scope="col">%s</th>`, html.EscapeString(column))
	}
	m.out.WriteString(`</tr></thead><tbody>`)
	for _, row := range table.Rows {
		m.out.WriteString(`<tr>`)
		for _, cell := range row {
			fmt.Fprintf(&m.out, `<td>%s</td>`, html.EscapeString(cell))
		}
		m.out.WriteString(`</tr>`)
	}
	m.out.WriteString(`</tbody></table></div>`)
	if table.OmittedRows != 0 || table.OmittedColumns != 0 {
		fmt.Fprintf(&m.out, `<div class="api-result-meta">Showing %d of %d rows and %d of %d columns.</div>`,
			len(table.Rows), len(table.Rows)+table.OmittedRows, len(table.Columns), len(table.Columns)+table.OmittedColumns)
	}
}

const previewAPIScript = `
  document.addEventListener("click", event => {
    const button = event.target.closest?.("main button.api-run-btn");
    if (!button || !event.isTrusted) return;
    const {offset, version, document} = button.dataset;
    if (![offset, version, document].every(value => value && /^\d+$/.test(value))) return;
    const now = Date.now();
    if (now - Number(button.dataset.lastRun || 0) < 600) return;
    button.dataset.lastRun = String(now);
    button.textContent = "…";
    setTimeout(() => { button.textContent = "Run"; }, 600);
    window.location.href = "` + previewAPIScheme + `://run?offset=" + offset + "&version=" + version + "&document=" + document;
  });
`

const previewAPICSS = `
.api-block, details.api-block-result {
  margin: 8px 0; padding: 12px; border: 1px solid rgba(127,127,127,.25);
  border-radius: 8px; background: rgba(127,127,127,.06);
}
.api-block { position: relative; padding-inline-end: 60px; overflow-wrap: anywhere; }
.api-method, .api-url { font: .85em ui-monospace, monospace; }
.api-method { display: inline-block; margin-inline-end: 12px; font-weight: 600; }
.api-run-btn {
  position: absolute; top: 8px; inset-inline-end: 8px; min-width: 34px;
  padding: 3px 12px; border-radius: 5px; border: 0;
  font: 600 11px/14px system-ui, sans-serif; color: white; background: #3fb950; cursor: pointer;
}
div.api-block-result { margin: 10px 0; }
.api-result-title { font-weight: 600; margin-bottom: 6px; }
.api-block-result summary { cursor: pointer; }
.api-block-result > summary { font-weight: 600; margin-bottom: 8px; }
.api-block-result .api-result-body { margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; font-size: .85em; }
.api-result-table table { width: 100%; font-size: .88em; }
.api-result-table :is(th, td) { padding: 6px 10px; vertical-align: top; }
.api-raw-response { margin-top: 8px; font-size: .82em; }
.api-raw-response pre { margin: 6px 0 0; white-space: pre-wrap; overflow-wrap: anywhere; max-height: 320px; overflow: auto; }
.api-result-meta { margin-top: 6px; font-size: .82em; opacity: .7; }
details.api-block-result > .api-result-meta { border-top: 1px solid rgba(127,127,127,.15); margin-top: 10px; padding-top: 6px; }
`
