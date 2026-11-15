package vaultindex

import "strings"

// Wikilink describes a reference in raw Markdown, with a half-open byte range.
// Target includes the optional heading. Label is the alias or the target.
type Wikilink struct {
	Start, End    int
	Target, Label string
	Embed         bool
}

// Wikilinks scans raw inline Markdown for unescaped Wikilinks and embeds.
// Pass only prose spans when scanning a parsed document.
func Wikilinks(text string) []Wikilink {
	var links []Wikilink
	for cursor := 0; cursor+1 < len(text); {
		next := strings.Index(text[cursor:], "[[")
		if next < 0 {
			break
		}
		open := cursor + next
		cursor = open + 2
		if escaped(text, open) {
			continue
		}
		close := cursor
		for close+1 < len(text) {
			pair := text[close : close+2]
			if text[close] == '\n' || text[close] == '\r' ||
				((pair == "]]" || pair == "[[") && !escaped(text, close)) {
				break
			}
			close++
		}
		cursor = close
		if close+1 >= len(text) || text[close:close+2] != "]]" {
			continue
		}
		cursor += 2
		target, label := wikiTargetAndLabel(text[open+2 : close])
		if target == "" {
			continue
		}
		embed := open > 0 && text[open-1] == '!' && !escaped(text, open-1)
		start := open
		if embed {
			start--
		}
		links = append(links, Wikilink{Start: start, End: cursor, Target: target, Label: label, Embed: embed})
	}
	return links
}

func wikiTargetAndLabel(text string) (target, label string) {
	target, label, alias := strings.Cut(text, "|")
	if alias {
		// GFM tables require the alias separator to be escaped.
		target = strings.TrimSuffix(target, "\\")
	}
	target, label = strings.TrimSpace(target), strings.TrimSpace(label)
	if label == "" {
		label = target
	}
	return target, label
}

func escaped(text string, pos int) bool {
	start := pos
	for start > 0 && text[start-1] == '\\' {
		start--
	}
	return (pos-start)%2 != 0
}
