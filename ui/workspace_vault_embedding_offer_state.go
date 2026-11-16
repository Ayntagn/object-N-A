package ui

import (
	"fmt"
	"maps"
	"path/filepath"

	"cyberwriter/internal/core/workspace"
)

type vaultEmbeddingOfferMode uint8

const (
	vaultEmbeddingOfferHidden vaultEmbeddingOfferMode = iota
	vaultEmbeddingOfferBuild
	vaultEmbeddingOfferDownload
)

const vaultEmbeddingOfferNever = "never"

// EV-UI-300. The decision belongs to the mounted Vault path, even when a
// different spelling of that path was used to open the Vault.
func normalizedVaultEmbeddingOfferPath(root string) string {
	if root == "" {
		return ""
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return resolved
	}
	return absolute
}

func (app *workspaceWindow) vaultEmbeddingOfferDeferred() *map[string]bool {
	if app.application != nil {
		return &app.application.deferredVaultOffers
	}
	return &app.deferredVaultOffers
}

func (app *workspaceWindow) vaultEmbeddingOfferMode() vaultEmbeddingOfferMode {
	path := app.vaultEmbeddingOfferPath
	if app.vault == nil || path == "" || app.preferences == nil || app.fileSearch != "" || app.graph.tag != "" ||
		(*app.vaultEmbeddingOfferDeferred())[path] || app.preferences.VaultEmbeddingsOfferDecisions[path] == vaultEmbeddingOfferNever {
		return vaultEmbeddingOfferHidden
	}
	state := app.aiVaultContextState()
	if state == nil || !state.loaded || state.request != nil || state.index != nil || state.problem != "" {
		return vaultEmbeddingOfferHidden
	}
	if state.assetsMissing {
		return vaultEmbeddingOfferDownload
	}
	return vaultEmbeddingOfferBuild
}

func (app *workspaceWindow) activateVaultEmbeddingOffer(vault *workspace.Vault) {
	if app.modalOpen || app.vault != vault {
		return
	}
	switch app.vaultEmbeddingOfferMode() {
	case vaultEmbeddingOfferBuild:
		app.buildAIVaultContext()
	case vaultEmbeddingOfferDownload:
		app.downloadVaultEmbeddingModel()
	}
}

func (app *workspaceWindow) dismissVaultEmbeddingOffer(vault *workspace.Vault, permanent bool) {
	if app.modalOpen || app.vault != vault || app.vaultEmbeddingOfferMode() == vaultEmbeddingOfferHidden {
		return
	}
	path := app.vaultEmbeddingOfferPath
	if permanent {
		preferences := *app.preferences
		preferences.VaultEmbeddingsOfferDecisions = maps.Clone(preferences.VaultEmbeddingsOfferDecisions)
		if preferences.VaultEmbeddingsOfferDecisions == nil {
			preferences.VaultEmbeddingsOfferDecisions = make(map[string]string)
		}
		preferences.VaultEmbeddingsOfferDecisions[path] = vaultEmbeddingOfferNever
		if err := preferences.write(); err != nil {
			app.problem = fmt.Sprintf("Could not save Vault index offer preference. %v", err)
			return
		}
		app.preferences.VaultEmbeddingsOfferDecisions = preferences.VaultEmbeddingsOfferDecisions
	} else {
		deferred := app.vaultEmbeddingOfferDeferred()
		if *deferred == nil {
			*deferred = make(map[string]bool)
		}
		(*deferred)[path] = true
	}
	for _, window := range app.documentWindows() {
		if window.vaultEmbeddingOfferPath == path && window.win != nil && !window.win.IsDestroyed() {
			window.win.Invalidate()
		}
	}
}
