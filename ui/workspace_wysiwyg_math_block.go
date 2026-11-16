package ui

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/richtext"
)

type visualDisplayMath struct {
	formula     *wysiwyg.Formula
	source      editor.State
	offset      int
	language    bool
	query       string
	searchFocus bool
}

var latexSourceTokens = regexp.MustCompile(`\\(?:[A-Za-z]+|.)|%.*|[0-9]+(?:\.[0-9]+)?`)

var latexSourceLanguage = editor.Language{
	Name: "LaTeX", LineComment: "%", TabSize: 2,
	Tokenizer: editor.TokenizerFunc(func(line string, _ uint64, tokens []editor.Token) ([]editor.Token, uint64) {
		end := 0
		for _, span := range latexSourceTokens.FindAllStringIndex(line, -1) {
			if span[0] > end {
				tokens = append(tokens, editor.Token{End: span[0], Kind: editor.TokenText})
			}
			kind := editor.TokenNumber
			switch line[span[0]] {
			case '\\':
				kind = editor.TokenFunction
			case '%':
				kind = editor.TokenComment
			}
			end = span[1]
			tokens = append(tokens, editor.Token{End: end, Kind: kind})
		}
		return tokens, 0
	}),
}

// EV-UI-153: source and preview occupy the same block. The source uses the
// app's code editor, and the preview uses the same KaTeX engine as inline math.
func (w *workspaceWYSIWYG) drawDisplayFormula(c *mygoui.Context, app *workspaceWindow, renderer *workspaceMathRender,
	padding, viewportWidth float32, box richtext.ParagraphBox, f *wysiwyg.Formula) {
	const inset, toolbarHeight = 20, 30
	const gutterWidth, editorInset = float32(31), float32(8)
	if viewportWidth <= 0 {
		return
	}
	width := max(1, viewportWidth-2*(padding+inset))
	theme := app.resolvePreviewTheme(app.previewThemeSelection())
	colors := w.math.appearance
	fill, ink := mygoui.Hex(theme.CodeBackgroundColor), mygoui.Hex(theme.CodeTextColor)
	loadBundledFont(colors.font)
	font := colors.font + ", sans-serif"
	readOnly := !w.editable(app)
	preview := f.IsLaTeX() && strings.TrimSpace(f.Source) != ""
	showSource := !w.math.hidden[f.ID] || !preview
	var img workspaceMathImage
	var fallback *mygoui.TextLayout
	previewWidth, previewHeight := float32(0), float32(0)
	if preview {
		img, _ = renderer.formula(app.win, f.Source, true, float32(w.fontSize()), ink, 0)
		previewWidth, previewHeight = img.Width, img.Height
		if img.Image == nil {
			fallback = mygoui.ShapeText(f.Source, mygoui.Font{Family: visualMathMono, Size: 15, LineHeight: 1.4}, 0)
			previewWidth, previewHeight = fallback.Size()
		}
	}
	previewLineHeight := previewHeight
	if img.Image != nil {
		// The raster extends a point on each side of the KaTeX line.
		previewLineHeight -= 2
	}
	previewRowHeight := max(21, previewLineHeight) + 30
	height := float32(8 + inset)
	height += toolbarHeight
	var sourceHeight float32
	var lineNumbers []int
	if showSource {
		sourceFont := mygoui.Font{Family: "monospace", Size: 15, LineHeight: 1.4}
		wrap := max(1, width-gutterWidth-4)
		rows := 0
		for _, line := range strings.Split(f.Source, "\n") {
			lineNumbers = append(lineNumbers, rows)
			rows++
			if w, _ := mygoui.ShapeText(line, sourceFont, 0).Size(); w > wrap {
				rows += len(mygoui.ShapeText(line, sourceFont, wrap).Lines()) - 1
			}
			if rows >= 18 {
				break
			}
		}
		sourceHeight = 8.390625 + float32(max(1, min(rows, 18))*21)
		height += sourceHeight
	}
	if preview {
		height += previewRowHeight
		if showSource {
			height += 35
		}
	}
	if f.SetVisual(1, height+2*visualMathMargin, 0, nil) {
		w.math.relayout(c)
	}
	label := f.Language + " code block"
	if f.IsLaTeX() {
		label = "LaTeX formula"
	}
	card := mygoui.Box(c.Key(f.ID).Key(box.Start)).Absolute().Left(padding).Right(padding).Top(box.Top + visualMathMargin).
		Height(height).Background(fill).Label(label).Role(mygoui.RoleGroup).PassThrough()
	state := mygoui.Local(card, "source", func() visualDisplayMath {
		return visualDisplayMath{source: editor.State{Buffer: editor.NewBuffer(f.Source), Font: mygoui.Font{Family: "monospace", Size: 15},
			LineHeight: 1.4, SoftWrap: true, Indent: editor.IndentSpaces, TabSize: 2, NoScrollPastEnd: true,
			NoFolding: true, NoAutoClose: true, NoIndentGuides: true, NoActiveLine: true, NoScrollbars: true,
			LineNumbers: editor.LineNumbersOff}}
	})
	buffer := state.source.Buffer
	state.offset = box.Start
	if state.formula != f {
		if buffer.String() != f.Source {
			buffer.Reset(f.Source)
		}
		language := editor.LanguageNamed(f.Language)
		if f.IsLaTeX() {
			language = &latexSourceLanguage
		}
		buffer.SetLanguage(language)
		state.formula = f
	} else if w.updateDisplayMath(state) {
		w.session.Sync(app.draft)
		c.Invalidate()
	}
	card.Children(func() {
		mygoui.Column(c).Fill().Padding(8, inset, inset).PassThrough().Children(func() {
			w.displayMathToolbar(c, app, card, box.Start, f, state, colors, font, preview, showSource)
			if showSource {
				state.source.Label = f.Language + " source"
				state.source.Theme, state.source.ReadOnly = displayMathSourceTheme(c, colors.surface, f.IsLaTeX()), readOnly
				state.source.OnInput = func(event mygoui.InputEvent) bool { return w.displayMathKey(app, box.Start, f, state, event) }
				mygoui.Row(c.Key("display math source row")).FillWidth().Height(sourceHeight).Shrink(0).Background(colors.surface).
					Children(func() {
						mygoui.Box(c).Width(gutterWidth - editorInset).Height(sourceHeight).Shrink(0).Clip().Role(mygoui.RoleNone).
							IgnoreInput(true).Background(mygoui.Hex("#282c34")).Draw(func(p *mygoui.Painter, r mygoui.Rect) {
							for i, row := range lineNumbers {
								y := r.Y + 4 + float32(row*21) - float32(state.source.ScrollY())
								p.Text(r.X+8, y, strconv.Itoa(i+1), 14, mygoui.Hex("#7d8799"))
							}
						})
						source := editor.View(c.Key("display math source"), &state.source).Grow(1).MinWidth(0).Height(sourceHeight)
						if w.math.focusBlock == f.ID && !readOnly {
							source.Focus()
							w.math.focusBlock = ""
						}
						if source.Focused() {
							w.math.activeSource = state
						}
					})
			}
			if !preview {
				return
			}
			if showSource {
				mygoui.Box(c).FillWidth().Height(1).Shrink(0).Margin(6, 0).Background(colors.separator).Role(mygoui.RoleNone)
				mygoui.Text(c, "PREVIEW").Font(font).FontSize(12).FontWeight(600).FixedLineHeight(16).Height(16).Shrink(0).
					TextColor(colors.ink.Alpha(.6)).Margin(0, 0, 6).IgnoreInput(true)
			}
			row := mygoui.ScrollHorizontal(c.Key("display math preview")).FillWidth().Height(previewRowHeight).
				Shrink(0).Role(mygoui.RoleGroup).Label("LaTeX preview")
			row.Children(func() {
				mygoui.Box(c).Width(max(width, previewWidth)).Height(previewRowHeight).IgnoreInput(true).
					Draw(func(p *mygoui.Painter, r mygoui.Rect) {
						x, y := r.X+(r.W-previewWidth)/2, r.Y+(r.H-previewHeight)/2
						if img.Image != nil {
							p.Image(img.Image, mygoui.Rect{X: x, Y: y, W: previewWidth, H: previewHeight}, mygoui.Contain)
						} else {
							p.TextLayout(fallback, x, y, ink)
						}
					})
			})
		})
	})
}

// A save or tab switch can arrive before the buffer's next paint.
func (w *workspaceWYSIWYG) updateDisplayMath(state *visualDisplayMath) bool {
	f := state.formula
	if f == nil {
		return false
	}
	value := state.source.Buffer.String()
	if value == f.Source {
		return false
	}
	if next, ok := w.session.ChangeFormula(state.offset, f, value, f.Language); ok {
		state.formula = next
		return true
	}
	return false
}

func displayMathSourceTheme(c *mygoui.Context, background mygoui.Color, latex bool) *editor.Theme {
	theme := editor.DefaultTheme(c.Theme())
	theme.Background = background
	theme.Text = mygoui.Hex("#abb2bf")
	theme.LineNumber = mygoui.Hex("#7d8799")
	theme.ActiveLineNumber = theme.LineNumber
	if latex {
		theme.Tokens[editor.TokenFunction].Color = mygoui.Hex("#e5c07b")
		theme.Tokens[editor.TokenNumber].Color = mygoui.Hex("#d19a66")
		theme.Tokens[editor.TokenComment].Color = theme.LineNumber
	}
	return theme
}

func (w *workspaceWYSIWYG) displayMathToolbar(c *mygoui.Context, app *workspaceWindow, card mygoui.Element, offset int,
	f *wysiwyg.Formula, state *visualDisplayMath, colors visualBlockColors, font string, preview, showSource bool) {
	mygoui.Row(c).FillWidth().Height(30).Shrink(0).Gap(2).AlignItems(mygoui.Start).PassThrough().Children(func() {
		language := mathToolbarButton(c, f.Language, "chevron.down", colors, font, 22).
			MinWidth(70).Radius(4).Background(colors.surface.Mix(colors.ink, .04)).HasPopup(mygoui.PopupListBox).Disabled(!w.editable(app)).
			OnClick(func() {
				state.language = !state.language
				state.query, state.searchFocus = "", state.language
			})
		w.displayMathLanguages(c, app, language, offset, f, state)
		mygoui.Box(c).Grow(1).IgnoreInput(true)
		copy := mathToolbarButton(c, "Copy", "doc.on.doc", colors, font, 24).Width(68.140625).Margin(3, 0).
			Radius(12).OnClick(func() { c.Services().WriteClipboard(state.source.Buffer.String()) })
		var toggle mygoui.Element
		if preview {
			copy.Radius(12, 4, 4, 12)
			label, symbol := "Edit", "pencil.and.outline"
			if showSource {
				label, symbol = "Hide", "eye.slash"
			}
			toggle = mathToolbarButton(c, label, symbol, colors, font, 24).Width(62.703125).Margin(3, 0).Radius(4, 12, 12, 4).
				OnClick(func() {
					if w.math.hidden == nil {
						w.math.hidden = make(map[string]bool)
					}
					if showSource {
						w.math.hidden[f.ID] = true
						w.focus = true
					} else {
						delete(w.math.hidden, f.ID)
					}
				})
		}
		if app.win != nil && !card.Hovered() && !state.language && !c.Preferences().HighContrast {
			for _, control := range []mygoui.Element{language, copy, toggle} {
				if !control.Focused() && !control.Hovered() {
					control.Opacity(0)
				}
			}
		}
	})
}

func mathToolbarButton(c *mygoui.Context, label, symbol string, colors visualBlockColors, font string, height float32) mygoui.Element {
	button := mygoui.ButtonBase(c.Key(label)).Label(label).Height(height).Shrink(0).Padding(0, 8).
		Gap(5).AlignItems(mygoui.Center).Justify(mygoui.Center).Cursor(mygoui.CursorPointer).KeepFocus().
		Background(colors.surface.Mix(colors.ink, .16))
	button.Children(func() {
		if height == 24 {
			chromeIconColor(c, symbol, 11, colors.ink.Alpha(.7)).IgnoreInput(true)
		}
		mygoui.Text(c, label).Font(font).FontSize(12).FontWeight(600).TextColor(colors.ink.Alpha(.7)).Role(mygoui.RoleNone).IgnoreInput(true)
		if height == 22 {
			chromeIconColor(c, symbol, 9, colors.ink.Alpha(.7)).IgnoreInput(true)
		}
	})
	return button
}

var visualMathLanguages = func() []string {
	names := []string{"LaTeX"}
	for _, language := range editor.Languages() {
		names = append(names, language.Name)
	}
	slices.SortFunc(names, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	return slices.Compact(names)
}()

func (w *workspaceWYSIWYG) displayMathLanguages(c *mygoui.Context, app *workspaceWindow, anchor mygoui.Element, offset int,
	f *wysiwyg.Formula, state *visualDisplayMath) {
	if !state.language {
		return
	}
	var choices []string
	for _, name := range visualMathLanguages {
		if strings.Contains(strings.ToLower(name), strings.ToLower(state.query)) {
			choices = append(choices, name)
		}
	}
	choose := func(name string) {
		state.language = false
		if next, ok := w.session.ChangeFormula(offset, f, state.source.Buffer.String(), name); ok {
			state.formula = nil
			delete(w.math.hidden, next.ID)
			w.sync(app)
		}
	}
	mygoui.PopoverBase(c.Key("math language picker"), anchor, &state.language, func(panel mygoui.Element) {
		panel.Width(198).Padding(6).Radius(8).Background(c.Theme().Surface).Label("Code language").Role(mygoui.RoleListBox).
			Shadow(0, 2, 6, 0, mygoui.RGBA(0, 0, 0, .12))
		mygoui.Column(c).FillWidth().Gap(4).Children(func() {
			input := mygoui.TextInput(c, &state.query).FillWidth().Placeholder("Search language").Label("Search language").
				OnSubmit(func() {
					if len(choices) > 0 {
						choose(choices[0])
					}
				})
			if state.searchFocus {
				state.searchFocus = false
				input.Focus()
			}
			mygoui.Scroll(c).FillWidth().Height(max(28, min(224, float32(len(choices))*28))).Children(func() {
				for _, name := range choices {
					mygoui.Button(c.Key(name), name).Role(mygoui.RoleOption).AccessSelected(strings.EqualFold(f.Language, name)).
						FillWidth().Height(28).Shrink(0).OnClick(func() { choose(name) })
				}
			})
		})
	})
}

func (w *workspaceWYSIWYG) displayMathKey(app *workspaceWindow, offset int, f *wysiwyg.Formula, state *visualDisplayMath, event mygoui.InputEvent) bool {
	if !w.editable(app) {
		return false
	}
	if w.math.newFence == f && w.math.newFenceVersion == w.session.Document.Version() &&
		markdownShortcutUndo(event) && !state.source.Buffer.CanUndo() {
		// The empty code editor has no edits yet. Undo its insertion in the
		// containing document and restore the opening line's focus.
		w.math.newFence = nil
		w.session.Document.Undo()
		w.sync(app)
		w.focus = true
		return true
	}
	if event.Kind != mygoui.InputKeyDown {
		return false
	}
	text := state.source.Buffer.String()
	if event.Key == mygoui.KeyEscape && event.Mods == 0 {
		return true
	}
	if event.Key == mygoui.KeyEnter && (event.Mods == mygoui.Cmd || event.Mods == mygoui.Ctrl) {
		w.session.ExitFormula(offset, false)
		w.sync(app)
		w.focus = true
		return true
	}
	if event.Key == mygoui.KeyBackspace && event.Mods == 0 && text == "" && f.Source == "" {
		w.session.ExitFormula(offset, true)
		w.sync(app)
		w.focus = true
		return true
	}
	selection := state.source.Selection()
	if event.Key == mygoui.KeyDown && event.Mods == 0 && selection.Empty() && selection.Head == len(text) {
		doc := w.session.Document
		if offset+1 < doc.Text().Len() {
			doc.SetSelection(offset+2, offset+2)
			w.focus = true
			return true
		}
	}
	return false
}
