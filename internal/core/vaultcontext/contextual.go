package vaultcontext

import (
	"context"
	"slices"
	"strings"
)

// BuildContextual indexes the selected depth using vectors from a single local
// model. The first chunk fixes the vector dimension. An embedding failure
// leaves the caller's previous index untouched.
func BuildContextual(ctx context.Context, docs []Document, depth Depth, modelID string, embed func(context.Context, string) ([]float32, error)) (*Index, error) {
	if strings.TrimSpace(modelID) == "" || embed == nil {
		return nil, errSearchEngine
	}
	index, err := prepareIndex(ctx, docs, depth)
	if err != nil {
		return nil, err
	}
	d := &index.data
	d.Engine, d.ModelID = Contextual, modelID
	progress := startChunkProgress(ctx, len(d.Chunks))
	for at := range d.Chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		chunk := &d.Chunks[at]
		vector, err := embed(ctx, chunk.Text)
		if err != nil {
			return nil, err
		}
		if d.Dimension == 0 {
			d.Dimension = len(vector)
		}
		if !validVector(vector, d.Dimension) {
			return nil, errEmbeddingVector
		}
		chunk.Vector = slices.Clone(vector)
		progress(at + 1)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return index, nil
}

// SearchVector ranks at most five chunks using a query from the model named by
// ModelID. No results returns nil, nil. Cancellation returns the context error.
func (i *Index) SearchVector(ctx context.Context, query []float32) ([]Match, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if i.data.Engine != Contextual {
		return nil, errSearchEngine
	}
	if i.ChunkCount() == 0 {
		return nil, nil
	}
	if !validVector(query, i.Dimension()) {
		return nil, errEmbeddingVector
	}
	return i.search(ctx, query)
}
