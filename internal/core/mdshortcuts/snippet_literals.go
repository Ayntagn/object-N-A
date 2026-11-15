package mdshortcuts

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

type snippetParser struct {
	source string
	pos    int
	err    error
}

func (p *snippetParser) fail(at int, format string, args ...any) {
	if p.err != nil {
		return
	}
	prefix := p.source[:at]
	line := 1 + strings.Count(prefix, "\n") + strings.Count(prefix, "\r") - strings.Count(prefix, "\r\n")
	column := 1 + utf8.RuneCountInString(prefix[strings.LastIndexAny(prefix, "\r\n")+1:])
	p.err = fmt.Errorf("line %d, column %d: %s", line, column, fmt.Sprintf(format, args...))
}

func (p *snippetParser) peek() byte {
	for p.err == nil && p.pos < len(p.source) {
		text := p.source[p.pos:]
		r, size := utf8.DecodeRuneInString(text)
		switch {
		case unicode.IsSpace(r) || r == '\ufeff':
			p.pos += size
		case strings.HasPrefix(text, "//"):
			end := strings.IndexAny(text, "\r\n")
			if end < 0 {
				end = len(text)
			}
			p.pos += end
		case strings.HasPrefix(text, "/*"):
			end := strings.Index(text[2:], "*/")
			if end < 0 {
				p.fail(p.pos, "unterminated block comment")
				return 0
			}
			p.pos += end + 4
		default:
			return text[0]
		}
	}
	return 0
}

func (p *snippetParser) position() int {
	p.peek()
	return p.pos
}

func (p *snippetParser) atEnd() bool {
	at := p.position()
	return p.err == nil && at == len(p.source)
}

func (p *snippetParser) take(mark byte) bool {
	if p.peek() != mark {
		return false
	}
	p.pos++
	return true
}

func (p *snippetParser) expect(mark byte, message string) {
	if !p.take(mark) {
		p.fail(p.pos, "%s", message)
	}
}

func isSnippetLetter(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_' || b == '$'
}

func (p *snippetParser) identifier() string {
	if !isSnippetLetter(p.peek()) {
		return ""
	}
	start := p.pos
	for p.pos < len(p.source) {
		b := p.source[p.pos]
		if !isSnippetLetter(b) && (b < '0' || b > '9') {
			break
		}
		p.pos++
	}
	return p.source[start:p.pos]
}

func (p *snippetParser) field() string {
	if mark := p.peek(); mark == '"' || mark == '\'' {
		return p.quoted("field name")
	}
	value := p.identifier()
	if value == "" {
		p.fail(p.pos, "expected a field name")
	}
	return value
}

func (p *snippetParser) boolean() bool {
	start := p.position()
	switch p.identifier() {
	case "true":
		return true
	case "false":
		return false
	default:
		p.fail(start, "enabled must be true or false")
		return false
	}
}

func (p *snippetParser) number() float64 {
	start := p.position()
	for p.pos < len(p.source) && strings.ContainsRune("0123456789.+-eE", rune(p.source[p.pos])) {
		p.pos++
	}
	value, err := strconv.ParseFloat(p.source[start:p.pos], 64)
	if err != nil {
		p.fail(start, "priority must be a finite number")
	}
	return value
}

func (p *snippetParser) quoted(field string) string {
	quote := p.peek()
	start := p.pos
	if quote != '"' && quote != '\'' {
		p.fail(start, "%s must be a quoted string", field)
		return ""
	}
	p.pos++
	var out strings.Builder
	for p.err == nil && p.pos < len(p.source) {
		b := p.source[p.pos]
		switch b {
		case quote:
			p.pos++
			if !utf8.ValidString(out.String()) {
				p.fail(start, "invalid UTF-8 in %s", field)
			}
			return out.String()
		case '\r', '\n':
			p.fail(p.pos, "use an escape for a newline in a quoted string")
		case '\\':
			if r, emit := p.stringEscape(); emit {
				out.WriteRune(r)
			}
		default:
			out.WriteByte(b)
			p.pos++
		}
	}
	p.fail(start, "unterminated quoted string")
	return ""
}

func (p *snippetParser) stringEscape() (rune, bool) {
	start := p.pos
	p.pos++
	if p.pos < len(p.source) {
		mark := p.source[p.pos]
		p.pos++
		switch mark {
		case '"', '\'', '/', '\\':
			return rune(mark), true
		case '\n':
			return 0, false
		case '\r':
			if p.pos < len(p.source) && p.source[p.pos] == '\n' {
				p.pos++
			}
			return 0, false
		case '0':
			if p.pos == len(p.source) || p.source[p.pos] < '0' || p.source[p.pos] > '9' {
				return 0, true
			}
		case 'u':
			return p.unicodeEscape(start)
		case 'b', 'f', 'n', 'r', 't', 'v', 'x':
			r, _, rest, err := strconv.UnquoteChar(p.source[start:], 0)
			if err == nil {
				p.pos = len(p.source) - len(rest)
				return r, true
			}
		}
	}
	p.fail(start, "invalid string escape")
	return 0, false
}

func (p *snippetParser) unicodeEscape(start int) (rune, bool) {
	if p.pos < len(p.source) && p.source[p.pos] == '{' {
		p.pos++
		length := strings.IndexByte(p.source[p.pos:], '}')
		if length >= 1 && length <= 6 {
			if r, ok := p.hexRune(length); ok && utf8.ValidRune(r) {
				p.pos++
				return r, true
			}
		}
	} else if r, ok := p.hexRune(4); ok {
		if utf8.ValidRune(r) {
			return r, true
		}
		if utf16.IsSurrogate(r) && strings.HasPrefix(p.source[p.pos:], "\\u") {
			p.pos += 2
			if next, ok := p.hexRune(4); ok {
				if pair := utf16.DecodeRune(r, next); pair != utf8.RuneError {
					return pair, true
				}
			}
		}
	}
	p.fail(start, "invalid Unicode escape")
	return 0, false
}

func (p *snippetParser) hexRune(length int) (rune, bool) {
	if p.pos+length > len(p.source) {
		return 0, false
	}
	value, err := strconv.ParseUint(p.source[p.pos:p.pos+length], 16, 32)
	p.pos += length
	return rune(value), err == nil
}

func (p *snippetParser) regularExpression() (pattern, flags string) {
	start := p.pos
	p.pos++
	inClass := false
	for p.pos < len(p.source) {
		b := p.source[p.pos]
		switch b {
		case '\r', '\n':
			p.fail(p.pos, "a regular expression literal must stay on one line")
			return "", ""
		case '\\':
			if p.pos+1 < len(p.source) && p.source[p.pos+1] != '\r' && p.source[p.pos+1] != '\n' {
				p.pos++
			}
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '/':
			if !inClass {
				pattern = p.source[start+1 : p.pos]
				p.pos++
				flagsAt := p.pos
				for p.pos < len(p.source) && isSnippetLetter(p.source[p.pos]) {
					p.pos++
				}
				return pattern, p.source[flagsAt:p.pos]
			}
		}
		p.pos++
	}
	p.fail(start, "unterminated regular expression literal")
	return "", ""
}
