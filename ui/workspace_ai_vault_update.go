package ui

import (
	"context"
	"maps"
	"slices"
	"strings"

	"cyberwriter/internal/core/vaultcontext"
	"cyberwriter/internal/core/workspace"
)

type aiVaultBuildSnapshot struct {
	vault         *workspace.Vault
	files         map[string]aiVaultFileStamp
	depth         vaultcontext.Depth
	previous      *vaultcontext.Index
	previousFiles map[string]aiVaultFileStamp
	previousSkips []vaultcontext.MapSkippedFile
}

func (s aiVaultBuildSnapshot) build(ctx context.Context) (*vaultcontext.Index, []vaultcontext.MapSkippedFile, error) {
	if s.depth == vaultcontext.Quick {
		return s.buildIndex(ctx, "", 0, nil)
	}
	return buildContextualVaultIndex(ctx, s)
}

// EV-UI-292 / EV-UI-298. Reusing contextual chunks requires the same
// model and depth. Quick incremental builds retain the saved TF-IDF corpus.
func (s aiVaultBuildSnapshot) buildIndex(ctx context.Context, modelID string, dimension int, embed func(context.Context, string) ([]float32, error)) (*vaultcontext.Index, []vaultcontext.MapSkippedFile, error) {
	previous := s.previous
	reuse := previous != nil && previous.Depth() == s.depth
	if reuse && embed == nil {
		reuse = previous.HasFastStatistics()
	} else if reuse {
		reuse = previous.Engine() == vaultcontext.Contextual && previous.ModelID() == modelID &&
			(previous.Dimension() == 0 || previous.Dimension() == dimension)
	}
	if reuse && maps.Equal(s.files, s.previousFiles) {
		return previous, s.previousSkips, ctx.Err()
	}
	paths, docs, reasons, err := s.candidates(ctx, reuse)
	if err != nil {
		return nil, nil, err
	}
	var index *vaultcontext.Index
	switch {
	case embed == nil && reuse:
		index, err = vaultcontext.UpdateFast(ctx, previous, paths, docs, s.depth)
	case embed == nil:
		index, err = vaultcontext.Build(ctx, docs, s.depth)
	case reuse:
		index, err = vaultcontext.UpdateContextual(ctx, previous, paths, docs, s.depth, modelID, embed)
	default:
		index, err = vaultcontext.BuildContextual(ctx, docs, s.depth, modelID, embed)
	}
	if err != nil {
		return nil, nil, err
	}
	for _, path := range index.UnembeddedPaths() {
		if reasons[path] == "" {
			reasons[path] = "No embeddable content after stripping frontmatter / code"
		}
	}
	var skipped []vaultcontext.MapSkippedFile
	for _, path := range slices.Sorted(maps.Keys(reasons)) {
		skipped = append(skipped, vaultcontext.MapSkippedFile{Path: path, Reason: reasons[path]})
	}
	return index, skipped, nil
}

func (s aiVaultBuildSnapshot) candidates(ctx context.Context, reuse bool) ([]string, []vaultcontext.Document, map[string]string, error) {
	previousPaths := make(map[string]bool)
	previousSkips := make(map[string]string)
	withoutChunks := make(map[string]bool)
	if reuse {
		for _, path := range s.previous.Paths() {
			previousPaths[path] = true
		}
		for _, file := range s.previousSkips {
			previousSkips[file.Path] = file.Reason
		}
		for _, path := range s.previous.UnembeddedPaths() {
			withoutChunks[path] = true
		}
	}

	paths := make([]string, 0, len(s.files))
	var docs []vaultcontext.Document
	reasons := make(map[string]string)
	for _, path := range slices.Sorted(maps.Keys(s.files)) {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		prior, unchanged := s.previousFiles[path]
		if reuse && unchanged && prior == s.files[path] {
			reason := previousSkips[path]
			if reason != "" {
				reasons[path] = reason
				if previousPaths[path] {
					paths = append(paths, path)
				}
				continue
			}
			if previousPaths[path] && !withoutChunks[path] {
				paths = append(paths, path)
				continue
			}
		}
		// EV-UI-297. A cached index can lack the transient skip reason.
		// Read such a candidate to recover the reason on this update.
		doc, err := s.vault.ReadTextDocument(path)
		if err != nil {
			reasons[path] = "Couldn't read file (encoding?)"
			continue
		}
		if strings.TrimSpace(doc.Text) == "" {
			reasons[path] = "File is empty"
		}
		paths = append(paths, path)
		docs = append(docs, vaultcontext.Document{Path: doc.Path, Text: doc.Text})
	}
	return paths, docs, reasons, nil
}
