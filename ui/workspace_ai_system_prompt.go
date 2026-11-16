package ui

import (
	mygoui "github.com/egoist/mygo/ui"
	"github.com/go-text/typesetting/segmenter"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var aiSystemPromptCountPrinter = message.NewPrinter(language.English)

func (p *workspacePreferences) aiSystemPrompt() string {
	if p.AISystemPrompt != nil {
		return *p.AISystemPrompt
	}
	return workspaceAIWritingInstructions
}

// EV-UI-285. A missing preference uses the default. An explicitly empty
// prompt remains empty in both Settings and the next outgoing request.
func (app *workspaceWindow) drawAISystemPromptSettings(c *mygoui.Context) {
	tr := app.tr
	title := tr("System Prompt", "系统提示词")
	value := app.preferences.aiSystemPrompt()
	var segments segmenter.Segmenter
	segments.InitWithString(value)
	characters := 0
	for graphemes := segments.GraphemeIterator(); graphemes.Next(); {
		characters++
	}
	count := aiSystemPromptCountPrinter.Sprintf(tr("%d chars", "%d 个字符"), characters)
	save := func(prompt string) {
		app.preferences.AISystemPrompt = &prompt
		app.preferencesChanged()
	}
	app.drawAISettingsSection(c, title, "text.bubble", func() {
		content := mygoui.Column(c.Key("system prompt editor")).FillWidth().Gap(8)
		expanded := mygoui.Local(content, "expanded", func() bool { return false })
		content.Children(func() {
			theme := c.Theme()
			mygoui.Row(c).FillWidth().Gap(8).AlignItems(mygoui.Center).Children(func() {
				mygoui.Text(c, tr("Instructions sent with every AI request", "与每次 AI 请求一同发送的指令")).
					FontSize(12).Grow(1).MinWidth(0)
				mygoui.Text(c, count).FontSize(11).TextColor(theme.TextMuted).Shrink(0)
				symbol, label := "chevron.down", tr("Go Down", "展开系统提示词")
				if *expanded {
					symbol, label = "chevron.up", tr("Go Up", "收起系统提示词")
				}
				help := tr("Edit the system prompt", "编辑系统提示词")
				chromeControl(c.Key("edit system prompt"), label, false, help).Size(20, 20).Center().Shrink(0).
					Expanded(*expanded).OnClick(func() { *expanded = !*expanded }).Children(func() {
					aiWorkspaceIcon(c, symbol, symbol, 11, theme.TextMuted)
				})
			})
			if *expanded {
				mygoui.TextArea(c.Key("system prompt text"), &value).FillWidth().MinWidth(0).
					Height(200).MinHeight(120).Font("monospace").FontSize(12).Padding(8).
					Label(title).Background(theme.Background).Radius(6).Border(1, theme.Border).
					OnChange(func() { save(value) })
				mygoui.Button(c, tr("Reset to Default", "恢复默认提示词")).AlignSelf(mygoui.Start).
					OnClick(func() { save(workspaceAIWritingInstructions) })
			}
		})
	})
}
