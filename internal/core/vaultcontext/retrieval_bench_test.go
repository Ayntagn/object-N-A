// spec: EV-UI-290 / EV-MYGO-195
package vaultcontext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"testing"
)

func BenchmarkVaultRetrieval(b *testing.B) {
	index, query := retrievalFixture(16000, 256)
	ctx := context.Background()
	b.Run("Query", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := index.SearchVector(ctx, query); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("SaveJSON", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := json.NewEncoder(io.Discard).Encode(index); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("ReopenJSON", func(b *testing.B) {
		data, err := json.Marshal(index)
		if err != nil {
			b.Fatal(err)
		}
		b.SetBytes(int64(len(data)))
		b.ReportAllocs()
		for b.Loop() {
			var opened Index
			if err := json.Unmarshal(data, &opened); err != nil {
				b.Fatal(err)
			}
		}
		b.ReportMetric(float64(len(data)), "index-bytes")
	})
	b.Run("SavePacked", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := index.WritePacked(io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("ReopenPacked", func(b *testing.B) {
		var data bytes.Buffer
		if err := index.WritePacked(&data); err != nil {
			b.Fatal(err)
		}
		b.SetBytes(int64(data.Len()))
		b.ReportAllocs()
		for b.Loop() {
			if _, err := ReadPacked(bytes.NewReader(data.Bytes())); err != nil {
				b.Fatal(err)
			}
		}
		b.ReportMetric(float64(data.Len()), "index-bytes")
	})
}

func retrievalFixture(chunks, dimensions int) (*Index, []float32) {
	index := &Index{data: storedIndex{
		Version: indexVersion, Depth: Balanced, Engine: Contextual,
		ModelID: "benchmark", Dimension: dimensions,
		Paths: make([]string, 0, chunks/4), Chunks: make([]indexedChunk, 0, chunks),
	}}
	rng := rand.New(rand.NewSource(319))
	query := make([]float32, dimensions)
	for i := range query {
		query[i] = 2*rng.Float32() - 1
	}
	for i := range chunks {
		if i%4 == 0 {
			index.data.Paths = append(index.data.Paths, fmt.Sprintf("Folder/Note %05d.md", i/4))
		}
		vector := make([]float32, dimensions)
		for j := range vector {
			vector[j] = (float32(i%4)*0.16)*query[j] + 2*rng.Float32() - 1
		}
		chunk := indexedChunk{
			File: i / 4, Offset: (i % 4) * 64,
			Text: "缓存与查询原文 / synthetic passage, UTF-8 offset preserved.",
		}
		chunk.setVector(vector)
		index.data.Chunks = append(index.data.Chunks, chunk)
	}
	return index, query
}
