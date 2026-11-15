package ui

import (
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

type workspaceAI struct {
	prompt               string
	submitted            []workspaceAITurn
	scroll               mygoui.ScrollState
	request              *workspaceAIRequest
	replaceSelection     bool
	contextFlyout        *mygo.Window
	historyFlyout        *mygo.Window
	guideFlyout          *mygo.Window
	conversationID       string
	conversationDocument string
	savedTurns           int
}

type aiComposerOptions struct {
	model            string
	send             func(string) bool
	streamToEditor   bool
	toggleWriteBack  func()
	attachContext    func()
	clearContext     func()
	showContext      func(mygoui.Rect)
	context          *workspaceAIContext
	showHistory      func(mygoui.Rect)
	showGuide        func(mygoui.Rect)
	openFeatureGuide func()
	rawText          bool
	toggleRawText    func()
	sidebarRight     bool
	moveSidebar      func()
	insertReply      func(string)
}

func (app *workspaceWindow) aiComposerOptions() aiComposerOptions {
	options := aiComposerOptions{
		model: app.preferences.AIConnection.Model, send: app.submitAIChat,
		streamToEditor: app.preferences.AIStreamIntoEditor, toggleWriteBack: app.toggleAIWriteBack,
		clearContext: app.clearAIContext, showContext: app.showAIContext,
		context:          app.activeAIContext(),
		showHistory:      app.showAIConversationHistory,
		showGuide:        app.showAIWorkspaceGuide,
		openFeatureGuide: app.showFeatureGuide,
		rawText:          app.preferences.AIChatRawText, toggleRawText: func() {
			app.preferences.AIChatRawText = !app.preferences.AIChatRawText
			app.preferencesChanged()
		},
		sidebarRight: app.preferences.AIPanelRight, moveSidebar: func() {
			app.setAIPanelRight(!app.preferences.AIPanelRight)
		},
	}
	if app.workspaceTab != nil && app.draft != nil && (app.sourceEditing() || app.displayedMode() == wysiwygMode && app.wys.editable(app)) {
		options.attachContext = app.attachAIContext
		options.insertReply = app.insertAIReply
	}
	return options
}

func (app *workspaceWindow) toggleAIWriteBack() {
	app.preferences.AIStreamIntoEditor = !app.preferences.AIStreamIntoEditor
	app.writePreferences()
}

func (s *workspaceAI) appendQuickAction(prompt string) {
	if prompt == "" {
		return
	}
	s.submitted = append(s.submitted, workspaceAITurn{prompt: prompt})
	s.scroll.Y = math.MaxFloat32
}

type workspaceAIColors struct {
	text, muted, accent, accentText, link mygoui.Color
	border, fill, guide, prompt, message  mygoui.Color
	warning, warningInk, settings         mygoui.Color
}

func aiWorkspaceColors(c *mygoui.Context, blackout bool) workspaceAIColors {
	t := c.Theme()
	if blackout && !c.Preferences().HighContrast {
		return workspaceAIColors{
			text: mygoui.Hex("#e3e2e6"), muted: mygoui.Hex("#94949e"), accent: mygoui.Hex("#ffaaf7"),
			accentText: mygoui.Hex("#351136"), link: mygoui.Hex("#a9c7ff"), border: mygoui.Hex("#4a314b"),
			fill: mygoui.Hex("#18101a"), guide: mygoui.Hex("#0c0c10"), prompt: mygoui.Hex("#18181c"), message: mygoui.Hex("#161a26"),
			warning: mygoui.Hex("#170d08"), warningInk: mygoui.Hex("#ff992e"), settings: mygoui.Hex("#2b272a"),
		}
	}
	return workspaceAIColors{
		text: t.Text, muted: t.TextMuted, accent: t.Accent, accentText: t.AccentText, link: t.Accent,
		border: t.Border, fill: t.Accent.Alpha(0.09), guide: t.Surface, prompt: t.Surface, message: t.Accent.Alpha(0.12),
		warning: t.Danger.Alpha(0.09), warningInk: t.Danger, settings: t.SurfacePressed,
	}
}

// EV-UI-225/226: the model notice and composer stay fixed while the conversation scrolls.
func (s *workspaceAI) draw(c *mygoui.Context, blackout bool, drawTabs, openSettings, closePanel func(), composer aiComposerOptions) {
	colors := aiWorkspaceColors(c, blackout)
	formatSymbol, formatHelp := "textformat", "Rendered markdown (click for raw)"
	if composer.rawText {
		formatSymbol, formatHelp = "text.alignleft", "Raw text (click for rendered)"
	}
	sideSymbol, sideHelp := "sidebar.right", "Move to right side"
	if composer.sidebarRight {
		sideSymbol, sideHelp = "sidebar.left", "Move to left side"
	}
	mygoui.Toolbar(c.Key("AI workspace toolbar"), func() {
		drawTabs()
		mygoui.Spacer(c)
		for i, button := range [...]struct {
			symbol, fallback, label, help string
			action                        func()
		}{
			{"clock.arrow.circlepath", "", "Chat history", "", nil},
			{"plus.message", "bubble.left.and.text.bubble.right", "New conversation", "", s.newConversation},
			{formatSymbol, "", "Rendered markdown or raw text", formatHelp, composer.toggleRawText},
			{"questionmark.circle", "", "AI Workspace guide", "", nil},
			{"gearshape", "", "AI Settings", "", openSettings},
			{sideSymbol, "rectangle.split.2x1", "Move AI sidebar", sideHelp, composer.moveSidebar},
			{"xmark", "", "Close AI sidebar", "", closePanel},
		} {
			history := button.label == "Chat history" && composer.showHistory != nil
			guide := button.label == "AI Workspace guide" && composer.showGuide != nil
			control := chromeControl(c.Key(button.label), button.label, button.action == nil && !history && !guide).KeepFocus()
			if button.help != "" {
				control.Tooltip(button.help)
			}
			if i < 6 {
				control.MarginEnd(4)
			}
			ink := colors.muted
			if button.symbol == "textformat" {
				control.Background(colors.fill)
				ink = colors.accent
			}
			if button.action != nil {
				control.OnClick(button.action)
			}
			if history {
				control.OnClick(func() { composer.showHistory(control.Bounds()) })
			}
			if guide {
				control.OnClick(func() { composer.showGuide(control.Bounds()) })
			}
			control.Children(func() { aiWorkspaceIcon(c, button.symbol, button.fallback, 15, ink) })
		}
	}).Label("AI Workspace tools").Height(34).Shrink(0).FillWidth().Gap(0).Padding(4, 16, 0).AlignItems(mygoui.Center)
	if composer.model == "" {
		drawAINoModel(c, colors, openSettings)
	}
	if composer.streamToEditor {
		composer.insertReply = nil
	}
	s.drawAIConversation(c, colors, openSettings, composer)
	drawAIContext(c, colors, composer)
	s.drawAIComposer(c, colors, composer)
}

func aiWorkspaceIcon(c *mygoui.Context, symbol, fallback string, size float32, color mygoui.Color) {
	if fallback == "" {
		fallback = symbol
	}
	sfsymbols.Symbol(c, symbol).Size(size).Natural().Color(color).Fallback(chromeIconFallbacks[fallback]).Role(mygoui.RoleNone)
}

func drawAINoModel(c *mygoui.Context, colors workspaceAIColors, openSettings func()) {
	const inset = 26
	mygoui.Column(c.Key("AI model notice")).Width(vaultPanelWidth-2*inset).Height(105).Shrink(0).Margin(0, inset).Padding(12, 10).
		Gap(8).AlignItems(mygoui.Center).Background(colors.warning).Children(func() {
		mygoui.Row(c).Height(15).Gap(8).AlignItems(mygoui.Center).Children(func() {
			aiWorkspaceIcon(c, "exclamationmark.triangle.fill", "", 14, colors.warningInk)
			mygoui.Text(c, "No AI model connected").FontSize(13).FontWeight(650).TextColor(colors.text).SingleLine()
		})
		mygoui.Text(c, "Connect a local model (Ollama, LM Studio) or cloud\nprovider (Claude, OpenRouter) to get started.").
			Height(28).FillWidth().FontSize(11).FixedLineHeight(14).TextAlign(mygoui.Center).TextColor(colors.text)
		mygoui.ButtonBase(c.Key("open AI settings")).Size(131, 22).Center().Gap(4).Radius(7).Label("Open AI Settings").
			Background(colors.settings).Disabled(openSettings == nil).OnClick(openSettings).Children(func() {
			aiWorkspaceIcon(c, "gearshape", "", 12, colors.link)
			mygoui.Text(c, "Open AI Settings").FontSize(11).FontWeight(650).TextColor(colors.link).Role(mygoui.RoleNone)
		})
	})
}

func (s *workspaceAI) drawAIConversation(c *mygoui.Context, colors workspaceAIColors, openSettings func(), composer aiComposerOptions) {
	const inset = 4
	conversation := mygoui.Scroll(c.Key("AI conversation")).Grow(1).MinHeight(0).Width(vaultPanelWidth-2*inset).Margin(0, inset).
		Label("AI Conversation").TrackScroll(&s.scroll)
	if len(s.submitted) == 0 {
		conversation.Padding(20, 24).Children(func() {
			drawAIWorkspaceGuide(c, colors, openSettings, composer.openFeatureGuide)
		})
		return
	}
	conversation.Padding(4, 12, 12).Children(func() {
		mygoui.Column(c).FillWidth().Gap(20).Children(func() {
			for i, turn := range s.submitted {
				mygoui.Column(c).FillWidth().Gap(6).Children(func() {
					mygoui.Row(c).FillWidth().Justify(mygoui.End).Children(func() {
						mygoui.Column(c).MaxWidth(292).Padding(12).Radius(8, 0, 0, 8).Background(colors.message).
							BorderWidth(0, 2, 0, 0).BorderColor(colors.link.Alpha(0.48)).Children(func() {
							mygoui.Text(c, turn.prompt).MaxWidth(268).FontSize(14).FixedLineHeight(18).TextColor(colors.text)
						})
					})
					running := s.request != nil && s.request.turn == i
					if running || turn.reply != "" || turn.problem != "" {
						latest := i == len(s.submitted)-1 && s.request == nil && turn.reply != ""
						s.drawAIReply(c, colors, turn, running, latest, composer.rawText, composer.insertReply)
					}
				})
			}
		})
	})
}

func (s *workspaceAI) drawAIReply(c *mygoui.Context, colors workspaceAIColors, turn workspaceAITurn, running, latest, rawText bool, insertReply func(string)) {
	mygoui.Column(c).FillWidth().Gap(8).Children(func() {
		mygoui.Row(c).FillWidth().Height(18).AlignItems(mygoui.Center).Gap(5).Children(func() {
			if running && turn.reply == "" {
				mygoui.Text(c, "Thinking...").FontSize(12).TextColor(colors.muted).Role(mygoui.RoleStatus)
			} else {
				aiWorkspaceIcon(c, "sparkle", "sparkles", 10, colors.accent)
				mygoui.Text(c, turn.model).FontSize(12).TextColor(colors.muted)
			}
			if running {
				mygoui.Spacer(c)
				mygoui.ButtonBase(c.Key("stop AI response")).Size(14, 14).Center().Label("Stop response").
					OnClick(s.stopResponse).Children(func() {
					aiWorkspaceIcon(c, "stop.circle.fill", "xmark.circle.fill", 14, colors.accent)
				})
			}
		})
		if turn.reply != "" {
			// EV-UI-257: the reply body keeps the same inset and card in both modes.
			mygoui.Column(c.Key("AI reply body")).FillWidth().MinWidth(0).Padding(12, 12, 12, 10).
				Background(colors.prompt).Radius(0, 8, 8, 0).
				BorderWidth(0, 0, 0, 2).BorderColor(colors.accent.Alpha(0.29)).Children(func() {
				if rawText || running {
					mygoui.Text(c, turn.reply).FillWidth().FontSize(14).FixedLineHeight(18).TextColor(colors.text)
				} else {
					theme := markdown.DefaultTheme(c.Theme())
					theme.FontSize, theme.LineHeight = 14, 1.15
					theme.Text, theme.Strong, theme.Link, theme.Muted = colors.text, colors.text, colors.link, colors.muted
					theme.BlockSpacing, theme.ItemSpacing = 0.55, 0.25
					for i := range theme.Headings {
						theme.Headings[i] = markdown.HeadingStyle{Size: max(1, 1.24-float32(i)*0.1), Weight: 650}
					}
					markdown.View(c, turn.reply, &markdown.Options{Theme: theme}).FillWidth().MinWidth(0)
				}
			})
		}
		if turn.problem != "" {
			mygoui.Text(c, turn.problem).FillWidth().FontSize(13).FixedLineHeight(17).TextColor(colors.warningInk).
				Role(mygoui.RoleAlert)
		}
		if latest {
			// EV-UI-231/242. Actions are attached to the newest available reply.
			mygoui.Row(c.Key("AI reply actions")).FillWidth().Height(22).Padding(0, 5.5, 0, 4).
				AlignItems(mygoui.Center).Children(func() {
				if insertReply != nil {
					mygoui.ButtonBase(c.Key("insert AI reply")).Size(136, 22).Center().Gap(6).KeepFocus().
						Radius(11).Background(colors.link.Alpha(0.11)).Label("Insert into Editor").
						OnClick(func() { insertReply(turn.reply) }).Children(func() {
						aiWorkspaceIcon(c, "text.cursor", "textformat", 12, colors.link)
						mygoui.Text(c, "Insert into Editor").FontSize(11).TextColor(colors.link).Role(mygoui.RoleNone)
					})
				}
				mygoui.Spacer(c)
				mygoui.ButtonBase(c.Key("copy AI reply")).Size(11, 14).Center().KeepFocus().Label("Copy").
					OnClick(func() { c.WriteClipboard(turn.reply) }).Children(func() {
					aiWorkspaceIcon(c, "doc.on.doc", "", 14, colors.text)
				})
			})
		}
	})
}

// EV-UI-225 / EV-UI-245. The guide is the same in an empty chat with or without a model.
func drawAIWorkspaceGuide(c *mygoui.Context, colors workspaceAIColors, openSettings, openFeatureGuide func()) {
	mygoui.Column(c).FillWidth().AlignItems(mygoui.Center).Gap(16).Children(func() {
		mygoui.Column(c).AlignItems(mygoui.Center).Gap(4).Children(func() {
			aiGuideIcon(c, "sparkles", "", "Sparkle", 32, colors.accent)
			mygoui.Text(c, "AI Workspace").FontSize(17).FixedLineHeight(20).Bold().TextColor(c.Theme().Text)
			mygoui.Text(c, "Ask about what you're writing.").FontSize(15).FixedLineHeight(18).TextColor(colors.muted)
		})
		mygoui.Column(c).AlignItems(mygoui.Center).Gap(3).Children(func() {
			for _, prompt := range [...]string{
				"Summarize this note", "Improve this paragraph", "Diagram this as a flowchart", "Turn this into a table",
			} {
				mygoui.ButtonBase(c.Key(prompt)).Height(25).Padding(0, 12).Center().Radius(12).Label(prompt).Disabled(true).
					Border(1, colors.border).Background(colors.link.Alpha(0.09)).Children(func() {
					mygoui.Text(c, prompt).FontSize(13).TextColor(colors.text).Role(mygoui.RoleNone)
				})
			}
		})
		drawAIVaultGuide(c, colors)
		drawAIControlsGuide(c, colors)
		mygoui.Column(c).FillWidth().AlignItems(mygoui.Center).Gap(5).Children(func() {
			mygoui.Text(c, "Sends your selection, or the whole note if nothing is selected.").
				MaxWidth(284).TextAlign(mygoui.Center).FontSize(12).FixedLineHeight(15).TextColor(colors.muted)
			mygoui.Text(c, commandPaletteShortcut("⌘J")+" runs quick actions on selected text.").
				TextAlign(mygoui.Center).FontSize(12).FixedLineHeight(15).TextColor(colors.muted)
			mygoui.Row(c).Margin(10, 0, 0).Gap(12).AlignItems(mygoui.Center).Children(func() {
				aiWorkspaceLink(c, "Open the guide", colors.accent, openFeatureGuide)
				mygoui.Text(c, "·").FontSize(12).TextColor(colors.muted).Role(mygoui.RoleNone)
				aiWorkspaceLink(c, "AI Settings", colors.accent, openSettings)
			})
		})
	})
}

func aiWorkspaceLink(c *mygoui.Context, label string, color mygoui.Color, action func()) {
	mygoui.ButtonBase(c.Key(label)).Height(15).Label(label).Disabled(action == nil).OnClick(action).Children(func() {
		mygoui.Text(c, label).FontSize(12).FixedLineHeight(15).FontWeight(600).TextColor(color).Role(mygoui.RoleNone)
	})
}

func drawAIVaultGuide(c *mygoui.Context, colors workspaceAIColors) {
	mygoui.Column(c.Key("AI Vault guide")).FillWidth().Padding(12, 11).Gap(9).Radius(8).
		Background(colors.fill).Border(1, colors.border).Children(func() {
		mygoui.Row(c).Gap(8).AlignItems(mygoui.Center).Children(func() {
			mygoui.Box(c).Size(16, 16).Shrink(0).Children(func() {
				aiGuideIcon(c, "rectangle.stack.badge.person.crop", "rectangle.on.rectangle", "Rectangle Stack With A Person Badge", 19, colors.accent).
					Attach(mygoui.AnchorCenter, mygoui.AnchorCenter)
			})
			mygoui.Text(c, "CHAT WITH YOUR VAULT").FontSize(11).FontWeight(700).LetterSpacing(1).TextColor(colors.accent)
		})
		mygoui.Text(c, "Index your vault once, on-device, and the AI pulls in relevant notes you didn't mention.").
			FontSize(12).FixedLineHeight(14).TextColor(colors.text)
		mygoui.Row(c).Gap(7).AlignItems(mygoui.Start).Children(func() {
			aiGuideIcon(c, "lock.shield", "questionmark.circle", "Padlock In A Shield", 11, colors.muted)
			mygoui.Text(c, "Embeddings stay on your Mac; only the retrieved snippet text is sent to the AI").
				Grow(1).MinWidth(0).FontSize(11).FixedLineHeight(14).TextColor(colors.text)
		})
		mygoui.ButtonBase(c).Height(28.5).FillWidth().Margin(2, 0, 0).Center().Gap(6).Radius(6).Background(colors.accent).
			Label("Build Vault Embeddings").Disabled(true).Children(func() {
			aiWorkspaceIcon(c, "sparkles", "", 13, colors.accentText)
			mygoui.Text(c, "Build Vault Embeddings").FontSize(11).FontWeight(650).TextColor(colors.accentText).Role(mygoui.RoleNone)
		})
	})
}

func drawAIControlsGuide(c *mygoui.Context, colors workspaceAIColors) {
	type guideControl struct{ symbol, fallback, label, detail string }
	mygoui.Column(c.Key("AI controls guide")).FillWidth().Padding(12).Gap(12).Radius(8).Background(colors.guide).Children(func() {
		for _, group := range [...]struct {
			caption string
			rows    [5]guideControl
		}{
			{"CHAT INPUT", [5]guideControl{
				{"doc.viewfinder", "doc.text", "Document In A Viewfinder Frame", "Attach this note, or your selection, as context"},
				{"paperclip", "doc.on.doc", "Attachments", "Attach reference files"},
				{"rectangle.stack", "rectangle.on.rectangle", "Stack Of Rectangles", "Pull in relevant vault notes"},
				{"text.append", "list.bullet.indent", "Move To Bottom Of List", "Stream the reply into your note"},
				{"arrow.left.arrow.right.square", "arrow.left.arrow.right", "Left And Right Arrows In A Square", "Replace the selection with the reply"},
			}},
			{"PANEL CONTROLS", [5]guideControl{
				{"clock.arrow.circlepath", "", "Clock With A Circular Arrow", "Chat history"},
				{"plus.message", "bubble.left.and.text.bubble.right", "Plus Sign On A Message Bubble", "New conversation"},
				{"textformat", "", "Text Formatting", "Rendered markdown or raw text"},
				{"questionmark.circle", "", "Question Mark In A Circle", "AI Workspace guide"},
				{"gearshape", "", "Gear Shape", "AI Settings"},
			}},
		} {
			mygoui.Column(c).FillWidth().Gap(7).Children(func() {
				mygoui.Text(c, group.caption).FontSize(11).FontWeight(700).LetterSpacing(1).TextColor(colors.muted)
				for _, row := range group.rows {
					mygoui.Row(c).FillWidth().Gap(12).AlignItems(mygoui.Start).Children(func() {
						mygoui.Box(c).Size(16, 15).Shrink(0).Children(func() {
							aiGuideIcon(c, row.symbol, row.fallback, row.label, 15, colors.text).
								Attach(mygoui.AnchorCenter, mygoui.AnchorCenter)
						})
						mygoui.Text(c, row.detail).Grow(1).MinWidth(0).FontSize(12).FixedLineHeight(15).TextColor(colors.text)
					})
				}
			})
		}
	})
}

func aiGuideIcon(c *mygoui.Context, symbol, fallback, label string, size float32, color mygoui.Color) mygoui.Element {
	if fallback == "" {
		fallback = symbol
	}
	return sfsymbols.Symbol(c, symbol).Size(size).Natural().Color(color).Fallback(chromeIconFallbacks[fallback]).Label(label)
}

// EV-UI-242 / EV-UI-261. Context and Notes share one 15 pt row above the composer.
func drawAIContext(c *mygoui.Context, colors workspaceAIColors, composer aiComposerOptions) {
	context := composer.context
	if context == nil {
		return
	}
	attached, hasNotes := context.snapshot != "", context.hasNotes()
	if !attached && !hasNotes {
		return
	}
	label, width := "+Notes", float32(57)
	if attached {
		label, width = "+Context "+strconv.Itoa(context.characters), 80.5
	}
	caption := []mygoui.Span{{Text: label}}
	if attached && hasNotes {
		const suffix = " +notes"
		label += suffix
		caption = append(caption, mygoui.Span{Text: suffix, Size: 10})
	}
	mygoui.Row(c.Key("AI attached context")).Height(15).Shrink(0).Margin(0, 16).
		Gap(5).AlignItems(mygoui.Center).Children(func() {
		chip := mygoui.ButtonBase(c.Key("AI context chip")).MinWidth(width).Height(15).Padding(0, 6).Center().
			Radius(4).Background(colors.fill).KeepFocus().Label(label).Children(func() {
			mygoui.RichText(c, caption...).FontSize(11).TextColor(colors.accent).Role(mygoui.RoleNone)
		})
		if composer.showContext != nil && chip.Clicked() {
			composer.showContext(chip.Bounds())
		}
		if attached {
			mygoui.ButtonBase(c.Key("clear AI context")).Size(6, 6).Center().KeepFocus().
				Label("Remove context").Disabled(composer.clearContext == nil).OnClick(composer.clearContext).Children(func() {
				aiWorkspaceIcon(c, "xmark", "", 6, colors.muted)
			})
		}
	})
}

func (s *workspaceAI) drawAIComposer(c *mygoui.Context, colors workspaceAIColors, composer aiComposerOptions) {
	const inset = 16
	placeholder := "Ask Not configured..."
	if composer.model != "" {
		placeholder = "Ask " + composer.model + "..."
	}
	mygoui.Column(c.Key("AI composer")).Width(vaultPanelWidth-2*inset).Height(66).Shrink(0).Margin(4, inset, 12).Padding(11, 7, 7).
		Gap(4).Radius(11).Background(colors.prompt).Border(1, colors.border).Children(func() {
		mygoui.TextAreaBase(c.Key("AI prompt"), &s.prompt).FillWidth().Height(20).Padding(0, 6).FontSize(14).
			Label("AI prompt").Placeholder(placeholder).TextColor(colors.text).OnTextKey(func(key mygoui.TextKey) bool {
			if key.Mods != 0 || key.Key != mygoui.KeyEnter || composer.send == nil || composer.model == "" {
				return false
			}
			if composer.send(key.Text()) {
				key.SetText("")
			}
			return true
		})
		mygoui.Row(c).Height(22).Gap(5).AlignItems(mygoui.Center).Children(func() {
			for i, control := range [...]struct {
				symbol, fallback, label string
				action                  func()
			}{
				{"doc.viewfinder", "doc.text", "Attach current note", composer.attachContext},
				{"paperclip", "doc.on.doc", "Attachments", nil},
				{"rectangle.stack", "rectangle.on.rectangle", "Vault", nil},
			} {
				button := mygoui.ButtonBase(c.Key(control.label)).Size(22, 22).Center().KeepFocus().
					Label(control.label).Disabled(control.action == nil).OnClick(control.action)
				if i == 2 {
					button.Size(59.5, 21).MarginStart(4.5).Gap(3).Border(1, colors.muted.Alpha(0.33)).Radius(6)
				}
				button.Children(func() {
					aiWorkspaceIcon(c, control.symbol, control.fallback, 14, colors.text)
					if i == 2 {
						mygoui.Text(c, "Vault").FontSize(11).TextColor(colors.muted).Role(mygoui.RoleNone)
					}
				})
			}
			// EV-UI-231. Chat-only and streaming share this button's position.
			modeSymbol, modeFallback, modeLabel, modeInk := "text.append", "arrow.down.to.line", "Chat only", colors.text
			if composer.streamToEditor {
				modeSymbol, modeFallback, modeLabel, modeInk = "text.cursor", "textformat", "Insert Text", colors.accent
			}
			mygoui.ButtonBase(c.Key("AI write-back mode")).Size(22, 22).MarginStart(3.5).Center().KeepFocus().
				Label(modeLabel).Tooltip(modeLabel).AccessSelected(composer.streamToEditor).
				Disabled(composer.toggleWriteBack == nil).OnClick(composer.toggleWriteBack).Children(func() {
				aiWorkspaceIcon(c, modeSymbol, modeFallback, 14, modeInk)
			})
			if composer.streamToEditor {
				symbol, ink := "arrow.left.arrow.right.square", colors.text
				if s.replaceSelection {
					symbol, ink = "arrow.left.arrow.right.square.fill", colors.accent
				}
				mygoui.ButtonBase(c.Key("Replace Selection")).Size(22, 22).Center().KeepFocus().
					Label("Replace Selection").Tooltip("Replace Selection").AccessSelected(s.replaceSelection).
					OnClick(func() { s.replaceSelection = !s.replaceSelection }).Children(func() {
					aiWorkspaceIcon(c, symbol, "arrow.left.arrow.right", 14, ink)
				})
			}
			mygoui.Spacer(c)
			button := mygoui.ButtonBase(c.Key("send AI message")).Size(22, 22).Center().Radius(11).MarginEnd(4)
			// EV-UI-231: while a reply streams, the bottom Send control becomes Stop.
			if s.request != nil {
				button.Background(colors.link).Label("Stop").OnClick(s.stopResponse).Children(func() {
					mygoui.Box(c).Size(8, 8).Radius(1).Background(colors.guide).Role(mygoui.RoleNone)
				})
				return
			}
			active := composer.model != "" && composer.send != nil && strings.TrimSpace(s.prompt) != ""
			background, ink := colors.muted.Alpha(0.28), colors.link.Alpha(0.4)
			if active {
				background, ink = colors.link, colors.guide
				button.OnClick(func() { composer.send(s.prompt) })
			}
			button.Background(background).Label("Send").Disabled(!active).Children(func() {
				aiWorkspaceIcon(c, "arrow.up", "textformat.size.larger", 12, ink)
			})
		})
	})
}
