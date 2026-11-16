//go:build !darwin

package ui

import (
	"context"

	"cyberwriter/internal/core/vaultcontext"
)

func buildContextualVaultIndex(context.Context, aiVaultBuildSnapshot) (*vaultcontext.Index, []vaultcontext.MapSkippedFile, error) {
	return nil, nil, errVaultContextualUnsupported
}

func searchContextualVaultIndex(context.Context, *vaultcontext.Index, string) ([]vaultcontext.Match, error) {
	return nil, errVaultContextualUnsupported
}

func requestVaultContextualAssets(context.Context) error {
	return errVaultContextualUnsupported
}
