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
			reader := drawAIWorkspaceGuidePopover(c, app.blackout)
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
	symbol, fallback, iconLabel, text string
}

var aiWorkspaceGuideSections = [...]struct {
	title string
	rows  []aiWorkspaceGuideRow
}{
	{"GETTING STARTED", []aiWorkspaceGuideRow{
		{"link", "", "Link", "Connect a model in **Settings > AI** (local or cloud)"},
		{"text.cursor", "textformat", "Insert Text", "Type a request and press Enter or click send"},
		{"arrow.up", "textformat.size.larger", "Up", "When streaming is off, click **Insert into Editor** to add a reply to your document"},
	}},
	{"CONTEXT", []aiWorkspaceGuideRow{
		{"doc.viewfinder", "doc.text", "Document In A Viewfinder Frame", "Click the document icon to attach your current document or selection as context"},
		{"pencil", "pencil.and.outline", "Edit", "Click the context badge to review the attached text before sending"},
		{"note.text", "doc.text", "Notes", "Add **Notes** for recurring references (colors, file paths, style rules) across messages in this chat"},
	}},
	{"STREAM INTO EDITOR", []aiWorkspaceGuideRow{
		{"text.cursor", "textformat", "Insert Text", "Toggle **Insert Text** on to stream replies directly into your document at the cursor"},
		{"eye", "", "Show", "Watch the live preview update as the response streams in"},
		{"arrow.uturn.backward", "arrow.counterclockwise", "Undo", "**⌘Z** undoes the entire streamed insertion at once"},
	}},
	{"WHAT THE AI CAN GENERATE", []aiWorkspaceGuideRow{
		{"text.badge.checkmark", "doc.text", "Text With A Checkmark Badge", "Standard Markdown - headings, lists, tables, links, images, footnotes"},
		{"chevron.left.forwardslash.chevron.right", "", "Embed Code", "Inline HTML/CSS - styled <div>, <span>, colored text, layouts"},
		{"chart.bar", "", "Chart Column", "Mermaid diagrams - flowcharts, sequence, class, state, gantt, pie, ER, mindmap, timeline, git graph, and more"},
		{"doc.text", "", "Plain Text Document", "Page breaks with [pagebreak]"},
	}},
	{"SUPPORTED PROVIDERS", []aiWorkspaceGuideRow{
		{"hare", "forward.end.fill", "Increase Speed", "Ollama - local, port 11434"},
		{"desktopcomputer", "rectangle", "Computer", "LM Studio - local, port 1234"},
		{"apple.logo", "sparkles", "Apple Logo", "MLX - local, port 8080"},
		{"bolt", "forward.end.fill", "Flash", "vLLM - local, port 8000"},
		{"globe", "network", "Globe", "OpenRouter - compatible cloud endpoint"},
		{"brain.head.profile", "sparkles", "Head Profile Showing The Brain", "Claude (Anthropic) via a compatible endpoint"},
		{"server.rack", "network", "Server Rack", "Any OpenAI-compatible endpoint"},
	}},
	{"TIPS", []aiWorkspaceGuideRow{
		{"lightbulb", "sun.max", "Light Bulb", "Specify an output format in your request, such as Markdown, a table, or a Mermaid diagram"},
		{"gearshape", "", "Gear Shape", "Use **Context Notes** for reusable style rules, colors, and file references in this chat"},
		{"arrow.clockwise", "arrow.counterclockwise", "Refresh", "Click **Refresh** next to Model in Settings > AI to discover available models"},
		{"terminal", "", "Terminal Command Prompt", "Use **Test** to check the AI connection"},
	}},
}

func drawAIWorkspaceGuidePopover(c *mygoui.Context, blackout bool) mygoui.Element {
	colors := aiWorkspaceColors(c, blackout)
	muted := colors.muted
	if blackout && !c.Preferences().HighContrast {
		muted = mygoui.Hex("#c4c3cd")
	}
	return mygoui.Scroll(c.Key("AI workspace guide scroll")).Fill().MinHeight(0).MinWidth(0).
		Label("AI Workspace Guide").Children(func() {
		mygoui.Column(c).FillWidth().Padding(16).Gap(16).Children(func() {
			mygoui.Row(c).Height(19).AlignItems(mygoui.Center).Gap(7).Children(func() {
				aiGuideIcon(c, "sparkles", "", "Sparkle", 16, colors.accent)
				mygoui.Text(c, "AI Workspace Guide").FontSize(15).Bold().FixedLineHeight(19).
					TextColor(colors.text).Role(mygoui.RoleHeading).Level(1)
			})
			for _, section := range aiWorkspaceGuideSections {
				mygoui.Column(c.Key(section.title)).FillWidth().Gap(8).Children(func() {
					mygoui.Text(c, section.title).FontSize(11).FontWeight(650).LetterSpacing(0.65).FixedLineHeight(14).
						TextColor(muted).Role(mygoui.RoleHeading).Level(2)
					for _, row := range section.rows {
						mygoui.Row(c.Key(row.text)).FillWidth().Gap(8).AlignItems(mygoui.Start).Children(func() {
							mygoui.Box(c).Size(16, 15).Shrink(0).Children(func() {
								aiGuideIcon(c, row.symbol, row.fallback, row.iconLabel, 15, muted).
									Attach(mygoui.AnchorCenter, mygoui.AnchorCenter)
							})
							detail := commandPaletteShortcut(row.text)
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
			mygoui.Text(c, "Use the sparkle button to toggle this panel").FillWidth().TextAlign(mygoui.Center).
				FontSize(11).FixedLineHeight(13).TextColor(muted)
		})
	})
}
