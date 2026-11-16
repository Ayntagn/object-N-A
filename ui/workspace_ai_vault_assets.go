package ui

import (
	"context"
	"errors"
	"time"

	"github.com/egoist/mygo"
)

var (
	errVaultContextualAssetsMissing = errors.New("download Apple's embedding model to build this depth")
	errVaultContextualUnsupported   = errors.New("contextual embeddings require a supported macOS version. Choose Quick to use the cross-platform local index")
)

// EV-UI-299. Asset preparation is a separate user action. Success makes
// Build Index available again without starting a build.
func (app *workspaceWindow) downloadVaultEmbeddingModel() {
	state := app.aiVaultContextState()
	if state == nil || !state.assetsMissing || state.request != nil || app.modalOpen || app.win == nil || app.win.IsDestroyed() {
		return
	}
	root := state.root
	ctx, cancel := context.WithCancel(context.Background())
	request := &workspaceAIVaultBuild{cancel: cancel, kind: aiVaultDownload}
	state.request, state.problem = request, ""
	app.invalidateAIVaultContext(root)
	go func() {
		defer cancel()
		err := requestVaultContextualAssets(ctx)
		if ctx.Err() != nil {
			return
		}
		mygo.RunOnMain(func() {
			if state.request != request {
				return
			}
			state.request = nil
			state.assetsMissing = err != nil
			if err != nil {
				state.assetFailure = time.Now()
				state.problem = err.Error()
			} else {
				state.assetFailure = time.Time{}
			}
			app.invalidateAIVaultContext(root)
		})
	}()
}
