package ui

import (
	"runtime"

	"cyberwriter/internal/core/aiworkspace"
	mygoui "github.com/egoist/mygo/ui"
)

type workspaceAIConnectionForm struct {
	Draft               aiworkspace.Config
	Models              []string
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
	app.drawAISettingsSection(c, tr("Local AI Services", "本地 AI 服务"), "network", func() {
		mygoui.Row(c).Height(22).AlignItems(mygoui.Center).Children(func() {
			mygoui.Text(c, tr("Auto-detected services on this computer", "自动检测此电脑上的服务")).FontSize(12).TextColor(c.Theme().TextMuted)
			mygoui.Spacer(c)
			mygoui.ButtonBase(c).Height(18).Label(tr("Rescan", "重新扫描")).Disabled(true).Children(func() {
				mygoui.Text(c, tr("Rescan", "重新扫描")).FontSize(11).TextColor(c.Theme().TextMuted)
			})
		})
		if runtime.GOOS == "darwin" {
			drawAIServiceRow(c, "Apple Intelligence", "", "sparkles", tr("Unavailable", "不可用"))
		}
		for _, service := range [...]struct{ name, address, icon string }{
			{"Ollama", "localhost:11434", "gauge.with.dots.needle.67percent"},
			{"LM Studio", "localhost:1234", "desktopcomputer"},
			{"MLX (mlx-lm)", "localhost:8080", "apple.logo"},
			{"vLLM", "localhost:8000", "bolt"},
		} {
			drawAIServiceRow(c, service.name, service.address, service.icon, tr("Not connected", "未连接"))
		}
	})
	app.drawAISettingsSection(c, tr("Cloud Services", "云端服务"), "globe", func() {
		mygoui.Row(c).AlignItems(mygoui.Center).Children(func() {
			mygoui.Text(c, tr("Requires an API key", "需要 API 密钥")).FontSize(12).TextColor(c.Theme().TextMuted)
			mygoui.Spacer(c)
			mygoui.Button(c, tr("Show 7", "显示 7 项")).Disabled(true)
		})
	})
	app.drawAISettingsSection(c, tr("Connection", "连接"), "link", func() {
		app.aiConnectionForm.draw(c, app.interfaceLanguage(), app.refreshAIModels, app.testAIConnection, app.saveAIConnection)
	})
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
	connection := mygoui.Column(c.Key("AI connection")).FillWidth().Gap(14)
	revealKey := mygoui.Local(connection, "show API key", func() bool { return false })
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
				chromeControl(c.Key("refresh AI models"), tr("Refresh", "刷新模型"), form.Refreshing || refresh == nil).
					Size(31.5, 20).KeepFocus().OnClick(refresh).Children(func() {
					aiWorkspaceIcon(c, "arrow.clockwise", "arrow.counterclockwise", 14, c.Theme().TextMuted)
				})
			})
		})
		aiConnectionField(c, apiKey, func() {
			mygoui.Row(c).FillWidth().AlignItems(mygoui.Center).Gap(6).Children(func() {
				key := aiConnectionInput(c, &form.Draft.APIKey, apiKey, "sk-...").Grow(1).MinWidth(0)
				label, symbol := tr("Show", "显示密钥"), "eye"
				if *revealKey {
					label, symbol = tr("Hide", "隐藏密钥"), "eye.slash"
				} else {
					key.Password()
				}
				chromeControl(c.Key("show API key"), label, false).Size(17, 20).KeepFocus().
					OnClick(func() { *revealKey = !*revealKey }).Children(func() {
					aiWorkspaceIcon(c, symbol, "", 14, c.Theme().TextMuted)
				})
			})
		})
		mygoui.Row(c).FillWidth().AlignItems(mygoui.Center).Gap(8).Margin(15, 0, 0).Children(func() {
			mygoui.Button(c, tr("Test", "测试")).Size(56.5, 20).Disabled(form.Testing || test == nil).OnClick(test)
			mygoui.Button(c, "cURL").Size(66, 20).Disabled(true)
			mygoui.Spacer(c)
			mygoui.Button(c, tr("Save", "保存")).Size(45.5, 20).Disabled(save == nil).OnClick(save)
		})
		if form.Status != "" {
			role, color := mygoui.RoleStatus, c.Theme().TextMuted
			if form.StatusError {
				role, color = mygoui.RoleAlert, c.Theme().Danger
			}
			mygoui.Text(c, aiConnectionStatus(language, form.Status)).FillWidth().FontSize(12).Wrap().Role(role).TextColor(color)
		}
	})
}

func aiConnectionStatus(language workspaceLanguage, status string) string {
	switch status {
	case "Connected":
		return language.text(status, "已连接")
	case "Settings saved":
		return language.text(status, "设置已保存")
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
	input := mygoui.TextInputBase(c.Key(label), value).Label(label).Placeholder(placeholder).Height(23).
		MinWidth(0).Padding(0, 6).FontSize(13).Radius(4).Background(theme.Background).Border(1, theme.Border)
	if input.Focused() {
		input.BorderColor(theme.Accent)
	}
	return input
}

func (app *workspaceWindow) drawAISettingsSection(c *mygoui.Context, title, icon string, body func()) {
	theme := c.Theme()
	fill, badge, border := theme.Surface, theme.SurfacePressed, theme.Border
	if app.blackout && !c.Preferences().HighContrast {
		fill, badge, border = mygoui.Hex("#18191d"), mygoui.Hex("#212226"), mygoui.Hex("#28292d")
	}
	mygoui.Column(c.Key(title)).FillWidth().Gap(12).Children(func() {
		mygoui.Row(c).Height(26).Padding(0, 2).AlignItems(mygoui.Center).Gap(8).Children(func() {
			mygoui.Box(c).Size(26, 26).Radius(6).Background(badge).Center().Children(func() {
				aiWorkspaceIcon(c, icon, "gearshape", 14, theme.TextMuted)
			})
			mygoui.Text(c, title).Role(mygoui.RoleHeading).Level(2).FontSize(14).FontWeight(650)
		})
		mygoui.Column(c).FillWidth().Padding(12, 16).Radius(8).Background(fill).
			Border(1, border).Children(body)
	})
}

func drawAIServiceRow(c *mygoui.Context, title, address, icon, status string) {
	theme := c.Theme()
	mygoui.Row(c.Key(title)).FillWidth().Height(47).Gap(12).
		AlignItems(mygoui.Center).Children(func() {
		aiWorkspaceIcon(c, icon, "desktopcomputer", 16, theme.TextMuted)
		mygoui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
			mygoui.Text(c, title).FontSize(13).FontWeight(600).SingleLine()
			if address != "" {
				mygoui.Text(c, address).FontSize(11).TextColor(theme.TextMuted)
			}
		})
		mygoui.Box(c).Size(6, 6).Radius(3).Background(theme.TextMuted)
		mygoui.Text(c, status).FontSize(11).TextColor(theme.TextMuted).SingleLine()
	})
}
