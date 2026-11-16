package vaultcontext

// spec: EV-UI-298 / EV-MYGO-203

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestContextualUpdateReusesUnchangedChunks(t *testing.T) {
	ctx := context.Background()
	const model = "local-fixture"
	const header = "---\ntitle: 中文 fixture\n---\n"
	unchanged := strings.TrimSpace(strings.Repeat("kept coastal geology ", 5))
	updated := strings.TrimSpace(strings.Repeat("new estuary field notes ", 5))
	added := strings.TrimSpace(strings.Repeat("added river delta ", 5))
	previous, err := BuildContextual(ctx, []Document{
		{Path: "Kept.md", Text: header + unchanged},
		{Path: "Deleted.md", Text: strings.Repeat("deleted forest sample ", 5)},
		{Path: "Changed.md", Text: strings.Repeat("old estuary field notes ", 5)},
		{Path: "Now Short.md", Text: strings.Repeat("removed mountain sample ", 5)},
		{Path: "Always Short.md", Text: "too short"},
	}, Balanced, model, func(context.Context, string) ([]float32, error) {
		return []float32{1, 0}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(previous)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"Added.md", "Always Short.md", "Changed.md", "Kept.md", "Now Short.md"}
	var embedded []string
	index, err := UpdateContextual(ctx, previous, paths, []Document{
		{Path: "Added.md", Text: added},
		{Path: "Changed.md", Text: header + updated},
		{Path: "Now Short.md", Text: "short"},
	}, Balanced, model, func(_ context.Context, source string) ([]float32, error) {
		embedded = append(embedded, source)
		return []float32{1, 0}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(embedded, []string{added + "\n", updated + "\n"}) || !slices.Equal(index.Paths(), paths) ||
		!slices.Equal(index.UnembeddedPaths(), []string{"Always Short.md", "Now Short.md"}) {
		t.Fatalf("embedded %q, paths %v, unembedded %v", embedded, index.Paths(), index.UnembeddedPaths())
	}
	paths[0] = "Should not alias.md"
	matches, err := index.SearchVector(ctx, []float32{1, 0})
	if err != nil || len(matches) != 3 {
		t.Fatalf("updated matches %v, %v", matches, err)
	}
	want := []Match{
		{Path: "Kept.md", Text: unchanged + "\n", Offset: len(header)},
		{Path: "Added.md", Text: added + "\n", Offset: 0},
		{Path: "Changed.md", Text: updated + "\n", Offset: len(header)},
	}
	for at, match := range matches {
		if expected := want[at]; expected.Path != match.Path || expected.Text != match.Text || expected.Offset != match.Offset {
			t.Fatalf("incorrect result: %+v", match)
		}
	}
	after, err := json.Marshal(previous)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("updating mutated the previously published index: %v", err)
	}
}

func TestFastUpdateRetainsCorpusAcrossDeletionAndReload(t *testing.T) {
	ctx := context.Background()
	discovery := strings.Repeat("lantern ", 10)
	previous, err := Build(ctx, []Document{
		{Path: "Kept.md", Text: discovery},
		{Path: "Absent.md", Text: discovery},
		{Path: "Removed.md", Text: strings.Repeat("quarry ", 10)},
	}, Quick)
	if err != nil {
		t.Fatal(err)
	}
	// Build 145 snapshots encoded no separate corpus count.
	previous.data.CorpusSize = nil
	merged, err := UpdateFast(ctx, previous, []string{"Added.md", "Kept.md"},
		[]Document{{Path: "Added.md", Text: discovery}}, Quick)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := merged.Search(ctx, "lantern")
	if err != nil || len(matches) != 2 || matches[0].Path != "Kept.md" || matches[1].Path != "Added.md" {
		t.Fatalf("updated search: %v, %v", matches, err)
	}
	empty, err := Prune(ctx, merged, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	var restored Index
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.ChunkCount() != 0 || !restored.HasFastStatistics() {
		t.Fatalf("empty index lost its previous corpus: %s", data)
	}
	next, err := UpdateFast(ctx, &restored, []string{"New.md"}, []Document{{Path: "New.md", Text: discovery}}, Quick)
	if err != nil {
		t.Fatal(err)
	}
	matches, err = next.Search(ctx, "lantern")
	if err != nil || len(matches) != 1 || matches[0].Path != "New.md" {
		t.Fatalf("restored search: %v, %v", matches, err)
	}
}

func TestContextualUpdateRejectsUnavailableSnapshotBeforeEmbedding(t *testing.T) {
	ctx := context.Background()
	embed := func(context.Context, string) ([]float32, error) {
		t.Fatal("unexpected embedding")
		return nil, nil
	}
	previous, err := BuildContextual(ctx, []Document{{Path: "Known.md", Text: "short"}}, Deep, "fixture", embed)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"Known.md", "Unavailable.md"}
	if index, err := UpdateContextual(ctx, previous, paths, nil, Deep, "fixture", embed); index != nil || !errors.Is(err, errDocument) {
		t.Fatalf("missing snapshot returned %v, %v", index, err)
	}
	if index, err := UpdateContextual(ctx, previous, paths[:1], nil, Balanced, "fixture", embed); index != nil || err == nil {
		t.Fatalf("mismatched depth returned %v, %v", index, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if index, err := UpdateContextual(canceled, previous, paths, nil, Deep, "fixture", embed); index != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled update returned %v, %v", index, err)
	}
}
