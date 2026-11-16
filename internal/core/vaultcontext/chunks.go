package vaultcontext

import (
	"context"
	"iter"
	"strings"
	"unicode"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
	"github.com/go-text/typesetting/segmenter"
)

type Depth string

const (
	Quick    Depth = "quick"
	Balanced Depth = "balanced"
	Deep     Depth = "deep"
)

func (d Depth) valid() bool { return d == Quick || d == Balanced || d == Deep }

const (
	minimumCharacters = 50
	maximumWords      = 184
	maximumPieces     = 25
)

type piece struct {
	text   string
	offset int
}

// EV-UI-290. Depth selects the chunk boundaries independently of the embedder.
func splitDocument(ctx context.Context, source string, depth Depth) ([]piece, error) {
	prefix, _, body := workspace.SplitFrontmatter(source)
	body, err := prepareBody(ctx, body)
	if err != nil {
		return nil, err
	}
	var segments segmenter.Segmenter
	if depth == Quick {
		text := firstWords(body)
		if enoughText(&segments, text) {
			return []piece{{text: text, offset: len(prefix)}}, ctx.Err()
		}
		return nil, ctx.Err()
	}

	wordLimit := 192 // floor(words × 1.25) ≤ 240
	if depth == Deep {
		wordLimit = 160 // floor(words × 1.25) ≤ 200
	}
	var pieces []piece
	add := func(next piece) {
		if len(pieces) < maximumPieces && enoughText(&segments, next.text) {
			pieces = append(pieces, next)
		}
	}
	for section := range sections(piece{text: body, offset: len(prefix)}) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(pieces) == maximumPieces {
			break
		}
		if !moreWords(section.text, wordLimit) {
			add(section)
			continue
		}
		at := section.offset
		for text := range strings.SplitSeq(section.text, "\n\n") {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			paragraph := piece{text: text, offset: at}
			at += len(text) + 2
			switch {
			case !moreWords(text, wordLimit):
				add(paragraph)
			case depth == Balanced:
				paragraph.text = firstWords(text)
				add(paragraph)
			default:
				for sentence := range sentences(paragraph) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					add(sentence)
					if len(pieces) == maximumPieces {
						break
					}
				}
			}
			if len(pieces) == maximumPieces {
				break
			}
		}
	}
	return pieces, ctx.Err()
}

func prepareBody(ctx context.Context, source string) (string, error) {
	var body strings.Builder
	fence := ""
	source = strings.ReplaceAll(source, "\r\n", "\n")
	for line := range strings.SplitSeq(source, "\n") {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case fence != "":
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
		case strings.HasPrefix(trimmed, "````"):
			fence = "````"
		case strings.HasPrefix(trimmed, "```"):
			fence = "```"
		default:
			body.WriteString(line)
		}
		body.WriteByte('\n')
	}
	return body.String(), nil
}

func sections(body piece) iter.Seq[piece] {
	return func(yield func(piece) bool) {
		start, at := 0, 0
		for line := range strings.SplitSeq(body.text, "\n") {
			trimmed := strings.TrimSpace(line)
			if at > start && (strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ")) {
				if !yield(piece{text: body.text[start : at-1], offset: body.offset + start}) {
					return
				}
				start = at
			}
			at += len(line) + 1
		}
		if start < len(body.text) {
			yield(piece{text: body.text[start:], offset: body.offset + start})
		}
	}
}

// The native Deep backend uses NLTokenizer sentence boundaries. Use Unicode
// sentence punctuation for the cross-platform split of very long paragraphs.
func sentences(paragraph piece) iter.Seq[piece] {
	return func(yield func(piece) bool) {
		start := 0
		for at, char := range paragraph.text {
			end := at + utf8.RuneLen(char)
			switch char {
			case '.', '!', '?':
				next, _ := utf8.DecodeRuneInString(paragraph.text[end:])
				if end < len(paragraph.text) && !unicode.IsSpace(next) {
					continue
				}
			case '。', '！', '？':
			default:
				continue
			}
			if !yield(piece{text: paragraph.text[start:end], offset: paragraph.offset + start}) {
				return
			}
			start = end
		}
		if start < len(paragraph.text) {
			yield(piece{text: paragraph.text[start:], offset: paragraph.offset + start})
		}
	}
}

func enoughText(segments *segmenter.Segmenter, text string) bool {
	text = strings.TrimSpace(text)
	if len(text) < minimumCharacters {
		return false
	}
	segments.InitWithString(text)
	graphemes := segments.GraphemeIterator()
	for range minimumCharacters {
		if !graphemes.Next() {
			return false
		}
	}
	return true
}

func firstWords(text string) string {
	words := make([]string, 0, maximumWords)
	for word := range strings.FieldsSeq(text) {
		words = append(words, word)
		if len(words) == maximumWords {
			break
		}
	}
	return strings.Join(words, " ")
}

func moreWords(text string, limit int) bool {
	for range strings.FieldsSeq(text) {
		if limit == 0 {
			return true
		}
		limit--
	}
	return false
}
