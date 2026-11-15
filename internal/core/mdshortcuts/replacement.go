package mdshortcuts

import (
	"fmt"
	"strconv"
	"strings"
)

type partKind uint8

const (
	literal partKind = iota
	selection
	capture
	stop
	placeholder
)

type part struct {
	kind  partKind
	index int
	text  string
}

var escapeTemplate = strings.NewReplacer(`\`, `\\`, `$`, `\$`, `}`, `\}`)

func parseReplacement(source string, regex bool, groupCount int) ([]part, bool, int, error) {
	var parts []part
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			parts = append(parts, part{kind: literal, text: text.String()})
			text.Reset()
		}
	}
	defaults := map[int]string{}
	stops := map[int]bool{}
	visual := false
	for i := 0; i < len(source); {
		if escapedTemplateByte(source, i) {
			text.WriteByte(source[i+1])
			i += 2
			continue
		}
		if strings.HasPrefix(source[i:], "${VISUAL}") {
			flush()
			parts = append(parts, part{kind: selection})
			visual = true
			i += len("${VISUAL}")
			continue
		}
		if regex && strings.HasPrefix(source[i:], "[[") {
			end := digitsEnd(source, i+2)
			if end > i+2 && strings.HasPrefix(source[end:], "]]") {
				index, err := strconv.Atoi(source[i+2 : end])
				if err != nil || index >= groupCount {
					return nil, false, 0, fmt.Errorf("capture %s has no matching group", source[i:end+2])
				}
				flush()
				parts = append(parts, part{kind: capture, index: index})
				i = end + 2
				continue
			}
		}
		if source[i] != '$' {
			text.WriteByte(source[i])
			i++
			continue
		}
		p, end, err := parseStop(source, i)
		if err != nil {
			return nil, false, 0, err
		}
		if end == i {
			text.WriteByte('$')
			i++
			continue
		}
		if p.kind == placeholder {
			if old, ok := defaults[p.index]; ok && old != p.text {
				return nil, false, 0, fmt.Errorf("tab stop $%d has conflicting defaults", p.index)
			}
			defaults[p.index] = p.text
		}
		stops[p.index] = true
		flush()
		parts = append(parts, p)
		i = end
	}
	flush()
	return parts, visual, len(stops), nil
}

func escapedTemplateByte(source string, i int) bool {
	return source[i] == '\\' && i+1 < len(source) && strings.ContainsRune(`$}\`, rune(source[i+1]))
}

func digitsEnd(source string, start int) int {
	for start < len(source) && source[start] >= '0' && source[start] <= '9' {
		start++
	}
	return start
}

func parseStop(source string, at int) (part, int, error) {
	start := at + 1
	braced := start < len(source) && source[start] == '{'
	if braced {
		start++
	}
	end := digitsEnd(source, start)
	if start == end {
		return part{}, at, nil
	}
	index, err := strconv.Atoi(source[start:end])
	if err != nil || index >= 1<<31-1 {
		return part{}, at, fmt.Errorf("invalid tab stop at byte %d", at)
	}
	p := part{kind: stop, index: index}
	if !braced {
		return p, end, nil
	}
	if end < len(source) {
		switch source[end] {
		case '}':
			return p, end + 1, nil
		case ':':
			if text, next, ok := placeholderText(source, end+1); ok {
				return part{kind: placeholder, index: index, text: text}, next, nil
			}
		}
	}
	return part{}, at, fmt.Errorf("invalid placeholder at byte %d", at)
}

func placeholderText(source string, start int) (string, int, bool) {
	var text strings.Builder
	depth := 0
	for i := start; i < len(source); i++ {
		if escapedTemplateByte(source, i) {
			i++
			text.WriteByte(source[i])
			continue
		}
		switch source[i] {
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return text.String(), i + 1, true
			}
			depth--
		}
		text.WriteByte(source[i])
	}
	return "", start, false
}

func renderReplacement(parts []part, selected string, groups []string) string {
	var text strings.Builder
	for _, p := range parts {
		switch p.kind {
		case literal:
			text.WriteString(escapeTemplate.Replace(p.text))
		case selection:
			text.WriteString(escapeTemplate.Replace(selected))
		case capture:
			text.WriteString(escapeTemplate.Replace(groups[p.index]))
		case stop:
			// Latex Suite numbers the first stop 0. TextMate reserves 0 for exit.
			fmt.Fprintf(&text, "${%d}", p.index+1)
		case placeholder:
			fmt.Fprintf(&text, "${%d:%s}", p.index+1, escapeTemplate.Replace(p.text))
		}
	}
	return text.String()
}
