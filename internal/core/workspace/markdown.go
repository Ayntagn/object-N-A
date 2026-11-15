package workspace

import "strings"

// SplitFrontmatter returns the preserved prefix, its YAML or TOML text, and the body.
func SplitFrontmatter(source string) (prefix, properties, body string) {
	body = strings.TrimPrefix(source, "\ufeff")
	bom := source[:len(source)-len(body)]
	open, rest, more := strings.Cut(body, "\n")
	marker := strings.TrimRight(open, " \t\r")
	if !more || (marker != "---" && marker != "+++") {
		return bom, "", body
	}
	for tail := rest; tail != ""; {
		line, next, _ := strings.Cut(tail, "\n")
		end := strings.TrimRight(line, " \t\r")
		if end == marker || (marker == "---" && end == "...") {
			return source[:len(source)-len(next)], rest[:len(rest)-len(tail)], next
		}
		tail = next
	}
	return bom, "", body
}
