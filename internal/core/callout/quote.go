package callout

import "strings"

// ReadQuoteHeader reads the marker at a parsed quote's source offset.
// titleStart and lineEnd are absolute offsets in source.
func ReadQuoteHeader(source string, start int) (header Header, titleStart, lineEnd int, ok bool) {
	if start < 0 || start >= len(source) || source[start] != '>' {
		return Header{}, 0, 0, false
	}
	line := source[start+1:]
	if end := strings.IndexAny(line, "\r\n"); end >= 0 {
		line = line[:end]
	}
	content := strings.TrimLeft(line, " \t")
	header, ok = ParseHeader(content)
	lineEnd = start + 1 + len(line)
	titleStart = lineEnd - len(content) + header.TitleOffset
	return
}

// ExplicitQuoteBreak reports whether the source bytes of a SoftBreak
// contain a marker for each enclosing quote. A lazy continuation keeps
// the regular Markdown soft break.
func ExplicitQuoteBreak(raw string, depth int) bool {
	if depth <= 0 || raw == "" || raw[0] != '\n' && raw[0] != '\r' {
		return false
	}
	raw = strings.TrimLeft(raw, "\r\n")
	for range depth {
		raw = strings.TrimLeft(raw, " \t")
		if raw == "" || raw[0] != '>' {
			return false
		}
		raw = raw[1:]
	}
	return true
}
