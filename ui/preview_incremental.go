package ui

import (
	"strings"
	"time"

	"cyberwriter/internal/core/markdownhighlight"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const maxCachedPreviewBytes = 2 << 20

type previewRenderCache struct {
	paragraphs map[string]previewRenderPiece
	fences     map[previewFenceKey]string
	codeTheme  *editor.Theme
}

type previewRenderPiece struct {
	block previewTransportBlock
	math  bool
}

type livePreviewRender struct {
	html, title         string
	blocks              []previewTransportBlock
	bytes               int
	math, mermaid, vega bool
	prepare             time.Duration
	rendered, reused    int
	fenceHits           int
}

func (r livePreviewRender) fullHTML() string {
	if r.blocks == nil {
		return r.html
	}
	var out strings.Builder
	out.Grow(r.bytes)
	for _, block := range r.blocks {
		out.WriteString(block.markup())
	}
	return out.String()
}

// renderLive keeps the shared parse tree, then renders independent blocks.
// Raw HTML and snippet templates can span the AST's top-level boundaries,
// so those documents use the original contiguous renderer.
func (m *previewMarkup) renderLive(text, fallbackTitle string, cache *previewRenderCache, measure bool) livePreviewRender {
	var result livePreviewRender
	var started time.Time
	if measure {
		started = time.Now()
	}
	m.prepare(text, true)
	if measure {
		result.prepare = time.Since(started)
	}
	if len(text) < longPreviewBytes || len(text) > maxCachedPreviewBytes || !m.hasPreviewBlockBoundaries() {
		if cache != nil {
			*cache = previewRenderCache{}
		}
		if m.resources != nil {
			// Full DOM replacements should use self-contained images.
			m.resources.fileImages = nil
		}
		m.blocks(0)
		m.writeSnippetHoverTemplates()
		result.html = m.out.String()
		result.title, result.bytes = m.renderedTitle(fallbackTitle), len(result.html)
		result.math = containsPreviewMath(result.html)
		result.mermaid = strings.Contains(result.html, mermaidBlockStart)
		result.vega = strings.Contains(result.html, vegaBlockMarker)
		return result
	}
	var next previewRenderCache
	if cache != nil {
		next = previewRenderCache{paragraphs: make(map[string]previewRenderPiece, len(cache.paragraphs)), codeTheme: m.codeTheme}
		m.fenceHighlights = &previewFencePass{next: make(map[previewFenceKey]string, len(cache.fences))}
		if cache.codeTheme == m.codeTheme {
			m.fenceHighlights.previous = cache.fences
		}
	}
	m.highlights = markdownhighlight.Markers(m.doc)
	source := m.doc.Source()
	result.blocks = make([]previewTransportBlock, 0, m.doc.Len()-1)
	var prior int
	for i := range m.doc.Children(0) {
		previous := prior
		prior = i
		block := m.doc.Block(i)
		var raw string
		var line int
		if cache != nil && block.Kind == parser.Paragraph {
			candidate := source[block.Start:block.End]
			// References, includes, figures and author HTML can depend on
			// other blocks, the Vault index, or per-frame asset budgets.
			if !strings.ContainsAny(candidate, "[<") {
				raw, line = candidate, m.sourceLineAt(block.Start)
			}
		}
		piece, reused := next.paragraphs[raw]
		if !reused && raw != "" && line != 0 {
			piece, reused = cache.paragraphs[raw]
		}
		if reused && line != 0 {
			piece.block = piece.block.withLine(line)
			result.reused++
		} else {
			m.out.Reset()
			m.writeBlock(0, i, previous)
			html := m.out.String()
			result.rendered++
			if html == "" {
				continue
			}
			piece = previewRenderPiece{block: makePreviewTransportBlock(html), math: containsPreviewMath(html)}
			result.mermaid = result.mermaid || strings.Contains(html, mermaidBlockStart)
			result.vega = result.vega || strings.Contains(html, vegaBlockMarker)
		}
		if raw != "" && line != 0 {
			next.paragraphs[raw] = piece
		}
		result.blocks = append(result.blocks, piece.block)
		result.bytes += len(piece.block.html)
		result.math = result.math || piece.math
	}
	result.title = m.renderedTitle(fallbackTitle)
	if cache != nil {
		result.fenceHits = m.fenceHighlights.hits
		next.fences = m.fenceHighlights.next
		if result.bytes > maxCachedPreviewBytes {
			next = previewRenderCache{}
		}
		*cache = next
	}
	return result
}

func (m *previewMarkup) hasPreviewBlockBoundaries() bool {
	if len(m.details) != 0 || m.snippets != nil {
		return false
	}
	for i := 1; i < m.doc.Len(); i++ {
		if m.doc.Block(i).Kind == parser.HTMLBlock {
			return false
		}
	}
	return true
}

type previewFenceKey struct{ language, source string }

type previewFencePass struct {
	previous, next map[previewFenceKey]string
	bytes, hits    int
}

// Only reuse the highlighted content. The anchor and clipboard address
// still come from the current preview frame, including after edits above it.
func (m *previewMarkup) writeLiveHighlightedFence(language, anchor, source string) bool {
	pass := m.fenceHighlights
	if pass == nil {
		return m.writeHighlightedFence(language, anchor, source)
	}
	key := previewFenceKey{language, source}
	html, current := pass.next[key]
	found := current
	if !current {
		html, found = pass.previous[key]
	}
	if found {
		pass.hits++
	} else {
		fragment := previewMarkup{codeTheme: m.codeTheme}
		if fragment.writeHighlightedFence(language, "", source) {
			html = fragment.out.String()
		}
	}
	if bytes := len(html) + len(language) + len(source); !current && pass.bytes+bytes <= maxCachedPreviewBytes {
		pass.next[key] = html
		pass.bytes += bytes
	}
	if html == "" {
		return false
	}
	m.out.WriteString("<pre")
	m.out.WriteString(anchor)
	m.out.WriteString(html[len("<pre"):])
	return true
}
