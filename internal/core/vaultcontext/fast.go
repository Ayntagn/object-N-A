package vaultcontext

import (
	"context"
	"maps"
	"math"
	"slices"
	"unicode"

	"github.com/go-text/typesetting/segmenter"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
	"object-na.local/vectorsearch"
)

const (
	fastDimensions = 256
	fastModelID    = "TFIDFEmbedder.v1.256d"
	maxTokens      = 4096
)

// Build creates a Fast TF-IDF index with the selected chunk boundaries. Choose
// this engine for Quick or when explicitly building the native fast fallback.
func Build(ctx context.Context, docs []Document, depth Depth) (*Index, error) {
	index, err := prepareIndex(ctx, docs, depth)
	if err != nil {
		return nil, err
	}
	d := &index.data
	d.Engine, d.ModelID, d.Dimension = Fast, fastModelID, fastDimensions
	corpus := len(d.Chunks)
	d.CorpusSize = &corpus
	d.Frequencies = make(map[string]int)
	// Quick prepares one chunk for each note with embeddable text.
	if report := buildProgressObserver(ctx); report != nil && depth == Quick && corpus > 0 && ctx.Err() == nil {
		report(BuildProgress{Phase: BuildWordStatistics, Total: corpus})
	}
	counts := make([]map[string]int, len(d.Chunks))
	var segments segmenter.Segmenter
	for at, chunk := range d.Chunks {
		counts[at], err = tokenCounts(ctx, &segments, chunk.Text)
		if err != nil {
			return nil, err
		}
		for token := range counts[at] {
			d.Frequencies[token]++
		}
	}
	progress := startChunkProgress(ctx, len(d.Chunks))
	for at, terms := range counts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		d.Chunks[at].setVector(tfidf(terms, d.Frequencies, corpus))
		progress(at + 1)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return index, nil
}

// Search scores a query against a Fast index and returns at most five matches.
// A query with no matches returns nil, nil. Cancellation returns the context
// error and discards any partial ranking.
func (i *Index) Search(ctx context.Context, query string) ([]Match, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if i.data.Engine != Fast {
		return nil, errSearchEngine
	}
	var segments segmenter.Segmenter
	counts, err := tokenCounts(ctx, &segments, query)
	if err != nil {
		return nil, err
	}
	return i.search(ctx, tfidf(counts, i.data.Frequencies, i.data.fastCorpusSize()))
}

func (d *storedIndex) fastCorpusSize() int {
	if d.CorpusSize != nil {
		return *d.CorpusSize
	}
	return len(d.Chunks)
}

// EV-UI-290. The native Fast tokenizer lowercases, splits on non-alphanumeric
// characters and retains up to 4096 tokens of at least two grapheme clusters.
func tokenCounts(ctx context.Context, segments *segmenter.Segmenter, text string) (map[string]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	text = norm.NFC.String(cases.Lower(language.Und).String(text))
	segments.InitWithString(text)
	counts := make(map[string]int)
	seen, start, letters := 0, 0, 0
	graphemes := segments.GraphemeIterator()
	for at := 0; graphemes.Next(); at++ {
		if at%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		span := graphemes.Grapheme()
		if first := span.Text[0]; unicode.IsLetter(first) || unicode.IsNumber(first) {
			if letters == 0 {
				start = span.OffsetInBytes
			}
			letters++
			continue
		}
		if letters >= 2 {
			counts[text[start:span.OffsetInBytes]]++
			seen++
			if seen == maxTokens {
				return counts, ctx.Err()
			}
		}
		letters = 0
	}
	if letters >= 2 {
		counts[text[start:]]++
	}
	return counts, ctx.Err()
}

func tfidf(counts, frequencies map[string]int, corpusSize int) []float32 {
	weights := make([]float32, fastDimensions)
	// Sorting by token gives deterministic float sums when terms share a bucket.
	for _, token := range slices.Sorted(maps.Keys(counts)) {
		idf := math.Log(float64(max(1, corpusSize)) / float64(max(1, frequencies[token])))
		weight := math.Log(1+float64(counts[token])) * max(0, idf)
		var bucket uint8 = 5381 & 0xff
		for byteIndex := range len(token) {
			bucket = bucket*33 + token[byteIndex]
		}
		weights[bucket] += float32(weight)
	}
	if norm := vectorsearch.Norm(weights); norm > 0 {
		for at, weight := range weights {
			weights[at] = float32(float64(weight) / norm)
		}
	}
	return weights
}
