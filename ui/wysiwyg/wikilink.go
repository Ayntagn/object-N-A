package wysiwyg

import (
	"crypto/rand"
	"strings"

	"cyberwriter/internal/core/vaultindex"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

var wikiLabelEscapes = strings.NewReplacer(
	"\\", "\\\\", "[", "\\[", "]", "\\]", "*", "\\*", "_", "\\_", "`", "\\`",
	"~", "\\~", "<", "\\<", "&", "\\&",
)

// Project simple Wikilinks as MyGo links. An address belongs to one occurrence
// so adjacent references remain separate runs even when their targets match.
func projectWithWikilinks(parsed *parser.Document, source string, formulas map[string]*Formula, images map[string]*Image) (richtext.Text, string, map[string]string, bool) {
	var links map[string]string
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
				if link.Embed || raw[link.Start:link.End] != "[["+link.Target+"]]" {
					continue
				}
				address := "cyberwriter-wikilink:" + rand.Text()
				if links == nil {
					links = make(map[string]string)
				}
				links[address] = link.Target
				projected.WriteString(source[start : inline.Start+link.Start])
				projected.WriteString("[" + wikiLabelEscapes.Replace(link.Target) + "](" + address + ")")
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
	text, markdown, ok := projectMarkdown(parsed, source, formulas, images)
	return text, markdown, links, ok
}

// WikilinkTarget resolves an address created by this visual session.
func (s *Session) WikilinkTarget(address string) (string, bool) {
	target, ok := s.wikilinks[address]
	return target, ok
}

func restoreWikilinks(source string, links map[string]string) string {
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
			target, ok := links[inline.Dest]
			if inline.Kind != parser.Link || !ok || inline.Start < start {
				continue
			}
			label := inlines[index+1 : inline.Next]
			restored.WriteString(source[start:inline.Start])
			restored.WriteString("[[" + target)
			if len(label) != 1 || label[0].Kind != parser.Text || label[0].Text != target {
				raw := source[inline.Start:inline.End]
				if end := strings.LastIndex(raw, "]("); end > 0 {
					restored.WriteString("|" + raw[1:end])
				}
			}
			restored.WriteString("]]")
			start = inline.End
		}
	}
	restored.WriteString(source[start:])
	return restored.String()
}
