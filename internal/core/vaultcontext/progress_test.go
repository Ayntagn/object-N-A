// spec: EV-UI-307

package vaultcontext

import (
	"fmt"
	"slices"
	"testing"
)

func TestBuildProgressCountsNewChunks(t *testing.T) {
	const total = 101
	var docs []Document
	for at := range total {
		docs = append(docs, Document{Path: fmt.Sprintf("Note %d.md", at),
			Text: "These notes describe the coastal geology of a river delta."})
	}
	docs = append(docs, Document{Path: "Skipped.md"})
	var updates []BuildProgress
	ctx := WithBuildProgress(t.Context(), func(progress BuildProgress) {
		updates = append(updates, progress)
	})
	previous, err := Build(ctx, docs, Quick)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 102 || updates[0] != (BuildProgress{Phase: BuildWordStatistics, Total: total}) ||
		updates[1] != (BuildProgress{Phase: BuildChunks, Total: total}) {
		t.Fatalf("expected word statistics and 101 integer percentage updates, got %v", updates)
	}
	for percent, update := range updates[1:] {
		if update.Phase != BuildChunks || update.Total != total || update.Processed*100/total != percent {
			t.Fatalf("at %d%% got %+v", percent, update)
		}
	}
	updates = nil
	changed := []Document{
		{Path: docs[0].Path, Text: "The coastal geology chapter now describes a new estuary and its tidal flats."},
		{Path: "Skipped.md", Text: "Short"},
	}
	merged, err := UpdateFast(ctx, previous, previous.Paths(), changed, Quick)
	if err != nil {
		t.Fatal(err)
	}
	want := []BuildProgress{{Phase: BuildChunks, Total: 1}, {Phase: BuildChunks, Processed: 1, Total: 1}}
	if merged.ChunkCount() != total || !slices.Equal(updates, want) {
		t.Fatalf("expected %d committed chunks and progress %v, got %d and %v", total, want, merged.ChunkCount(), updates)
	}
}
