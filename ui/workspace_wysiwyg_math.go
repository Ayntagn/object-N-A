package ui

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

const visualMathMargin = 4
const visualMathMono = "SF Mono, SFMono-Regular, Menlo, Monaco, Consolas, monospace"

type visualMathState struct {
	inline          map[string]visualInlineBox
	layoutRevision  uint8
	popup           visualInlinePopup
	input           visualMathInput
	focusBlock      string
	newFence        *wysiwyg.Formula
	newFenceVersion uint64
	hidden          map[string]bool
	activeSource    *visualDisplayMath
	appearance      visualBlockColors
}

type visualInlineBox struct {
	frame    mygoui.Rect
	descent  float32
	image    mygoui.ImageSource
	fallback *mygoui.TextLayout
	ink      mygoui.Color
	selected bool
}

type visualInlinePopup struct {
	id, value string
	formula   *wysiwyg.Formula
}

type visualMathInput struct {
	doc     *richtext.Document
	version uint64
}

func (m *visualMathState) style(tag string, span *mygoui.Span) {
	if box, ok := m.inline[tag]; ok {
		span.ObjectDescent = box.descent
	}
}

func (w *workspaceWYSIWYG) mathTagStyle(tag string, span *mygoui.Span) {
	w.math.style(tag, span)
	if tag == wysiwyg.FormulaColorTag {
		span.Font, span.Size, span.Color = visualMathMono, float32(w.fontSize())*.85, w.math.appearance.ink.Alpha(.5)
	}
}

func (m *visualMathState) relayout(c *mygoui.Context) {
	m.layoutRevision++
	c.Invalidate()
}

// EV-UI-152/153: closing a dollar pair or typing space after a bare $$
// acts on committed body input. Applying it to an undo or source reload
// would immediately recreate the object the user just removed.
func (w *workspaceWYSIWYG) watchVisualInput(app *workspaceWindow, edit mygoui.Element, doc *richtext.Document) {
	edit.ObserveInput(func(ev mygoui.InputEvent) {
		if ev.Kind == mygoui.InputPointerDown && ev.Button == 0 {
			w.tableSelection = visualTableSelection{}
		}
		if ev.Kind == mygoui.InputText && w.editable(app) {
			w.session.PrepareCalloutTyping()
		}
		if ev.Kind == mygoui.InputText && strings.TrimRight(ev.Text, "$ \t") != ev.Text {
			w.math.input = visualMathInput{doc, doc.Version()}
		} else {
			w.math.input = visualMathInput{}
		}
	})
	edit.OnChange(func() {
		input := w.math.input
		w.math.input = visualMathInput{}
		if input.doc != doc || input.version == doc.Version() || !edit.Focused() || !w.editable(app) {
			return
		}
		a, z := doc.Selection()
		text := doc.Text()
		start, end := text.ParagraphRange(z)
		if a != z || start == z || text.ParagraphStyleAt(start).CodeBlock || text.AttributesAt(z-1).Code {
			return
		}
		value := text.Slice(start, z).String()
		style := text.ParagraphStyleAt(start)
		if (value == "$$ " || value == "$$\t") && z == end && style.Heading == 0 && !style.Quote && style.List == richtext.NoList {
			doc.SetSelection(start, z)
			w.insertVisualDisplayMath(app, "")
			return
		}
		if offset, source, ok := closingInlineFormula(value); ok {
			start += utf8.RuneCountInString(value[:offset])
			for _, run := range text.Runs(start, z) {
				if run.Attributes.Code || run.Attributes.Attachment != nil {
					return
				}
			}
			w.session.ReplaceInlineFormula(start, z, source)
			w.sync(app)
		}
	})
}

var closingInlineMath = regexp.MustCompile(`(?:^|[^$\\])\$([^$\n]+)\$$`)

func closingInlineFormula(value string) (int, string, bool) {
	match := closingInlineMath.FindStringSubmatchIndex(value)
	if match == nil {
		return 0, "", false
	}
	return match[2] - 1, value[match[2]:match[3]], true
}

func (w *workspaceWYSIWYG) drawInlineFormulas(c *mygoui.Context, app *workspaceWindow, edit mygoui.Element, renderer *workspaceMathRender,
	font mygoui.Font, viewport mygoui.Rect, padding float32, text richtext.Text, boxes []richtext.ParagraphBox, anchor, caret int) {
	selected, selectionOK := wysiwyg.FormulaAt(text, min(anchor, caret))
	selectedAt := min(anchor, caret)
	if !selectionOK || selected.Block || max(anchor, caret)-min(anchor, caret) != 1 || !w.editable(app) {
		selected = nil
		w.math.popup = visualInlinePopup{}
	}
	var openAt mygoui.Rect
	var visible map[string]bool
	editable := w.editable(app)
	colors := w.math.appearance
	for _, box := range boxes {
		_, end := text.ParagraphRange(box.Start)
		for offset, run := range text.Runs(box.Start, end) {
			object := run.Attributes.Attachment
			if object == nil {
				continue
			}
			f, ok := object.Data.(*wysiwyg.Formula)
			if !ok || f.Block {
				continue
			}
			if visible == nil {
				visible = make(map[string]bool)
			}
			visible[f.Tag()] = true
			// The suffix is a separate text run and remains part of the note.
			tail := text.Slice(offset+1, min(end, offset+32)).String()
			ink, length := wysiwyg.FormulaColor(tail, colors.ink)
			if length == 0 {
				ink = colors.accent
			}
			wrap := max(1, int(viewport.W-2*padding-8))
			w.measureInlineFormula(c, app, renderer, f, font.Size, ink, f == selected && offset == selectedAt, wrap)
			r := w.math.inline[f.Tag()].frame
			r.X, r.Y = r.X-viewport.X, r.Y-viewport.Y
			if r.W <= 0 || r.H <= 0 || r.Y+r.H <= 0 || r.Y >= viewport.H {
				continue
			}
			if f == selected && offset == selectedAt {
				openAt = r
			}
			if editable {
				mygoui.ButtonBase(c.Key(f.Tag()).Key(offset)).Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H).
					Label("Inline formula " + f.Source).AccessSelected(f == selected && offset == selectedAt).KeepFocus().TabStop(false).
					Cursor(mygoui.CursorPointer).OnClick(func() {
					if w.editable(app) {
						w.session.Document.SetSelection(offset, offset+1)
						w.focus = true
					}
				})
			}
		}
	}
	for tag := range w.math.inline {
		if !visible[tag] {
			delete(w.math.inline, tag)
		}
	}
	if !editable {
		return
	}
	doc := w.session.Document
	if selected != nil {
		offset := min(anchor, caret)
		edit.OnShortcut(0, mygoui.KeyEscape, func() {}).OnShortcut(0, mygoui.KeyRight, func() {
			doc.SetSelection(offset+1, offset+1)
		}).OnShortcut(0, mygoui.KeyLeft, func() {
			doc.SetSelection(offset, offset)
		}).OnShortcut(0, mygoui.KeyEnter, func() {
			w.session.SplitBeforeInlineFormula(offset)
			w.sync(app)
		})
		if openAt.W > 0 && !w.blockMenu.open && !app.commandPalette.open && !app.quickSwitcher.open && !w.linkOpen {
			w.drawInlineMathPopup(c, app, colors, viewport, openAt, offset, selected)
		}
	} else if anchor == caret {
		if before, ok := wysiwyg.FormulaAt(text, caret-1); ok && !before.Block {
			edit.OnShortcut(0, mygoui.KeyLeft, func() { doc.SetSelection(caret-1, caret) })
		}
	}
}

func (w *workspaceWYSIWYG) measureInlineFormula(c *mygoui.Context, app *workspaceWindow, renderer *workspaceMathRender,
	f *wysiwyg.Formula, size float32, ink mygoui.Color, selected bool, wrap int) {
	img, _ := renderer.formula(app.win, f.Source, false, size, ink, wrap)
	var fallback *mygoui.TextLayout
	width, ascent, descent := img.Width, img.Ascent, img.Height-img.Ascent
	if img.Image == nil {
		value := f.Source
		if value == "" {
			value = "$$"
		}
		font := mygoui.Font{Family: visualMathMono, Size: size, LineHeight: 1.4}
		fallback = mygoui.ShapeText(value, font, float32(wrap))
		var height float32
		width, height = fallback.Size()
		ascent, descent = size, max(size*.4, height-size)
	}
	const inset = 4
	ascent, descent = ascent+2, descent+2
	tag := f.Tag()
	if w.math.inline == nil {
		w.math.inline = make(map[string]visualInlineBox)
	}
	boxes, services := w.math.inline, c.Services()
	metrics := boxes[tag]
	metrics.descent, metrics.image, metrics.fallback = descent, img.Image, fallback
	metrics.ink, metrics.selected = ink, selected
	boxes[tag] = metrics
	changed := f.SetVisual(min(float32(wrap), width)+2*inset, ascent, descent, func(p *mygoui.Painter, r mygoui.Rect) {
		box, ok := boxes[tag]
		if !ok {
			return
		}
		if box.frame != r {
			box.frame = r
			boxes[tag] = box
			services.Invalidate()
		}
		if box.selected {
			p.Fill(r, box.ink.Alpha(.15), 0)
		}
		r.X += inset
		r.W = max(1, r.W-2*inset)
		r.Y += 2
		r.H = max(1, r.H-4)
		if box.image != nil {
			p.Image(box.image, r, mygoui.Contain)
		} else if box.fallback != nil {
			_, h := box.fallback.Size()
			p.Clip(r, 0, func() { p.TextLayout(box.fallback, r.X, r.Y+(r.H-h)/2, box.ink) })
		}
	})
	if changed {
		w.math.relayout(c)
	}
}

// EV-UI-152: the one-line TeX field is 174 points wide until its content
// grows, and the picker stays anchored to the selected inline object.
func (w *workspaceWYSIWYG) drawInlineMathPopup(c *mygoui.Context, app *workspaceWindow, colors visualBlockColors,
	viewport, anchor mygoui.Rect, offset int, f *wysiwyg.Formula) {
	state := &w.math.popup
	if state.id != f.ID || state.formula != f {
		*state = visualInlinePopup{f.ID, f.Source, f}
	}
	font := mygoui.Font{Family: visualMathMono, Size: 15, LineHeight: 1.2}
	width, _ := mygoui.ShapeText(state.value, font, 0).Size()
	fieldWidth := min(max(1, viewport.W-52), max(174, min(294, width)))
	lines := len(mygoui.ShapeText(state.value, font, fieldWidth).Lines())
	fieldHeight := float32(max(1, min(6, lines)) * 18)
	width = fieldWidth + 48
	height := max(24, fieldHeight) + 4
	x := max(0, min(viewport.W-width, anchor.X+(anchor.W-width)/2))
	y := anchor.Y + anchor.H + 10
	if y+height > viewport.H {
		y = max(0, anchor.Y-height-10)
	}
	commit := func(value string) {
		if w.editable(app) {
			if next, ok := w.session.ChangeFormula(offset, f, value, f.Language); ok {
				state.formula = next
				w.sync(app)
			}
			state.value = value
			w.focus = true
		}
	}
	mygoui.Row(c.Key("inline formula popover")).Absolute().Left(x).Top(y).Size(width, height).
		Padding(2, 6, 2, 12).AlignItems(mygoui.Start).Gap(6).Role(mygoui.RoleGroup).Label("Edit inline formula").
		Background(colors.surface).Radius(8).Shadow(0, 2, 6, 0, mygoui.RGBA(0, 0, 0, .12)).Children(func() {
		mygoui.TextAreaBase(c.Key(f.ID), &state.value).Label("Inline formula source").Size(fieldWidth, fieldHeight).
			MinHeight(0).Padding(0).Font(visualMathMono).FontSize(15).FixedLineHeight(18).FocusRing(false).
			TextColor(colors.ink).OnTextKey(func(key mygoui.TextKey) bool {
			if key.Key == mygoui.KeyEnter && key.Mods == 0 {
				commit(key.Text())
				return true
			}
			return key.Key == mygoui.KeyEscape || key.Key == mygoui.KeyEnter && key.Mods&(mygoui.Cmd|mygoui.Ctrl) != 0
		})
		check := mygoui.Hex("#817567")
		mygoui.ButtonBase(c.Key("apply inline formula")).Size(24, 24).Radius(8).KeepFocus().
			Label("Apply formula").Cursor(mygoui.CursorPointer).OnClick(func() { commit(state.value) }).
			DrawOver(func(p *mygoui.Painter, r mygoui.Rect) {
				p.StrokePolyline([][2]float32{{r.X + 6, r.Y + 12}, {r.X + 10, r.Y + 16}, {r.X + 18, r.Y + 8}}, 2, check)
			})
	})
}

func (w *workspaceWYSIWYG) insertVisualDisplayMath(app *workspaceWindow, source string) {
	doc := w.session.Document
	a, z := doc.Selection()
	atEnd := max(a, z) == doc.Text().Len()
	if f, ok := w.session.InsertDisplayFormula(source); ok {
		w.sync(app)
		w.focus = !atEnd
		if atEnd {
			w.math.focusBlock = f.ID
		}
	}
}
