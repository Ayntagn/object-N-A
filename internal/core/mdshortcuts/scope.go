package mdshortcuts

import "strings"

type mode uint8

const (
	textMode mode = iota
	inlineMathMode
	blockMathMode
	codeMode
)

func (s Scope) allows(current mode) bool {
	switch s {
	case Any:
		return current != codeMode
	case Text:
		return current == textMode
	case Math:
		return current == inlineMathMode || current == blockMathMode
	case InlineMath:
		return current == inlineMathMode
	case BlockMath:
		return current == blockMathMode
	default:
		return false
	}
}

// markdownMode classifies the start of the trigger. Inline math needs a
// closing dollar on the same line. An open display block still counts while typing.
func markdownMode(prefix, contents, suffix string) mode {
	var math mode
	inlineCode, fenceLength := 0, 0
	var fence byte
	for prefix != "" {
		line, rest, more := strings.Cut(prefix, "\n")
		prefix = rest
		if marker, length, tail := fenceMarker(line); length >= 3 {
			if fence == 0 && math != blockMathMode && inlineCode == 0 && (marker != '`' || !strings.ContainsRune(tail, '`')) {
				fence, fenceLength = marker, length
				continue
			}
			if fence == marker && length >= fenceLength && strings.TrimSpace(tail) == "" {
				fence = 0
				continue
			}
		}
		if fence != 0 {
			continue
		}
		if more && strings.TrimSpace(line) == "" && math != blockMathMode {
			inlineCode = 0
		}
		boundary := len(line)
		if !more && strings.ContainsRune(line, '$') {
			if first, _, multiline := strings.Cut(contents, "\n"); multiline {
				line += first
			} else {
				tail, _, _ := strings.Cut(suffix, "\n")
				line = line + contents + tail
			}
		}
		for i := 0; i < boundary; {
			mark := line[i]
			if mark == '\\' && inlineCode == 0 && i+1 < len(line) {
				i += 2
				continue
			}
			if mark != '`' && mark != '$' {
				i++
				continue
			}
			end := i + runLength(line[i:], mark)
			count := end - i
			switch {
			case mark == '`' && math == textMode:
				inlineCode = toggleCodeSpan(inlineCode, count)
			case mark == '$' && inlineCode == 0:
				switch {
				case count == 1 && math == textMode:
					if close := inlineMathClose(line, i); close >= 0 {
						if close >= boundary {
							return inlineMathMode
						}
						end = close + 1
					}
				case count == 2 && math == textMode:
					math = blockMathMode
				case count == 2 && math == blockMathMode:
					math = textMode
				}
			}
			i = end
		}
	}
	if fence != 0 || inlineCode != 0 {
		return codeMode
	}
	return math
}

func inlineMathClose(line string, open int) int {
	if open+1 == len(line) || mathDelimiterSpace(line[open+1]) {
		return -1
	}
	codeTicks := 0
	for i := open + 1; i < len(line); i++ {
		switch line[i] {
		case '\\':
			if codeTicks == 0 {
				i++
			}
		case '`':
			count := runLength(line[i:], '`')
			codeTicks = toggleCodeSpan(codeTicks, count)
			i += count - 1
		case '$':
			// An unrelated opening dollar, a dollar inside code, or a display
			// delimiter cannot close this span.
			if codeTicks != 0 || mathDelimiterSpace(line[i-1]) {
				return -1
			}
			if i+1 < len(line) && (line[i+1] == '$' || (line[i+1] >= '0' && line[i+1] <= '9')) {
				return -1
			}
			return i
		}
	}
	return -1
}

func mathDelimiterSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r'
}

func toggleCodeSpan(open, count int) int {
	if open == count {
		return 0
	}
	if open == 0 {
		return count
	}
	return open
}

func fenceMarker(line string) (byte, int, string) {
	trimmed := strings.TrimLeft(line, " ")
	if len(trimmed) == 0 || len(line)-len(trimmed) > 3 || trimmed[0] != '`' && trimmed[0] != '~' {
		return 0, 0, ""
	}
	marker := trimmed[0]
	count := runLength(trimmed, marker)
	return marker, count, trimmed[count:]
}

func runLength(text string, mark byte) int {
	count := 0
	for count < len(text) && text[count] == mark {
		count++
	}
	return count
}
