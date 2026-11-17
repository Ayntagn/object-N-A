package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const (
	mixedSpacingOther = iota
	mixedSpacingHan
	mixedSpacingLatin
)

// mixedLanguageInsertion only changes the current input. A generated space
// before the existing right-hand text should stay to the right of the caret.
func mixedLanguageInsertion(snapshot editor.Snapshot, start, end int, input string) (text string, trailingSpace bool) {
	if input == "" || strings.ContainsAny(input, "\r\n\u2028\u2029") ||
		start < 0 || end < start || end > snapshot.Len() {
		return input, false
	}
	leftRune := mixedSpacingPreviousRune(snapshot, start)
	rightRune, _ := utf8.DecodeRuneInString(snapshot.Slice(end, min(snapshot.Len(), end+utf8.UTFMax)))
	positions := mixedSpacingBoundaries(leftRune, input, rightRune)
	if len(positions) == 0 {
		return input, false
	}

	lineStart := snapshot.LineStart(snapshot.LineOf(start))
	left := snapshot.Slice(lineStart, start)
	right := snapshot.Slice(end, snapshot.LineEnd(snapshot.LineOf(end)))
	line := left + input + right
	document, bodyStart, frontmatter := mixedSpacingMarkdown(snapshot, lineStart, line)
	if frontmatter {
		return input, false
	}
	return mixedSpacingApply(input, positions, func(position int) bool {
		at := len(left) + position
		return mixedSpacingInSyntax(line, at) || mixedSpacingInMarkdown(document, lineStart+at-bodyStart)
	})
}

// spaceMixedLanguage also handles rendered editor text, whose parser provides
// its own syntax boundaries. Zero denotes a missing adjacent character.
func spaceMixedLanguage(left rune, input string, right rune) (text string, trailingSpace bool) {
	if input == "" || strings.ContainsAny(input, "\r\n\u2028\u2029") {
		return input, false
	}
	return mixedSpacingApply(input, mixedSpacingBoundaries(left, input, right), nil)
}

func mixedSpacingApply(input string, positions []int, protected func(int) bool) (string, bool) {
	var result strings.Builder
	last := 0
	for _, position := range positions {
		if protected != nil && protected(position) {
			continue
		}
		result.WriteString(input[last:position])
		result.WriteByte(' ')
		last = position
	}
	if result.Len() == 0 {
		return input, false
	}
	result.WriteString(input[last:])
	return result.String(), last == len(input)
}

func mixedSpacingPreviousRune(snapshot editor.Snapshot, start int) rune {
	for offset := start; offset > 0; {
		r, width := utf8.DecodeLastRuneInString(snapshot.Slice(max(0, offset-utf8.UTFMax), offset))
		if !unicode.IsMark(r) {
			return r
		}
		offset -= width
	}
	return 0
}

func mixedSpacingBoundaries(left rune, input string, right rune) []int {
	previous := mixedSpacingScript(left)
	var positions []int
	for offset, r := range input {
		if unicode.IsMark(r) {
			continue
		}
		current := mixedSpacingScript(r)
		if previous != mixedSpacingOther && current != mixedSpacingOther && previous != current {
			positions = append(positions, offset)
		}
		previous = current
	}
	if next := mixedSpacingScript(right); previous != mixedSpacingOther && next != mixedSpacingOther && previous != next {
		positions = append(positions, len(input))
	}
	return positions
}

func mixedSpacingScript(r rune) int {
	switch {
	case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
		return mixedSpacingLatin
	case r < utf8.RuneSelf:
		return mixedSpacingOther
	case unicode.Is(unicode.Han, r):
		return mixedSpacingHan
	case unicode.IsLetter(r) && unicode.Is(unicode.Latin, r):
		return mixedSpacingLatin
	default:
		return mixedSpacingOther
	}
}

func mixedSpacingMarkdown(snapshot editor.Snapshot, lineStart int, line string) (*parser.Document, int, bool) {
	firstLine := line
	if lineStart > 0 {
		firstLine = snapshot.Line(0)
	}
	firstLine = strings.TrimRight(strings.TrimPrefix(firstLine, "\ufeff"), " \t")
	mayHaveFrontmatter := firstLine == "---" || firstLine == "+++"
	const delimiters = "`~$\\[<"
	hasSyntax := strings.ContainsAny(line, delimiters+"\t") || strings.Contains(line, "    ")
	if !hasSyntax && !mayHaveFrontmatter {
		// Scan the existing pieces before the edited line without copying
		// the document. Plain prose does not need a Markdown parse.
		snapshot.Chunks(0, lineStart, func(chunk string) bool {
			hasSyntax = strings.ContainsAny(chunk, delimiters)
			return !hasSyntax
		})
		if !hasSyntax {
			return nil, 0, false
		}
	}

	bodyStart := 0
	if mayHaveFrontmatter {
		prefix, _, _ := workspace.SplitFrontmatter(snapshot.Slice(0, lineStart))
		if prefix == "" || prefix == "\ufeff" {
			return nil, 0, true
		}
		bodyStart = len(prefix)
	}

	// Earlier inline markup cannot cross a blank line. Fences, display
	// math, HTML, and continuations indented under a list can cross one.
	restart := lineStart
	for prev := snapshot.LineOf(lineStart) - 1; prev >= 0; prev-- {
		at := snapshot.LineStart(prev)
		if at < bodyStart || strings.Trim(snapshot.Line(prev), " \t") == "" {
			break
		}
		restart = at
	}
	if restart > bodyStart && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") ||
		mixedSpacingMayCrossBlank(snapshot, bodyStart, restart)) {
		restart = bodyStart
	}
	body := snapshot.Slice(restart, lineStart) + line
	if restart == 0 {
		trimmed := strings.TrimPrefix(body, "\ufeff")
		restart = len(body) - len(trimmed)
		body = trimmed
	}
	if !strings.ContainsAny(body, delimiters+"\t") && !strings.Contains(body, "    ") {
		return nil, restart, false
	}
	// Streaming mode recognizes syntax still being entered, including
	// partial link targets.
	doc := parser.New(parser.Default | parser.ExtDollarMath)
	doc.SetStreaming(true)
	doc.Set(body)
	return doc, restart, false
}

func mixedSpacingMayCrossBlank(snapshot editor.Snapshot, start, end int) bool {
	marker := func(text string) bool {
		return strings.Contains(text, "```") || strings.Contains(text, "~~~") ||
			strings.Contains(text, "$$") || strings.Contains(text, `\[`) || strings.Contains(text, "<")
	}
	var tail string
	found := false
	snapshot.Chunks(start, end, func(chunk string) bool {
		// Check both the piece and the delimiter bytes around its start.
		join := tail + chunk[:min(len(chunk), 2)]
		found = marker(chunk) || marker(join)
		if len(chunk) >= 2 {
			tail = chunk[len(chunk)-2:]
		} else {
			tail = join[max(0, len(join)-2):]
		}
		return !found
	})
	return found
}

func mixedSpacingInMarkdown(doc *parser.Document, at int) bool {
	if doc == nil {
		return false
	}
	for i := 1; i < doc.Len(); i++ {
		block := doc.Block(i)
		if at < block.Start {
			break
		}
		if at >= block.End {
			i = block.Next - 1
			continue
		}
		switch block.Kind {
		case parser.CodeBlock, parser.MathBlock, parser.HTMLBlock:
			return true
		}
		inlines := doc.Inlines(i)
		for j, span := range inlines {
			if at <= span.Start || at >= span.End {
				continue
			}
			switch span.Kind {
			case parser.Code, parser.Math, parser.HTML, parser.FootnoteRef:
				return true
			case parser.Link, parser.Image:
				// Only the nested label is prose. The rest of the link
				// contains its destination, reference ID, or title.
				if span.Auto || span.Next <= j+1 ||
					at < inlines[j+1].Start || at > inlines[span.Next-1].End {
					return true
				}
			}
		}
	}
	return false
}

// Check syntax that the Markdown parser does not expose as prose spans,
// including Vault tags and incomplete wiki targets.
func mixedSpacingInSyntax(line string, at int) bool {
	before := line[:at]
	if opener := strings.LastIndex(before, "[["); opener >= 0 && mixedSpacingUnescaped(line, opener) {
		if !strings.Contains(before[opener:], "]]") && !strings.Contains(before[opener:], "|") {
			return true
		}
	}
	for _, markers := range [][2]string{{`\(`, `\)`}, {`\[`, `\]`}} {
		if opener := strings.LastIndex(before, markers[0]); opener >= 0 &&
			mixedSpacingUnescaped(line, opener) && opener > strings.LastIndex(before, markers[1]) {
			return true
		}
	}
	// The matching reference definition can sit outside the parsed excerpt.
	if opener := strings.LastIndex(before, "]["); opener >= 0 && strings.Contains(before[:opener], "[") &&
		!strings.Contains(before[opener+2:], "]") {
		return true
	}
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "[") {
		if end := strings.IndexByte(trimmed, ']'); end > 1 && strings.HasPrefix(trimmed[end:], "]:") {
			return true
		}
	}

	tagStart := strings.LastIndexFunc(before, func(r rune) bool { return !mixedSpacingTagRune(r) })
	if tagStart >= 0 && before[tagStart] == '#' && mixedSpacingUnescaped(line, tagStart) {
		r, _ := utf8.DecodeLastRuneInString(before[:tagStart])
		if r != '#' && !mixedSpacingTagRune(r) {
			return true
		}
	}
	tokenStart := 0
	if space := strings.LastIndexFunc(before, unicode.IsSpace); space >= 0 {
		_, width := utf8.DecodeRuneInString(before[space:])
		tokenStart = space + width
	}
	token := strings.TrimLeft(before[tokenStart:], "<([\"'")
	return strings.Contains(token, "://") || strings.HasPrefix(token, "www.") || strings.HasPrefix(token, "mailto:")
}

func mixedSpacingTagRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '_' || r == '-' || r == '/'
}

func mixedSpacingUnescaped(text string, at int) bool {
	i := at
	for i > 0 && text[i-1] == '\\' {
		i--
	}
	return (at-i)%2 == 0
}
