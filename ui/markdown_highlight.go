package ui

import (
	"sort"
	"strings"

	"cyberwriter/internal/core/markdownhighlight"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func (m *previewMarkup) highlightProse(n parser.Inline) {
	first := sort.Search(len(m.highlights), func(i int) bool { return m.highlights[i].Offset >= n.Start })
	if first == len(m.highlights) || m.highlights[first].Offset >= n.End {
		m.prose(n)
		return
	}
	source := m.doc.Source()
	fragment := n
	writeText := func(end int) {
		if end > fragment.Start {
			fragment.End = end
			fragment.Text = previewLiteral(source[fragment.Start:end])
			m.prose(fragment)
		}
	}
	for _, marker := range m.highlights[first:] {
		if marker.Offset >= n.End {
			break
		}
		writeText(marker.Offset)
		if marker.Opening {
			m.out.WriteString("<mark class=\"cw-highlight\">")
		} else {
			m.out.WriteString("</mark>")
		}
		fragment.Start = marker.Offset + 2
	}
	writeText(n.End)
}

type nativeHighlightProjection struct{ source, rendered string }

func nativeHighlightedMarkdown(owner mygoui.Element, source string, extensions parser.Extensions) string {
	state := mygoui.Local(owner, "Markdown highlight projection", func() nativeHighlightProjection { return nativeHighlightProjection{} })
	if state.source != source {
		state.source, state.rendered = source, source
		if strings.Contains(source, "==") {
			doc := parser.New(extensions)
			doc.Set(source)
			state.rendered = markdownhighlight.Rewrite(doc, "<mark>", "</mark>")
		}
	}
	return state.rendered
}
