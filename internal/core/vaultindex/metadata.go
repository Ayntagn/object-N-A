package vaultindex

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/markdown/parser"
)

type noteMetadata struct {
	tags  []string
	links []noteLink
}

type noteLink struct {
	target   string
	markdown bool
}

func parseMetadata(properties, body string) noteMetadata {
	metadata := noteMetadata{tags: frontmatterTags(properties)}
	if strings.Contains(body, "[") || mayHaveInlineTag(body) {
		metadata.parseBody(body)
	}
	metadata.tags = uniqueTags(metadata.tags)
	return metadata
}

func mayHaveInlineTag(source string) bool {
	for {
		_, rest, found := strings.Cut(source, "#")
		if !found {
			return false
		}
		if next, _ := utf8.DecodeRuneInString(rest); tagRune(next) {
			return true
		}
		source = rest
	}
}

func uniqueTags(tags []string) []string {
	slices.Sort(tags)
	tags = slices.Clone(slices.Compact(tags))
	for i := range tags {
		tags[i] = strings.Clone(tags[i])
	}
	return tags
}

func (metadata *noteMetadata) parseBody(body string) {
	document := parser.New(parser.GFM | parser.ExtFootnotes | parser.ExtMath | parser.ExtDollarMath)
	document.Set(body)
	for block := 1; block < document.Len(); block++ {
		inlines := document.Inlines(block)
		if len(inlines) == 0 {
			continue
		}
		start, end, plainAt := inlines[0].Start, inlines[0].Start, 0
		for at := 0; at < len(inlines); {
			inline := inlines[at]
			end = max(end, inline.End)
			switch inline.Kind {
			case parser.Link, parser.Image, parser.Code, parser.HTML, parser.Math, parser.FootnoteRef:
				metadata.scanProse(body, start, inline.Start, inlines[plainAt:at])
				if inline.Kind == parser.Link && !inline.Auto {
					metadata.links = append(metadata.links, noteLink{target: inline.Dest, markdown: true})
				}
				start, plainAt, at = inline.End, inline.Next, inline.Next
			default:
				at++
			}
		}
		metadata.scanProse(body, start, end, inlines[plainAt:])
	}
}

func (m *noteMetadata) scanProse(source string, start, end int, inlines []parser.Inline) {
	if start >= end {
		return
	}
	prose := source[start:end]
	// A Wikilink can span several text nodes when its label uses emphasis.
	links := Wikilinks(prose)
	for _, link := range links {
		m.links = append(m.links, noteLink{target: link.Target})
	}
	linkAt := 0
	for _, inline := range inlines {
		if inline.Kind != parser.Text {
			continue
		}
		from, to := inline.Start-start, inline.End-start
		for linkAt < len(links) && links[linkAt].End <= from {
			linkAt++
		}
		for linkAt < len(links) && links[linkAt].Start < to {
			link := links[linkAt]
			if link.Start > from {
				m.addInlineTags(prose[from:link.Start])
			}
			from = link.End
			if from >= to {
				break
			}
			linkAt++
		}
		if from < to {
			m.addInlineTags(prose[from:to])
		}
	}
}

func (m *noteMetadata) addInlineTags(text string) {
	for cursor := 0; cursor < len(text); {
		next := strings.IndexByte(text[cursor:], '#')
		if next < 0 {
			break
		}
		marker := cursor + next
		cursor = marker + 1
		previous, _ := utf8.DecodeLastRuneInString(text[:marker])
		if previous == '#' || tagRune(previous) || escaped(text, marker) {
			continue
		}
		end := cursor
		for end < len(text) {
			char, width := utf8.DecodeRuneInString(text[end:])
			if !tagRune(char) {
				break
			}
			end += width
		}
		if end > cursor {
			m.tags = append(m.tags, strings.ToLower(text[cursor:end]))
			cursor = end
		}
	}
}

func tagRune(char rune) bool {
	return unicode.IsLetter(char) || unicode.IsDigit(char) || unicode.IsMark(char) ||
		char == '_' || char == '-' || char == '/'
}

// The top-level tags field may hold a single tag, an inline list, or a
// block list. Other frontmatter properties do not contribute tags.
func frontmatterTags(properties string) []string {
	var tags []string
	list := false
	for line := range strings.SplitSeq(properties, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if list && (strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "-\t")) {
			tags = appendPropertyTag(tags, trimmed[1:])
			continue
		}
		list = false
		if strings.TrimLeft(line, " \t") != line {
			continue
		}
		key, value, found := strings.Cut(trimmed, ":")
		if !found || strings.TrimSpace(key) != "tags" {
			continue
		}
		value = strings.TrimSpace(value)
		switch {
		case value == "" || strings.HasPrefix(value, "#"):
			list = true
		case strings.HasPrefix(value, "["):
			if items, _, closed := strings.Cut(value[1:], "]"); closed {
				for item := range strings.SplitSeq(items, ",") {
					tags = appendPropertyTag(tags, item)
				}
			}
		default:
			tags = appendPropertyTag(tags, value)
		}
	}
	return tags
}

func appendPropertyTag(tags []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "#") {
		return tags
	}
	if quote := value[0]; quote == '"' || quote == '\'' {
		unquoted, _, closed := strings.Cut(value[1:], string(quote))
		if !closed {
			return tags
		}
		value = unquoted
	} else {
		value, _, _ = strings.Cut(value, " #")
		value, _, _ = strings.Cut(value, "\t#")
	}
	tag := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if tag != "" && strings.IndexFunc(tag, func(char rune) bool { return !tagRune(char) }) < 0 {
		tags = append(tags, strings.ToLower(tag))
	}
	return tags
}
