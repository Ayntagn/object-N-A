package ui

import (
	"fmt"
	"html"
	"path"
	"strconv"
	"strings"

	"cyberwriter/internal/core/exportdoc"
	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/ui/wysiwyg"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type figureRender struct {
	numbered bool
	pdf      bool
	counter  int
}

type deferredWikiFigure struct {
	marker, src, alt, caption, anchor string
}

// EV-UI-133/134: captions belong to images that occupy a paragraph.
func (m *previewMarkup) figureParagraph(in []parser.Inline, anchor string, root bool) bool {
	var start int
	for start < len(in) && blankInline(in[start]) {
		start++
	}
	if start == len(in) {
		return false
	}
	n := in[start]
	if n.Kind == parser.Image {
		alt := strings.TrimSpace(parser.PlainText(in[start+1 : n.Next]))
		if root && start == 0 {
			if transform, _, ok := wysiwyg.ReadBlockImageTransform(m.doc.Source(), in); ok {
				m.writeTransformedFigure(m.markdownImageSource(n.Dest), alt, alt, n.Title, anchor, transform)
				return true
			}
		}
		for index := n.Next; index < len(in); index++ {
			if !blankInline(in[index]) {
				return false
			}
		}
		if alt == "" {
			return false
		}
		m.writeFigure(m.markdownImageSource(n.Dest), alt, n.Title, anchor)
		return true
	}
	if root && start == 0 {
		if link, transform, _, ok := wysiwyg.ReadWikiBlockImage(m.doc.Source(), in); ok {
			if src := m.imageSource(link.Target, true); src != "" {
				alt, caption := path.Base(link.Target), ""
				width, _ := strconv.Atoi(link.Label)
				if link.Label != link.Target && width <= 0 {
					alt, caption = link.Label, link.Label
				}
				m.writeTransformedFigure(src, alt, caption, "", anchor, transform)
				return true
			}
		}
	}
	if len(in) != 1 || n.Kind != parser.Text || m.resources == nil {
		return false
	}
	raw := strings.TrimSpace(m.rawInline(n))
	links := vaultindex.Wikilinks(raw)
	if len(links) != 1 || !links[0].Embed || links[0].Start != 0 || links[0].End != len(raw) {
		return false
	}
	_, caption, hasAlias := strings.Cut(raw[3:len(raw)-2], "|")
	caption = strings.TrimSpace(caption)
	if !hasAlias || caption == "" {
		return false
	}
	target := links[0].Target
	switch strings.ToLower(path.Ext(target)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
	default:
		return false
	}
	src := m.imageSource(target, true)
	if width, err := strconv.Atoi(caption); err == nil && width > 0 {
		if src == "" {
			return false
		}
		fmt.Fprintf(&m.out, "<p%s>", anchor)
		m.writeImage(src, "", "", width)
		m.out.WriteString("</p>")
		return true
	}
	if m.standalone != htmlInApp {
		if src == "" {
			return false
		}
		// The standalone renderer assigns Wiki image numbers after the Markdown AST.
		marker := "<!--cw-wiki-figure-" + strconv.Itoa(len(m.wikiFigures)) + "-->"
		m.wikiFigures = append(m.wikiFigures, deferredWikiFigure{marker: marker, src: src, alt: target, caption: caption, anchor: anchor})
		m.out.WriteString(marker)
		return true
	}
	m.writeFigure(src, caption, "", anchor)
	return true
}

func blankInline(inline parser.Inline) bool {
	return inline.Kind == parser.Text && strings.TrimSpace(inline.Text) == ""
}

func (m *previewMarkup) writeFigure(src, alt, title, anchor string) {
	m.writeCaptionedFigure(src, alt, alt, title, anchor)
}

func (m *previewMarkup) writeCaptionedFigure(src, alt, caption, title, anchor string) {
	label, caption := exportdoc.FigureCaption(caption, m.figures.numbered, &m.figures.counter)
	fmt.Fprintf(&m.out, `<figure class="md-figure"%s>`, anchor)
	if src == "" {
		src = "data:,"
	}
	m.writeImage(src, alt, title, 0)
	m.out.WriteString("<figcaption>")
	if label != "" {
		fmt.Fprintf(&m.out, `<span class="figure-label">%s</span> `, label)
	}
	m.out.WriteString(html.EscapeString(caption))
	m.out.WriteString("</figcaption></figure>")
}

func (m *previewMarkup) fillWikiFigures(body string) string {
	if len(m.wikiFigures) == 0 {
		return body
	}
	replacements := make([]string, 0, len(m.wikiFigures)*2)
	m.out.Reset()
	for _, figure := range m.wikiFigures {
		m.writeCaptionedFigure(figure.src, figure.alt, figure.caption, "", figure.anchor)
		replacements = append(replacements, figure.marker, m.out.String())
		m.out.Reset()
	}
	return strings.NewReplacer(replacements...).Replace(body)
}

func (m *previewMarkup) writeImage(src, alt, title string, width int) {
	fmt.Fprintf(&m.out, `<img src="%s" alt="%s"`, src, html.EscapeString(alt))
	if title != "" {
		fmt.Fprintf(&m.out, ` title="%s"`, html.EscapeString(title))
	}
	if width > 0 {
		fmt.Fprintf(&m.out, ` width="%d"`, width)
	}
	if m.standalone != htmlInApp && src != "data:," && !strings.HasPrefix(src, "data:") {
		m.out.WriteString(` loading="lazy"`)
	}
	m.out.WriteByte('>')
}
