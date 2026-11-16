package ui

import (
	"regexp"
	"strings"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// The pinned Markdown and fenced-language lexers leave this kind unused.
const sourceHexToken = editor.TokenInvalid

var (
	sourceHexPattern           = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`)
	sourceMarkdownExpandedHex  = sourceMarkdownHexLanguage(false)
	sourceMarkdownCollapsedHex = sourceMarkdownHexLanguage(true)
)

type sourceHexColors struct {
	draft     *editor.Buffer
	version   uint64
	collapsed bool
}

func (tab *workspaceTab) applySourceHexColors(language *editor.Language) {
	if language != sourceMarkdownExpandedHex && language != sourceMarkdownCollapsedHex {
		tab.sourceHex = sourceHexColors{}
		tab.code.TextBackgrounds = nil
		return
	}
	snapshot := tab.draft.Snapshot()
	current := sourceHexColors{
		draft: tab.draft, version: snapshot.Version(), collapsed: language == sourceMarkdownCollapsedHex,
	}
	if tab.sourceHex != current {
		tab.sourceHex = current
		tab.code.TextBackgrounds = sourceHexRanges(snapshot.String(), current.collapsed)
	}
	if len(tab.code.TextBackgrounds) > 0 {
		theme := *tab.code.Theme
		// MyGo treats zero-alpha token styles as the theme foreground.
		// At minimum alpha, use the canvas ink so a transparent swatch
		// does not leave a contrasting glyph.
		mask := theme.Background
		mask.A = 1
		theme.Tokens[sourceHexToken] = editor.TokenStyle{Color: mask}
		tab.code.Theme = &theme
	}
}

// EV-UI-260 / EV-UI-262: a swatch covers the hash, or the complete literal
// when collapsed. Both use the original UTF-8 byte positions.
func sourceHexRanges(text string, collapsed bool) []editor.TextBackground {
	if !strings.Contains(text, "#") {
		return nil
	}
	var ranges []editor.TextBackground
	for _, match := range sourceHexPattern.FindAllStringIndex(text, -1) {
		start, end := match[0], match[1]
		switch end - start {
		case 4, 5, 7, 9:
		default:
			continue
		}
		color := mygoui.Hex(text[start:end])
		if !collapsed {
			end = start + 1
		}
		ranges = append(ranges, editor.TextBackground{Start: start, End: end, Color: color})
	}
	return ranges
}

func sourceMarkdownHexLanguage(collapsed bool) *editor.Language {
	language := *sourceMarkdownLanguage
	base := language.Tokenizer
	language.Tokenizer = editor.TokenizerFunc(func(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
		ranges := sourceHexRanges(line, collapsed)
		if len(ranges) == 0 {
			return base.Tokenize(line, state, tokens)
		}
		original, next := base.Tokenize(line, state, nil)
		// Keep the original token styles and fence metadata around each swatch.
		position, index := 0, 0
		for _, swatch := range ranges {
			for index < len(original) && original[index].End <= swatch.Start {
				token := original[index]
				if token.End > position {
					tokens = append(tokens, token)
					position = token.End
				}
				index++
			}
			if position < swatch.Start {
				token := editor.Token{End: swatch.Start, Kind: editor.TokenText}
				if index < len(original) {
					token = original[index]
					token.End = swatch.Start
				}
				tokens = append(tokens, token)
			}
			tokens = append(tokens, editor.Token{End: swatch.End, Kind: sourceHexToken})
			position = swatch.End
		}
		for _, token := range original[index:] {
			if token.End > position {
				tokens = append(tokens, token)
			}
		}
		return tokens, next
	})
	return &language
}
