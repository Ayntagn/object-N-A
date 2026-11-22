// Package vaultcontext indexes selected note text for local AI retrieval.
package vaultcontext

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"object-na.local/vectorsearch"
)

type Engine string

const (
	Fast       Engine = "fast"
	Contextual Engine = "contextual"
)

const (
	indexVersion  = 1
	maxDimensions = 4096
	searchLimit   = 5
	minimumScore  = float64(float32(0.30))
)

var (
	errIndexFormat     = errors.New("invalid Vault Context index")
	errIndexDepth      = errors.New("invalid Vault Context depth")
	errDocument        = errors.New("invalid or duplicate Vault Context document")
	errSearchEngine    = errors.New("index requires a query from the same embedding engine")
	errEmbeddingVector = errors.New("invalid Vault Context embedding vector")
)

// Document is a UTF-8 note snapshot with a Vault-relative slash path.
type Document struct {
	Path string
	Text string
}

// Match contains the original path and indexed text. Offset is a UTF-8 byte
// coordinate in the preprocessed note, including any removed frontmatter
// prefix. Length is the byte count of Text.
type Match struct {
	Path   string
	Text   string
	Offset int
	Length int
	Score  float64
}

// Index is immutable after building or decoding. Decode a fresh Index before
// publishing it to concurrent readers.
type Index struct {
	data storedIndex
}

type storedIndex struct {
	Version     int            `json:"version"`
	Engine      Engine         `json:"engine"`
	ModelID     string         `json:"modelIdentifier"`
	Dimension   int            `json:"dimension"`
	Depth       Depth          `json:"depth"`
	Paths       []string       `json:"files"`
	Frequencies map[string]int `json:"docFrequencies,omitempty"`
	CorpusSize  *int           `json:"tfidfTotalDocs,omitempty"`
	Chunks      []indexedChunk `json:"chunks"`
}

type indexedChunk struct {
	File   int       `json:"file"`
	Offset int       `json:"byteOffset"`
	Text   string    `json:"text"`
	Vector []float32 `json:"vector"`
	norm   float64
}

func (chunk *indexedChunk) setVector(vector []float32) {
	chunk.Vector = vector
	chunk.norm = vectorsearch.Norm(vector)
}

func (i *Index) Depth() Depth    { return i.data.Depth }
func (i *Index) Engine() Engine  { return i.data.Engine }
func (i *Index) ModelID() string { return i.data.ModelID }
func (i *Index) Dimension() int  { return i.data.Dimension }
func (i *Index) FileCount() int  { return len(i.data.Paths) }
func (i *Index) ChunkCount() int { return len(i.data.Chunks) }

// Paths returns the supplied note paths in their index order, including notes
// too short to contribute a match.
func (i *Index) Paths() []string { return slices.Clone(i.data.Paths) }

// UnembeddedPaths reports the readable files that did not yield any chunks.
func (i *Index) UnembeddedPaths() []string {
	hasChunk := make([]bool, len(i.data.Paths))
	for _, chunk := range i.data.Chunks {
		hasChunk[chunk.File] = true
	}
	var paths []string
	for at, path := range i.data.Paths {
		if !hasChunk[at] {
			paths = append(paths, path)
		}
	}
	return paths
}

func (i *Index) MarshalJSON() ([]byte, error) { return json.Marshal(i.data) }

func (i *Index) UnmarshalJSON(data []byte) error {
	var decoded storedIndex
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if !decoded.valid() {
		return errIndexFormat
	}
	for at := range decoded.Chunks {
		chunk := &decoded.Chunks[at]
		chunk.setVector(chunk.Vector)
	}
	i.data = decoded
	return nil
}

func (d *storedIndex) valid() bool {
	if d.Version != indexVersion || !d.Depth.valid() || strings.TrimSpace(d.ModelID) == "" || d.Dimension > maxDimensions {
		return false
	}
	switch d.Engine {
	case Fast:
		if d.Dimension != fastDimensions || d.ModelID != fastModelID || d.fastCorpusSize() < 0 {
			return false
		}
	case Contextual:
		if d.Dimension < 0 || len(d.Frequencies) != 0 || d.CorpusSize != nil || (d.Dimension == 0 && len(d.Chunks) != 0) {
			return false
		}
	default:
		return false
	}
	seen := make(map[string]struct{}, len(d.Paths))
	for _, name := range d.Paths {
		if !validPath(name, seen) {
			return false
		}
	}
	for token, df := range d.Frequencies {
		if token == "" || df < 1 || df > d.fastCorpusSize() {
			return false
		}
	}
	for _, chunk := range d.Chunks {
		if chunk.File < 0 || chunk.File >= len(d.Paths) || chunk.Offset < 0 || chunk.Text == "" || !validVector(chunk.Vector, d.Dimension) {
			return false
		}
	}
	return true
}

func prepareIndex(ctx context.Context, docs []Document, depth Depth) (*Index, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !depth.valid() {
		return nil, errIndexDepth
	}
	index := &Index{data: storedIndex{Version: indexVersion, Depth: depth, Paths: make([]string, 0, len(docs))}}
	seen := make(map[string]struct{}, len(docs))
	for file, doc := range docs {
		if !validPath(doc.Path, seen) || !utf8.ValidString(doc.Text) {
			return nil, errDocument
		}
		index.data.Paths = append(index.data.Paths, doc.Path)
		pieces, err := splitDocument(ctx, doc.Text, depth)
		if err != nil {
			return nil, err
		}
		for _, piece := range pieces {
			index.data.Chunks = append(index.data.Chunks, indexedChunk{File: file, Offset: piece.offset, Text: strings.Clone(piece.text)})
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return index, nil
}

func validPath(name string, seen map[string]struct{}) bool {
	if _, duplicate := seen[name]; duplicate || name == "." || !fs.ValidPath(name) {
		return false
	}
	seen[name] = struct{}{}
	return true
}

func validVector(vector []float32, dimension int) bool {
	if dimension < 1 || dimension > maxDimensions || len(vector) != dimension {
		return false
	}
	for _, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return false
		}
	}
	return true
}

// EV-UI-290. Ordinary Chat requests up to five chunks with cosine ≥ 0.30.
// Ties retain index order.
func (i *Index) search(ctx context.Context, query []float32) ([]Match, error) {
	selected, err := vectorsearch.CosineTopK(ctx, query, len(i.data.Chunks), searchLimit, minimumScore, func(at int) vectorsearch.Vector {
		chunk := &i.data.Chunks[at]
		return vectorsearch.Vector{Values: chunk.Vector, Norm: chunk.norm}
	})
	if err != nil || len(selected) == 0 {
		return nil, err
	}
	matches := make([]Match, len(selected))
	for at, match := range selected {
		chunk := &i.data.Chunks[match.Index]
		matches[at] = Match{Path: i.data.Paths[chunk.File], Text: chunk.Text, Offset: chunk.Offset,
			Length: len(chunk.Text), Score: match.Score}
	}
	return matches, nil
}
