package ui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	mygoui "github.com/egoist/mygo/ui"
)

const (
	startPreviewSideInset     = 24
	startPreviewCardGap       = 20
	startPreviewTopBreak      = 700
	startPreviewGuideMinWidth = 280
	startPreviewMaxWidth      = 900
	startPreviewRecentLimit   = 5
)

type startPreviewColors struct {
	background, ink, muted, accent mygoui.Color
	card, border, row, rowBorder   mygoui.Color
	hover, key                     mygoui.Color
}

func (app *workspaceWindow) startPreviewColors(c *mygoui.Context) startPreviewColors {
	theme := c.Theme()
	if app.blackout && !c.Preferences().HighContrast {
		return startPreviewColors{
			background: mygoui.RGB(0, 0, 0), ink: mygoui.RGB(180, 180, 180), muted: mygoui.RGB(128, 128, 128),
			accent: mygoui.RGB(133, 189, 247), card: mygoui.RGB(13, 13, 13), border: mygoui.RGB(33, 33, 33),
			row: mygoui.RGB(25, 25, 25), rowBorder: mygoui.RGB(45, 45, 45), hover: mygoui.RGB(35, 35, 35),
			key: mygoui.RGB(33, 33, 33),
		}
	}
	return startPreviewColors{
		background: theme.Background, ink: theme.Text, muted: theme.TextMuted, accent: theme.Accent,
		card: theme.Background.Mix(theme.Text, .05), border: theme.Background.Mix(theme.Text, .12),
		row: theme.Background.Mix(theme.Text, .09), rowBorder: theme.Background.Mix(theme.Text, .16),
		hover: theme.SurfaceHover, key: theme.Background.Mix(theme.Text, .12),
	}
}

// EV-UI-315: the Start Preview scrolls below the regular preview toolbar.
// The top cards use a viewport breakpoint. The remaining grid sizes its
// columns from the space left after the horizontal page insets.
func (app *workspaceWindow) drawStartPreview(c *mygoui.Context, paneWidth float32) {
	colors := app.startPreviewColors(c)
	documents := app.welcomeDocuments()
	documents = documents[:min(startPreviewRecentLimit, len(documents))]
	width := min(startPreviewMaxWidth, max(0, paneWidth-2*startPreviewSideInset))
	topColumns := 1
	if paneWidth > startPreviewTopBreak {
		topColumns = 2
	}
	guideColumns := max(1, int((width+startPreviewCardGap)/(startPreviewGuideMinWidth+startPreviewCardGap)))
	zoom := float32(normalizedPreviewZoom(app.previewZoom)) / 100
	viewport := mygoui.ScrollBoth(c.Key("start-preview")).Grow(1).MinHeight(0).MinWidth(0).FillWidth().
		Label("Welcome preview").Focusable()
	type previewScroll struct {
		offset mygoui.ScrollState
		zoom   float32
	}
	state := mygoui.Local(viewport, "preview scroll", func() previewScroll { return previewScroll{zoom: zoom} })
	if state.zoom != zoom {
		// EV-UI-331. Reposition once using the laid-out welcome viewport.
		if height := viewport.LayoutBounds().H; height > 0 {
			state.offset.Y = max(0, (state.offset.Y-height/2)*zoom/state.zoom)
		}
		state.zoom = zoom
	}
	viewport.TrackScroll(&state.offset).Background(colors.background).Children(func() {
		page := mygoui.Column(c).Width(paneWidth).Shrink(0).Padding(40, startPreviewSideInset).AlignItems(mygoui.Center)
		if zoom != 1 {
			// EV-UI-315: zoom magnifies the laid-out preview without moving the source.
			// Reserve the transformed extent so the last card remains scrollable.
			layout := page.LayoutBounds()
			if layout.H == 0 || layout.W != paneWidth {
				c.After(16 * time.Millisecond)
			}
			extraWidth := paneWidth * (zoom - 1)
			page.Margin(0, max(0, extraWidth), layout.H*(zoom-1), 0).
				TransformOrigin(0, 0).Scale(zoom, zoom)
		}
		page.Children(func() {
			app.drawStartPreviewHeader(c, colors, func() {
				viewport.Focus()
				app.startPageFilePicker()
			})
			mygoui.Grid(c.Key("start-preview-top")).Width(width).Columns(topColumns).Gap(startPreviewCardGap).
				Margin(0, 0, 24).Children(func() {
				app.drawStartPreviewRecents(c, colors, documents)
				drawStartPreviewFeatures(c, colors, startPreviewQuickStart)
			})
			mygoui.Grid(c.Key("start-preview-guide")).Width(width).Columns(guideColumns).Gap(startPreviewCardGap).Children(func() {
				for _, section := range startPreviewGuide {
					drawStartPreviewFeatures(c, colors, section)
				}
				drawStartPreviewSyntax(c, colors)
				drawStartPreviewShortcuts(c, colors)
			})
			drawStartPreviewFooter(c, colors, width)
		})
	})
}

func (app *workspaceWindow) drawStartPreviewHeader(c *mygoui.Context, colors startPreviewColors, openFile func()) {
	mygoui.Column(c).FillWidth().Shrink(0).AlignItems(mygoui.Center).Margin(0, 0, 32).Children(func() {
		mygoui.Text(c, "Let's Get To Work, Together").FillWidth().Wrap().TextAlign(mygoui.Center).
			FontSize(28).FixedLineHeight(42).FontWeight(700).TextColor(colors.ink).
			Margin(0, 0, 8).Role(mygoui.RoleHeading).Level(1)
		mygoui.Text(c, "Type in the editor on the left, or open an existing file.").
			FillWidth().Wrap().TextAlign(mygoui.Center).FontSize(15).FixedLineHeight(22.5).TextColor(colors.muted)
		mygoui.Row(c).FillWidth().Shrink(0).Justify(mygoui.Center).Gap(12).Margin(16, 0, 0).Children(func() {
			drawStartPreviewAction(c, colors, "📂", "Open File...", 141).OnClick(openFile)
			guide := drawStartPreviewAction(c, colors, "📖", "User Guide", 140)
			guide.OnClick(func() { app.openUserGuide(guide.ClickModifiers(), true) })
		})
	})
}

func drawStartPreviewAction(c *mygoui.Context, colors startPreviewColors, icon, title string, width float32) mygoui.Element {
	label := icon + " " + title
	button := mygoui.ButtonBase(c.Key(title)).Width(width).Height(40).Shrink(1).MinWidth(0).Gap(6).
		Padding(0, 6).Radius(8).Border(1, colors.border).Label(label).Cursor(mygoui.CursorPointer)
	fill := colors.card
	if button.Hovered() {
		fill = colors.row
	}
	button.Background(fill).Children(func() {
		mygoui.Text(c, icon).FontSize(16).Role(mygoui.RoleNone)
		mygoui.Text(c, title).Shrink(1).MinWidth(0).SingleLine().Ellipsis("…").
			FontSize(14).FixedLineHeight(18).FontWeight(500).TextColor(colors.ink).Role(mygoui.RoleNone)
	})
	return button
}

func drawStartPreviewCard(c *mygoui.Context, colors startPreviewColors, icon, title string, content func()) {
	mygoui.Column(c.Key(title)).MinWidth(0).Padding(20).Radius(12).
		Background(colors.card).Border(1, colors.border).Children(func() {
		mygoui.Row(c).FillWidth().Height(24).Shrink(0).Gap(8).Margin(0, 0, 16).Children(func() {
			mygoui.Text(c, icon).Width(23).Shrink(0).FontSize(16).Role(mygoui.RoleNone)
			mygoui.Text(c, title).MinWidth(0).SingleLine().Ellipsis("…").
				FontSize(13).FixedLineHeight(19.5).LetterSpacing(.5).FontWeight(600).
				TextColor(colors.accent).Role(mygoui.RoleHeading).Level(2)
		})
		content()
	})
}

func (app *workspaceWindow) drawStartPreviewRecents(c *mygoui.Context, colors startPreviewColors, documents []workspaceWelcomeDocument) {
	drawStartPreviewCard(c, colors, "📂", "RECENT DOCUMENTS", func() {
		if len(documents) == 0 {
			mygoui.Column(c).FillWidth().Padding(24, 0).AlignItems(mygoui.Center).Children(func() {
				mygoui.Text(c, "📝").FontSize(32).FixedLineHeight(48).Opacity(.5).Margin(0, 0, 8).Role(mygoui.RoleNone)
				for _, line := range [...]string{"No recent documents yet.", "Open or create a file to get started."} {
					mygoui.Text(c, line).FillWidth().Wrap().TextAlign(mygoui.Center).
						FontSize(13).FixedLineHeight(19.5).TextColor(colors.muted)
				}
			})
			return
		}
		home, _ := os.UserHomeDir()
		mygoui.Column(c).FillWidth().Gap(8).Children(func() {
			for _, document := range documents {
				app.drawStartPreviewRecent(c, colors, document, home)
			}
		})
	})
}

func (app *workspaceWindow) drawStartPreviewRecent(c *mygoui.Context, colors startPreviewColors, document workspaceWelcomeDocument, home string) {
	label := document.Title + ", " + document.Folder
	folder := document.Folder
	if filepath.IsAbs(home) {
		relative, err := filepath.Rel(home, folder)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			folder = filepath.Join("~", relative)
		}
	}
	row := mygoui.ButtonBase(c.Key(document.Path)).FillWidth().Height(59.5).Gap(12).Padding(0, 14).
		Justify(mygoui.Start).Radius(8).Border(1, colors.rowBorder).
		Label(label).Cursor(mygoui.CursorPointer).OnClick(func() { app.openWelcomeDocument(document.Path) })
	fill := colors.row
	if row.Hovered() {
		fill = colors.hover
	}
	row.Background(fill).Children(func() {
		mygoui.Text(c, "📄").Width(25).Shrink(0).FontSize(20).Opacity(.6).Role(mygoui.RoleNone)
		mygoui.Column(c).Grow(1).MinWidth(0).Children(func() {
			mygoui.Text(c, document.Title).FontSize(14).FixedLineHeight(21).FontWeight(500).
				SingleLine().Ellipsis("…").TextColor(colors.ink).Role(mygoui.RoleNone)
			mygoui.Text(c, folder).FontSize(11).FixedLineHeight(16.5).
				SingleLine().Ellipsis("…").TextColor(colors.muted).Role(mygoui.RoleNone)
		})
	})
}

type startPreviewFeature struct {
	title, shortcut, description string
}

type startPreviewSection struct {
	icon, title string
	features    []startPreviewFeature
	aside       string
}

func drawStartPreviewFeatures(c *mygoui.Context, colors startPreviewColors, section startPreviewSection) {
	drawStartPreviewCard(c, colors, section.icon, section.title, func() {
		mygoui.Column(c).FillWidth().Gap(14).Children(func() {
			for _, feature := range section.features {
				mygoui.Column(c).FillWidth().Gap(4).Children(func() {
					spans := []mygoui.Span{{Text: feature.title, Weight: 600}}
					if feature.shortcut != "" {
						spans = append(spans, mygoui.Span{Text: "  "}, startPreviewCodeSpan(shortcutLabel(feature.shortcut), colors))
					}
					mygoui.RichText(c, spans...).FillWidth().Wrap().FontSize(13).
						FixedLineHeight(19.5).TextColor(colors.ink)
					drawStartPreviewDescription(c, colors, feature.description)
				})
			}
		})
		if section.aside != "" {
			mygoui.Box(c).FillWidth().Height(1).Shrink(0).Background(colors.border).Margin(14, 0)
			drawStartPreviewDescription(c, colors, section.aside)
		}
	})
}

func drawStartPreviewDescription(c *mygoui.Context, colors startPreviewColors, description string) {
	// The guillemets in the static help copy designate inline code and key caps.
	parts := strings.Split(description, "«")
	spans := []mygoui.Span{{Text: parts[0]}}
	for _, part := range parts[1:] {
		code, following, _ := strings.Cut(part, "»")
		spans = append(spans, startPreviewCodeSpan(commandPaletteShortcut(code), colors), mygoui.Span{Text: following})
	}
	mygoui.RichText(c, spans...).FillWidth().Wrap().FontSize(12).FixedLineHeight(16.8).TextColor(colors.muted)
}

func startPreviewCodeSpan(text string, colors startPreviewColors) mygoui.Span {
	return mygoui.Span{
		Text: text, Font: "monospace", Size: 12, Color: colors.accent,
		Background: colors.key, Radius: 4, PaddingX: 7, PaddingY: 3,
	}
}

func drawStartPreviewSyntax(c *mygoui.Context, colors startPreviewColors) {
	examples := [...]struct {
		input string
		out   mygoui.Span
	}{
		{"# Heading 1", mygoui.Span{Text: "Heading 1", Size: 16, Weight: 700}},
		{"**bold**", mygoui.Span{Text: "bold", Weight: 600}},
		{"*italic*", mygoui.Span{Text: "italic", Italic: true}},
		{"~~strike~~", mygoui.Span{Text: "strike", Strikethrough: true}},
		{"`code`", startPreviewCodeSpan("code", colors)},
		{"[link](url)", mygoui.Span{Text: "link", Color: colors.accent}},
		{"- list item", mygoui.Span{Text: "• list item"}},
	}
	drawStartPreviewCard(c, colors, "📝", "MARKDOWN SYNTAX", func() {
		mygoui.Column(c).FillWidth().Gap(10).Children(func() {
			for _, example := range examples {
				mygoui.Row(c).FillWidth().Height(24.5).Shrink(0).Gap(12).Children(func() {
					mygoui.Text(c, example.input).Width(140).MinWidth(0).SingleLine().Ellipsis("…").
						Padding(0, 8).Radius(5).Background(colors.key).
						Font("monospace").FontSize(11).FixedLineHeight(24.5).TextColor(colors.ink)
					mygoui.Text(c, "→").FontSize(12).TextColor(colors.muted).Role(mygoui.RoleNone)
					mygoui.RichText(c, example.out).MinWidth(0).SingleLine().Ellipsis("…").FontSize(13).TextColor(colors.ink)
				})
			}
		})
	})
}

func drawStartPreviewShortcuts(c *mygoui.Context, colors startPreviewColors) {
	groups := [...][3][2]string{
		{{"⌘ J", "AI Quick Actions"}, {"⌘ P", "Quick Switcher"}},
		{{"⌘ B", "Bold selection"}, {"⌘ I", "Italic selection"}, {"⌘ K", "Insert link"}},
		{{"⌘ E", "Quick export PDF"}, {"⌘ F", "Find"}, {"⌘ ⇧ F", "Focus Mode"}},
		{{"⌘ /", "Command palette"}, {"⌘ ⇧ O", "Outline"}, {"⌘ ⌥ S", "Snippets shade"}},
		{{"⌘ O", "Open document"}, {"⌘ ⇧ /", "All shortcuts"}},
	}
	drawStartPreviewCard(c, colors, "⌨️", "KEYBOARD SHORTCUTS", func() {
		for i, group := range groups {
			if i != 0 {
				mygoui.Box(c).FillWidth().Height(1).Shrink(0).Background(colors.border).Margin(14, 0)
			}
			mygoui.Column(c).FillWidth().Gap(10).Children(func() {
				for _, shortcut := range group {
					if shortcut[0] == "" {
						continue
					}
					mygoui.Row(c).FillWidth().Height(24.5).Shrink(0).Gap(12).Children(func() {
						label := shortcutLabel(shortcut[0])
						mygoui.Text(c, label).MinWidth(70).Shrink(0).Padding(0, 8).TextAlign(mygoui.Center).
							Radius(5).Background(colors.key).Font("monospace").FontSize(11).
							FixedLineHeight(24.5).FontWeight(500).TextColor(colors.ink)
						mygoui.Text(c, shortcut[1]).MinWidth(0).SingleLine().Ellipsis("…").
							FontSize(13).FixedLineHeight(19.5).TextColor(colors.ink)
					})
				}
			})
		}
	})
}

func drawStartPreviewFooter(c *mygoui.Context, colors startPreviewColors, width float32) {
	mygoui.Column(c).Width(width).Margin(32, 0, 0).Gap(12).Children(func() {
		mygoui.RichText(c,
			mygoui.Span{Text: "Press "}, startPreviewCodeSpan(shortcutLabel("⌘ ⇧ /"), colors),
			mygoui.Span{Text: " anytime to see all keyboard shortcuts"},
		).FillWidth().Wrap().TextAlign(mygoui.Center).FontSize(12).FixedLineHeight(18).TextColor(colors.muted)
	})
}

var startPreviewQuickStart = startPreviewSection{
	icon: "🚀", title: "QUICK START",
	features: []startPreviewFeature{
		{title: "✨ AI-Powered Writing", description: "Open AI from the toolbar to chat with your model, or press «⌘J» in the editor for Quick Actions."},
		{title: "📊 Diagrams & Charts", description: "Build diagrams with «```mermaid» and data charts with «```vega-lite» - bind a chart to a vault «.csv»/«.json» or a live URL."},
		{title: "📄 Export Anywhere", description: "PDF, Word, and HTML export with custom themes. Quick export with «⌘E» or full options with «⌘⇧E»."},
		{title: "🔗 File Vault", description: "Open a vault folder to browse your notes. Follow «[[wikilinks]]» and embed images with «![[image.png]]»."},
	},
}

var startPreviewGuide = [...]startPreviewSection{
	{
		icon: "✨", title: "AI WORKSPACE",
		features: []startPreviewFeature{
			{title: "Quick Actions", shortcut: "⌘ J", description: "Open the Quick Actions menu for the current note or selection."},
			{title: "AI Chat Panel", description: "Open AI from the toolbar. Attach note context, ask a question, or stream a reply into the editor."},
			{title: "Local & Cloud Models", description: "Choose a local or cloud model in Settings → AI. Use Ollama, LM Studio, Claude, or an OpenAI-compatible endpoint."},
			{title: "Insert a Reply", description: "Insert a reply at the caret or replace the selected text from the AI panel."},
			{title: "File Attachments", description: "Click the 📎 button to attach text files or PDFs as context for your AI conversation."},
		},
	},
	{
		icon: "📊", title: "MERMAID DIAGRAMS",
		features: []startPreviewFeature{
			{title: "Live Diagram Rendering", description: "Use «```mermaid» code blocks to render diagrams in Markdown Preview."},
			{title: "16 Diagram Types", description: "Flowcharts, sequence diagrams, class diagrams, state diagrams, ER diagrams, Gantt charts, pie charts, mind maps, timelines, git graphs, and more."},
			{title: "Config Support", description: "Use YAML frontmatter inside mermaid blocks to customize themes, colors, and layout options."},
		},
		aside: "Quick insert from Insert > Diagrams menu, or type «```mermaid» directly.",
	},
	{
		icon: "📈", title: "CHARTS & LIVE DATA",
		features: []startPreviewFeature{
			{title: "Vega-Lite Charts", description: "Drop a «```vega-lite» spec for bar, line, scatter, pie, heatmap and more - with colors, tooltips, and hover. Insert from + › Charts or the palette."},
			{title: "Charts From Your Data", description: `Point a chart's «"data": {"url": ...}» at a vault «.json»/«.csv» or a web URL to load its data in preview.`},
			{title: "Live API Data", description: "A «```api» block is an HTTP request you Run from the preview; the response renders inline as a table or JSON."},
			{title: "Vault Queries", description: "A «```query» block builds a live, Dataview-style index of your notes by tag, folder, or property."},
		},
	},
	{
		icon: "🔗", title: "VAULT & WIKILINKS",
		features: []startPreviewFeature{
			{title: "Open Vault", description: "Browse any folder of Markdown files. Click the folder icon in the toolbar or use the Files tab in the sidebar."},
			{title: "[[Wikilinks]]", description: "Click to open linked files. Use «[[page|display]]» to set a label or «[[folder/page]]» to specify a path."},
			{title: "Callouts & Tags", description: "«> [!note]» callout blocks with 14 types. «#tags» render as styled pills."},
			{title: "Image Embeds", description: "Use «![[image.png]]» to show a vault image in the rendered note."},
			{title: "Knowledge Graph", description: "Interactive graph of vault files and wikilink connections. Open from the toolbar or sidebar."},
			{title: "Backlinks & Quick Switcher", description: "See which files link to the current page. Quickly jump between vault files."},
		},
	},
	{
		icon: "📄", title: "PREVIEW & EXPORT",
		features: []startPreviewFeature{
			{title: "Export Formats", description: "Export to PDF, Word (.docx), or HTML. Use «⌘E» for quick PDF or «⌘⇧E» for all options."},
			{title: "Custom Themes", description: "Choose a built-in theme or create your own in Settings → Themes."},
			{title: "PDF Preview & Document Map", description: "Live PDF preview with real-time updates. Grid view to see all pages at once."},
			{title: "CSS Styling", description: "Inline HTML/CSS for colored text, highlights, underlines, and custom layouts in PDF exports."},
		},
	},
	{
		icon: "💡", title: "POWER FEATURES",
		features: []startPreviewFeature{
			{title: "🖼️ Drag & Drop Images", description: "Drag images into the editor or paste from clipboard. Auto-saved next to your document."},
			{title: "📑 Auto Table of Contents", description: "Generate a clickable TOC from headings with «⌘⇧T»."},
			{title: "🎯 Focus Mode", description: "Press «⌘⇧F» to focus on the editor and preview."},
			{title: "🔄 Session Restore", description: "Return to your workspace tabs after relaunch."},
			{title: "📝 Special Syntax", description: "«[pagebreak]» for page breaks, «- [ ]» for task lists, GFM tables, fenced code blocks with syntax highlighting."},
			{title: "🧮 LaTeX Math", description: "Inline «$E=mc^2$» and block «$$...$$» math expressions rendered with KaTeX."},
			{title: "🎨 Diagram Colors", description: "Use a Mermaid card's Background menu to change its background color."},
			{title: "📋 Custom Templates", description: "Save any document as a reusable template. Your templates appear in the template picker and File > New from Template."},
			{title: "🎬 Slides & Flash Cards", description: "Present any document as slides, or study it as spaced-repetition flash cards, from the ▶ Play menu in the editor toolbar."},
			{title: "✂️ Snippets", description: "Capture reusable text and pin favorites. Open the snippets shade with «⌘⌥S», or embed and link snippets inline."},
			{title: "📑 Properties & Frontmatter", description: "YAML frontmatter shows as an editable Properties panel, and «```query» blocks turn those fields into live tables."},
			{title: "⌘ Command Palette", description: "Press «⌘/» to search and execute any command - formatting, diagrams, templates, view modes, settings, and more."},
		},
	},
}
