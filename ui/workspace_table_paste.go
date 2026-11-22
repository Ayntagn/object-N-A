package ui

import (
	"strings"

	"cyberwriter/ui/wysiwyg"
)

// pasteMarkdownTable recognizes a complete Markdown table on the clipboard.
// Keep each row's original spacing and inline Markdown for the saved note.
func pasteMarkdownTable(text string) (string, bool) {
	if !strings.Contains(text, "|") || !strings.Contains(text, "\n") {
		return "", false
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for len(lines) > 0 && strings.Trim(lines[0], " \t") == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.Trim(lines[len(lines)-1], " \t") == "" {
		lines = lines[:len(lines)-1]
	}
	markdown := strings.Join(lines, "\n")
	if !wysiwyg.SupportsMarkdownTable(markdown) {
		return "", false
	}
	return markdown, true
}
