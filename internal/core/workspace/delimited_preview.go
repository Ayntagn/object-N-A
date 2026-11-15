package workspace

import (
	"path"
	"strings"
	"unicode/utf8"

	"github.com/go-text/typesetting/segmenter"
)

// ParseDelimitedPreview returns the physical rows used in CSV and TSV previews.
// The first row is the header. Quotes are literal, and each row retains its own
// cell count and empty cells. Unsupported extensions and empty tables return nil.
func ParseDelimitedPreview(source, filename string) [][]string {
	separator := delimitedSeparator(filename)
	if separator == "" {
		return nil
	}
	lines := strings.FieldsFunc(source, func(r rune) bool {
		switch r {
		case '\n', '\v', '\f', '\r', '\u0085', '\u2028', '\u2029':
			return true
		}
		return false
	})
	if len(lines) == 0 {
		return nil
	}
	rows := make([][]string, len(lines))
	var segments segmenter.Segmenter
	for i, line := range lines {
		rows[i] = splitDelimitedPreviewRow(line, separator, &segments)
	}
	return rows
}

// IsDelimitedPreviewPath reports whether the source preview uses a table.
func IsDelimitedPreviewPath(filename string) bool {
	return delimitedSeparator(filename) != ""
}

func delimitedSeparator(filename string) string {
	filename = strings.TrimLeft(path.Base(filename), ".")
	switch strings.ToLower(path.Ext(filename)) {
	case ".csv":
		return ","
	case ".tsv":
		return "\t"
	default:
		return ""
	}
}

func splitDelimitedPreviewRow(line, separator string, segments *segmenter.Segmenter) []string {
	// Swift splits CSV on standalone comma graphemes. A byte split is equivalent
	// for TSV and for rows composed entirely of ASCII.
	if separator == "\t" || strings.IndexFunc(line, func(r rune) bool { return r >= utf8.RuneSelf }) < 0 {
		return strings.Split(line, separator)
	}
	if err := segments.InitWithString(line); err != nil {
		return strings.Split(line, separator)
	}
	var cells []string
	start := 0
	graphemes := segments.GraphemeIterator()
	for graphemes.Next() {
		part := graphemes.Grapheme()
		if part.LengthInBytes == 1 && line[part.OffsetInBytes] == ',' {
			cells = append(cells, line[start:part.OffsetInBytes])
			start = part.OffsetInBytes + 1
		}
	}
	return append(cells, line[start:])
}
