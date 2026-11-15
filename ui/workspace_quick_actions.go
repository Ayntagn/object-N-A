package ui

import (
	"strconv"
	"strings"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

type quickActionDestination uint8

const (
	quickActionInsert quickActionDestination = iota
	quickActionChat
)

type workspaceQuickActions struct {
	open, selection, focusPrompt bool
	toneOpen                     bool
	prompt                       string
	context                      string
	destination                  quickActionDestination
	contextChars                 int
	tab                          *workspaceTab
	version                      uint64
}

func (app *workspaceWindow) canOpenQuickActions() bool {
	return !app.modalOpen && app.workspaceTab != nil && app.draft != nil && app.drawing == nil && app.presentation == nil && app.canInsertSnippet()
}

func (app *workspaceWindow) openQuickActions() {
	if !app.canOpenQuickActions() || app.quickActions.open {
		return
	}
	if app.commandPalette.open {
		app.dismissCommandPalette()
	}
	if app.quickSwitcher.open {
		app.dismissQuickSwitcher()
	}
	focusWorkspaceSurface(app.win)
	context := app.snippetSelection()
	wysiwyg := app.displayedMode() == wysiwygMode
	s := workspaceQuickActions{
		open: true, selection: context != "", focusPrompt: wysiwyg,
		destination: quickActionChat, tab: app.workspaceTab, version: app.draft.Version(),
	}
	if !s.selection {
		context = app.workingText()
	}
	s.context = context
	s.contextChars = workspace.MeasureText(context).Characters
	if app.preferences.AIStreamIntoEditor {
		s.destination = quickActionInsert
	}
	app.quickActions = s
	if !wysiwyg && app.vaultPanelVisible() && !app.showingProperties() && !app.showingAI() {
		// EV-UI-217: Source openings transfer initial focus to Vault Search.
		app.focusVaultSearch = true
	}
}

func (app *workspaceWindow) dismissQuickActions() {
	app.quickActions = workspaceQuickActions{}
}

func (app *workspaceWindow) setQuickActionDestination(destination quickActionDestination) {
	if app.quickActions.destination == destination {
		return
	}
	app.quickActions.destination = destination
	app.preferences.AIStreamIntoEditor = destination == quickActionInsert
	app.writePreferences()
}

func (app *workspaceWindow) drawQuickActions(c *mygoui.Context) {
	s := &app.quickActions
	if s.open && (app.workspaceTab == nil || app.draft == nil || s.tab != app.workspaceTab || app.draft.Version() != s.version) {
		app.dismissQuickActions()
	}
	s.draw(c, app.blackout, app.runQuickAction, app.setQuickActionDestination, app.dismissQuickActions)
}

func (app *workspaceWindow) runQuickAction(instruction string) {
	if !app.quickActions.open {
		return
	}
	// EV-UI-227: the running output choice controls visibility before the current choice takes effect.
	if app.quickActionRoute == quickActionChat {
		app.openAIWorkspace()
	}
	app.quickActionRoute = app.quickActions.destination
	if app.quickActions.context != "" {
		instruction += "\n\n" + app.quickActions.context
	}
	app.ai.appendQuickAction(instruction)
	app.dismissQuickActions()
}

const quickActionSummaryInstruction = "Summarize the following text concisely, extracting core themes and actionable points.\n" +
	"IMPORTANT: Output ONLY the result - no commentary, no explanations, no code fences. Match the exact format of the input (Markdown, HTML, plain text, etc). " +
	"Do not wrap the output in ```markdown``` or any other block. Return raw content only."

type quickActionRow struct {
	title, detail, symbol, fallback string
	badge, hint                     string
}

const (
	quickActionToneTitle        = "Change Tone"
	quickActionToneHeaderHeight = 18
	quickActionRowHeight        = 49
)

var quickActionRows = [...]quickActionRow{
	{title: "Summarize Selection", detail: "Extract core themes and actionable points", symbol: "doc.text.magnifyingglass", fallback: "doc.text", hint: "⌘1"},
	{title: "Rewrite Selection", detail: "Improve clarity and editorial flow", symbol: "arrow.triangle.2.circlepath", fallback: "arrow.counterclockwise", badge: "Popular", hint: "⌘2"},
	{title: "Explain Concept", detail: "Deep dive into technical terminology", symbol: "lightbulb", fallback: "info.circle", hint: "⌘3"},
	{title: quickActionToneTitle, detail: "Formal, Creative, or Technical", symbol: "textformat", fallback: "textformat"},
	{title: "Continue Writing", detail: "Extend the text naturally", symbol: "text.badge.plus", fallback: "list.bullet.indent"},
	{title: "Fix Grammar & Spelling", detail: "Correct errors, preserve voice", symbol: "checkmark.circle", fallback: "checkmark.circle"},
}

var quickActionTones = [...]quickActionRow{
	{title: "Formal", detail: "Professional and polished", symbol: "building.columns", fallback: "building.columns"},
	{title: "Creative", detail: "Engaging and expressive", symbol: "paintbrush", fallback: "paintbrush"},
	{title: "Technical", detail: "Precise and detailed", symbol: "wrench.and.screwdriver", fallback: "gearshape"},
}

type quickActionsColors struct {
	surface, text, detail, accent, onAccent mygoui.Color
	field, border, hint, separator, hover   mygoui.Color
	badge, badgeText, warning, warningFill  mygoui.Color
}

func quickActionsPalette(c *mygoui.Context, blackout bool) quickActionsColors {
	t := c.Theme()
	if blackout && !c.Preferences().HighContrast {
		return quickActionsColors{
			surface: mygoui.RGB(24, 24, 28), text: mygoui.RGB(227, 226, 230), detail: mygoui.RGB(151, 151, 159),
			accent: mygoui.RGB(255, 170, 247), onAccent: mygoui.RGB(42, 15, 39),
			field: mygoui.RGB(5, 5, 6), border: mygoui.RGB(43, 43, 48), hint: mygoui.RGB(138, 141, 169),
			separator: mygoui.RGB(29, 29, 33), hover: mygoui.RGB(36, 36, 42),
			badge: mygoui.RGB(60, 50, 43), badgeText: mygoui.RGB(241, 180, 103),
			warning: mygoui.RGB(189, 154, 27), warningFill: mygoui.RGB(35, 33, 28),
		}
	}
	return quickActionsColors{
		surface: t.Surface, text: t.Text, detail: t.TextMuted, accent: t.Accent, onAccent: t.AccentText,
		field: t.Background, border: t.Border, hint: t.TextMuted, separator: t.Border.Alpha(0.45), hover: t.SurfaceHover,
		badge: t.Accent.Alpha(0.18), badgeText: t.Text,
		warning: t.TextMuted, warningFill: t.Background,
	}
}

// EV-UI-215: the panel is an in-window overlay. Keep its fixed control
// heights and let the action list scroll when the workspace is short.
func (s *workspaceQuickActions) draw(c *mygoui.Context, blackout bool, submit func(string), choose func(quickActionDestination), dismiss func()) {
	if !s.open {
		return
	}
	w, h := c.Size()
	footerHeight := float32(34)
	if !s.selection {
		footerHeight += 25
	}
	rows := quickActionRows[:]
	listKey := "quick actions list"
	headingHeight := float32(140)
	if s.toneOpen {
		rows = quickActionTones[:]
		listKey = "quick action tones"
		headingHeight += quickActionToneHeaderHeight
	}
	width := min(float32(420), w-16)
	height := min(headingHeight+float32(len(rows)*quickActionRowHeight)+footerHeight, h-16)
	if width <= 0 || height <= 0 {
		return
	}
	colors := quickActionsPalette(c, blackout)
	top := min((h-height)/2+64, h-height-8)
	mygoui.Overlay(c, func() {
		mygoui.Box(c.Key("quick actions backdrop")).Absolute().Top(0).Left(0).Size(w, h).
			Background(mygoui.Transparent).Role(mygoui.RoleNone).OnClick(dismiss)
		panel := mygoui.Column(c.Key("quick actions panel")).Absolute().Top(top).Left((w-width)/2).
			Size(width, height).Role(mygoui.RoleDialog).Label("Quick Actions").Radius(10).Clip().
			Background(colors.surface).Shadow(0, 8, 24, 0, mygoui.RGB(0, 0, 0).Alpha(0.3))
		panel.Children(func() {
			s.drawDestinations(c, colors, choose)
			s.drawPrompt(c, colors, submit, dismiss)
			mygoui.Row(c).Height(29).Shrink(0).Padding(0, 16, 4).AlignItems(mygoui.End).Children(func() {
				mygoui.Text(c, "QUICK ACTIONS").Role(mygoui.RoleHeading).Level(2).FontSize(10).FontWeight(700).
					LetterSpacing(1).TextColor(colors.detail)
			})
			if s.toneOpen {
				s.drawToneHeading(c, colors)
			}
			mygoui.Scroll(c.Key(listKey)).Grow(1).MinHeight(0).FillWidth().Label("Quick Actions list").Children(func() {
				mygoui.Column(c).FillWidth().Padding(0, 4).Children(func() {
					for _, row := range rows {
						drawQuickActionRow(c, row, colors, func() {
							switch row.title {
							case quickActionToneTitle:
								s.toneOpen = true
							case "Summarize Selection":
								submit(quickActionSummaryInstruction)
							default:
								submit(row.title)
							}
						})
					}
				})
			})
			s.drawContext(c, colors, footerHeight)
		})
	})
}

func (s *workspaceQuickActions) drawToneHeading(c *mygoui.Context, colors quickActionsColors) {
	mygoui.Row(c).FillWidth().Height(quickActionToneHeaderHeight).Shrink(0).Padding(0, 16, 4).Gap(2).
		AlignItems(mygoui.Center).Children(func() {
		mygoui.ButtonBase(c.Key("quick actions back")).Size(10, 14).Label("Back").KeepFocus().
			OnClick(func() { s.toneOpen = false }).Children(func() {
			chromeIconColor(c, "chevron.left", 11, colors.text)
		})
		mygoui.Text(c, quickActionToneTitle).Role(mygoui.RoleHeading).Level(3).FontSize(11).FontWeight(650).TextColor(colors.text)
	})
}

func (s *workspaceQuickActions) drawDestinations(c *mygoui.Context, colors quickActionsColors, choose func(quickActionDestination)) {
	mygoui.Row(c).FillWidth().Height(65).Shrink(0).Padding(12, 12, 7).Gap(8).
		BorderWidth(0, 0, 1, 0).BorderColor(colors.separator).Children(func() {
		s.drawDestination(c, quickActionChat, "Send to Chat", "Response in AI panel", "bubble.left.and.bubble.right", "bubble.left.and.text.bubble.right", colors, choose)
		s.drawDestination(c, quickActionInsert, "Insert into Editor", "Streams into document", "character.cursor.ibeam", "textformat", colors, choose)
	})
}

func (s *workspaceQuickActions) drawDestination(c *mygoui.Context, destination quickActionDestination, title, detail, symbol, fallback string, colors quickActionsColors, choose func(quickActionDestination)) {
	selected := s.destination == destination
	button := mygoui.ButtonBase(c.Key(title)).Grow(1).Basis(0).MinWidth(0).Height(45).Radius(8).Border(1, mygoui.Transparent).
		Label(title+", "+detail).AccessSelected(selected).KeepFocus().Padding(0, 11).Gap(8).
		Justify(mygoui.Start).AlignItems(mygoui.Center).OnClick(func() { choose(destination) })
	ink, description := colors.text, colors.detail
	if selected {
		button.Background(colors.accent)
		ink, description = colors.onAccent, colors.onAccent.Alpha(0.78)
	} else {
		button.BorderColor(colors.border)
		ink = colors.detail
	}
	button.Children(func() {
		quickActionIcon(c, symbol, fallback, 16, ink)
		mygoui.Column(c).Grow(1).MinWidth(0).Justify(mygoui.Center).Children(func() {
			mygoui.Text(c, title).FontSize(11).FontWeight(650).TextColor(ink).SingleLine().Ellipsis("…")
			mygoui.Text(c, detail).FontSize(11).TextColor(description).SingleLine().Ellipsis("…")
		})
	})
}

func (s *workspaceQuickActions) drawPrompt(c *mygoui.Context, colors quickActionsColors, submit func(string), dismiss func()) {
	const promptLabel = "Custom instruction for AI..."
	mygoui.Row(c).FillWidth().Height(46).Shrink(0).Padding(1, 16, 0).Gap(8).AlignItems(mygoui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(colors.separator).Children(func() {
		quickActionIcon(c, "sparkles", "sparkles", 16, colors.accent)
		mygoui.Row(c).Grow(1).MinWidth(0).Height(28).Padding(0, 8).AlignItems(mygoui.Center).
			Background(colors.field).Radius(8).Children(func() {
			input := mygoui.TextAreaBase(c.Key("quick actions instruction"), &s.prompt).Grow(1).MinWidth(0).Height(20).
				FontSize(14).TextColor(colors.text).Label(promptLabel).Placeholder(promptLabel).
				OnShortcut(0, mygoui.KeyEscape, dismiss).
				OnTextKey(func(k mygoui.TextKey) bool {
					if k.Mods != 0 || k.Key != mygoui.KeyEnter {
						return false
					}
					// EV-UI-218/224: Return submits a filled prompt and leaves an empty one open.
					if text := k.Text(); strings.TrimSpace(text) != "" {
						submit(text)
					}
					return true
				})
			if s.focusPrompt {
				input.Focus()
				s.focusPrompt = false
			}
		})
		mygoui.Box(c).Size(29, 16).Shrink(0).Radius(4).Background(colors.hover).Center().Children(func() {
			mygoui.Text(c, "ESC").FontSize(9).FontWeight(700).TextColor(colors.detail)
		})
	})
}

func drawQuickActionRow(c *mygoui.Context, row quickActionRow, colors quickActionsColors, submit func()) {
	label := row.title + ", " + row.detail
	if row.badge != "" {
		label += ", " + row.badge
	}
	if row.hint != "" {
		label += ", " + row.hint
	}
	button := mygoui.ButtonBase(c.Key(row.title)).FillWidth().Height(quickActionRowHeight).Shrink(0).Radius(6).
		Padding(0, 12).Gap(12).Justify(mygoui.Start).AlignItems(mygoui.Center).Label(label).KeepFocus().OnClick(submit)
	if button.Hovered() {
		button.Background(colors.hover)
	}
	button.Children(func() {
		mygoui.Box(c).Size(20, 22).Center().Shrink(0).Children(func() {
			quickActionIcon(c, row.symbol, row.fallback, 16, colors.detail)
		})
		mygoui.Column(c).Grow(1).MinWidth(0).Justify(mygoui.Center).Children(func() {
			mygoui.Text(c, row.title).FontSize(13).FontWeight(650).TextColor(colors.text).SingleLine().Ellipsis("…")
			mygoui.Text(c, row.detail).FontSize(11).FontWeight(600).TextColor(colors.detail).SingleLine().Ellipsis("…")
		})
		if row.badge != "" {
			mygoui.Box(c).Padding(2, 5).Radius(3).Background(colors.badge).Children(func() {
				mygoui.Text(c, row.badge).FontSize(8).FontWeight(700).TextColor(colors.badgeText).SingleLine()
			})
		}
		if row.hint != "" {
			mygoui.Text(c, row.hint).FontSize(10).TextColor(colors.hint).SingleLine()
		}
		if row.title == quickActionToneTitle {
			chromeIconColor(c, "chevron.right", 9, colors.detail)
		}
	})
}

func (s *workspaceQuickActions) drawContext(c *mygoui.Context, colors quickActionsColors, height float32) {
	footer := mygoui.Row(c).FillWidth().Height(height).Shrink(0).Padding(0, 16).Gap(6).AlignItems(mygoui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(colors.separator)
	if !s.selection {
		footer.Background(colors.warningFill)
	}
	footer.Children(func() {
		if s.selection {
			ink := colors.accent.Alpha(0.68)
			quickActionIcon(c, "text.badge.checkmark", "doc.text", 10, ink)
			mygoui.Text(c, "Selected text as context").FontSize(10).TextColor(ink).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
		} else {
			quickActionIcon(c, "exclamationmark.triangle", "exclamationmark.triangle", 13, colors.warning)
			mygoui.Column(c).Grow(1).MinWidth(0).Justify(mygoui.Center).Children(func() {
				mygoui.Text(c, "Entire document as context").FontSize(11).FontWeight(600).TextColor(colors.text).SingleLine().Ellipsis("…")
				mygoui.Text(c, "Select specific text first to narrow the scope").FontSize(11).TextColor(colors.detail).SingleLine().Ellipsis("…")
			})
		}
		mygoui.Text(c, strconv.Itoa(s.contextChars)).FontSize(10).FontWeight(650).TextColor(colors.detail).SingleLine()
		mygoui.Text(c, "chars").FontSize(9).TextColor(colors.detail).SingleLine()
	})
}

func quickActionIcon(c *mygoui.Context, symbol, fallback string, size float32, ink mygoui.Color) {
	sfsymbols.Symbol(c, symbol).Size(size).Natural().Color(ink).Fallback(chromeIconFallbacks[fallback]).Role(mygoui.RoleNone)
}
