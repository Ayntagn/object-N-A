package shortsearch

import (
	"context"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// CacheVersion distinguishes paths across schema and Unicode table versions.
const CacheVersion = schemaVersion + "-" + cases.UnicodeVersion + "-" + norm.Version

// Keep normalization in step with the application's exact matcher.
func fold(text string) string { return norm.NFC.String(cases.Fold().String(text)) }

func documentTerms(ctx context.Context, text string) (string, error) {
	const checkEvery = 16 << 10
	if err := ctx.Err(); err != nil {
		return "", err
	}
	text = fold(strings.TrimPrefix(text, "\ufeff"))
	var terms strings.Builder
	seen := make(map[uint64]struct{})
	var previous rune
	nextCancellationCheck := checkEvery
	for offset, current := range text {
		if offset >= nextCancellationCheck {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			nextCancellationCheck = offset + checkEvery
		}
		if offset > 0 {
			addTerm(&terms, seen, gram(previous, current), " ")
		}
		previous = current
	}
	return terms.String(), ctx.Err()
}

func queryTerms(query string) string {
	// A subset of the necessary bigrams is sufficient for a safe filter.
	const maxTerms = 32
	var terms strings.Builder
	seen := make(map[uint64]struct{}, maxTerms)
	var previous rune
	for offset, current := range fold(query) {
		if offset > 0 {
			addTerm(&terms, seen, gram(previous, current), " AND ")
			if len(seen) == maxTerms {
				break
			}
		}
		previous = current
	}
	return terms.String()
}

func gram(first, second rune) uint64 { return uint64(first)<<21 | uint64(second) }

func addTerm(terms *strings.Builder, seen map[uint64]struct{}, key uint64, separator string) {
	if _, exists := seen[key]; exists {
		return
	}
	seen[key] = struct{}{}
	if terms.Len() != 0 {
		terms.WriteString(separator)
	}
	// An ASCII letter followed by hex digits is always one bare FTS5 term.
	var token [12]byte
	token[0] = 'g'
	terms.Write(strconv.AppendUint(token[:1], key, 16))
}
