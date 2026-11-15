package workspace

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-text/typesetting/segmenter"
)

// FileInspectionMode selects the text generated from a saved Vault file.
type FileInspectionMode uint8

const (
	InspectPlainText FileInspectionMode = iota
	InspectHexDump
)

// FileInspection is the initial text and title of a new, unsaved document.
type FileInspection struct {
	Text  string
	Title string
}

// InspectFile reads a regular Vault file without editing it or opening a
// revision for saving back to the original path.
func (v *Vault) InspectFile(relative string, mode FileInspectionMode) (FileInspection, error) {
	var label string
	var limit int
	switch mode {
	case InspectPlainText:
		label, limit = "plain text", 16<<20
	case InspectHexDump:
		label, limit = "hex dump", 2<<20
	default:
		return FileInspection{}, fmt.Errorf("unknown file inspection mode %d", mode)
	}
	native, err := lifecyclePath(relative)
	if err != nil {
		return FileInspection{}, err
	}
	dir, err := openVaultDirectory(v.root, filepath.Dir(native))
	if err != nil {
		return FileInspection{}, err
	}
	defer dir.Close()
	name := filepath.Base(native)
	data, opened, err := readRegularFilePrefix(dir, name, int64(limit)+1)
	if err != nil {
		if opened == nil {
			return FileInspection{}, err
		}
		// The native reader treats a read failure after opening as empty data.
		data = nil
	}

	var header string
	if len(data) > limit {
		data = data[:limit]
		header = name + " · " + label + " - first " + inspectionByteCount(limit) + " (truncated)\n\n"
	} else {
		header = name + " · " + label + " - " + inspectionByteCount(len(data)) + "\n\n"
	}
	var text string
	if mode == InspectHexDump {
		text = inspectionHexDump(header, data)
	} else {
		text = header + inspectionPlainText(data)
	}
	return FileInspection{Text: text, Title: FileInspectionTitle(text)}, nil
}

func inspectionPlainText(data []byte) string {
	if utf8.Valid(data) {
		return strings.TrimPrefix(string(data), "\ufeff")
	}
	var text strings.Builder
	text.Grow(len(data))
	for _, b := range data {
		text.WriteRune(rune(b))
	}
	return text.String()
}

func inspectionByteCount(count int) string {
	// Foundation's File style uses decimal units, rounding KB to whole units
	// and MB to at most one fractional digit in the supported size range.
	switch {
	case count == 0:
		return "Zero KB"
	case count == 1:
		return "1 byte"
	case count < 1000:
		return strconv.Itoa(count) + " bytes"
	case count < 999_500:
		return strconv.Itoa((count+500)/1000) + " KB"
	default:
		tenths := (count + 50_000) / 100_000
		whole := strconv.Itoa(tenths / 10)
		if fraction := tenths % 10; fraction != 0 {
			whole += "." + strconv.Itoa(fraction)
		}
		return whole + " MB"
	}
}

func inspectionHexDump(header string, data []byte) string {
	const digits = "0123456789ABCDEF"
	const offsetWidth, byteFieldWidth = 8, 50
	const bytesStart = offsetWidth + 2
	const asciiStart = bytesStart + byteFieldWidth + 1
	const columns = 16
	const rowWidth = asciiStart + columns + 2
	var text strings.Builder
	text.Grow(len(header) + ((len(data)+columns-1)/columns)*rowWidth)
	text.WriteString(header)

	// The byte field is 50 columns wide, including its trailing spaces
	// and the gap between byte groups. The ASCII field is unpadded.
	var row [rowWidth]byte
	for i := range row {
		row[i] = ' '
	}
	row[asciiStart-1] = '|'
	for start := 0; start < len(data); start += columns {
		for i := range offsetWidth {
			row[i] = digits[(start>>(4*(offsetWidth-1-i)))&15]
		}
		chunk := data[start:min(start+columns, len(data))]
		column := bytesStart
		for i, b := range chunk {
			row[column], row[column+1] = digits[b>>4], digits[b&15]
			column += 3
			if i == columns/2-1 {
				column++
			}
			if b < 0x20 || b > 0x7e {
				b = '.'
			}
			row[asciiStart+i] = b
		}
		for i := column; i < asciiStart-1; i++ {
			row[i] = ' '
		}
		end := asciiStart + len(chunk)
		row[end], row[end+1] = '|', '\n'
		text.Write(row[:end+2])
	}
	return text.String()
}

var inspectionHeading = regexp.MustCompile(`^#{1,6}[\t\n\v\f\r\x{85}\p{Z}]+([^\r\n\v\f\x{85}\x{2028}\x{2029}]+)$`)

// FileInspectionTitle derives a document title from an inspection draft.
func FileInspectionTitle(text string) string {
	text = inspectionTitlePrefix(text)
	var fallback string
	for lines, start := 0, 0; lines < 50 && start < len(text); lines++ {
		end := start
		// Swift splits on the LF character. CRLF is a distinct character.
		for end < len(text) && (text[end] != '\n' || (end > start && text[end-1] == '\r')) {
			end++
		}
		line := strings.TrimFunc(text[start:end], inspectionHorizontalSpace)
		start = end + 1
		if line == "" || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "```") {
			continue
		}
		if match := inspectionHeading.FindStringSubmatch(line); match != nil {
			if heading := strings.TrimFunc(match[1], inspectionHorizontalSpace); heading != "" {
				return sanitizeInspectionTitle(heading)
			}
		}
		if fallback == "" && !strings.HasPrefix(line, "#") {
			fallback = line
		}
	}
	return sanitizeInspectionTitle(fallback)
}

func inspectionTitlePrefix(text string) string {
	const limit = 5000
	units := 0
	for offset, r := range text {
		if units == limit {
			return text[:offset]
		}
		if r > 0xffff {
			units++
			if units == limit {
				// NSString's prefix ends on a lone high surrogate here.
				return text[:offset] + string(utf8.RuneError)
			}
		}
		units++
	}
	return text
}

func inspectionHorizontalSpace(r rune) bool {
	return r == '\t' || r == '\u200b' || unicode.Is(unicode.Zs, r)
}

func sanitizeInspectionTitle(title string) string {
	var clean strings.Builder
	space := false
	for _, r := range title {
		if strings.ContainsRune("*_`[]/\\:?\"<>|", r) {
			continue
		}
		if r != ' ' || !space {
			clean.WriteRune(r)
		}
		space = r == ' '
	}
	title = strings.TrimFunc(clean.String(), inspectionHorizontalSpace)
	if len(title) > 50 {
		var segments segmenter.Segmenter
		_ = segments.InitWithString(title)
		graphemes := segments.GraphemeIterator()
		for count := 0; graphemes.Next(); count++ {
			if count == 50 {
				title = title[:graphemes.Grapheme().OffsetInBytes]
				if lastSpace := strings.LastIndexByte(title, ' '); lastSpace >= 0 {
					title = title[:lastSpace]
				}
				break
			}
		}
	}
	title = strings.Trim(title, ".")
	if title == "" {
		return "Untitled"
	}
	return title
}
