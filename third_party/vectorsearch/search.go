// Package vectorsearch selects exact cosine matches over prepared vectors.
package vectorsearch

import (
	"context"
	"errors"
	"math"
)

// ErrDimension reports a row whose length differs from the query's.
var ErrDimension = errors.New("vectorsearch: mismatched vector dimension")

// Vector describes immutable float32 values and their precomputed norm.
type Vector struct {
	Values []float32
	Norm   float64
}

// Match reports the row index and its cosine score.
type Match struct {
	Index int
	Score float64
}

// Norm computes a vector's Euclidean norm with float64 accumulation.
func Norm(values []float32) float64 {
	var squared float64
	for _, value := range values {
		v := float64(value)
		squared += v * v
	}
	return math.Sqrt(squared)
}

// CosineTopK selects at most k rows meeting minimum, in descending score
// order with input order retained across ties. The finite values in each
// row must match query's dimension. The row's Norm must come from Norm.
// A cancelled call discards its results.
func CosineTopK(ctx context.Context, query []float32, count, k int, minimum float64, row func(int) Vector) ([]Match, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	queryNorm := Norm(query)
	if queryNorm == 0 || count <= 0 || k <= 0 {
		return nil, nil
	}
	k = min(count, k)
	var best []Match
	for index := range count {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		vector := row(index)
		if len(vector.Values) != len(query) {
			return nil, ErrDimension
		}
		if vector.Norm == 0 {
			continue
		}
		var dot float64
		for at, value := range vector.Values {
			dot += float64(value) * float64(query[at])
		}
		score := dot / (vector.Norm * queryNorm)
		if !(score >= minimum) {
			continue
		}
		score = min(1, score)
		if len(best) == k && score <= best[k-1].Score {
			continue
		}
		if best == nil {
			best = make([]Match, 0, k)
		}
		if len(best) < k {
			best = append(best, Match{})
		}
		at := len(best) - 1
		for at > 0 && score > best[at-1].Score {
			best[at] = best[at-1]
			at--
		}
		best[at] = Match{Index: index, Score: score}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return best, nil
}
