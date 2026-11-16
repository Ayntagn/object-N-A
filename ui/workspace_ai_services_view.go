package ui

import (
	"fmt"
	"runtime"
	"slices"

	"cyberwriter/internal/core/aiworkspace"
	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-280. Online services add a Use button beside their model count.
func (app *workspaceWindow) drawAILocalServiceSettings(c *mygoui.Context) {
	tr := app.tr
	scanning := app.aiServiceRequest != nil
	savedBase := app.preferences.AIConnection.BaseURL
	if savedBase == "" {
		savedBase = defaultAIBaseURL
	}
	app.drawAISettingsSection(c, tr("Local AI Services", "本地 AI 服务"), "network", func() {
		mygoui.Row(c).Height(22).AlignItems(mygoui.Center).Children(func() {
			description := tr("Auto-detected services on this computer", "自动检测此电脑上的服务")
			if runtime.GOOS == "darwin" {
				description = tr("Auto-detected services on this Mac", "自动检测此 Mac 上的服务")
			}
			mygoui.Text(c, description).FontSize(11).TextColor(c.Theme().TextMuted)
			mygoui.Spacer(c)
			if scanning {
				mygoui.Spinner(c).Size(12, 12).Role(mygoui.RoleStatus).
					Label(tr("Scanning local AI services", "正在扫描本地 AI 服务"))
				return
			}
			label := tr("Rescan", "重新扫描")
			mygoui.ButtonBase(c.Key("rescan local services")).Size(38.5, 14).Center().Label(label).
				OnClick(app.scanAILocalServices).Children(func() {
				mygoui.Text(c, label).FontSize(11).Role(mygoui.RoleNone)
			})
		})
		if runtime.GOOS == "darwin" {
			drawAILocalServiceRow(c, "Apple Intelligence", tr("On-device language model", "设备端语言模型"), "apple.intelligence", func() {
				drawAILocalServiceStatus(c, tr("Unavailable", "不可用"), false)
			})
		}
		for _, service := range aiworkspace.LocalServices() {
			drawAILocalServiceRow(c, service.Name, service.Address, service.Icon, func() {
				index := slices.IndexFunc(app.aiServiceResults, func(result aiworkspace.LocalServiceResult) bool {
					return result.Service.BaseURL == service.BaseURL
				})
				if index < 0 && scanning {
					mygoui.Spinner(c).Size(12, 12).Role(mygoui.RoleStatus).
						Label(fmt.Sprintf(tr("Scanning %s", "正在扫描 %s"), service.Name))
					return
				}
				if index < 0 || !app.aiServiceResults[index].Online {
					drawAILocalServiceStatus(c, tr("Offline", "离线"), false)
					return
				}
				result := app.aiServiceResults[index]
				count := len(result.Models)
				modelCount := tr("%d models", "%d 个模型")
				if count == 1 {
					modelCount = tr("%d model", "%d 个模型")
				}
				drawAILocalServiceStatus(c, fmt.Sprintf(modelCount, count), true)
				inUse := savedBase == service.BaseURL
				label := tr("Use", "使用")
				if inUse {
					label = tr("In Use", "使用中")
				}
				mygoui.Button(c.Key("use local service"), label).Height(20).Padding(0, 8).
					FontSize(11).Disabled(inUse).OnClick(func() { app.useAILocalService(result) })
			})
		}
	})
}

func drawAILocalServiceRow(c *mygoui.Context, title, address, icon string, actions func()) {
	muted := c.Theme().TextMuted
	mygoui.Row(c.Key(title)).FillWidth().Height(47).Gap(10).AlignItems(mygoui.Center).
		Role(mygoui.RoleGroup).Label(title).Children(func() {
		mygoui.Box(c).Size(20, 20).Shrink(0).Center().Children(func() {
			aiWorkspaceIcon(c, icon, "desktopcomputer", 20, muted)
		})
		mygoui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
			mygoui.Text(c, title).FontSize(13).FontWeight(600).FixedLineHeight(15).SingleLine()
			if address != "" {
				mygoui.Text(c, address).FontSize(11).FixedLineHeight(13).TextColor(muted).SingleLine()
			}
		})
		actions()
	})
}

func drawAILocalServiceStatus(c *mygoui.Context, label string, online bool) {
	ink := c.Theme().TextMuted.Alpha(0.3)
	if online {
		ink = c.Theme().Success
	}
	mygoui.Row(c).Gap(4).AlignItems(mygoui.Center).Children(func() {
		mygoui.Box(c).Size(6, 6).Radius(3).Background(ink)
		mygoui.Text(c, label).FontSize(10).FixedLineHeight(13).TextColor(c.Theme().TextMuted).SingleLine()
	})
}
