package wysiwyg

import (
	"crypto/rand"
	"strings"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

var wikiLabelEscapes = strings.NewReplacer(
	"\\", "\\\\", "[", "\\[", "]", "\\]", "*", "\\*", "_", "\\_", "`", "\\`",
	"~", "\\~", "<", "\\<", "&", "\\&",
)

type visualWikilink struct {
	target, label, token string
}

// Project simple Wikilinks and aliased PDF references as MyGo links. Each
// occurrence has its own address so adjacent references remain separate runs.
func projectWithWikilinks(parsed *parser.Document, source string, formulas map[string]*Formula, images map[string]*Image, calloutBody bool) (richtext.Text, string, map[string]visualWikilink, bool) {
	var links map[string]visualWikilink
	var projected strings.Builder
	start := 0
	for block := 1; block < parsed.Len(); block++ {
		inlines := parsed.Inlines(block)
		for index := 0; index < len(inlines); index++ {
			inline := inlines[index]
			if inline.Kind == parser.Link || inline.Kind == parser.Image {
				index = inline.Next - 1
				continue
			}
			if inline.Kind != parser.Text || inline.Start < start || inline.End > len(source) || !strings.Contains(inline.Text, "[[") {
				continue
			}
			raw := source[inline.Start:inline.End]
			for _, link := range vaultindex.Wikilinks(raw) {
				token := raw[link.Start:link.End]
				name, _, _ := strings.Cut(link.Target, "#")
				if link.Embed || token != "[["+link.Target+"]]" && !workspace.IsPDFPath(strings.TrimSpace(name)) {
					continue
				}
				address := "cyberwriter-wikilink:" + rand.Text()
				if links == nil {
					links = make(map[string]visualWikilink)
				}
				links[address] = visualWikilink{target: link.Target, label: link.Label, token: token}
				projected.WriteString(source[start : inline.Start+link.Start])
				projected.WriteString("[" + wikiLabelEscapes.Replace(link.Label) + "](" + address + ")")
				start = inline.Start + link.End
			}
		}
	}
	if links != nil {
		projected.WriteString(source[start:])
		source = projected.String()
		parsed = parser.New(Extensions)
		parsed.Set(source)
	}
	text, markdown, ok := projectMarkdown(parsed, source, formulas, images, calloutBody)
	return text, markdown, links, ok
}

// WikilinkTarget resolves an address created by this visual session.
func (s *Session) WikilinkTarget(address string) (string, bool) {
	link, ok := s.wikilinks[address]
	return link.target, ok
}

func restoreWikilinks(source string, links map[string]visualWikilink) string {
	if len(links) == 0 || !strings.Contains(source, "cyberwriter-wikilink:") {
		return source
	}
	parsed := parser.New(Extensions)
	parsed.Set(source)
	var restored strings.Builder
	start := 0
	for block := 1; block < parsed.Len(); block++ {
		inlines := parsed.Inlines(block)
		for index, inline := range inlines {
			link, ok := links[inline.Dest]
			if inline.Kind != parser.Link || !ok || inline.Start < start {
				continue
			}
			label := inlines[index+1 : inline.Next]
			restored.WriteString(source[start:inline.Start])
			if len(label) == 1 && label[0].Kind == parser.Text && label[0].Text == link.label {
				restored.WriteString(link.token)
			} else {
				restored.WriteString("[[" + link.target)
				raw := source[inline.Start:inline.End]
				if end := strings.LastIndex(raw, "]("); end > 0 {
					restored.WriteString("|" + raw[1:end])
				}
				restored.WriteString("]]")
			}
			start = inline.End
		}
	}
	restored.WriteString(source[start:])
	return restored.String()
}
