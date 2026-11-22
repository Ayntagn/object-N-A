package vaultindex

import (
	"context"
	"iter"
	"time"

	"cyberwriter/internal/core/workspace"
	"object-na.local/shortsearch"
)

const (
	sidebarBodyFileLimit  = 8 << 20
	sidebarBodyBatchLimit = 2 << 20
)

// SidebarBodyCacheVersion also distinguishes how the saved source was passed
// to shortsearch. The module alone removes the initial BOM.
const SidebarBodyCacheVersion = shortsearch.CacheVersion + "-2"

// RefreshSidebarBodyIndex brings a disposable candidate cache up to date with
// the saved files from the Files snapshot. Searches independently validate its
// versions, including while a refresh is still in flight.
func RefreshSidebarBodyIndex(ctx context.Context, vault *workspace.Vault, entries []workspace.Entry, filename string) error {
	if vault == nil || !vault.HasIndexedTextVersions() || filename == "" {
		return nil
	}
	store, err := shortsearch.Open(ctx, filename)
	if err != nil {
		return err
	}
	defer store.Close()
	versions, err := store.Versions(ctx)
	if err != nil {
		return err
	}
	var batch []shortsearch.Document
	var batchBytes int
	flush := func() error {
		err := store.Update(ctx, batch, nil)
		clear(batch)
		batch, batchBytes = batch[:0], 0
		return err
	}
	for path := range sidebarBodyPaths(entries) {
		if err := ctx.Err(); err != nil {
			return err
		}
		known := versions[path]
		delete(versions, path)
		text, version, readErr := vault.ReadIndexedText(path, known, sidebarBodyFileLimit)
		if readErr != nil || version == "" || version == known {
			continue
		}
		batch = append(batch, shortsearch.Document{Path: path, Version: version, Text: text})
		batchBytes += len(text)
		if batchBytes >= sidebarBodyBatchLimit || len(batch) >= 64 {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	removed := make([]string, 0, len(versions))
	for path := range versions {
		removed = append(removed, path)
	}
	if len(batch) != 0 || len(removed) != 0 {
		return store.Update(ctx, batch, removed)
	}
	return ctx.Err()
}

func sidebarBodyPaths(entries []workspace.Entry) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, entry := range entries {
			if entry.IsDir {
				for path := range sidebarBodyPaths(entry.Children) {
					if !yield(path) {
						return
					}
				}
			} else if sidebarSearchableBody(entry.Path) && !yield(entry.Path) {
				return
			}
		}
	}
}

// SearchSidebarWithBodyIndex uses the disk index as a conservative negative
// filter. Files without a verified cache version use the ordinary matcher.
func (index *Index) SearchSidebarWithBodyIndex(ctx context.Context, vault *workspace.Vault,
	entries []workspace.Entry, query, graphTag, filename string) []SidebarSearchHit {
	if vault == nil || filename == "" || !vault.HasIndexedTextVersions() || SidebarSearchQueryLength(query) < 2 {
		return index.SearchSidebar(ctx, vault, entries, query, graphTag)
	}
	lookup, cancel := context.WithTimeout(ctx, 40*time.Millisecond)
	defer cancel()
	candidates, err := lookupSidebarBodies(lookup, filename, TrimSidebarSearchQuery(query))
	if err != nil {
		return index.SearchSidebar(ctx, vault, entries, query, graphTag)
	}
	return index.searchSidebar(ctx, vault, entries, query, graphTag, func(path string) bool {
		if _, match := candidates.Matches[path]; match {
			return true
		}
		version := candidates.Versions[path]
		return version == "" || vault.IndexedTextVersion(path) != version
	})
}

func lookupSidebarBodies(ctx context.Context, filename, query string) (shortsearch.CandidateSet, error) {
	store, err := shortsearch.Open(ctx, filename)
	if err != nil {
		return shortsearch.CandidateSet{}, err
	}
	defer store.Close()
	return store.Lookup(ctx, query)
}
