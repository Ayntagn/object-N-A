// Package mermaid recognizes the fenced blocks used for Mermaid diagrams.
package mermaid

import "strings"

// Background reads the optional card background from a Mermaid info string.
// EV-UI-047 observed the case-insensitive language and background aliases.
func Background(info string) (background string, ok bool) {
	info = strings.TrimSpace(info)
	lower := strings.ToLower(info)
	if lower == "mermaid" {
		return "", true
	}
	if !strings.HasPrefix(lower, "mermaid[") {
		return "", false
	}
	options, _, closed := strings.Cut(info[len("mermaid["):], "]")
	if closed {
		for entry := range strings.SplitSeq(options, ",") {
			key, value, hasValue := strings.Cut(entry, ":")
			key = strings.ToLower(strings.TrimSpace(key))
			if hasValue && (key == "bg" || key == "background") {
				background = strings.TrimSpace(value)
			}
		}
	}
	return background, true
}
