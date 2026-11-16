package ui

import (
	"strings"

	"cyberwriter/internal/core/aiworkspace"
	mygoui "github.com/egoist/mygo/ui"
)

type workspaceAIConnectionForm struct {
	Draft               aiworkspace.Config
	Models              []string
	ProfileID           string
	RevealKey           bool
	Refreshing, Testing bool
	Status              string
	StatusError         bool
}

// EV-UI-226: the AI workspace settings action selects this page in Settings.
func (app *workspaceWindow) aiSettings(c *mygoui.Context) {
	mygoui.Column(c).FillWidth().Gap(18).Children(func() {
		app.drawAISettingsContent(c)
	})
}

func (app *workspaceWindow) drawAISettingsContent(c *mygoui.Context) {
	tr := app.tr
	app.drawAILocalServiceSettings(c)
	app.drawAICloudServiceSettings(c)
	app.drawAISettingsSection(c, tr("Connection", "连接"), "link", func() {
		app.drawAIProfileControls(c)
		app.aiConnectionForm.draw(c, app.interfaceLanguage(), app.refreshAIModels, app.testAIConnection, app.saveAIConnection)
	})
	app.drawAISystemPromptSettings(c)
	app.drawAIDocumentContextSettings(c)
	app.drawAIVaultContextSettings(c)
	app.drawAISettingsSection(c, tr("Panel", "面板"), "sidebar.right", func() {
		label := tr("Panel Position", "面板位置")
		settingsRow(c, label, tr("Which side of the editor the AI panel appears", "选择 AI 面板在编辑器哪一侧显示"), "sidebar.left", 40, func() {
			selected := 0
			if app.preferences.AIPanelRight {
				selected = 1
			}
			app.settingsSegments(c, label, []string{tr("Left", "左侧"), tr("Right", "右侧")}, 114, selected, func(i int) {
				app.setAIPanelRight(i == 1)
			})
		})
	})
}

func (app *workspaceWindow) setAIPanelRight(right bool) {
	if app.preferences.AIPanelRight != right {
		app.preferences.AIPanelRight = right
		app.preferencesChanged()
	}
}

func (form *workspaceAIConnectionForm) draw(c *mygoui.Context, language workspaceLanguage, refresh, test, save func()) {
	tr := language.text
	connection := mygoui.Column(c.Key("AI connection")).FillWidth().Gap(12)
	baseURL := tr("Base URL", "服务地址")
	model := tr("Model", "模型")
	apiKey := tr("API Key (optional)", "API 密钥（可选）")
	connection.Children(func() {
		aiConnectionField(c, baseURL, func() {
			aiConnectionInput(c, &form.Draft.BaseURL, baseURL, "http://localhost:1234/v1").FillWidth()
		})
		aiConnectionField(c, model, func() {
			mygoui.Row(c).FillWidth().AlignItems(mygoui.Center).Gap(6).Children(func() {
				if len(form.Models) == 0 {
					aiConnectionInput(c, &form.Draft.Model, model, "model-name").Grow(1)
				} else {
					mygoui.Select(c.Key("pick AI model"), &form.Draft.Model, form.Models).
						Label(model).Size(187, 24).MinWidth(0)
				}
				mygoui.Button(c.Key("refresh AI models"), "").Size(31.5, 20).Padding(0).KeepFocus().
					Label(tr("Refresh", "刷新模型")).Tooltip(tr("Fetch available models", "获取可用模型")).
					Busy(form.Refreshing).Disabled(form.Refreshing || refresh == nil).OnClick(refresh).
					Children(func() {
						if form.Refreshing {
							mygoui.Spinner(c).Size(16, 16).Role(mygoui.RoleNone)
						} else {
							aiWorkspaceIcon(c, "arrow.clockwise", "arrow.counterclockwise", 11, c.Theme().Text)
						}
					})
			})
		})
		aiConnectionField(c, apiKey, func() {
			mygoui.Row(c).FillWidth().AlignItems(mygoui.Center).Gap(6).Children(func() {
				key := aiConnectionInput(c, &form.Draft.APIKey, apiKey, "sk-...").Grow(1).MinWidth(0)
				label, symbol := tr("Show", "显示密钥"), "eye"
				if form.RevealKey {
					label, symbol = tr("Hide", "隐藏密钥"), "eye.slash"
				} else {
					key.Password()
				}
				chromeControl(c.Key("show API key"), label, false).Size(17, 20).KeepFocus().
					OnClick(func() { form.RevealKey = !form.RevealKey }).Children(func() {
					aiWorkspaceIcon(c, symbol, "", 14, c.Theme().TextMuted)
				})
			})
		})
		mygoui.Row(c).FillWidth().AlignItems(mygoui.Center).Gap(8).Margin(15, 0, 0).Children(func() {
			label := tr("Test", "测试")
			mygoui.Button(c.Key("test AI connection"), "").Size(56.5, 20).Padding(0, 5).Gap(4).
				Label(label).KeepFocus().Busy(form.Testing).Disabled(form.Testing || test == nil).OnClick(test).
				Children(func() {
					mygoui.Box(c).Size(12, 12).Center().Shrink(0).Role(mygoui.RoleNone).Children(func() {
						if form.Testing {
							mygoui.Spinner(c).Size(12, 12).Role(mygoui.RoleNone)
						} else {
							aiWorkspaceIcon(c, "bolt", "forward.end.fill", 10, c.Theme().Text)
						}
					})
					mygoui.Text(c, label).FontSize(11).Role(mygoui.RoleNone)
				})
			mygoui.Button(c, "cURL").Size(66, 20).Padding(0, 5).FontSize(11).OnClick(func() {
				command, err := aiworkspace.ConnectionCommand(form.Draft)
				if err != nil {
					form.Status, form.StatusError = err.Error(), true
					return
				}
				c.WriteClipboard(command)
				form.Status, form.StatusError = "cURL copied to clipboard", false
			})
			mygoui.Spacer(c)
			mygoui.Button(c, tr("Save", "保存")).Size(45.5, 20).Padding(0, 5).FontSize(11).
				Disabled(save == nil).OnClick(save)
		})
		form.drawFeedback(c, language)
	})
}

// EV-UI-288. The feedback keeps its place during a connection test.
func (form *workspaceAIConnectionForm) drawFeedback(c *mygoui.Context, language workspaceLanguage) {
	if form.Status == "" {
		return
	}
	theme := c.Theme()
	role, ink := mygoui.RoleStatus, theme.TextMuted
	var symbol, fallback string
	switch {
	case form.StatusError:
		role, ink = mygoui.RoleAlert, theme.Danger
		symbol, fallback = "xmark.circle.fill", "xmark.circle.fill"
	case form.Status == "Connected" || strings.HasPrefix(form.Status, "Connected - ") ||
		form.Status == "Settings saved" || form.Status == "cURL copied to clipboard":
		ink = theme.Success
		symbol, fallback = "checkmark.circle.fill", "circle"
	}
	mygoui.Row(c.Key("AI connection feedback")).FillWidth().MinWidth(0).Gap(8).
		AlignItems(mygoui.Start).Children(func() {
		if symbol != "" {
			mygoui.Box(c).Size(12, 14).Center().Shrink(0).Role(mygoui.RoleNone).Children(func() {
				aiWorkspaceIcon(c, symbol, fallback, 12, ink)
			})
		}
		mygoui.Text(c, aiConnectionStatus(language, form.Status)).Grow(1).MinWidth(0).
			FontSize(11).FixedLineHeight(14).Wrap().Role(role).TextColor(ink)
	})
}

func aiConnectionStatus(language workspaceLanguage, status string) string {
	if model, ok := strings.CutPrefix(status, "Connected - "); ok {
		return language.text(status, "已连接 - "+model)
	}
	switch status {
	case "Connected":
		return language.text(status, "已连接")
	case "Settings saved":
		return language.text(status, "设置已保存")
	case "cURL copied to clipboard":
		return language.text(status, "cURL 已复制到剪贴板")
	case "Could not save the API key in the system credential store.":
		return language.text(status, "无法将 API 密钥保存到系统凭据存储。")
	case "Could not save the AI connection.":
		return language.text(status, "无法保存 AI 连接。")
	default:
		return status
	}
}

func aiConnectionField(c *mygoui.Context, label string, body func()) {
	mygoui.Column(c.Key(label)).FillWidth().Gap(4).Children(func() {
		mygoui.Text(c, label).FontSize(12).FontWeight(600).FixedLineHeight(14)
		body()
	})
}

func aiConnectionInput(c *mygoui.Context, value *string, label, placeholder string) mygoui.Element {
	theme := c.Theme()
	input := mygoui.TextInputBase(c.Key(label), value).Label(label).Placeholder(placeholder).Height(25).
		MinWidth(0).Padding(0, 6).FontSize(13).Radius(4).Background(theme.Background).Border(1, theme.Border)
	if input.Focused() {
		input.BorderColor(theme.Accent)
	}
	return input
}

func (app *workspaceWindow) drawAISettingsSection(c *mygoui.Context, title, icon string, body func()) {
	theme := c.Theme()
	badge, border := theme.SurfacePressed, theme.Border
	if app.blackout && !c.Preferences().HighContrast {
		badge, border = mygoui.Hex("#1d1d20"), mygoui.Hex("#2d2d31")
	}
	mygoui.Column(c.Key(title)).FillWidth().Gap(12).Children(func() {
		mygoui.Row(c).Height(26).Padding(0, 2).AlignItems(mygoui.Center).Gap(8).Children(func() {
			mygoui.Box(c).Size(26, 26).Radius(6).Background(badge).Border(1, border).Center().Children(func() {
				aiWorkspaceIcon(c, icon, "gearshape", 14, theme.TextMuted)
			})
			mygoui.Text(c, title).Role(mygoui.RoleHeading).Level(2).FontSize(14).FontWeight(650)
		})
		card := app.settingsCard(c).Padding(12, 16)
		if app.blackout && !c.Preferences().HighContrast {
			card.LinearGradient(mygoui.LinearGradient{
				From: mygoui.Hex("#232326"), To: mygoui.Hex("#17171a"), Angle: 180, End: 0.5,
			})
		}
		card.Children(body)
	})
}
