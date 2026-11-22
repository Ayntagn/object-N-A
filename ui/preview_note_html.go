package ui

import (
	"fmt"
	"net/url"
	"strings"

	"cyberwriter/internal/core/notehtml"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func (m *previewMarkup) noteHTMLPolicy() notehtml.Policy {
	return notehtml.Policy{
		Link: func(target string) string {
			dest, _ := m.previewLinkDestination(target)
			return dest
		},
		Image: m.noteHTMLImage,
		Print: m.figures != nil && m.figures.pdf,
	}
}

func (m *previewMarkup) noteHTMLImage(source string) string {
	if embedded := m.imageSource(source, false); embedded != "" {
		return embedded
	}
	if m.figures == nil || !m.figures.pdf {
		if u, err := url.Parse(source); err == nil && (u.Scheme == "https" || u.Scheme == "http") {
			return previewExternalURL(source)
		}
	}
	if m.standalone != htmlInApp {
		return standaloneRelativeURL(source)
	}
	return ""
}

func (m *previewMarkup) writeNoteHTMLBlock(index int, block *parser.Block) bool {
	markup, ok := m.noteHTMLPolicy().Fragment(m.doc.Lines(index))
	if ok && markup != "" {
		fmt.Fprintf(&m.out, `<div class="note-html"%s>%s</div>`, m.lineAttribute(block), markup)
	}
	return ok
}

// Scope raw inline tags to a single Markdown inline sequence, so user tags
// cannot close any of the surrounding generated document structure.
type previewInlineHTML struct {
	open   []string
	hidden string
}

func (state *previewInlineHTML) write(m *previewMarkup, tag notehtml.InlineTag) {
	if state.hidden != "" {
		if tag.Name == state.hidden && tag.Close {
			state.hidden = ""
		}
		return
	}
	if tag.DropContent {
		if !tag.Close && !tag.Void {
			state.hidden = tag.Name
		}
		return
	}
	if tag.Name == "" {
		return
	}
	if tag.Close {
		if len(state.open) > 0 && state.open[len(state.open)-1] == tag.Name {
			state.pop(m)
		}
		return
	}
	if tag.Name == "a" && m.inLink {
		return
	}
	if tag.Markup == "" {
		return
	}
	m.out.WriteString(tag.Markup)
	if !tag.Void {
		state.open = append(state.open, tag.Name)
		if tag.Name == "a" {
			m.inLink = true
		}
	}
}

func (state *previewInlineHTML) pop(m *previewMarkup) {
	last := len(state.open) - 1
	tag := state.open[last]
	state.open = state.open[:last]
	fmt.Fprintf(&m.out, "</%s>", tag)
	if tag == "a" {
		m.inLink = false
	}
}

// SVG and MathML need their complete subtree for namespace-aware filtering.
// MyGo can tokenize these roots as inline tags instead of an HTML block.
func (m *previewMarkup) writeInlineForeignHTML(in []parser.Inline, start, end int) (int, bool) {
	root, closing, empty := foreignHTMLTag(in[start].Text)
	if root == "" || closing {
		return start, false
	}
	last := start
	if !empty {
		depth := 1
		for i := in[start].Next; i < end; i = in[i].Next {
			if in[i].Kind != parser.HTML {
				continue
			}
			name, close, selfClosing := foreignHTMLTag(in[i].Text)
			if name != root || selfClosing {
				continue
			}
			if !close {
				depth++
				continue
			}
			depth--
			if depth == 0 {
				last = i
				break
			}
		}
		if last == start {
			return start, false
		}
	}
	source := m.doc.Source()
	from, to := in[start].Start, in[last].End
	if from < 0 || to <= from || to > len(source) {
		return start, false
	}
	markup, ok := m.noteHTMLPolicy().Fragment(source[from:to])
	if ok {
		m.out.WriteString(markup)
	}
	return last, ok
}

func foreignHTMLTag(raw string) (name string, closing, empty bool) {
	if len(raw) < 5 || raw[0] != '<' {
		return "", false, false
	}
	tag := raw[1:]
	tag, closing = strings.CutPrefix(tag, "/")
	end := strings.IndexAny(tag, " \t\r\n/>")
	if end < 0 {
		return "", false, false
	}
	switch {
	case strings.EqualFold(tag[:end], "svg"):
		name = "svg"
	case strings.EqualFold(tag[:end], "math"):
		name = "math"
	}
	return name, closing, strings.HasSuffix(raw, "/>")
}
