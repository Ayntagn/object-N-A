// Package callout defines Markdown callout headers and document styles.
package callout

import (
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Header struct {
	Kind, Title string
	Fold        byte
	TitleOffset int
}

var headerPattern = regexp.MustCompile(`^\\?\[!([A-Za-z](?:[A-Za-z0-9_-]|\\_)*)\\?\]([+-]?)[\t ]*`)

// ParseHeader reads the first line of a quote after its > prefix.
func ParseHeader(line string) (Header, bool) {
	match := headerPattern.FindStringSubmatchIndex(line)
	if match == nil {
		return Header{}, false
	}
	header := Header{Kind: strings.ReplaceAll(line[match[2]:match[3]], `\_`, "_"), Title: line[match[1]:], TitleOffset: match[1]}
	if match[4] < match[5] {
		header.Fold = line[match[4]]
	}
	return header, true
}

type Style struct {
	Kind  string `json:"kind"`
	Name  string `json:"name,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"`
}

// The first eleven kinds follow the original Alert menu (EV-UI-155).
var presets = [...]Style{
	{"note", "Note", "📝", "#a9c7ff"},
	{"tip", "Tip", "💡", "#10b981"},
	{"info", "Info", "ℹ️", "#3b82f6"},
	{"warning", "Warning", "⚠️", "#f59e0b"},
	{"danger", "Danger", "🔴", "#ef4444"},
	{"success", "Success", "✅", "#10b981"},
	{"question", "Question", "❓", "#f59e0b"},
	{"example", "Example", "📋", "#8b5cf6"},
	{"quote", "Quote", "💬", "#7f7f7f"},
	{"bug", "Bug", "🐛", "#ef4444"},
	{"abstract", "Abstract", "📄", "#06b6d4"},
	{"important", "Important", "❗", "#8b5cf6"},
	{"todo", "Todo", "☑️", "#3b82f6"},
	{"failure", "Failure", "❌", "#ef4444"},
}

const menuPresets = 11

var aliases = map[string]string{
	"hint": "tip", "caution": "warning", "attention": "warning", "error": "danger",
	"check": "success", "done": "success", "help": "question", "faq": "question",
	"cite": "quote", "summary": "abstract", "tldr": "abstract", "fail": "failure", "missing": "failure",
}

// Resolve applies the user's style for kind, falling back to the palette of
// its alias or to Note. A custom style can override any of the three fields.
func Resolve(kind string, overrides []Style) Style {
	kind = strings.ToLower(kind)
	if kind == "" {
		kind = "note"
	}
	base := kind
	if alias, ok := aliases[kind]; ok {
		base = alias
	}
	style := presets[0]
	for _, preset := range presets {
		if preset.Kind == base {
			style = preset
			break
		}
	}
	if style.Kind != kind {
		style.Kind = kind
		style.Name = displayName(kind)
	}
	if base != kind {
		style = applyStyle(style, base, overrides)
	}
	return applyStyle(style, kind, overrides)
}

func applyStyle(style Style, kind string, overrides []Style) Style {
	for _, override := range overrides {
		if override.Kind != kind {
			continue
		}
		if override.Name != "" {
			style.Name = override.Name
		}
		if override.Icon != "" {
			style.Icon = override.Icon
		}
		if override.Color != "" {
			style.Color = override.Color
		}
		break
	}
	return style
}

func displayName(kind string) string {
	words := strings.FieldsFunc(kind, func(r rune) bool { return r == '-' || r == '_' })
	for index, word := range words {
		first, length := utf8.DecodeRuneInString(word)
		words[index] = string(unicode.ToUpper(first)) + word[length:]
	}
	return strings.Join(words, " ")
}

// Menu keeps preset positions and adds any custom types in saved order.
func Menu(overrides []Style) []Style {
	styles := make([]Style, 0, menuPresets+len(overrides))
	shown := make(map[string]bool, cap(styles))
	for _, preset := range presets[:menuPresets] {
		styles = append(styles, Resolve(preset.Kind, overrides))
		shown[preset.Kind] = true
	}
	for _, override := range overrides {
		if !shown[override.Kind] {
			styles = append(styles, Resolve(override.Kind, overrides))
			shown[override.Kind] = true
		}
	}
	return styles
}

var (
	ErrKind   = errors.New("type must start with a letter and contain at most 32 letters, digits, hyphens or underscores")
	ErrName   = errors.New("name must be a single line of up to 64 characters")
	ErrIcon   = errors.New("icon must be a single line of up to 12 characters")
	ErrColor  = errors.New("enter a hex color such as #A9C7FF")
	styleKind = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
)

func Normalize(style Style) (Style, error) {
	style.Kind = strings.ToLower(strings.TrimSpace(style.Kind))
	style.Name = strings.TrimSpace(style.Name)
	style.Icon = strings.TrimSpace(style.Icon)
	style.Color = strings.ToLower(strings.TrimSpace(style.Color))
	if !styleKind.MatchString(style.Kind) {
		return Style{}, ErrKind
	}
	if !plainLabel(style.Name, 64) {
		return Style{}, ErrName
	}
	if !plainLabel(style.Icon, 12) {
		return Style{}, ErrIcon
	}
	if style.Color != "" {
		digits := strings.TrimPrefix(style.Color, "#")
		if _, err := hex.DecodeString(digits); len(digits) != 6 || err != nil {
			return Style{}, ErrColor
		}
		style.Color = "#" + digits
	}
	return style, nil
}

func plainLabel(text string, limit int) bool {
	return utf8.ValidString(text) && utf8.RuneCountInString(text) <= limit && strings.IndexFunc(text, func(r rune) bool {
		return unicode.IsControl(r) || r == '\u2028' || r == '\u2029'
	}) < 0
}
