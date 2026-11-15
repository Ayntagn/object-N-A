package ui

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"cyberwriter/internal/core/mermaid"
	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

const diagramSourceHint = "⌘Enter to commit · Esc to cancel · click outside to commit"
const diagramMargin = 16
const diagramPadding = 16

type workspaceDiagramEdit struct {
	diagram *wysiwyg.Diagram
	offset  int
	source  string
	focus   bool
	height  float32
	resize  *fencedSourceResize
	grip    mygoui.Handle
}

type fencedSourceResize struct{ y, height float32 }

func (w *workspaceWYSIWYG) finishDiagramEdit(app *workspaceWindow, commit, focus bool) {
	edit := w.diagramEdit
	if edit.diagram == nil {
		return
	}
	w.diagramEdit = workspaceDiagramEdit{}
	if commit && w.session != nil && w.session.Current(app.draft) {
		if w.session.ChangeDiagram(edit.offset, edit.diagram, edit.source, edit.diagram.Info) {
			w.session.Sync(app.draft)
		}
	}
	w.diagramLayout = !w.diagramLayout
	w.focus = focus
}

func (w *workspaceWYSIWYG) beginDiagramEdit(app *workspaceWindow, offset int, diagram *wysiwyg.Diagram) {
	if !w.editable(app) || w.diagramEdit.diagram == diagram {
		return
	}
	w.finishDiagramEdit(app, true, false)
	w.finishQueryEdit(app, true, false)
	w.finishAPIEdit(app, true, false)
	w.diagramEdit = workspaceDiagramEdit{diagram: diagram, offset: offset, source: diagram.Source, focus: true}
}

func (w *workspaceWYSIWYG) selectVegaDiagram(app *workspaceWindow, offset int, diagram *wysiwyg.Diagram) {
	if !w.editable(app) || diagram.VegaMode() == "" {
		return
	}
	if current, ok := w.session.DiagramAt(offset); ok && current == diagram {
		w.session.Document.SetSelection(offset, offset+1)
		w.focus = true
	}
}

func (w *workspaceWYSIWYG) drawDiagram(c *mygoui.Context, app *workspaceWindow, padding float32, box richtext.ParagraphBox, diagram *wysiwyg.Diagram, revealControls bool) {
	if diagram.VegaMode() != "" {
		w.drawVegaCard(c, app, padding, box, diagram, revealControls)
		return
	}
	theme := c.Theme()
	background, _ := mermaid.Background(diagram.Info)
	cardColor := theme.Background.Mix(mygoui.RGB(127, 127, 127), .04)
	border := theme.Background.Mix(mygoui.RGB(127, 127, 127), .2)
	if strings.EqualFold(background, "none") || strings.EqualFold(background, "transparent") {
		cardColor, border = theme.Background, mygoui.Transparent
	} else if strings.HasPrefix(background, "#") && (len(background) == 7 || len(background) == 4) {
		if custom := mygoui.Hex(background); custom.A != 0 {
			cardColor = custom
		}
	}
	card := mygoui.Box(c.Key(box.Index)).Absolute().Left(padding).Right(padding).Top(box.Top+diagramMargin).
		Height(box.Height-2*diagramMargin).Radius(8).Clip().Role(mygoui.RoleGroup).Label("Mermaid diagram").
		Background(cardColor).Border(1, border).PassThrough()
	card.Children(func() {
		if w.diagramEdit.diagram == diagram && w.diagramEdit.offset == box.Start {
			w.drawMermaidSource(c, app, diagram)
			return
		}
		stage := mygoui.Box(c.Key("diagram stage")).Absolute().Top(diagramPadding).Bottom(diagramPadding).
			Left(diagramPadding).Right(diagramPadding).PassThrough()
		stage.Children(func() {
			w.drawDiagramWeb(c, app, stage, box.Start, visualDiagramFrame{
				diagram: diagram, canvas: theme.Background, card: cardColor, text: theme.Text, link: theme.Accent,
			})
		})
		w.drawDiagramControls(c, app, box.Start, diagram, background, revealControls || card.Hovered())
	})
}

// EV-UI-150: Vega has a separate stage, a single edit action and an
// inline source editor, all inside the same block attachment.
func (w *workspaceWYSIWYG) drawVegaCard(c *mygoui.Context, app *workspaceWindow, padding float32, box richtext.ParagraphBox, diagram *wysiwyg.Diagram, revealControls bool) {
	theme := app.resolvePreviewTheme(app.previewThemeSelection())
	canvas, ink := mygoui.Hex(theme.BackgroundColor), mygoui.Hex(theme.BodyTextColor)
	gray := mygoui.RGB(127, 127, 127)
	fill := canvas.Mix(gray, .04)
	family, _, _ := strings.Cut(theme.BodyFontFamily, ",")
	family = strings.Trim(family, " '\"")
	loadBundledFont(family)
	font := family + ", sans-serif"
	// RichText includes a line descent in its paragraph box. Keep it
	// outside the card so the stage has exactly the measured height.
	top, height := box.Top+diagramMargin, diagram.Height()-2*diagramMargin
	card := mygoui.Box(c.Key(box.Index)).Absolute().Left(padding).Right(padding).Top(top).Height(height).
		Radius(8).Clip().Role(mygoui.RoleGroup).Label("Vega chart").Background(fill).Border(1, gray.Alpha(.2)).PassThrough()
	editing := w.diagramEdit.diagram == diagram && w.diagramEdit.offset == box.Start
	if editing {
		const outlineInset = 4
		type outlineKey int
		mygoui.Box(c.Key(outlineKey(box.Index))).Absolute().Left(padding-outlineInset).Right(padding-outlineInset).
			Top(top-outlineInset).Height(height+2*outlineInset).Radius(8+outlineInset).
			Border(2, mygoui.RGBA(100, 150, 255, .4)).IgnoreInput(true).Role(mygoui.RoleNone)
	}
	card.Children(func() {
		if editing {
			w.drawVegaSource(c, app, card.Bounds().W, diagram, canvas, ink, font)
			return
		}
		stage := mygoui.Box(c.Key("diagram stage")).Absolute().Top(diagramPadding).Bottom(diagramPadding).
			Left(diagramPadding).Right(diagramPadding).PassThrough()
		stage.Children(func() {
			w.drawDiagramWeb(c, app, stage, box.Start, visualDiagramFrame{
				diagram: diagram, canvas: canvas, card: fill, text: ink, codeBackground: mygoui.Hex(theme.CodeBackgroundColor),
			})
		})
		button := mygoui.Button(c, "Edit source").Absolute().Top(6).Right(6).Size(72.6875, 21).
			Font(font).FontSize(10).FontWeight(500).TextColor(ink).Padding(0).Radius(4).
			Background(canvas.Mix(ink, .14)).Border(1, ink.Alpha(.45)).KeepFocus().Disabled(!w.editable(app)).
			OnClick(func() { w.beginDiagramEdit(app, box.Start, diagram) })
		if button.Hovered() {
			button.Background(canvas.Mix(ink, .26))
		}
		opacity := float32(0)
		if revealControls || card.Hovered() || button.Focused() || c.Preferences().HighContrast {
			opacity = 1
		}
		button.Opacity(button.Animate("opacity", opacity, 150*time.Millisecond))
	})
}

func (w *workspaceWYSIWYG) drawVegaSource(c *mygoui.Context, app *workspaceWindow, cardWidth float32, diagram *wysiwyg.Diagram, canvas, ink mygoui.Color, font string) {
	const hintGap = 8.296875
	const hintLineHeight = 13
	const hintFontSize = 10
	const inset = diagramPadding + 1 // Includes the border around the card.
	rows := min(30, max(6, strings.Count(diagram.Source, "\n")+3))
	inputHeight := float32(float64(rows)*1.4+1.2) * 12
	if w.diagramEdit.height > 0 {
		inputHeight = w.diagramEdit.height
	}
	hint := diagramSourceShortcutHint()
	hintWidth := max(0, cardWidth-2*inset)
	hintLayout := mygoui.ShapeText(hint, mygoui.Font{Family: font, Size: hintFontSize, LineHeight: 1.3}, hintWidth)
	hintHeight := float32(len(hintLayout.Lines()) * hintLineHeight)
	w.setDiagramHeight(c, diagram, inputHeight+hintGap+hintHeight+2*(diagramMargin+inset))
	gray := mygoui.RGB(127, 127, 127)
	mygoui.Column(c).Fill().Padding(diagramPadding).Gap(hintGap).Children(func() {
		input := w.diagramSourceInput(c, app, "Vega source", inputHeight).
			FixedLineHeight(18).Padding(10).Radius(6).FocusRing(false).TextColor(ink).
			Background(canvas.Mix(gray, .08)).Border(1, gray.Alpha(.35))
		if input.Focused() {
			input.Background(canvas.Mix(gray, .04)).BorderColor(mygoui.RGBA(100, 150, 255, .6))
		}
		mygoui.Text(c, hint).FillWidth().Height(hintHeight).Shrink(0).Font(font).FontSize(hintFontSize).
			FixedLineHeight(hintLineHeight).TextColor(gray.Alpha(.7)).IgnoreInput(true)
		grip := mygoui.Box(c.Key("Vega source resize")).Absolute().Right(diagramPadding+1).Top(diagramPadding+inputHeight-13).
			Size(12, 12).KeepFocus().Cursor(mygoui.CursorResizeNS).Role(mygoui.RoleSplitter).Label("Resize Vega source").Bind(&w.diagramEdit.grip)
		drawFencedSourceGrip(c, grip, &w.diagramEdit.grip, &w.diagramEdit.resize, &w.diagramEdit.height, inputHeight, gray)
	})
}

func drawFencedSourceGrip(c *mygoui.Context, grip mygoui.Element, handle *mygoui.Handle, drag **fencedSourceResize, height *float32, initialHeight float32, gray mygoui.Color) {
	grip.Draw(func(p *mygoui.Painter, r mygoui.Rect) {
		x, y := r.X+r.W-2, r.Y+r.H-2
		p.Line(x-3, y, x, y-3, 1, gray.Alpha(.7))
		p.Line(x-6, y, x, y-6, 1, gray.Alpha(.7))
	}).HandleInput(func(event mygoui.InputEvent) bool {
		y := handle.Bounds(c).Y + event.Y
		switch event.Kind {
		case mygoui.InputPointerDown:
			if event.Button != 0 {
				return false
			}
			*drag = &fencedSourceResize{y, initialHeight}
		case mygoui.InputPointerMove, mygoui.InputPointerUp:
			if *drag == nil {
				return false
			}
			if event.Canceled {
				*height = (*drag).height
			} else {
				*height = max(40, (*drag).height+y-(*drag).y)
			}
			if event.Kind == mygoui.InputPointerUp {
				*drag = nil
			}
		default:
			return false
		}
		return true
	})
}

func (w *workspaceWYSIWYG) drawMermaidSource(c *mygoui.Context, app *workspaceWindow, diagram *wysiwyg.Diagram) {
	const fontSize = 12
	rows := min(30, max(6, strings.Count(diagram.Source, "\n")+3))
	inputHeight := float32(math.Round((float64(rows)*1.4 + 1.2) * 12.1))
	w.setDiagramHeight(c, diagram, inputHeight+51+2*diagramMargin)
	mygoui.Column(c).Fill().Padding(diagramPadding).Gap(7).Children(func() {
		w.diagramSourceInput(c, app, "Mermaid source", inputHeight)
		mygoui.Text(c, diagramSourceShortcutHint()).FillWidth().FontSize(10).TextAlign(mygoui.End).TextColor(c.Theme().TextMuted)
	})
}

func (w *workspaceWYSIWYG) setDiagramHeight(c *mygoui.Context, diagram *wysiwyg.Diagram, height float32) {
	if diagram.Height() != height {
		diagram.ReserveHeight(height)
		w.diagramLayout = !w.diagramLayout
		c.Invalidate()
	}
}

func (w *workspaceWYSIWYG) diagramSourceInput(c *mygoui.Context, app *workspaceWindow, label string, inputHeight float32) mygoui.Element {
	edit := &w.diagramEdit
	return fencedSourceInput(c, &edit.source, &edit.focus, label, inputHeight, func(commit, focus bool) {
		w.finishDiagramEdit(app, commit, focus)
	})
}

func fencedSourceInput(c *mygoui.Context, source *string, focus *bool, label string, inputHeight float32, finish func(commit, focus bool)) mygoui.Element {
	input := mygoui.TextArea(c.Key(label), source).
		FillWidth().Height(inputHeight).Shrink(0).Font("monospace").FontSize(12).
		Label(label).OnTextKey(func(key mygoui.TextKey) bool {
		switch {
		case key.Key == mygoui.KeyEscape && key.Mods == 0:
			finish(false, true)
		case key.Key == mygoui.KeyEnter && (key.Mods == mygoui.Cmd || key.Mods == mygoui.Ctrl):
			finish(true, true)
		default:
			return false
		}
		return true
	})
	if *focus {
		end := utf8.RuneCountInString(*source)
		input.Focus().SetTextSelection(end, end)
		*focus = false
	} else if input.FocusLeft() {
		finish(true, false)
		c.Invalidate()
	}
	return input
}

func diagramSourceShortcutHint() string {
	if shortcutLabel("⌘ Enter") != "⌘ Enter" {
		return strings.Replace(diagramSourceHint, "⌘Enter", "Ctrl+Enter", 1)
	}
	return diagramSourceHint
}

func (w *workspaceWYSIWYG) drawDiagramControls(c *mygoui.Context, app *workspaceWindow, offset int, diagram *wysiwyg.Diagram, background string, reveal bool) {
	theme := c.Theme()
	trigger := mygoui.Row(c).Absolute().Top(7).Right(6).Gap(8).AlignItems(mygoui.Center)
	showBackground := mygoui.Local(trigger, "diagram background", func() bool { return false })
	color := mygoui.Local(trigger, "diagram color", func() mygoui.Color {
		if len(background) == 7 && strings.HasPrefix(background, "#") {
			return mygoui.Hex(background)
		}
		return mygoui.RGB(26, 26, 46)
	})
	change := func(info string) {
		if w.editable(app) && w.session.ChangeDiagram(offset, diagram, diagram.Source, info) {
			w.session.Sync(app.draft)
		}
		*showBackground = false
	}
	trigger.Children(func() {
		fill := theme.Background.Mix(theme.Text, .14)
		outline := theme.Text.Alpha(.45)
		pickBackground := mygoui.Button(c, "Background").Size(78, 22).Padding(0).FontSize(11).
			FontWeight(600).Background(fill).Border(1, outline).Radius(4).
			Disabled(!w.editable(app)).OnClick(func() { *showBackground = !*showBackground })
		editSource := mygoui.Button(c, "Edit source").Size(73, 22).Padding(0).FontSize(11).
			FontWeight(600).Background(fill).Border(1, outline).Radius(4).
			Disabled(!w.editable(app)).OnClick(func() {
			*showBackground = false
			w.beginDiagramEdit(app, offset, diagram)
		})
		if !reveal && !*showBackground && !trigger.FocusWithin() && !c.Preferences().HighContrast {
			pickBackground.Opacity(0)
			editSource.Opacity(0)
		}
	})
	if *showBackground {
		panel := mygoui.Row(c).Absolute().Top(37).Right(6).Height(32).Padding(4).Gap(5).
			Radius(6).Background(theme.Surface).Border(1, theme.Border).AlignItems(mygoui.Center)
		panel.Children(func() {
			mygoui.ColorWell(c, color).Label("Card background color").Size(35, 25).OnChange(func() {
				change(fmt.Sprintf("mermaid[bg:#%02x%02x%02x]", color.R, color.G, color.B))
			})
			mygoui.Button(c, "Theme").Height(22).FontSize(11).Padding(0, 8).OnClick(func() { change("mermaid") })
			mygoui.Button(c, "None").Height(22).FontSize(11).Padding(0, 8).OnClick(func() { change("mermaid[bg:none]") })
		})
		if panel.PressedOutside() && trigger.PressedOutside() {
			*showBackground = false
			c.Invalidate()
		}
	}
}

func (w *workspaceWYSIWYG) reserveDiagramHeight(diagram *wysiwyg.Diagram, height float64) bool {
	stageHeight := min(1600, max(60, height))
	next := float32(math.Ceil(stageHeight + 2*(diagramMargin+diagramPadding)))
	if diagram.VegaMode() != "" {
		next = float32(stageHeight + 2*(diagramMargin+diagramPadding+1))
	}
	if w.diagramEdit.diagram == diagram || diagram.Height() == next {
		return false
	}
	diagram.ReserveHeight(next)
	w.diagramLayout = !w.diagramLayout
	return true
}
