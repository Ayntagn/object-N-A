// spec: EV-UI-299. Vault Context model download states.
package ui

import (
	"testing"
	"time"

	mygoui "github.com/egoist/mygo/ui"
)

func TestVaultEmbeddingAssetActionsFollowSharedState(t *testing.T) {
	app := &workspaceWindow{preferences: &workspacePreferences{}}
	state := &workspaceAIVaultContext{assetsMissing: true, problem: "The build detected missing assets."}
	settings := mygoui.NewTester(func(c *mygoui.Context) {
		app.drawVaultEmbeddingAssetsSettings(c, state)
	}, 640, 160)
	chat := mygoui.NewTester(func(c *mygoui.Context) {
		app.drawVaultEmbeddingAssetsCard(c, aiWorkspaceColors(c, true), state)
	}, 270, 160)
	const settingsDownload = "Download Model"
	const chatDownload = "Download Apple's Embedding Model (~50 MB)"
	assertAction := func(view *mygoui.Tester, label string, visible bool) {
		t.Helper()
		node, found := view.AccessByRole(mygoui.RoleButton, label)
		if found != visible || found && node.Disabled {
			t.Fatalf("%q action visible=%t disabled=%t, want visible=%t", label, found, node.Disabled, visible)
		}
	}
	frame := func() { settings.Frame(); chat.Frame() }
	assertAction(settings, settingsDownload, true)
	assertAction(chat, chatDownload, true)
	assertAction(settings, "Open System Settings", false)

	state.request = &workspaceAIVaultBuild{kind: aiVaultDownload}
	frame()
	assertAction(settings, settingsDownload, false)
	assertAction(chat, chatDownload, false)
	if _, ok := settings.AccessByRole(mygoui.RoleStatus, "Downloading embedding model…"); !ok {
		t.Fatal("missing Settings download status")
	}
	if _, ok := chat.AccessByRole(mygoui.RoleStatus, "Downloading model…"); !ok {
		t.Fatal("missing Chat download status")
	}

	state.request = nil
	state.assetFailure = time.Unix(1, 0)
	state.problem = "Couldn't download embedding model. Open System Settings → Apple Intelligence and try again."
	frame()
	assertAction(settings, "Open System Settings", true)
	assertAction(settings, settingsDownload, true)
	assertAction(chat, chatDownload, true)

	state.assetsMissing, state.problem = false, ""
	frame()
	assertAction(settings, settingsDownload, false)
	assertAction(chat, chatDownload, false)
}
