package vaultcontext

// spec: EV-UI-292 / EV-MYGO-195

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestContextualIndexReloadsModelAndOwnsVectors(t *testing.T) {
	ctx := context.Background()
	docs := []Document{
		{Path: "Coral.md", Text: strings.Repeat("underwater coral ", 5)},
		{Path: "Pebbles.md", Text: strings.Repeat("pebbles on the shore ", 5)},
	}
	scratch := make([]float32, 3)
	index, err := BuildContextual(ctx, docs, Balanced, "fixture-contextual", func(_ context.Context, text string) ([]float32, error) {
		clear(scratch)
		if strings.Contains(text, "coral") {
			scratch[0] = 2
		} else {
			scratch[2] = 4
		}
		return scratch, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	clear(scratch)
	encoded, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Index
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	matches, err := decoded.SearchVector(ctx, []float32{1, 0, 0})
	if err != nil || len(matches) != 1 || matches[0].Path != "Coral.md" || decoded.Engine() != Contextual || decoded.ModelID() != "fixture-contextual" || decoded.Dimension() != 3 {
		t.Fatalf("contextual snapshot returned %v, %v", matches, err)
	}
	if _, err := decoded.SearchVector(ctx, []float32{1, 0}); !errors.Is(err, errEmbeddingVector) {
		t.Fatalf("wrong query dimension returned %v", err)
	}
	if _, err := BuildContextual(ctx, docs, Balanced, "fixture-contextual", func(context.Context, string) ([]float32, error) {
		return []float32{float32(math.NaN()), 0, 0}, nil
	}); !errors.Is(err, errEmbeddingVector) {
		t.Fatalf("invalid provider vector returned %v", err)
	}
}
