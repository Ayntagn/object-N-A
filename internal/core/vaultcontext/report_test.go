package vaultcontext

// spec: EV-UI-295 / EV-MYGO-201

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestVaultMapMarkdown(t *testing.T) {
	generated := time.Date(2026, 11, 16, 19, 0, 0, 0, time.FixedZone("fixture", 8*60*60))
	long := strings.Repeat("é", 80)
	index := Index{data: storedIndex{
		ModelID: fastModelID, Dimension: fastDimensions,
		Paths: []string{"Z.md", "P/Alpha\r.md", "Skipped.md"},
		Chunks: []indexedChunk{
			{File: 0, Text: "short demo"},
			{File: 1, Text: "α|\n🧭"},
			{File: 1, Text: long + "字"},
		},
	}}
	got := index.VaultMapMarkdown(MapReportOptions{
		VaultRoot: "/tmp/Report\r\nVault", IndexPath: "/tmp/\nindex.json", IndexSize: 5 << 19,
		GeneratedAt: generated, BuildDuration: 1500 * time.Millisecond,
		ModifiedAt: map[string]time.Time{"P/Alpha\r.md": generated.Add(-90 * time.Minute)},
		Skipped:    []MapSkippedFile{{Path: "Skipped.md", Reason: "Fixture unavailable"}},
	})
	want := "# Vault Embeddings Map\n\n" +
		"**Generated:** 2026-11-16T11:00:00Z\n" +
		"**Vault:** Report Vault\n" +
		"**Model:** `TFIDFEmbedder.v1.256d` · Latin script · 256-dim\n" +
		"**Index file:** `/tmp/ index.json` · 2.5 MB\n\n" +
		"## Summary\n\n| Metric | Value |\n|---|---|\n" +
		"| Files embedded | 2 |\n| Files skipped | 1 |\n| Total chunks | 3 |\n| Avg chunks / file | 1.5 |\n| Build time | 1.5 s |\n\n" +
		"## Embedded files\n\n" +
		"### P/Alpha .md\n*2 chunks · source modified 1 h ago*\n\n| # | Chars | Preview |\n|---|---|---|\n" +
		"| 1 | 4 | α\\| 🧭 |\n| 2 | 81 | " + long + "… |\n\n" +
		"### Z.md\n*1 chunk · source modified just now*\n\n| # | Chars | Preview |\n|---|---|---|\n" +
		"| 1 | 10 | short demo |\n\n" +
		"## Skipped files\n\n| File | Reason |\n|---|---|\n| `Skipped.md` | Fixture unavailable |\n"
	if got != want {
		t.Fatalf("report differs\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestVaultMapCapsBeforeWholeFile(t *testing.T) {
	index := Index{}
	for file := range 202 {
		index.data.Paths = append(index.data.Paths, fmt.Sprintf("Page-%03d.md", file))
		chunks := 25
		switch file {
		case 199:
			chunks = 24
		case 200:
			chunks = 2
		case 201:
			chunks = 1
		}
		for range chunks {
			index.data.Chunks = append(index.data.Chunks, indexedChunk{File: file, Text: "fixture"})
		}
	}
	out := index.VaultMapMarkdown(MapReportOptions{})
	if !strings.Contains(out, "### Page-200.md\n*2 chunks") || strings.Contains(out, "### Page-201.md") ||
		!strings.Contains(out, "> _… and 1 more chunks across 1 files not shown (vault map cap)._") {
		t.Fatal("report did not keep the whole last admitted file and describe the omitted group")
	}
}
