package workspace

import (
	"regexp"
	"strings"
	"unicode"
)

const contentsTitle = "Table of Contents"

// The native generator searches the source, including fenced examples.
var contentsHeading = regexp.MustCompile(`(?m)^(#{1,6})[\t\n\v\f\r\x{85}\p{Z}]+([^\r\n\x{85}\x{2028}\x{2029}]+)$`)

// MarkdownContents generates the editable list inserted at the current selection.
func MarkdownContents(source string) string {
	var out strings.Builder
	out.WriteString("## " + contentsTitle + "\n\n")
	for _, match := range contentsHeading.FindAllStringSubmatch(source, -1) {
		title := strings.TrimSpace(match[2])
		if title == "" || strings.EqualFold(title, contentsTitle) {
			continue
		}
		if at := strings.Index(match[0], title); at >= 0 {
			title = strings.TrimSpace(match[0][at:])
		}
		out.WriteString(strings.Repeat("  ", len(match[1])-1))
		out.WriteString("- [" + title + "](#" + HeadingAnchor(title) + ")\n")
	}
	return out.String()
}

// HeadingAnchor is the fragment spelling used by the contents generator.
func HeadingAnchor(title string) string {
	var out strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case r == ' ' || r == '-':
			if !dash {
				out.WriteByte('-')
			}
			dash = true
		case r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r):
			out.WriteRune(r)
			dash = false
		}
	}
	return out.String()
}
