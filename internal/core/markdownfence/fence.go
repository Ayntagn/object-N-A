// Package markdownfence locates and updates editable code fences in a note.
package markdownfence

import (
	"errors"
	"strings"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const extensions = parser.GFM | parser.ExtAlerts | parser.ExtFootnotes | parser.ExtMath | parser.ExtDollarMath

type Fence struct {
	// Start and End delimit the entire fence, excluding the closer's line break.
	Start, End int
	// CodeStart and CodeEnd delimit the raw body, including Markdown indentation.
	CodeStart, CodeEnd int
	Language           string
	// Code is the parsed body with LF line breaks and its Markdown prefixes removed.
	Code string
	// Source is the original, byte-for-byte contents of the fence.
	Source string

	note                string
	marker              byte
	opener, closer      int // Fence markers relative to Start.
	openSize, closeSize int
	prefix, lineBreak   string
}

// Find accepts a byte offset anywhere in an ordinary code fence, including
// its opening and closing lines.
func Find(source string, caret int) (Fence, bool) {
	preamble, _, body := workspace.SplitFrontmatter(source)
	if caret < len(preamble) || caret > len(source) {
		return Fence{}, false
	}
	document := parser.New(extensions)
	document.Set(body)
	offset := caret - len(preamble)
	for index := 1; index < document.Len(); index++ {
		block := document.Block(index)
		if block.Kind != parser.CodeBlock || !block.Fenced || block.Open {
			continue
		}
		start := lineStart(body, block.Start)
		if start > offset {
			break
		}
		if offset <= block.End {
			return readFence(source, body, len(preamble), document, index, start)
		}
	}
	return Fence{}, false
}

func readFence(source, body string, base int, doc *parser.Document, index, start int) (Fence, bool) {
	block := doc.Block(index)
	language := block.Language()
	switch strings.ToLower(language) {
	case "latex", "mermaid", "vega", "vega-lite", "query", "api", "api-result", "raw", "raw-html":
		return Fence{}, false
	}
	contentStart, eol := nextLine(body, block.Start)
	contentEnd := lineStart(body, block.End)
	if eol == "" || contentEnd < contentStart {
		return Fence{}, false
	}
	// A quote or list can end a fence without an explicit closing marker.
	if count := doc.LineCount(index); count > 0 {
		if _, last := doc.Line(index, count-1); last >= contentEnd {
			return Fence{}, false
		}
	}
	marker := body[block.Start]
	openSize := markerWidth(body[block.Start:contentStart], marker)
	closing := strings.TrimRight(body[contentEnd:block.End], " \t")
	closeSize := len(closing) - len(strings.TrimRight(closing, string(marker)))
	if closeSize < openSize {
		return Fence{}, false
	}
	// Quote markers continue on the body lines. List markers become spaces.
	prefix := strings.Map(func(r rune) rune {
		if r == '>' || r == ' ' || r == '\t' {
			return r
		}
		return ' '
	}, body[start:block.Start])
	if strings.HasSuffix(prefix, ">") {
		// Keep a code line's leading whitespace after the quote separator.
		prefix += " "
	}
	return Fence{
		Start: base + start, End: base + block.End,
		CodeStart: base + contentStart, CodeEnd: base + contentEnd,
		Language: language, Code: doc.Lines(index), Source: body[start:block.End],
		note: source, marker: marker, opener: block.Start - start,
		closer:   contentEnd - start + len(closing) - closeSize,
		openSize: openSize, closeSize: closeSize, prefix: prefix, lineBreak: eol,
	}, true
}

// Replacement returns the new text for the whole fence. The caller can
// replace its original range in one edit after checking the fence still matches.
func (f Fence) Replacement(code string) (string, error) {
	if f.lineBreak == "" || !utf8.ValidString(code) || strings.ContainsRune(code, 0) {
		return "", errors.New("invalid code fence or code text")
	}
	if strings.ContainsRune(code, '\r') {
		code = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(code)
	}
	if code != "" && !strings.HasSuffix(code, "\n") {
		code += "\n"
	}
	if code == f.Code {
		return f.Source, nil
	}
	lines := codeLines(code)
	body := f.updatedBody(lines)
	if replacement, ok := f.rebuild(body, code, f.openSize); ok {
		return replacement, nil
	}
	width := f.openSize
	for _, line := range lines {
		line = strings.Trim(line, " \t")
		if n := markerWidth(line, f.marker); n == len(line) && n >= width {
			width = n + 1
		}
	}
	if width > f.openSize {
		if replacement, ok := f.rebuild(body, code, width); ok {
			return replacement, nil
		}
	}
	return "", errors.New("could not preserve the Markdown code fence")
}

func (f Fence) updatedBody(lines []string) string {
	old := codeLines(f.Code)
	raw := f.Source[f.CodeStart-f.Start : f.CodeEnd-f.Start]
	original := make([]string, len(old))
	for i := range original {
		next, _ := nextLine(raw, 0)
		original[i], raw = raw[:next], raw[next:]
	}
	tail := 0
	for tail < min(len(old), len(lines)) && old[len(old)-tail-1] == lines[len(lines)-tail-1] {
		tail++
	}
	var body strings.Builder
	for i, line := range lines {
		index := i
		if i >= len(lines)-tail {
			index += len(old) - len(lines)
		}
		if index < len(old) && old[index] == line {
			body.WriteString(original[index])
			continue
		}
		body.WriteString(f.prefix)
		body.WriteString(line)
		body.WriteString(f.lineBreak)
	}
	return body.String()
}

func (f Fence) rebuild(body, code string, width int) (string, bool) {
	open := strings.Repeat(string(f.marker), width)
	close := strings.Repeat(string(f.marker), max(f.closeSize, width))
	replacement := f.Source[:f.opener] + open + f.Source[f.opener+f.openSize:f.CodeStart-f.Start] +
		body + f.Source[f.CodeEnd-f.Start:f.closer] + close + f.Source[f.closer+f.closeSize:]
	note := f.note[:f.Start] + replacement + f.note[f.End:]
	updated, ok := Find(note, f.Start)
	return replacement, ok && updated.Start == f.Start && updated.End == f.Start+len(replacement) &&
		updated.Code == code && updated.Language == f.Language
}

func codeLines(code string) []string {
	if code == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(code, "\n"), "\n")
}

func markerWidth(text string, marker byte) int {
	width := 0
	for width < len(text) && text[width] == marker {
		width++
	}
	return width
}

func lineStart(source string, offset int) int {
	return strings.LastIndexAny(source[:offset], "\r\n") + 1
}

func nextLine(source string, offset int) (int, string) {
	at := strings.IndexAny(source[offset:], "\r\n")
	if at < 0 {
		return len(source), ""
	}
	end := offset + at
	next := end + 1
	if source[end] == '\r' && next < len(source) && source[next] == '\n' {
		next++
	}
	return next, source[end:next]
}
