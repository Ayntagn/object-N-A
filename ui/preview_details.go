package ui

import (
	"fmt"
	"html"
	"sort"

	"cyberwriter/internal/core/markdowndetails"
)

type previewDetailsBoundary struct {
	summary string
	open    bool
	end     bool
}

func (m *previewMarkup) setSource(source string) {
	projected, blocks := markdowndetails.Project(source, m.doc.Extensions())
	inserted := setPreviewDocument(m.doc, projected)
	m.details = nil
	if len(blocks) == 0 {
		return
	}
	m.details = make(map[int]previewDetailsBoundary, len(blocks)*2)
	for _, block := range blocks {
		// The math normalizer can add a backslash before a false formula.
		start := block.Start + sort.SearchInts(inserted, block.Start)
		end := block.Close + sort.SearchInts(inserted, block.Close)
		m.details[start] = previewDetailsBoundary{summary: block.Summary, open: block.Open}
		m.details[end] = previewDetailsBoundary{end: true}
	}
}

func (m *previewMarkup) writeDetailsBoundary(at int) bool {
	boundary, ok := m.details[at]
	if !ok {
		return false
	}
	tag, titleTag := "details", "summary"
	open := ""
	if m.figures != nil && m.figures.pdf {
		tag, titleTag = "section", "div"
	} else if boundary.open {
		open = " open"
	}
	if boundary.end {
		fmt.Fprintf(&m.out, "</%s>", tag)
	} else {
		fmt.Fprintf(&m.out, `<%s class="markdown-details"%s%s><%s>%s</%s>`,
			tag, m.lineAttributeAt(at), open, titleTag, html.EscapeString(boundary.summary), titleTag)
	}
	return true
}
