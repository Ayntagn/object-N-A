package exportdoc

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ICU's \s follows Unicode White_Space. Go's \s matches only a subset.
const figureSpace = `[\t-\r\x{0085}\p{Z}]`

var figurePrefixes = [...]*regexp.Regexp{
	// ICU's case-insensitive matching also recognizes the "fi" ligature.
	regexp.MustCompile(`(?i)^(?:fi|ﬁ)g(?:ure|\.?)` + figureSpace + `*([0-9]{1,4})` + figureSpace + `*[.:)]?` + figureSpace + `+`),
	regexp.MustCompile(`^([0-9]{1,4})` + figureSpace + `*[.)]` + figureSpace + `+`),
}

// FigureCaption separates a caption from its label and updates the document's
// figure counter. An explicit number resets the counter even when automatic
// numbering is disabled.
func FigureCaption(caption string, automatic bool, counter *int) (label, text string) {
	text = strings.TrimFunc(caption, figureCaptionSpace)
	if strings.TrimSpace(text) == "" {
		return "", ""
	}
	for _, prefix := range figurePrefixes {
		if match := prefix.FindStringSubmatchIndex(text); match != nil {
			// The pattern accepts at most four ASCII digits, so Atoi succeeds.
			*counter, _ = strconv.Atoi(text[match[2]:match[3]])
			return "Figure " + strconv.Itoa(*counter) + ".", strings.TrimFunc(text[match[1]:], figureCaptionSpace)
		}
	}
	if automatic {
		*counter += 1
		label = "Figure " + strconv.Itoa(*counter) + "."
	}
	return label, text
}

// The native caption parser trims Foundation's CharacterSet.whitespaces.
func figureCaptionSpace(char rune) bool {
	return char == '\t' || char == '\u200b' || unicode.Is(unicode.Zs, char)
}
