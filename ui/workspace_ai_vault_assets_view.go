package ui

import (
	"log"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

const vaultEmbeddingSettingsURL = "x-apple.systempreferences:com.apple.preferences.AppleIntelligence"

// EV-UI-299. A download changes the status area while Settings keeps its
// regular depth and index controls.
func (app *workspaceWindow) drawVaultEmbeddingAssetsSettings(c *mygoui.Context, state *workspaceAIVaultContext) bool {
	if state == nil || state.request != nil && state.request.kind != aiVaultDownload {
		return false
	}
	downloading := state.request != nil
	if !downloading && !state.assetsMissing {
		return false
	}
	theme, tr := c.Theme(), app.tr
	label := tr("Apple's embedding model isn't downloaded yet (~50 MB).", "尚未下载 Apple 的嵌入模型（约 50 MB）。")
	failed := !state.assetFailure.IsZero() && state.problem != ""
	symbol, fallback, ink, role := "icloud.and.arrow.down", "arrow.down.to.line", theme.Accent, mygoui.RoleStatus
	if downloading {
		label = tr("Downloading embedding model…", "正在下载嵌入模型…")
	} else if failed {
		label, ink, role = state.problem, theme.Danger, mygoui.RoleAlert
		symbol, fallback = "exclamationmark.triangle.fill", "exclamationmark.triangle.fill"
	}
	mygoui.Column(c.Key("Vault embedding assets status")).FillWidth().MinWidth(0).Gap(8).Children(func() {
		mygoui.Row(c).FillWidth().MinWidth(0).Gap(8).AlignItems(mygoui.Center).Children(func() {
			aiWorkspaceIcon(c, symbol, fallback, 12, ink)
			mygoui.Text(c, label).Grow(1).MinWidth(0).FontSize(12).FixedLineHeight(15).
				TextColor(ink).Role(role)
		})
		if downloading {
			return
		}
		mygoui.Row(c).FillWidth().MinWidth(0).Gap(8).AlignItems(mygoui.Center).Wrap().Children(func() {
			caption := tr("Download Model", "下载模型")
			mygoui.ButtonBase(c.Key("download embedding model")).Padding(6, 14).Radius(6).KeepFocus().
				Background(theme.Accent).Label(caption).OnClick(app.downloadVaultEmbeddingModel).Children(func() {
				mygoui.Text(c, caption).FontSize(12).FontWeight(600).TextColor(theme.AccentText).Role(mygoui.RoleNone)
			})
			if failed {
				aiWorkspaceLink(c, tr("Open System Settings", "打开系统设置"), theme.Accent, openVaultEmbeddingSystemSettings)
			}
		})
	})
	return true
}

// EV-UI-299. This is the action area inside the existing AI Vault guide card.
func (app *workspaceWindow) drawVaultEmbeddingAssetsCard(c *mygoui.Context, colors workspaceAIColors, state *workspaceAIVaultContext) bool {
	if state == nil || state.request != nil && state.request.kind != aiVaultDownload {
		return false
	}
	if state.request != nil {
		mygoui.Row(c.Key("downloading Vault model")).FillWidth().MinWidth(0).Gap(6).AlignItems(mygoui.Center).Children(func() {
			mygoui.Spinner(c).Size(12, 12).Shrink(0).Role(mygoui.RoleNone)
			mygoui.Text(c, "Downloading model…").FontSize(11).TextColor(colors.muted).Role(mygoui.RoleStatus)
		})
		return true
	}
	if !state.assetsMissing {
		return false
	}
	if !state.assetFailure.IsZero() && state.problem != "" {
		mygoui.Text(c, state.problem).MinWidth(0).FontSize(11).TextColor(colors.warningInk).Role(mygoui.RoleAlert)
		aiWorkspaceLink(c, "Open System Settings", colors.accent, openVaultEmbeddingSystemSettings)
	}
	const caption = "Download Apple's Embedding Model (~50 MB)"
	mygoui.ButtonBase(c.Key("download Vault model")).FillWidth().MinWidth(0).Padding(7, 12).Gap(6).Center().
		Radius(6).Background(colors.accent).Label(caption).OnClick(app.downloadVaultEmbeddingModel).Children(func() {
		aiWorkspaceIcon(c, "icloud.and.arrow.down", "arrow.down.to.line", 11, colors.accentText)
		mygoui.Text(c, caption).MinWidth(0).Shrink(1).FontSize(11).FontWeight(600).
			TextColor(colors.accentText).Role(mygoui.RoleNone)
	})
	return true
}

func openVaultEmbeddingSystemSettings() {
	go func() {
		if err := mygo.Shell.OpenExternal(vaultEmbeddingSettingsURL); err != nil {
			log.Printf("Open Apple Intelligence settings: %v", err)
		}
	}()
}
