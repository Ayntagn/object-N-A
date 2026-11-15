package ui

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var exportHashtag = regexp.MustCompile(`#[A-Za-z][A-Za-z0-9_/-]*`)

// EV-UI-137 and EV-UI-140: the export renderer decorates tag text inside a Wiki link.
func standaloneWikilinkLabel(label string) string {
	var markup strings.Builder
	from := 0
	for _, match := range exportHashtag.FindAllStringIndex(label, -1) {
		start, end := match[0], match[1]
		previous, _ := utf8.DecodeLastRuneInString(label[:start])
		if start > 0 && !unicode.IsSpace(previous) {
			continue
		}
		if end < len(label) && (label[end] == ':' || label[end] == ';') {
			continue
		}
		tag := label[start:end]
		if exportHexColor(tag[1:]) {
			continue
		}
		markup.WriteString(html.EscapeString(label[from:start]))
		markup.WriteString(`<span class="tag">`)
		markup.WriteString(tag)
		markup.WriteString(`</span>`)
		from = end
	}
	markup.WriteString(html.EscapeString(label[from:]))
	return markup.String()
}

func exportHexColor(value string) bool {
	switch len(value) {
	case 3, 4, 6, 8:
		_, err := strconv.ParseUint(value, 16, 32)
		return err == nil
	}
	return false
}
