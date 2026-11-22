package vaultcontext

import (
	"context"
	"errors"
	"io/fs"
	"slices"

	"github.com/go-text/typesetting/segmenter"
)

var (
	errUpdateModel      = errors.New("Vault Context update requires the same embedding engine, model, and depth")
	errUpdateStatistics = errors.New("Vault Context update requires saved TF-IDF statistics")
)

// UpdateContextual publishes a new index from a previous contextual index of
// the same depth and model. Paths is the complete readable document list in
// caller order. Changed supplies snapshots of every added or revised document
// in paths. Leave out removed or unreadable paths. Retained chunks share
// immutable vectors with the previous index, and rebuilt chunks follow the
// retained chunks in input order.
func UpdateContextual(ctx context.Context, previous *Index, paths []string, changed []Document, depth Depth, modelID string, embed func(context.Context, string) ([]float32, error)) (*Index, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if previous == nil || previous.Engine() != Contextual || previous.Depth() != depth || previous.ModelID() != modelID {
		return nil, errUpdateModel
	}
	return previous.mergeUpdate(ctx, paths, changed, func() (*Index, error) {
		return BuildContextual(ctx, changed, depth, modelID, embed)
	})
}

// HasFastStatistics reports whether a prior Fast corpus can supply the basis
// for an incremental update. Pruning can leave those statistics with no chunks.
func (i *Index) HasFastStatistics() bool {
	return i != nil && i.Engine() == Fast && i.data.Frequencies != nil && i.data.fastCorpusSize() > 0
}

// UpdateFast updates the readable paths as in UpdateContextual. New chunks
// use the previous corpus's frequencies and total, and retained vectors keep
// their original weights and order.
func UpdateFast(ctx context.Context, previous *Index, paths []string, changed []Document, depth Depth) (*Index, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if previous == nil || previous.Engine() != Fast || previous.Depth() != depth {
		return nil, errUpdateModel
	}
	if !previous.HasFastStatistics() {
		return nil, errUpdateStatistics
	}
	return previous.mergeUpdate(ctx, paths, changed, func() (*Index, error) {
		index, err := prepareIndex(ctx, changed, depth)
		if err != nil {
			return nil, err
		}
		var segments segmenter.Segmenter
		corpus := previous.data.fastCorpusSize()
		progress := startChunkProgress(ctx, len(index.data.Chunks))
		for at := range index.data.Chunks {
			chunk := &index.data.Chunks[at]
			terms, err := tokenCounts(ctx, &segments, chunk.Text)
			if err != nil {
				return nil, err
			}
			chunk.setVector(tfidf(terms, previous.data.Frequencies, corpus))
			progress(at + 1)
		}
		return index, nil
	})
}

// Prune retains only the given previous paths, in caller order. It preserves
// the surviving vectors and Fast corpus statistics without embedding.
func Prune(ctx context.Context, previous *Index, paths []string) (*Index, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if previous == nil || (previous.Engine() != Fast && previous.Engine() != Contextual) {
		return nil, errUpdateModel
	}
	return previous.mergeUpdate(ctx, paths, nil, nil)
}

func (previous *Index) mergeUpdate(ctx context.Context, paths []string, changed []Document, rebuild func() (*Index, error)) (*Index, error) {
	positions := make(map[string]int, len(paths))
	for at, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, exists := positions[path]; exists || path == "." || !fs.ValidPath(path) {
			return nil, errDocument
		}
		positions[path] = at
	}
	type fileUpdate struct {
		available bool
		revised   bool
	}
	files := make([]fileUpdate, len(paths))
	for _, path := range previous.data.Paths {
		if at, retain := positions[path]; retain {
			files[at].available = true
		}
	}
	for _, source := range changed {
		at, include := positions[source.Path]
		if !include || files[at].revised {
			return nil, errDocument
		}
		files[at] = fileUpdate{available: true, revised: true}
	}
	for _, file := range files {
		if !file.available {
			return nil, errDocument
		}
	}

	var rebuilt storedIndex
	merged := previous.data
	if len(changed) > 0 {
		index, err := rebuild()
		if err != nil {
			return nil, err
		}
		rebuilt = index.data
		if merged.Dimension > 0 && rebuilt.Dimension > 0 && merged.Dimension != rebuilt.Dimension {
			return nil, errEmbeddingVector
		}
		merged.Dimension = max(merged.Dimension, rebuilt.Dimension)
	}
	if merged.Engine == Fast && merged.CorpusSize == nil {
		corpus := previous.data.fastCorpusSize()
		merged.CorpusSize = &corpus
	}
	merged.Paths = slices.Clone(paths)
	merged.Chunks = nil
	for _, chunk := range previous.data.Chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		at, retain := positions[previous.data.Paths[chunk.File]]
		if retain && !files[at].revised {
			chunk.File = at
			merged.Chunks = append(merged.Chunks, chunk)
		}
	}
	for _, chunk := range rebuilt.Chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		chunk.File = positions[rebuilt.Paths[chunk.File]]
		merged.Chunks = append(merged.Chunks, chunk)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &Index{data: merged}, nil
}
