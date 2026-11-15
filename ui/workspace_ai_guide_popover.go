package ui

import (
	"strings"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-256. NSPopover adds its system material and arrow to the 360 × 480 client.
const aiWorkspaceGuideWidth, aiWorkspaceGuideHeight = 360, 480

func (app *workspaceWindow) showAIWorkspaceGuide(anchor mygoui.Rect) {
	if app.win == nil || app.win.IsDestroyed() {
		return
	}
	s := &app.ai
	if s.guideFlyout != nil {
		s.guideFlyout.Close()
		return
	}
	flyout := mygo.NewFlyout(mygo.FlyoutOptions{
		Parent: app.win, Anchor: documentTitleAnchor(anchor), Placement: mygo.PlacementBottom,
		Width: aiWorkspaceGuideWidth, Height: aiWorkspaceGuideHeight,
		Popover: true, Focusable: true, Shadow: true,
		Content: mygoui.View(func(c *mygoui.Context) {
			app.applyAppearance(c)
			c.Root().Background(mygoui.Transparent)
			reader := drawAIWorkspaceGuidePopover(c, app.interfaceLanguage(), app.blackout)
			if !c.Vibrancy() {
				fill := c.Theme().SurfaceHover
				if app.blackout && !c.Preferences().HighContrast {
					fill = mygoui.Hex("#222224")
				}
				reader.Background(fill).Radius(16)
			}
		}),
	})
	if flyout == nil {
		return
	}
	s.guideFlyout = flyout
	flyout.OnClosed(func() {
		if s.guideFlyout == flyout {
			s.guideFlyout = nil
		}
	})
}

type aiWorkspaceGuideRow struct {
	symbol, fallback       string
	iconLabel, iconChinese string
	text, chinese          string
}

var aiWorkspaceGuideSections = [...]struct {
	title, chinese string
	rows           []aiWorkspaceGuideRow
}{
	{"GETTING STARTED", "开始使用", []aiWorkspaceGuideRow{
		{"link", "", "Link", "链接", "Connect a model in **Settings > AI** (local or cloud)", "在 **设置 > AI** 连接本地或云端模型"},
		{"text.cursor", "textformat", "Insert Text", "插入文字", "Type a request and press Enter or click send", "输入请求后按 Enter 或点击发送"},
		{"arrow.up", "textformat.size.larger", "Up", "向上", "When streaming is off, click **Insert into Editor** to add a reply to your document",
			"关闭流式写入后，点击 **Insert into Editor** 将回复加入文档"},
	}},
	{"CONTEXT", "上下文", []aiWorkspaceGuideRow{
		{"doc.viewfinder", "doc.text", "Document In A Viewfinder Frame", "附加文档",
			"Click the document icon to attach your current document or selection as context", "点击文档图标，将当前文档或选区附为上下文"},
		{"pencil", "pencil.and.outline", "Edit", "编辑", "Click the context badge to review the attached text before sending",
			"发送前可点击上下文标签，检查附加的文字"},
		{"note.text", "doc.text", "Notes", "备注", "Add **Notes** for recurring references (colors, file paths, style rules) across messages in this chat",
			"在 **Notes** 中记录本次对话反复使用的颜色、文件路径和样式规则"},
	}},
	{"STREAM INTO EDITOR", "流式写入编辑器", []aiWorkspaceGuideRow{
		{"text.cursor", "textformat", "Insert Text", "插入文字", "Toggle **Insert Text** on to stream replies directly into your document at the cursor",
			"开启 **Insert Text**，将回复逐段写入文档中的光标位置"},
		{"eye", "", "Show", "预览", "Watch the live preview update as the response streams in", "预览会随回复内容实时更新"},
		{"arrow.uturn.backward", "arrow.counterclockwise", "Undo", "撤销", "**⌘Z** undoes the entire streamed insertion at once",
			"按 **⌘Z** 可一次撤销整段流式写入"},
	}},
	{"WHAT THE AI CAN GENERATE", "AI 可生成的内容", []aiWorkspaceGuideRow{
		{"text.badge.checkmark", "doc.text", "Text With A Checkmark Badge", "Markdown 文本",
			"Standard Markdown - headings, lists, tables, links, images, footnotes", "标准 Markdown 内容，包括标题、列表、表格、链接、图片和脚注"},
		{"chevron.left.forwardslash.chevron.right", "", "Embed Code", "嵌入代码",
			"Inline HTML/CSS - styled <div>, <span>, colored text, layouts", "行内 HTML/CSS，包括设置了样式的 <div>、<span>、彩色文字和布局"},
		{"chart.bar", "", "Chart Column", "图表",
			"Mermaid diagrams - flowcharts, sequence, class, state, gantt, pie, ER, mindmap, timeline, git graph, and more",
			"Mermaid 图表，包括流程图、时序图、类图、状态图、甘特图、饼图、实体关系图、思维导图、时间线及 Git 图"},
		{"doc.text", "", "Plain Text Document", "文档", "Page breaks with [pagebreak]", "用 [pagebreak] 分页"},
	}},
	{"SUPPORTED PROVIDERS", "支持的服务", []aiWorkspaceGuideRow{
		{"hare", "forward.end.fill", "Increase Speed", "加速", "Ollama - local, port 11434", "Ollama，本地端口 11434"},
		{"desktopcomputer", "rectangle", "Computer", "电脑", "LM Studio - local, port 1234", "LM Studio，本地端口 1234"},
		{"apple.logo", "sparkles", "Apple Logo", "Apple 标志", "MLX - local, port 8080", "MLX，本地端口 8080"},
		{"bolt", "forward.end.fill", "Flash", "闪电", "vLLM - local, port 8000", "vLLM，本地端口 8000"},
		{"globe", "network", "Globe", "网络", "OpenRouter - compatible cloud endpoint", "OpenRouter，兼容的云端接口"},
		{"brain.head.profile", "sparkles", "Head Profile Showing The Brain", "模型", "Claude (Anthropic) via a compatible endpoint",
			"通过兼容接口使用 Claude（Anthropic）"},
		{"server.rack", "network", "Server Rack", "服务器", "Any OpenAI-compatible endpoint", "任何与 OpenAI 兼容的接口"},
	}},
	{"TIPS", "使用技巧", []aiWorkspaceGuideRow{
		{"lightbulb", "sun.max", "Light Bulb", "提示", "Specify an output format in your request, such as Markdown, a table, or a Mermaid diagram",
			"在请求中指定输出格式，例如 Markdown、表格或 Mermaid 图表"},
		{"gearshape", "", "Gear Shape", "设置", "Use **Context Notes** for reusable style rules, colors, and file references in this chat",
			"把本次对话需要反复使用的样式规则、颜色和文件引用写到 **Context Notes**"},
		{"arrow.clockwise", "arrow.counterclockwise", "Refresh", "刷新",
			"Click **Refresh** next to Model in Settings > AI to discover available models", "在设置的 AI 页点击模型旁的 **刷新**，查找可用模型"},
		{"terminal", "", "Terminal Command Prompt", "终端", "Use **Test** to check the AI connection", "用 **测试** 检查 AI 连接"},
	}},
}

func drawAIWorkspaceGuidePopover(c *mygoui.Context, language workspaceLanguage, blackout bool) mygoui.Element {
	colors := aiWorkspaceColors(c, blackout)
	muted := colors.muted
	if blackout && !c.Preferences().HighContrast {
		muted = mygoui.Hex("#c4c3cd")
	}
	return mygoui.Scroll(c.Key("AI workspace guide scroll")).Fill().MinHeight(0).MinWidth(0).
		Label(language.text("AI Workspace Guide", "AI 工作区指南")).Children(func() {
		mygoui.Column(c).FillWidth().Padding(16).Gap(16).Children(func() {
			mygoui.Row(c).Height(19).AlignItems(mygoui.Center).Gap(7).Children(func() {
				aiGuideIcon(c, "sparkles", "", language.text("Sparkle", "火花"), 16, colors.accent)
				mygoui.Text(c, language.text("AI Workspace Guide", "AI 工作区指南")).FontSize(15).Bold().FixedLineHeight(19).
					TextColor(colors.text).Role(mygoui.RoleHeading).Level(1)
			})
			for _, section := range aiWorkspaceGuideSections {
				mygoui.Column(c.Key(section.title)).FillWidth().Gap(8).Children(func() {
					mygoui.Text(c, language.text(section.title, section.chinese)).
						FontSize(11).FontWeight(650).LetterSpacing(0.65).FixedLineHeight(14).
						TextColor(muted).Role(mygoui.RoleHeading).Level(2)
					for _, row := range section.rows {
						mygoui.Row(c.Key(row.text)).FillWidth().Gap(8).AlignItems(mygoui.Start).Children(func() {
							mygoui.Box(c).Size(16, 15).Shrink(0).Children(func() {
								aiGuideIcon(c, row.symbol, row.fallback, language.text(row.iconLabel, row.iconChinese), 15, muted).
									Attach(mygoui.AnchorCenter, mygoui.AnchorCenter)
							})
							detail := commandPaletteShortcut(language.text(row.text, row.chinese))
							parts := strings.Split(detail, "**")
							spans := make([]mygoui.Span, len(parts))
							for index, text := range parts {
								spans[index].Text = text
								if index%2 == 1 {
									spans[index].Weight = 700
								}
							}
							mygoui.RichText(c, spans...).Grow(1).MinWidth(0).Wrap().FontSize(12).
								FixedLineHeight(15).TextColor(colors.text)
						})
					}
				})
			}
			mygoui.Text(c, language.text("Use the sparkle button to toggle this panel", "点击火花按钮打开或收起此面板")).
				FillWidth().TextAlign(mygoui.Center).
				FontSize(11).FixedLineHeight(13).TextColor(muted)
		})
	})
}
