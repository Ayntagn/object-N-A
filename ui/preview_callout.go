package ui

import (
	"encoding/json"
	"fmt"
	"html"
	"slices"
	"strings"

	"cyberwriter/internal/core/callout"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func (m *previewMarkup) writeQuote(i int, block *parser.Block) {
	m.quoteDepth++
	defer func() { m.quoteDepth-- }()
	header, titleStart, lineEnd, ok := callout.ReadQuoteHeader(m.doc.Source(), block.Start)
	if !ok {
		fmt.Fprintf(&m.out, "<blockquote%s>", m.lineAttribute(block))
		m.blocks(i)
		m.out.WriteString("</blockquote>")
		return
	}
	style := callout.Resolve(header.Kind, m.calloutStyles)
	tag, heading, open := "aside", "div", ""
	if header.Fold != 0 && (m.figures == nil || !m.figures.pdf) {
		tag, heading = "details", "summary"
		if header.Fold == '+' {
			open = " open"
		}
	}
	fmt.Fprintf(&m.out, `<%s class="callout" data-callout="%s" style="--callout-accent:%s"%s%s>`,
		tag, html.EscapeString(style.Kind), html.EscapeString(style.Color), m.lineAttribute(block), open)
	fmt.Fprintf(&m.out, `<%s class="callout-title"><span class="callout-icon" aria-hidden="true">%s</span><span class="callout-label">`,
		heading, html.EscapeString(style.Icon))

	first := i + 1
	var headerBlock *parser.Block
	var lead []parser.Inline
	if first < block.Next && m.doc.Block(first).Start < lineEnd {
		// GFM can already have removed a bare alert marker. A title and
		// body in the same paragraph need separate inline ranges.
		headerBlock = m.doc.Block(first)
		lead = m.doc.Inlines(first)
	}
	title := m.clipQuoteInlines(lead, titleStart, lineEnd)
	if len(title) > 0 {
		m.inlines(title, 0, len(title))
	} else {
		label := strings.TrimSpace(header.Title)
		if label == "" {
			label = style.Name
		}
		m.out.WriteString(html.EscapeString(label))
	}
	fmt.Fprintf(&m.out, `</span></%s><div class="callout-content">`, heading)
	if headerBlock != nil {
		if m.doc.LineCount(first) > 1 {
			_, start := m.doc.Line(first, 1)
			body := m.clipQuoteInlines(lead, start, headerBlock.End)
			m.inlineBlockContent("p", body, m.lineAttributeAt(start), false)
		}
		first = headerBlock.Next
	}
	m.blocksFrom(i, first)
	fmt.Fprintf(&m.out, "</div></%s>", tag)
}

// clipQuoteInlines keeps the original offsets, links and reference resolution
// while splitting a parser paragraph at the callout's first physical line.
func (m *previewMarkup) clipQuoteInlines(in []parser.Inline, from, to int) []parser.Inline {
	if from >= to {
		return nil
	}
	var kept []parser.Inline
	source := m.doc.Source()
	var appendRange func(int, int)
	appendRange = func(start, end int) {
		for i := start; i < end; i = in[i].Next {
			n := in[i]
			if n.End <= from || n.Start >= to {
				continue
			}
			if n.Start < from || n.End > to {
				n.Start, n.End = max(n.Start, from), min(n.End, to)
				if n.Next == i+1 {
					n.Kind = parser.Text
					n.Text = previewLiteral(source[n.Start:n.End])
				}
			}
			index := len(kept)
			kept = append(kept, n)
			if n.Next > i+1 {
				appendRange(i+1, n.Next)
			}
			kept[index].Next = len(kept)
		}
	}
	appendRange(0, len(in))
	return kept
}

// The PDF preview compares whole document snapshots. Serialize these small,
// immutable preference rows only when they change, keeping that key comparable.
func (p *workspacePreview) snapshotCalloutStyles(styles []callout.Style) string {
	if !slices.Equal(p.exportCallouts, styles) {
		p.exportCallouts = styles
		p.calloutSnapshot = ""
		if len(styles) > 0 {
			data, _ := json.Marshal(styles)
			p.calloutSnapshot = string(data)
		}
	}
	return p.calloutSnapshot
}

func (doc exportDocument) calloutStyles() []callout.Style {
	if doc.calloutStyleSnapshot == "" {
		return nil
	}
	var styles []callout.Style
	if err := json.Unmarshal([]byte(doc.calloutStyleSnapshot), &styles); err != nil {
		return nil
	}
	return styles
}

const previewCalloutCSS = `
  .callout[data-callout] {
    margin-block: var(--doc-paragraph-gap, 16px); padding: 8px 14px;
    border-inline-start: 4px solid var(--callout-accent); border-radius: 6px;
    background: var(--doc-code-background, rgba(127, 127, 127, .08));
    background: color-mix(in srgb, var(--callout-accent) 8%, transparent);
    color: var(--doc-text, CanvasText);
  }
  .callout[data-callout] > .callout-title {
    display: flex; align-items: baseline; gap: .55em; font-size: .95em;
    font-weight: 700; overflow-wrap: anywhere;
  }
  .callout[data-callout] > summary.callout-title { cursor: pointer; list-style: none; }
  .callout[data-callout] > summary.callout-title::-webkit-details-marker { display: none; }
  .callout[data-callout] > summary.callout-title::after {
    content: ""; width: .45em; height: .45em; margin-inline-start: auto;
    align-self: center; border: solid currentColor; border-width: 0 1.5px 1.5px 0;
    transform: rotate(-45deg);
  }
  .callout[data-callout][open] > summary.callout-title::after { transform: rotate(45deg); }
  .callout[data-callout] .callout-icon { flex: 0 0 auto; }
  .callout[data-callout] > .callout-content:not(:empty) { margin-top: 6px; }
  .callout[data-callout] > .callout-content > :first-child { margin-top: 0; }
  .callout[data-callout] > .callout-content > :last-child { margin-bottom: 0; }
`
