package ui

import (
	"fmt"
	"strings"
	"time"

	"cyberwriter/internal/core/vaultcontext"
	mygoui "github.com/egoist/mygo/ui"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type aiVaultDepthConfirmation struct {
	open  bool
	root  string
	depth vaultcontext.Depth
}

var aiVaultContextDepthOptions = [...]struct {
	depth                                vaultcontext.Depth
	name, chinese, detail, detailChinese string
}{
	{vaultcontext.Quick, "Quick", "快速", "Files only - fastest, retrieves whole notes", "按文件索引，速度最快，检索整篇笔记"},
	{vaultcontext.Balanced, "Balanced", "均衡", "Sections - sweet spot for find-related-notes", "按章节索引，适合查找相关笔记"},
	{vaultcontext.Deep, "Deep", "深度", "Paragraphs - precise passage retrieval, slow", "按段落索引，检索更精确，构建耗时更长"},
}

// EV-UI-226, EV-UI-289, EV-UI-291. The switches and depth are workspace
// preferences; build and clear operate on the currently mounted Vault.
func (app *workspaceWindow) drawAIVaultContextSettings(c *mygoui.Context) {
	tr := app.tr
	state := app.aiVaultContextState()
	mounted := state != nil
	indexed := mounted && state.index != nil
	busy := mounted && state.request != nil
	selected, recommended := app.aiVaultContextDepth(), app.recommendedAIVaultContextDepth()
	section := mygoui.Column(c.Key("AI Vault Context")).FillWidth().MinWidth(0).Gap(9)
	confirm := mygoui.Local(section, "confirm Vault Context depth", func() aiVaultDepthConfirmation { return aiVaultDepthConfirmation{} })
	section.Children(func() {
		mygoui.Row(c).Height(16).Gap(6.5).AlignItems(mygoui.Center).Children(func() {
			mygoui.Box(c).Size(18, 16).Center().Role(mygoui.RoleNone).Children(func() {
				aiWorkspaceIcon(c, "rectangle.stack.badge.person.crop", "rectangle.on.rectangle", 16, c.Theme().TextMuted)
			})
			mygoui.Text(c, tr("Vault Context", "资料库上下文")).FontSize(13).FontWeight(650).
				FixedLineHeight(16).Role(mygoui.RoleHeading).Level(2)
		})
		card := app.settingsCard(c).Padding(11)
		dividerInk := c.Theme().Border
		if app.blackout && !c.Preferences().HighContrast {
			// EV-UI-312: the visible Blackout section uses a single dark card.
			card.Background(mygoui.Hex("#0a0a0b")).Border(1, mygoui.Hex("#171719"))
			dividerInk = mygoui.Hex("#0f0f10")
		}
		card.Children(func() {
			app.drawAIVaultContextSwitches(c)
			mygoui.Divider(c).Margin(10, 0, 14).Background(dividerInk)
			app.drawAIVaultContextDepthRows(c, selected, recommended, func(depth vaultcontext.Depth) {
				if depth == selected {
					return
				}
				if indexed {
					*confirm = aiVaultDepthConfirmation{open: true, root: app.root, depth: depth}
					return
				}
				app.setAIVaultContextDepth(depth, false)
			})
			mygoui.Divider(c).Margin(10, 0, 14).Background(dividerInk)
			app.drawAIVaultContextStatus(c, state)
			mygoui.Row(c).FillWidth().Height(24).Gap(8).AlignItems(mygoui.Center).Margin(12, 0, 0).Children(func() {
				aiVaultIndexButton(c, tr("Build Index", "构建索引"), 90.5, !mounted || busy || indexed, app.buildAIVaultContext)
				aiVaultIndexButton(c, tr("Rebuild", "重建"), 69.5, !indexed || busy, app.buildAIVaultContext)
				aiVaultIndexButton(c, tr("Clear", "清除"), 56, !indexed, app.clearAIVaultContext)
				mygoui.Spacer(c)
				aiVaultIndexButton(c, tr("View Vault Map", "查看资料库地图"), 117.5, !indexed || app.aiVaultMapBusy, app.showAIVaultMap)
			})
			if app.aiVaultMapProblem != "" {
				mygoui.Text(c, app.aiVaultMapProblem).FontSize(12).Wrap().TextColor(c.Theme().Danger).Role(mygoui.RoleAlert)
			}
		})
		title := tr("Rebuild AI index?", "重建 AI 索引？")
		message := tr(
			"Switching depth invalidates the current index. Rebuild now to use the new setting, or change later from this page.",
			"修改深度后，当前索引将不再匹配。立即重建以使用新档位，也可以稍后在此页重建。",
		)
		choice := mygoui.AlertDialog(c.Key("confirm AI Vault depth"), &confirm.open, title, message,
			tr("Change Later", "稍后重建"), tr("Rebuild Now", "立即重建"))
		switch choice {
		case 0, 1, mygoui.AlertDismissed:
			if confirm.root == app.root {
				app.setAIVaultContextDepth(confirm.depth, choice == 1)
			}
			*confirm = aiVaultDepthConfirmation{}
		}
	})
}

func (app *workspaceWindow) drawAIVaultContextSwitches(c *mygoui.Context) {
	tr := app.tr
	prefs := app.preferences
	auto := prefs.aiVaultContextAutoUpdate()
	mygoui.Column(c).FillWidth().MinWidth(0).Gap(10).Children(func() {
		for _, row := range [...]struct {
			label, detail string
			value         *bool
			changed       func()
		}{
			{tr("Use vault embeddings as AI context", "使用资料库索引作为 AI 上下文"),
				tr("Adds the most semantically relevant chunks to each AI query.", "每次 AI 查询都会加入语义最相关的片段。"),
				&prefs.AIVaultContextEnabled, app.preferencesChanged},
			{tr("Auto-update when files change", "文件变化时自动更新"),
				tr("Re-embeds modified or new files in the background. No effect until an index exists.", "在后台更新修改或新建的文件。构建索引后才会生效。"),
				&auto, func() {
					prefs.AIVaultContextAutoUpdate = &auto
					app.preferencesChanged()
					app.syncAIVaultContext()
				}},
			{tr("Send vault structure with each request", "每次请求都发送资料库文件清单"),
				tr("Adds a compact list of filenames so the AI knows what files exist. ~30–60 chars per file.", "附加精简的文件名清单，让 AI 知道有哪些文件。每个文件约 30–60 个字符。"),
				&prefs.AIVaultContextSendStructure, app.preferencesChanged},
		} {
			mygoui.Row(c.Key(row.label)).FillWidth().MinHeight(32).Gap(12).AlignItems(mygoui.Start).Children(func() {
				mygoui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					mygoui.Text(c, row.label).FontSize(13).FontWeight(600).FixedLineHeight(16).Wrap()
					mygoui.Text(c, row.detail).FontSize(11).FixedLineHeight(14).TextColor(c.Theme().TextMuted).Wrap()
				})
				// EV-UI-291: Vault Context shares the Settings toggle style.
				app.settingsToggle(c.Key("toggle"), row.label, row.value, row.changed)
			})
		}
		if prefs.AIVaultContextSendStructure {
			ink := c.Theme().TextMuted
			if !c.Preferences().HighContrast {
				// EV-UI-312: the privacy note uses the active app appearance's warning color.
				switch app.appAppearance() {
				case appearanceCyber, appearanceBlackout:
					ink = mygoui.Hex("#e8b878")
				case appearancePLATO:
					ink = mygoui.Hex("#ffae57")
				}
			}
			mygoui.Text(c, tr(
				"Privacy: this sends vault filenames to whichever AI provider is selected.",
				"隐私提示：资料库的文件名会发送给当前选中的 AI 服务提供商。",
			)).FontSize(10).PaddingStart(4).TextColor(ink).Wrap()
		}
	})
}

func (app *workspaceWindow) drawAIVaultContextDepthRows(c *mygoui.Context, selected, recommended vaultcontext.Depth, change func(vaultcontext.Depth)) {
	tr := app.tr
	theme := c.Theme()
	mygoui.Column(c).FillWidth().MinWidth(0).Gap(2).Children(func() {
		mygoui.Text(c, tr("Context Depth", "上下文深度")).FontSize(13).FontWeight(650).FixedLineHeight(16).Role(mygoui.RoleHeading).Level(3)
		mygoui.Text(c, tr(
			"How finely each note is split before embedding. Trade-off: deeper = more accurate retrieval, slower to build.",
			"控制索引时笔记的切分粒度。切分越细，检索越精确，构建越慢。",
		)).FontSize(11).FixedLineHeight(14).TextColor(theme.TextMuted).Wrap()
		mygoui.Column(c).FillWidth().MinWidth(0).Margin(6, 0, 0).Gap(6).Children(func() {
			for _, option := range aiVaultContextDepthOptions {
				name, detail := tr(option.name, option.chinese), tr(option.detail, option.detailChinese)
				isSelected, isRecommended := option.depth == selected, option.depth == recommended
				caption := []string{name}
				if isRecommended {
					caption = append(caption, tr("Recommended", "推荐"))
				}
				caption = append(caption, detail)
				button := mygoui.ButtonBase(c.Key(option.depth)).FillWidth().MinWidth(0).MinHeight(29).
					Padding(0, 7).Gap(8).Justify(mygoui.Start).AlignItems(mygoui.Center).Radius(5).
					Border(1, theme.Border).Label(strings.Join(caption, ", ")).AccessSelected(isSelected).
					FocusRing(true).OnClick(func() { change(option.depth) })
				if isSelected {
					button.Background(theme.Accent.Alpha(0.10)).Border(1, theme.Accent.Alpha(0.45))
				} else if button.Hovered() {
					button.Background(theme.SurfaceHover)
				}
				button.Children(func() {
					symbol, ink := "circle", theme.TextMuted
					if isSelected {
						symbol, ink = "largecircle.fill.circle", theme.Accent
					}
					mygoui.Box(c).Size(14, 14).Center().Shrink(0).Role(mygoui.RoleNone).Children(func() {
						aiWorkspaceIcon(c, symbol, "circle-dot", 12, ink)
					})
					mygoui.Column(c).Grow(1).MinWidth(0).Children(func() {
						mygoui.Row(c).Gap(6).AlignItems(mygoui.Center).Children(func() {
							mygoui.Text(c, name).FontSize(12).FontWeight(600).FixedLineHeight(13).Role(mygoui.RoleNone)
							if isRecommended {
								mygoui.Text(c, tr("Recommended", "推荐")).FontSize(9).FixedLineHeight(11).
									TextColor(theme.Accent).Role(mygoui.RoleNone)
							}
						})
						mygoui.Text(c, detail).FontSize(10).FixedLineHeight(13).TextColor(theme.TextMuted).Wrap().Role(mygoui.RoleNone)
					})
				})
			}
		})
	})
}

func (app *workspaceWindow) drawAIVaultContextStatus(c *mygoui.Context, state *workspaceAIVaultContext) {
	if app.drawVaultEmbeddingAssetsSettings(c, state) {
		return
	}
	if state != nil && state.request != nil && state.request.kind == aiVaultBuild {
		app.drawAIVaultBuildProgress(c, state.request.progress)
		return
	}
	tr := app.tr
	theme := c.Theme()
	symbol, fallback, ink := "circle", "circle", theme.TextMuted
	label, role := tr("No index built yet for this vault.", "此资料库尚未构建索引。"), mygoui.RoleStatus
	switch {
	case state == nil:
		label = tr("Open a vault to enable embeddings.", "打开资料库后即可构建索引。")
		symbol, fallback = "folder.badge.questionmark", "folder"
	case state.request != nil:
		label = tr("Indexing…", "正在构建索引…")
		if state.request.kind == aiVaultLoad {
			label = tr("Loading index…", "正在加载索引…")
		} else if state.request.kind == aiVaultPrune {
			label = tr("Updating index…", "正在更新索引…")
		}
		symbol, fallback = "arrow.triangle.2.circlepath", "arrow.counterclockwise"
	case state.problem != "":
		label, role, ink = state.problem, mygoui.RoleAlert, theme.Danger
		symbol, fallback = "exclamationmark.triangle.fill", "exclamationmark.triangle.fill"
	case state.index != nil:
		label = fmt.Sprintf(tr("Indexed %d chunks from %d files", "已为 %d 个片段建立索引，来自 %d 个文件"),
			state.index.ChunkCount(), state.index.FileCount())
		if !state.builtAt.IsZero() {
			label += " · " + app.vaultIndexBuildAge(state.builtAt)
		}
		symbol, fallback, ink = "checkmark.seal.fill", "circle-dot", theme.Success
	}
	mygoui.Row(c.Key("AI Vault index status")).FillWidth().MinWidth(0).MinHeight(15).Gap(10).
		AlignItems(mygoui.Center).Children(func() {
		mygoui.Box(c).Size(12, 12).Center().Shrink(0).Role(mygoui.RoleNone).Children(func() {
			aiWorkspaceIcon(c, symbol, fallback, 12, ink)
		})
		mygoui.Text(c, label).Shrink(1).MinWidth(0).FontSize(12).FixedLineHeight(15).TextColor(ink).Wrap().Role(role)
	})
	if state != nil && state.request != nil {
		mygoui.Progress(c, -1).Height(4).FillWidth().Margin(6, 0, 0).Label(label)
	}
}

func aiVaultBuildProgressStatus(progress vaultcontext.BuildProgress, locale workspaceLanguage) (string, float64) {
	format := message.NewPrinter(language.Make(string(locale)))
	value := -1.0
	var label string
	switch progress.Phase {
	case vaultcontext.BuildWordStatistics:
		label = format.Sprintf(locale.text("Computing word statistics across %d notes…", "正在统计 %d 篇笔记的词频…"), progress.Total)
	case vaultcontext.BuildChunks:
		label = format.Sprintf(locale.text("Indexing %d of %d chunks…", "正在索引片段 %d / %d…"), progress.Processed, progress.Total)
		value = 0
		if progress.Total > 0 {
			value = float64(progress.Processed) / float64(progress.Total)
		}
	default:
		label = format.Sprintf(locale.text("Reading 0 of %d notes…", "正在读取笔记 0 / %d…"), progress.Total)
	}
	return label, value
}

// EV-UI-307/308. Preparing and indexing both put Cancel below the meter.
func (app *workspaceWindow) drawAIVaultBuildProgress(c *mygoui.Context, progress vaultcontext.BuildProgress) {
	label, value := aiVaultBuildProgressStatus(progress, app.interfaceLanguage())
	cancel := app.tr("Cancel", "取消")
	theme := c.Theme()
	iconInk, textInk, cancelInk := theme.Accent, theme.Text, theme.TextMuted
	if app.blackout && !c.Preferences().HighContrast {
		iconInk = blackoutTitleAccent
		textInk, cancelInk = mygoui.Hex("#e3e2e6"), mygoui.Hex("#c4c6d0")
	}
	mygoui.Column(c.Key("AI Vault build progress")).FillWidth().MinWidth(0).Gap(6).AlignItems(mygoui.Start).Children(func() {
		mygoui.Row(c).FillWidth().MinWidth(0).Gap(8).AlignItems(mygoui.Center).Children(func() {
			aiWorkspaceIcon(c, "arrow.triangle.2.circlepath", "arrow.counterclockwise", 12, iconInk)
			mygoui.Text(c, label).Shrink(1).MinWidth(0).FontSize(12).FixedLineHeight(15).
				TextColor(textInk).Wrap().Role(mygoui.RoleStatus)
		})
		mygoui.Progress(c, value).FillWidth().Height(4).Label(label)
		mygoui.ButtonBase(c.Key(cancel)).Center().KeepFocus().Label(cancel).OnClick(app.cancelAIVaultContextBuild).Children(func() {
			mygoui.Text(c, cancel).FontSize(11).SingleLine().TextColor(cancelInk).Role(mygoui.RoleNone)
		})
	})
}

func (app *workspaceWindow) vaultIndexBuildAge(builtAt time.Time) string {
	elapsed := time.Since(builtAt)
	if elapsed < time.Minute {
		return app.tr("just now", "刚刚")
	}
	for _, interval := range [...]struct {
		duration                  time.Duration
		singular, plural, chinese string
	}{
		{24 * time.Hour, "day", "days", "天"},
		{time.Hour, "hour", "hours", "小时"},
		{time.Minute, "minute", "minutes", "分钟"},
	} {
		if count := elapsed / interval.duration; count > 0 {
			unit := interval.plural
			if count == 1 {
				unit = interval.singular
			}
			return fmt.Sprintf(app.tr("%d "+unit+" ago", "%d "+interval.chinese+"前"), count)
		}
	}
	return ""
}

func aiVaultIndexButton(c *mygoui.Context, label string, width float32, disabled bool, action func()) {
	mygoui.Button(c.Key(label), "").Size(width, 24).Padding(0, 4).Label(label).KeepFocus().
		Disabled(disabled).OnClick(action).Children(func() {
		mygoui.Text(c, label).FontSize(13).SingleLine().Role(mygoui.RoleNone)
	})
}
