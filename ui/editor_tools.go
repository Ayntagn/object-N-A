package ui

import (
	"strings"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

var blackoutOutlineInk = mygoui.RGB(196, 198, 208)

// findMode is a request to focus a field the next time the find bar is built.
type findMode uint8

const (
	findClosed findMode = iota
	findQuery
	findReplace
	findReplaceQuery
)

type outlineHeading struct {
	title  string
	level  int
	offset int
}

type outlineCache struct {
	document *parser.Document
	visual   *richtext.Document
	version  uint64
	headings []outlineHeading
}

func (cache *outlineCache) readVisual(document *richtext.Document) {
	version := document.Version()
	if cache.visual == document && cache.version == version {
		return
	}
	cache.visual, cache.version = document, version
	cache.headings = cache.headings[:0]
	text := document.Text()
	for offset, paragraph := range text.Paragraphs() {
		// The original WYS jump skips block quotes, including Callout contents.
		if paragraph.Style.Heading == 0 || paragraph.Style.Quote {
			continue
		}
		_, end := text.ParagraphRange(offset)
		title := strings.TrimSpace(text.Slice(offset, end).PlainText())
		if title != "" {
			cache.headings = append(cache.headings, outlineHeading{title: title, level: paragraph.Style.Heading, offset: offset})
		}
	}
}

func (app *workspaceWindow) newSourceEditor(tab *workspaceTab) *editor.State {
	tab.draft.SetLanguage(app.sourceLanguage())
	return &editor.State{
		Buffer:          tab.draft,
		Label:           "Source editor",
		SoftWrap:        true,
		LineNumbers:     editor.LineNumbersOff,
		Whitespace:      editor.WhitespaceNone,
		NoIndentGuides:  true,
		NoScrollPastEnd: true,
		NoAutoClose:     true,
		NoActiveLine:    true,
		OnInput:         app.sourceInput,
		ContextMenu:     app.sourceContextMenu,
	}
}

func (app *workspaceWindow) sourceContextMenu(m *mygoui.Menu) {
	m.Separator()
	m.Submenu("Insert", app.insertMenu).Disabled(!app.sourceEditing())
	app.snippetCaptureMenu(m)
}

func (app *workspaceWindow) source(c *mygoui.Context) {
	if app.code == nil || app.mode == previewMode || app.mode == wysiwygMode {
		return
	}
	mygoui.Column(c.Key("source")).Grow(1).MinWidth(0).MinHeight(0).Children(func() {
		if !app.focusMode {
			app.formatToolbar(c)
		}
		mygoui.Column(c.Key("source content")).Grow(1).MinHeight(0).MinWidth(0).Children(func() {
			startPage := app.showsStartPage()
			editor := mygoui.Column(c.Key("editable source")).Grow(1).MinHeight(0).MinWidth(0)
			if startPage && !c.Preferences().HighContrast {
				// EV-UI-315: the focused empty editor remains visible below the picker.
				editor.Opacity(.12)
			}
			editor.Children(func() { app.drawSourceEditor(c, app.workspaceTab, "Source editor") })
			if startPage {
				width, _ := app.paneToolWidths(c)
				mygoui.Column(c.Key("start writing overlay")).Absolute().Left(0).Top(0).Right(0).Bottom(0).
					PassThrough().Clip().Children(func() { app.drawStartWriting(c, width) })
			}
		})
	})
}

func (app *workspaceWindow) drawSourceEditor(c *mygoui.Context, tab *workspaceTab, label string) {
	language := app.sourceLanguage()
	if tab.inspection && app.previewDocumentType() == documentPlainText {
		language = nil
	}
	code := tab.code
	tab.draft.SetLanguage(language)
	code.Label = label
	code.Theme = app.sourceEditorTheme(tab, c.Theme().Dark)
	code.Font.Size = float32(app.sourceFontSize())
	code.Font.Family = sourceEditorFontFamily(app.preferences.EditorFont)
	code.LineHeight = app.sourceEditorLineHeight(code.Font)
	code.LineNumbers = editor.LineNumbersOff
	if app.preferences.LineNumbers && app.blackout && !c.Preferences().HighContrast && sourceThemeForID(app.preferences.EditorTheme).id == systemSourceThemeID {
		theme := *code.Theme
		theme.LineNumber, theme.ActiveLineNumber = blackoutOutlineInk, blackoutOutlineInk
		code.Theme = &theme
	}
	code.NoScrollPastEnd = !app.preferences.ScrollPastEnd
	tab.applySourceHexColors(language)
	app.applySourceSpellCheck(c, tab)
	find := mygoui.Column(c.Key("document find")).Hidden(true)
	// EV-UI-023: opening Find shrinks the text viewport. Put the MyGo
	// controls in a sibling above it using the current frame's finder.
	code.RenderFindBar = func(c *mygoui.Context, floating mygoui.Element, f *editor.FindBar) {
		floating.Hidden(true)
		if tab == app.workspaceTab {
			app.documentFind(c, find.Hidden(false), f)
		}
	}
	viewport := mygoui.Row(c.Key("source viewport")).Grow(1).MinWidth(0).MinHeight(0).AlignItems(mygoui.Stretch)
	viewport.Children(func() {
		if app.preferences.LineNumbers {
			drawSourceLineNumbers(c, viewport, code)
		}
		source := editor.View(c.Key(code), code).Grow(1).MinHeight(0).MinWidth(0).Margin(sourceTextTopInset, 0, 0)
		if app.multiFile.active {
			source.IgnoreInput(app.modalOpen)
			source.ObserveInput(func(event mygoui.InputEvent) {
				switch event.Kind {
				case mygoui.InputPointerDown:
					app.selectMultiFileTile(tab)
					if event.Button == 0 && !app.modalOpen {
						app.focusSource = true
					}
				case mygoui.InputKeyDown, mygoui.InputText, mygoui.InputCommand:
					app.selectMultiFileTile(tab)
				}
			})
			// Tab traversal and assistive technology can focus an editor
			// before it receives a key. Route commands to that editor too.
			if source.Focused() && !app.focusSource {
				app.selectMultiFileTile(tab)
			}
		}
		app.sourceTypewriter(c, source, code)
		code.RenderFindBar = nil
		if app.focusSource && tab == app.workspaceTab {
			source.Focus()
			app.focusSource = false
		}
	})
}

func (app *workspaceWindow) registerEditorShortcuts(c *mygoui.Context) {
	if app.code == nil || app.modalOpen {
		return
	}
	c.OnWindowShortcut(mygoui.Cmd, mygoui.KeyF, func() { app.showFind(false) })
	c.OnWindowShortcut(mygoui.Cmd|mygoui.Alt, mygoui.KeyF, func() { app.showFind(true) })
	c.OnWindowShortcut(mygoui.Cmd|mygoui.Shift, mygoui.KeyK, app.insertImage)
	if app.displayedMode() == wysiwygMode && app.wys.find.open {
		c.OnWindowShortcut(mygoui.Cmd, mygoui.KeyG, func() { app.wys.find.step(false) })
		c.OnWindowShortcut(mygoui.Cmd|mygoui.Shift, mygoui.KeyG, func() { app.wys.find.step(true) })
	}
}

func (app *workspaceWindow) sourceInput(event mygoui.InputEvent) bool {
	if event.Kind == mygoui.InputText && event.Text != "" && !app.multiFile.active &&
		app.document.Path == "" && app.sourceEditing() && app.showsStartPage() {
		app.startTitleFromSource = true
	}
	if app.snippets.open && event.Kind == mygoui.InputKeyDown && event.Key == mygoui.KeyEscape && event.Mods == 0 {
		app.snippets.open = false
		return true
	}
	if event.Kind == mygoui.InputKeyDown && event.Key == mygoui.KeyF {
		switch event.Mods {
		case mygoui.Cmd:
			app.showFind(false)
			return true
		case mygoui.Cmd | mygoui.Alt:
			app.showFind(true)
			return true
		}
	}
	if event.Kind == mygoui.InputKeyDown && event.Key == mygoui.KeySemicolon && event.Mods == mygoui.Cmd {
		app.toggleLineNumbers()
		return true
	}
	if app.applySourceMarkdownShortcut(event) || app.completeSourceCodeFence(event) {
		return true
	}
	return app.formattingInput(event) || app.autoIndentSourceParagraph(event)
}

func (app *workspaceWindow) showFind(replace bool) {
	if app.code == nil || app.modalOpen {
		return
	}
	mode := app.displayedMode()
	if mode == wysiwygMode {
		if !app.wys.editable(app) {
			return
		}
		if app.wys.find.open && !replace {
			app.wys.closeFind()
		} else {
			app.wys.find.open = true
			app.wys.find.focus = true
		}
		return
	}
	if mode == previewMode {
		if app.multiFile.active {
			app.editMultiFileRendered(app.workspaceTab)
			if app.displayedMode() == wysiwygMode {
				app.wys.find.open, app.wys.find.focus = true, true
			}
			return
		}
		app.setMode(splitMode)
	}
	app.focusSource = false
	if replace {
		app.findMode = findReplace
		app.code.Run(editor.FindReplace)
	} else {
		app.findMode = findQuery
		app.code.Run(editor.Find)
	}
}

func (app *workspaceWindow) documentFind(c *mygoui.Context, bar mygoui.Element, f *editor.FindBar) {
	request := app.findMode
	app.findMode = findClosed
	if request != findClosed {
		f.SetReplacing(request != findQuery)
	}
	replacing := f.Replacing()
	height := float32(32)
	if replacing {
		height = 55
	}
	theme := c.Theme()
	field := func(value *string, label string) mygoui.Element {
		return mygoui.TextInputBase(c.Key(value), value).Height(18).Grow(1).MinWidth(24).
			Padding(0, 5).FontSize(12).Radius(4).Background(theme.Background).Border(1, theme.Border).
			Label(label).Placeholder(label)
	}
	button := func(text, label string) mygoui.Element {
		return mygoui.Button(c, text).Height(22).Padding(1, 6).FontSize(12).
			MinWidth(0).Shrink(1).Clip().KeepFocus().Label(label)
	}
	count, _ := f.Matches()
	bar.Height(height).Shrink(0).Justify(mygoui.Center).Gap(1).Padding(0, 8).
		Background(theme.Surface).Role(mygoui.RoleToolbar).Label("Find in document").Children(func() {
		mygoui.Row(c).Height(22).Gap(6).AlignItems(mygoui.Center).Children(func() {
			query := f.QueryInput(field(f.Query(), "Find in document..."))
			if request == findQuery || request == findReplaceQuery {
				query.Focus()
			}
			if status := f.Status(); status != "" {
				mygoui.Text(c, status).Role(mygoui.RoleStatus).FontSize(11).TextColor(theme.TextMuted).
					SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
			}
			if count > 0 {
				button("↑", "Previous match").OnClick(f.Previous)
				button("↓", "Next match").OnClick(f.Next)
			}
			if !replacing {
				button("Replace", "Replace").OnClick(func() { app.findMode = findReplace })
			}
			button("×", "Close").OnClick(f.Close)
		})
		if replacing {
			mygoui.Row(c).Height(22).Gap(6).AlignItems(mygoui.Center).Children(func() {
				replace := func(all bool) {
					if all {
						f.ReplaceAll()
					} else {
						f.Replace()
					}
				}
				button("⌃", "Go Up").OnClick(func() { app.findMode = findQuery })
				replacement := field(f.Replacement(), "Replace with...").
					OnSubmit(func() { replace(c.Modifiers()&mygoui.Cmd != 0) }).
					OnShortcut(0, mygoui.KeyEscape, f.Close)
				if request == findReplace {
					replacement.Focus()
				}
				enabled := count > 0
				button("Replace", "Replace").Disabled(!enabled).OnClick(func() { replace(false) })
				button("All", "All").Disabled(!enabled).OnClick(func() { replace(true) })
			})
		}
	})
}

func (app *workspaceWindow) outline(c *mygoui.Context) {
	if app.code == nil {
		return
	}
	var visual *richtext.Document
	if app.displayedMode() == wysiwygMode {
		app.wys.openSession(app)
		visual = app.wys.session.Document
	}
	cache := &app.outlineCache
	if visual != nil {
		cache.readVisual(visual)
	} else if version := app.draft.Version(); cache.visual != nil || cache.document == nil || cache.version != version {
		if cache.document == nil {
			cache.document = parser.New(parser.GFM)
		}
		cache.visual, cache.version = nil, version
		cache.document.Set(app.workingText())
		cache.headings = cache.headings[:0]
		for i := 1; i < cache.document.Len(); i++ {
			block := cache.document.Block(i)
			if block.Kind != parser.Heading {
				continue
			}
			title := strings.TrimSpace(parser.PlainText(cache.document.Inlines(i)))
			if title != "" {
				cache.headings = append(cache.headings, outlineHeading{title: title, level: block.Level, offset: block.Start})
			}
		}
	}
	version := cache.version
	theme := c.Theme()
	background := workspaceChromeSurface(c)
	if palette, ok := appearancePalettes[app.appAppearance()]; ok {
		background = palette.sidebar
	}
	headerInk, closeInk, headingInk, markerInk := theme.Text, theme.Text, theme.Text, theme.Accent
	panel := mygoui.Column(c.Key("outline")).Width(outlinePanelWidth).Shrink(0).MinHeight(0).
		Background(background).Label("Outline")
	headerHeight := float32(documentBarHeight)
	blackout := app.blackout && !c.Preferences().HighContrast
	if blackout {
		// Blackout's Outline card starts 4 points above the toolbar buttons.
		// EV-UI-216: the card's offset depends on whether the Vault is visible.
		inset := float32(6)
		if !app.vaultPanelVisible() {
			inset = 4
		}
		panel.Margin(4, -inset, 0, inset).Background(mygoui.RGB(3, 3, 6)).Radius(8).Clip().
			Draw(func(p *mygoui.Painter, r mygoui.Rect) {
				border := mygoui.Rect{X: r.X + 0.5, Y: r.Y + 0.5, W: r.W - 1, H: r.H - 1}
				p.Stroke(border, mygoui.RGB(34, 35, 39), 7.5, 1)
			})
		headerHeight = 30
		headerInk, closeInk = blackoutOutlineInk, blackoutToolbarGlyphInk
		headingInk, markerInk = mygoui.RGB(227, 226, 230), mygoui.RGB(1, 145, 255)
	}
	panel.Children(func() {
		mygoui.Row(c).Height(headerHeight).Shrink(0).Padding(0, 12, 0, 8).AlignItems(mygoui.Center).Children(func() {
			mygoui.Box(c).Size(26, 22).Shrink(0).MarginEnd(4.5).AlignItems(mygoui.Center).Justify(mygoui.Center).
				Children(func() { editorViewSymbol(c, "list.bullet.indent", headerInk) })
			mygoui.Text(c, "Outline").Role(mygoui.RoleHeading).Level(2).FontSize(13).FontWeight(700).TextColor(headerInk).
				MinWidth(0).SingleLine()
			mygoui.Spacer(c)
			chromeControl(c, "Close Outline", false).KeepFocus().OnClick(func() { app.outlineVisible = false }).Children(func() {
				editorViewSymbol(c, "xmark", closeInk)
			})
		})
		if len(cache.headings) == 0 {
			mygoui.Column(c).Grow(1).MinHeight(0).AlignItems(mygoui.Center).Justify(mygoui.Center).Children(func() {
				mygoui.Text(c, "No headings").FontSize(13).TextColor(theme.TextMuted)
			})
			return
		}
		mygoui.Scroll(c.Key(app.code)).Grow(1).MinHeight(0).Padding(3, 0, 0).Label("Outline headings").Children(func() {
			for _, heading := range cache.headings {
				height := float32(28)
				if blackout && heading.level <= 3 {
					// EV-UI-208. The first three levels have distinct row heights.
					height = float32(29 - heading.level)
				}
				row := mygoui.ButtonBase(c.Key(heading.offset)).Role(mygoui.RoleButton).Label(heading.title).
					Height(height).FillWidth().AlignItems(mygoui.Center).Justify(mygoui.Start).Gap(4).
					Padding(0, 8).PaddingStart(float32(12 + (heading.level-1)*10))
				if row.Hovered() {
					row.Background(theme.SurfaceHover)
				}
				row.OnClick(func() {
					if visual != nil {
						if app.wys.editable(app) && app.wys.session.Document == visual && visual.Version() == version {
							// EV-UI-208: keep the visual editor and reveal a caret at the heading's start.
							visual.SetSelection(heading.offset, heading.offset)
							app.wys.focus = true
						}
						return
					}
					if app.mode == previewMode || app.mode == wysiwygMode {
						app.setMode(splitMode)
					}
					app.code.GoTo(app.draft.Pos(heading.offset))
					app.focusSource = true
				}).Children(func() {
					if heading.level == 1 {
						mygoui.Box(c).Size(4, 4).Shrink(0).Radius(2).Background(markerInk).Role(mygoui.RoleNone)
					}
					title := mygoui.Text(c, heading.title).FontSize(13).TextColor(headingInk).
						Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
					if heading.level == 1 {
						title.FontWeight(700)
					}
				})
			}
		})
	})
}
