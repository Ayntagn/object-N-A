package ui

import (
	"fmt"

	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

type aiCloudService struct {
	name, baseURL, symbol, fallback string
	detail, detailChinese           string
}

// EV-UI-284. Cloud presets appear in this sequence in the expanded list.
var aiCloudServices = [...]aiCloudService{
	{"OpenAI", "https://api.openai.com/v1", "sparkle", "sparkles",
		"The standard OpenAI-compatible API - key from platform.openai.com",
		"标准 OpenAI 兼容 API，密钥来自 platform.openai.com"},
	{"Claude (Anthropic)", "https://api.anthropic.com", "brain.head.profile", "network",
		"API key from console.anthropic.com - uses native Claude API",
		"使用 Claude 原生 API，密钥来自 console.anthropic.com"},
	{"OpenRouter", "https://openrouter.ai/api/v1", "globe", "globe",
		`Free tier: use model "openrouter/free" - Paid: "openrouter/auto"`,
		`免费使用 "openrouter/free" 模型，付费使用 "openrouter/auto"`},
	{"DeepSeek", "https://api.deepseek.com/v1", "water.waves", "water.waves",
		"OpenAI-compatible - key from platform.deepseek.com",
		"兼容 OpenAI，密钥来自 platform.deepseek.com"},
	{"Groq", "https://api.groq.com/openai/v1", "bolt.horizontal", "play.fill",
		"Fast inference, OpenAI-compatible - key from console.groq.com",
		"快速推理，兼容 OpenAI，密钥来自 console.groq.com"},
	{"Mistral", "https://api.mistral.ai/v1", "wind", "text.alignleft",
		"OpenAI-compatible - key from console.mistral.ai",
		"兼容 OpenAI，密钥来自 console.mistral.ai"},
	{"GLM (Zhipu)", "https://open.bigmodel.cn/api/paas/v4", "g.circle", "globe",
		"Zhipu GLM, OpenAI-compatible - key from open.bigmodel.cn",
		"智谱 GLM，兼容 OpenAI，密钥来自 open.bigmodel.cn"},
}

func (app *workspaceWindow) drawAICloudServiceSettings(c *mygoui.Context) {
	tr := app.tr
	muted := c.Theme().TextMuted
	if app.blackout && !c.Preferences().HighContrast {
		muted = mygoui.Hex("#c4c6d0")
	}
	app.drawAISettingsSection(c, tr("Cloud Services", "云端服务"), "globe", func() {
		content := mygoui.Column(c.Key("cloud service presets")).FillWidth().MinWidth(0).Gap(10)
		expanded := mygoui.Local(content, "expanded", func() bool { return false })
		content.Children(func() {
			mygoui.Row(c).FillWidth().Height(24).AlignItems(mygoui.Center).Children(func() {
				mygoui.Text(c, tr("Requires an API key", "需要 API 密钥")).FontSize(11).TextColor(muted)
				mygoui.Spacer(c)
				label := fmt.Sprintf(tr("Show %d", "显示 %d 项"), len(aiCloudServices))
				if *expanded {
					label = tr("Hide", "隐藏")
				}
				toggle := mygoui.ButtonBase(c.Key("show cloud services")).Height(14).Gap(3).
					Label(label).Expanded(*expanded).FocusRing(true).Cursor(mygoui.CursorPointer).
					OnClick(func() { *expanded = !*expanded })
				if !*expanded && app.interfaceLanguage() == languageEnglish {
					toggle.Width(50.5).Justify(mygoui.End)
				}
				toggle.Children(func() {
					mygoui.Text(c, label).FontSize(11).FixedLineHeight(14).SingleLine().Role(mygoui.RoleNone)
					chevron := sfsymbols.Symbol(c, "chevron.down").Size(9).Natural().
						Fallback(chromeIconFallbacks["chevron.down"]).Color(c.Theme().Text).Role(mygoui.RoleNone)
					if *expanded {
						chevron.Rotate(180)
					}
				})
			})
			if !*expanded {
				return
			}
			for _, service := range aiCloudServices {
				app.drawAICloudService(c, service, muted)
			}
			mygoui.Box(c).FillWidth().Height(1).Background(c.Theme().Border).Role(mygoui.RoleNone)
			mygoui.Text(c, tr(
				"Any OpenAI-compatible endpoint works - tap New under Connection and paste its Base URL, model, and key.",
				"支持兼容 OpenAI 的端点。点击「连接」中的「新建」，粘贴服务地址、模型和密钥。",
			)).FillWidth().FontSize(10).FixedLineHeight(13).TextColor(muted).Wrap()
		})
	})
}

func (app *workspaceWindow) drawAICloudService(c *mygoui.Context, service aiCloudService, muted mygoui.Color) {
	tr := app.tr
	active := app.preferences.AIConnection.BaseURL == service.baseURL
	mygoui.Column(c.Key(service.name)).FillWidth().MinWidth(0).Gap(4).Padding(4, 0).
		Role(mygoui.RoleGroup).Label(service.name).Children(func() {
		mygoui.Row(c).FillWidth().Gap(10).AlignItems(mygoui.Center).Children(func() {
			mygoui.Box(c).Size(20, 20).Shrink(0).Center().Role(mygoui.RoleNone).Children(func() {
				aiWorkspaceIcon(c, service.symbol, service.fallback, 14, muted)
			})
			mygoui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
				mygoui.Text(c, service.name).FontSize(12).FontWeight(500).FixedLineHeight(15).SingleLine()
				mygoui.Text(c, service.baseURL).FontSize(10).FixedLineHeight(13).TextColor(muted.Alpha(0.5)).SingleLine()
			})
			label := tr("Use", "使用")
			if active {
				label = tr("In Use", "使用中")
			}
			mygoui.Button(c.Key("use cloud provider"), "").Height(20).Padding(0, 8).Radius(5).
				Label(label).Disabled(active).OnClick(func() { app.useAICloudService(service.baseURL, service.name) }).
				Children(func() {
					mygoui.Text(c, label).FontSize(11).SingleLine().Role(mygoui.RoleNone)
				})
		})
		mygoui.Text(c, tr(service.detail, service.detailChinese)).FillWidth().FontSize(10).
			FixedLineHeight(13).Padding(0, 0, 0, 30).TextColor(muted).Wrap()
	})
}
