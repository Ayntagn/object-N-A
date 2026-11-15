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

// markdownMode follows the source up to the trigger. Unclosed dollar
// delimiters count while typing, even before the Markdown parser sees math.
func markdownMode(prefix string) mode {
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
		for i := 0; i < len(line); {
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
				if inlineCode == 0 {
					inlineCode = count
				} else if inlineCode == count {
					inlineCode = 0
				}
			case mark == '$' && inlineCode == 0:
				switch {
				case count == 1 && math == textMode:
					math = inlineMathMode
				case count == 1 && math == inlineMathMode:
					math = textMode
				case count == 2 && math == textMode:
					math = blockMathMode
				case count == 2 && math == blockMathMode:
					math = textMode
				}
			}
			i = end
		}
		if more && math == inlineMathMode {
			math = textMode
		}
	}
	if fence != 0 || inlineCode != 0 {
		return codeMode
	}
	return math
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
