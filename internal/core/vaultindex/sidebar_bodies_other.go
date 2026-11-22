//go:build !darwin

package vaultindex

import (
	"context"

	"cyberwriter/internal/core/workspace"
)

// No precise file version is available to the sidebar on these platforms.
const SidebarBodyCacheVersion = ""

func RefreshSidebarBodyIndex(ctx context.Context, _ *workspace.Vault, _ []workspace.Entry, _ string) error {
	return ctx.Err()
}

func (index *Index) SearchSidebarWithBodyIndex(ctx context.Context, vault *workspace.Vault,
	entries []workspace.Entry, query, graphTag, _ string) []SidebarSearchHit {
	return index.SearchSidebar(ctx, vault, entries, query, graphTag)
}
