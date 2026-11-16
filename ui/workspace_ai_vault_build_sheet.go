package ui

import (
	"cyberwriter/internal/core/vaultcontext"
	mygoui "github.com/egoist/mygo/ui"
)

type workspaceAIVaultBuildSheet struct {
	root  string
	depth vaultcontext.Depth
}

// EV-UI-291 / EV-UI-292. The panel opens a choice of depths before building.
// The AI Settings buttons can start a build directly with the saved depth.
func (app *workspaceWindow) openAIVaultBuildSheet() {
	state := app.aiVaultContextState()
	if app.modalOpen || state == nil || state.request != nil {
		return
	}
	app.aiVaultBuildSheet = &workspaceAIVaultBuildSheet{root: app.root, depth: app.aiVaultContextDepth()}
	app.modalOpen = true
	app.invalidateAIChat()
}

func (app *workspaceWindow) drawAIVaultBuildSheet(c *mygoui.Context) {
	sheet := app.aiVaultBuildSheet
	if sheet == nil {
		return
	}
	closeSheet := func() {
		app.aiVaultBuildSheet = nil
		app.modalOpen = false
	}
	if app.root != sheet.root || !app.modalOpen {
		closeSheet()
		return
	}
	title := app.tr("Build AI Index", "构建 AI 索引")
	mygoui.Modal(c.Key("Build AI Index"), &app.modalOpen, func() {
		mygoui.Text(c, title).Role(mygoui.RoleHeading).Bold()
		mygoui.Text(c, app.tr(
			"Index your vault to find related notes in AI chat. Choose how finely each note is split.",
			"为资料库建立索引，让 AI 对话可以检索相关笔记。请选择笔记的切分粒度。",
		)).Wrap()
		app.drawAIVaultContextDepthRows(c, sheet.depth, app.recommendedAIVaultContextDepth(), func(depth vaultcontext.Depth) {
			sheet.depth = depth
		})
		mygoui.Row(c).Gap(8).Justify(mygoui.End).Children(func() {
			mygoui.Button(c, app.tr("Cancel", "取消")).OnClick(closeSheet)
			mygoui.PrimaryButton(c, app.tr("Build Index", "构建索引")).OnClick(func() {
				closeSheet()
				app.setAIVaultContextDepth(sheet.depth, true)
			})
		})
	}).Label(title).Width(440)
	if !app.modalOpen {
		app.aiVaultBuildSheet = nil
	}
}
