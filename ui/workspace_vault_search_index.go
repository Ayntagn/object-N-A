package ui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"cyberwriter/internal/core/vaultindex"
)

// The cache is attached to the mounted Vault. Its progress is independent of
// queries and the visible tag/link snapshot.
func (app *workspaceWindow) refreshSidebarBodyIndex() {
	vault, entries, win := app.vault, app.entries, app.win
	if vault == nil || win == nil || !vault.HasIndexedTextVersions() || app.index.cancelBody != nil {
		return
	}
	settings, err := preferenceFile()
	if err != nil {
		return
	}
	digest := sha256.Sum256([]byte(app.vaultEmbeddingOfferPath))
	directory := filepath.Join(filepath.Dir(settings), ".vault-search", vaultindex.SidebarBodyCacheVersion)
	filename := filepath.Join(directory, hex.EncodeToString(digest[:])+".sqlite")
	attachment, generation := app.index.attachment, app.index.generation
	ctx, cancel := context.WithCancel(context.Background())
	app.index.cancelBody = cancel
	go func() {
		err := os.MkdirAll(directory, 0o700)
		if err == nil {
			err = vaultindex.RefreshSidebarBodyIndex(ctx, vault, entries, filename)
		}
		win.Update(func() {
			defer cancel()
			if ctx.Err() != nil || win.IsDestroyed() || app.vault != vault || app.index.attachment != attachment {
				return
			}
			app.index.cancelBody = nil
			if err == nil {
				app.index.bodyCache = filename
			}
			if app.index.generation != generation {
				app.refreshSidebarBodyIndex()
			}
		})
	}()
}
