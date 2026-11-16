package vaultcontext

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-text/typesetting/segmenter"
	"golang.org/x/text/unicode/norm"
)

// MapReportOptions describes the Vault and index snapshot at report creation.
// Older cached indexes may supply only the Vault and index paths.
type MapReportOptions struct {
	VaultRoot     string
	IndexPath     string
	IndexSize     int64
	GeneratedAt   time.Time
	BuildDuration time.Duration
	ModifiedAt    map[string]time.Time
	Skipped       []MapSkippedFile
}

// MapSkippedFile describes a Vault-relative path and its observed skip reason.
type MapSkippedFile struct {
	Path   string
	Reason string
}

const mapReportChunkLimit = 5000

var (
	mapReportOneLine   = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")
	mapReportTableText = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "|", "\\|")
)

// VaultMapMarkdown describes the chunks in an immutable index. All paths and
// file metadata are supplied by the caller, so generation needs no disk I/O.
func (i *Index) VaultMapMarkdown(options MapReportOptions) string {
	groups := make(map[string][]string)
	for _, chunk := range i.data.Chunks {
		path := i.data.Paths[chunk.File]
		groups[path] = append(groups[path], chunk.Text)
	}
	paths := make([]string, 0, len(groups))
	for path := range groups {
		paths = append(paths, path)
	}
	slices.SortFunc(paths, func(a, b string) int {
		if order := strings.Compare(norm.NFC.String(a), norm.NFC.String(b)); order != 0 {
			return order
		}
		return strings.Compare(a, b)
	})
	var average float64
	if len(paths) > 0 {
		average = float64(i.ChunkCount()) / float64(len(paths))
	}
	lines := []string{
		"# Vault Embeddings Map",
		"",
		"**Generated:** " + options.GeneratedAt.UTC().Format(time.RFC3339),
		"**Vault:** " + mapReportOneLine.Replace(filepath.Base(options.VaultRoot)),
		fmt.Sprintf("**Model:** `%s` · Latin script · %d-dim", i.ModelID(), i.Dimension()),
		fmt.Sprintf("**Index file:** `%s` · %.1f MB", mapReportOneLine.Replace(options.IndexPath), float64(max(0, options.IndexSize))/(1<<20)),
		"",
		"## Summary",
		"",
		"| Metric | Value |",
		"|---|---|",
		fmt.Sprintf("| Files embedded | %d |", len(paths)),
		fmt.Sprintf("| Files skipped | %d |", len(options.Skipped)),
		fmt.Sprintf("| Total chunks | %d |", i.ChunkCount()),
		fmt.Sprintf("| Avg chunks / file | %.1f |", average),
	}
	if options.BuildDuration > 0 {
		lines = append(lines, fmt.Sprintf("| Build time | %.1f s |", options.BuildDuration.Seconds()))
	}
	lines = append(lines, "", "## Embedded files", "")

	var segments segmenter.Segmenter
	shown := 0
	for at, path := range paths {
		if shown >= mapReportChunkLimit {
			lines = append(lines, "", fmt.Sprintf(
				"> _… and %d more chunks across %d files not shown (vault map cap)._",
				i.ChunkCount()-shown, len(paths)-at))
			break
		}
		texts := groups[path]
		unit := "chunks"
		if len(texts) == 1 {
			unit = "chunk"
		}
		lines = append(lines,
			"### "+mapReportOneLine.Replace(path),
			fmt.Sprintf("*%d %s · source modified %s*", len(texts), unit, mapReportAge(options.GeneratedAt, options.ModifiedAt[path])),
			"",
			"| # | Chars | Preview |",
			"|---|---|---|",
		)
		for number, source := range texts {
			segments.InitWithString(source)
			count := 0
			for graphemes := segments.GraphemeIterator(); graphemes.Next(); {
				count++
			}
			preview := strings.TrimSpace(mapReportTableText.Replace(source))
			segments.InitWithString(preview)
			graphemes := segments.GraphemeIterator()
			for position := 0; graphemes.Next(); position++ {
				if position == 80 {
					preview = preview[:graphemes.Grapheme().OffsetInBytes] + "…"
					break
				}
			}
			lines = append(lines, fmt.Sprintf("| %d | %d | %s |", number+1, count, preview))
		}
		shown += len(texts)
		lines = append(lines, "")
	}
	if len(options.Skipped) > 0 {
		lines = append(lines, "## Skipped files", "", "| File | Reason |", "|---|---|")
		for _, file := range options.Skipped {
			lines = append(lines, fmt.Sprintf("| `%s` | %s |", mapReportTableText.Replace(file.Path), mapReportTableText.Replace(file.Reason)))
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func mapReportAge(generatedAt, modifiedAt time.Time) string {
	age := generatedAt.Sub(modifiedAt)
	switch {
	case modifiedAt.IsZero() || age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%d min ago", age/time.Minute)
	case age < 24*time.Hour:
		return fmt.Sprintf("%d h ago", age/time.Hour)
	default:
		return fmt.Sprintf("%d d ago", age/(24*time.Hour))
	}
}
