package ui

import (
	"slices"
	"strconv"

	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-278. The attachment counter measures characters. Token counts are
// the original preset descriptions shown in Settings.
type aiContextLimitPreset struct {
	id, name, translatedName, detail, translatedDetail string
	characters                                         int
}

var aiContextLimitPresets = [...]aiContextLimitPreset{
	{"small", "Small (3K)", "小（3K）", "~750 tokens", "约 750 个 token", 3_000},
	{"medium", "Medium (10K)", "中（10K）", "~2,500 tokens", "约 2,500 个 token", 10_000},
	{"large", "Large (20K)", "大（20K）", "~5,000 tokens", "约 5,000 个 token", 20_000},
	{"extraLarge", "Extra Large (50K)", "超大（50K）", "~12,500 tokens", "约 12,500 个 token", 50_000},
	{"unlimited", "Unlimited", "不限量", "Full document content", "完整文档内容", 0},
	{"custom", "Custom", "自定义", "Set your own limit", "设置字符上限", 0},
}

func (p *workspacePreferences) aiContextPreset() aiContextLimitPreset {
	for _, preset := range aiContextLimitPresets {
		if preset.id == p.AIContextLimitPreset {
			return preset
		}
	}
	return aiContextLimitPresets[2]
}

func (p *workspacePreferences) customContextLimit() int {
	if p.AICustomContextLimit > 0 {
		return p.AICustomContextLimit
	}
	return 10_000
}

func (p *workspacePreferences) documentContextLimit() int {
	preset := p.aiContextPreset()
	if preset.id == "custom" {
		return p.customContextLimit()
	}
	return preset.characters
}

const (
	minAIContextMessages     = 10
	maxAIContextMessages     = 200
	defaultAIContextMessages = 50
	aiContextMessageStep     = 10
)

func (p *workspacePreferences) contextMessageLimit() int {
	if p.AIContextMessageLimit <= 0 {
		return defaultAIContextMessages
	}
	return min(max(p.AIContextMessageLimit, minAIContextMessages), maxAIContextMessages)
}

func (app *workspaceWindow) drawAIDocumentContextSettings(c *mygoui.Context) {
	tr := app.tr
	preferences := app.preferences
	preset := preferences.aiContextPreset()
	selected := tr(preset.name, preset.translatedName)
	var choices [len(aiContextLimitPresets)]string
	for i, option := range aiContextLimitPresets {
		choices[i] = tr(option.name, option.translatedName)
	}
	app.drawAISettingsSection(c, tr("Context", "上下文"), "doc.text", func() {
		label := tr("Document Context Limit", "文档上下文上限")
		// EV-UI-310 aligns both controls to the Context section heading.
		settingsRow(c, label, tr(preset.detail, preset.translatedDetail), "doc.text", 56, func() {
			mygoui.Select(c, &selected, choices[:]).Size(140, 24).Label(label).OnChange(func() {
				if i := slices.Index(choices[:], selected); i >= 0 {
					preferences.AIContextLimitPreset = aiContextLimitPresets[i].id
					app.preferencesChanged()
				}
			})
		})
		if preset.id == "custom" {
			row := mygoui.Row(c.Key("custom context limit")).AlignItems(mygoui.Center).Gap(8)
			value := mygoui.Local(row, "characters", func() string { return strconv.Itoa(preferences.customContextLimit()) })
			row.Children(func() {
				label := tr("Custom limit", "自定义上限")
				mygoui.Text(c, label).FontSize(12)
				aiConnectionInput(c, value, label, "").Width(80).OnChange(func() {
					if limit, err := strconv.Atoi(*value); err == nil && limit > 0 && limit != preferences.AICustomContextLimit {
						preferences.AICustomContextLimit = limit
						app.preferencesChanged()
					}
				})
				mygoui.Text(c, tr("chars", "字符")).FontSize(11).TextColor(c.Theme().TextMuted)
			})
		}
		mygoui.Divider(c).FillWidth().Margin(12, 0, 12)
		app.drawAIContextMessageLimit(c)
	})
}

// EV-UI-282. This setting limits messages in the outgoing request.
func (app *workspaceWindow) drawAIContextMessageLimit(c *mygoui.Context) {
	tr := app.tr
	label := tr("Chat History in Context", "上下文中的聊天历史")
	limit := app.preferences.contextMessageLimit()
	value, count := float64(limit), strconv.Itoa(limit)
	mygoui.Column(c.Key("AI context message limit")).FillWidth().Gap(6).Children(func() {
		mygoui.Row(c).FillWidth().AlignItems(mygoui.Center).Children(func() {
			mygoui.Text(c, label).FontSize(13)
			mygoui.Spacer(c)
			mygoui.Text(c, tr(count+" messages", count+" 条消息")).FontSize(12).TextColor(c.Theme().TextMuted)
		})
		mygoui.Slider(c, &value, minAIContextMessages, maxAIContextMessages).
			FillWidth().Height(16).Step(aiContextMessageStep).Label(label).OnChange(func() {
			if changed := int(value); changed != limit {
				app.preferences.AIContextMessageLimit = changed
				app.preferencesChanged()
			}
		})
		mygoui.Text(c, tr("Number of previous messages included with each request", "每次请求包含的历史消息数")).
			FontSize(11).TextColor(c.Theme().TextMuted)
	})
}
