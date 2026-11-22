package notehtml

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const cssSpace = " \t\n\r"

// cleanStyle keeps passive declarations from a note's style attribute.
func cleanStyle(raw string) string {
	var clean []string
	add := func(declaration string) {
		property, value, ok := strings.Cut(declaration, ":")
		if !ok {
			return
		}
		property = strings.ToLower(strings.Trim(property, cssSpace))
		value = strings.Trim(value, cssSpace)
		// The shorthand can include an image, including one in a theme var.
		if property == "background" {
			property = "background-color"
		}
		if passiveStyleProperty(property) && passiveStyleValue(value) {
			clean = append(clean, property+": "+value)
		}
	}

	// A semicolon inside a string, function or comment is not a separator.
	start, depth := 0, 0
	var quote byte
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == '\\' && i+1 < len(raw) {
			i++
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '/':
			if strings.HasPrefix(raw[i:], "/*") {
				end := strings.Index(raw[i+2:], "*/")
				if end < 0 {
					return strings.Join(clean, "; ")
				}
				i += end + 3
			}
		case '(':
			depth++
		case ')':
			depth = max(0, depth-1)
		case ';':
			if depth == 0 {
				add(raw[start:i])
				start = i + 1
			}
		}
	}
	add(raw[start:])
	return strings.Join(clean, "; ")
}

func passiveStyleProperty(name string) bool {
	switch name {
	case "color", "background-color", "accent-color", "opacity",
		"text-align", "text-align-last", "text-indent", "text-transform", "text-decoration",
		"text-decoration-color", "text-decoration-line", "text-decoration-style", "text-decoration-thickness",
		"text-underline-offset", "vertical-align", "direction",
		"font", "font-family", "font-size", "font-style", "font-weight", "font-variant", "font-stretch",
		"font-feature-settings", "letter-spacing", "word-spacing", "line-height",
		"white-space", "overflow-wrap", "word-break", "hyphens",
		"width", "max-width", "min-width", "height", "max-height", "min-height", "box-sizing",
		"margin", "margin-top", "margin-bottom", "margin-left", "margin-right",
		"margin-inline", "margin-inline-start", "margin-inline-end", "margin-block", "margin-block-start", "margin-block-end",
		"padding", "padding-top", "padding-bottom", "padding-left", "padding-right",
		"padding-inline", "padding-inline-start", "padding-inline-end", "padding-block", "padding-block-start", "padding-block-end",
		"border", "border-width", "border-style", "border-color", "border-radius",
		"border-top", "border-bottom", "border-left", "border-right",
		"border-top-color", "border-bottom-color", "border-left-color", "border-right-color",
		"border-top-width", "border-bottom-width", "border-left-width", "border-right-width",
		"border-top-style", "border-bottom-style", "border-left-style", "border-right-style",
		"border-inline", "border-inline-start", "border-inline-end", "border-block", "border-block-start", "border-block-end",
		"border-collapse", "border-spacing", "table-layout", "caption-side", "empty-cells",
		"list-style-position", "list-style-type", "object-fit", "object-position",
		"break-before", "break-after", "break-inside", "page-break-before", "page-break-after", "page-break-inside":
		return true
	}
	return false
}

func passiveStyleValue(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}
	wordStart, depth := 0, 0
	var quote rune
	for i, r := range value {
		if r == '\\' || unicode.Is(unicode.Cf, r) || unicode.IsControl(r) && !strings.ContainsRune(cssSpace, r) {
			return false
		}
		if quote != 0 {
			if r == '\n' || r == '\r' {
				return false
			}
			if r == quote {
				quote = 0
				wordStart = i + 1
			}
			continue
		}
		if r >= utf8.RuneSelf || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case '(':
			if !passiveStyleFunction(value[wordStart:i]) {
				return false
			}
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		case '!':
			return i > 0 && depth == 0 && strings.EqualFold(strings.Trim(value[i+1:], cssSpace), "important")
		case '/':
			if strings.HasPrefix(value[i:], "/*") {
				return false
			}
		case '#', '.', ',', '%', '+', '*', ' ', '\t', '\n', '\r':
		default:
			return false
		}
		wordStart = i + 1
	}
	return quote == 0 && depth == 0
}

func passiveStyleFunction(name string) bool {
	switch strings.ToLower(name) {
	case "rgb", "rgba", "hsl", "hsla", "hwb", "lab", "lch", "oklab", "oklch", "color", "color-mix", "light-dark",
		"var", "calc", "min", "max", "clamp":
		return true
	}
	return false
}
