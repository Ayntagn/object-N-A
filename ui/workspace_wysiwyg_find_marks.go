package ui

import (
	"sort"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// The editor's text and undo remain untouched. Shape the visible matching
// paragraphs with MyGo's text layout and place the search marks behind them.
func (w *workspaceWYSIWYG) drawFindMarks(c *mygoui.Context, app *workspaceWindow, text richtext.Text,
	boxes []richtext.ParagraphBox, width float32, theme *richtext.Theme, font mygoui.Font) {
	f := &w.find
	if !f.open || len(f.matches) == 0 || width <= 0 {
		return
	}
	ink := mygoui.Hex(app.resolvePreviewTheme(app.previewThemeSelection()).LinkColor)
	if app.blackout {
		ink = blackoutTitleAccent
	}
	defaultStyle := richtext.DefaultTheme(c.Theme())
	var marks []struct {
		frame   mygoui.Rect
		current bool
	}
	for _, box := range boxes {
		_, end := text.ParagraphRange(box.Start)
		first := sort.Search(len(f.matches), func(i int) bool { return f.matches[i].Start >= box.Start })
		if first == len(f.matches) || f.matches[first].Start >= end {
			continue
		}
		paragraphWidth := width - 2*theme.Padding[0]
		if paragraphWidth <= 0 {
			break
		}
		style := text.ParagraphStyleAt(box.Start)
		spans, shapedFont, left, right := w.findParagraph(text, box.Start, end, style, font, theme, defaultStyle, paragraphWidth)
		layout := mygoui.ShapeRichTextWith(spans, shapedFont, mygoui.TextShape{
			Width: max(paragraphWidth-left-right, 1), Align: style.Align,
		})
		value := text.Slice(box.Start, end).String()
		units := make([]int, 1, end-box.Start+1)
		for _, letter := range value {
			n := 1
			if letter >= 0x10000 {
				n = 2
			}
			units = append(units, units[len(units)-1]+n)
		}
		for i := first; i < len(f.matches) && f.matches[i].Start < end; i++ {
			m := f.matches[i]
			for _, r := range layout.SelectionRects(mygoui.TextInputRange{Start: units[m.Start-box.Start], End: units[m.End-box.Start]}) {
				r.X += theme.Padding[0] + left
				r.Y += box.Top
				marks = append(marks, struct {
					frame   mygoui.Rect
					current bool
				}{r, i+1 == f.index})
			}
		}
	}
	if len(marks) == 0 {
		return
	}
	mygoui.Box(c.Key("visual find marks")).Absolute().Left(0).Top(0).Right(0).Bottom(0).
		ZIndex(-1).IgnoreInput(true).Role(mygoui.RoleNone).Draw(func(p *mygoui.Painter, r mygoui.Rect) {
		for _, mark := range marks {
			where := mark.frame
			where.X += r.X
			where.Y += r.Y
			if mark.current {
				p.Fill(where, ink.Alpha(.70), 2)
				p.Stroke(where, ink, 2, 1)
			} else {
				p.Fill(where, ink.Alpha(.28), 2)
			}
		}
	})
}

func (w *workspaceWYSIWYG) findParagraph(text richtext.Text, start, end int, style richtext.ParagraphStyle,
	font mygoui.Font, theme *richtext.Theme, defaults *richtext.Theme, width float32) ([]mygoui.Span, mygoui.Font, float32, float32) {
	var spans []mygoui.Span
	baseFont := font
	left, right := float32(style.Indent)*font.Size*defaults.Indent, float32(0)
	if style.List != richtext.NoList {
		left += font.Size * defaults.Indent
	}
	if style.Quote {
		left += font.Size * defaults.Indent * .75
	}
	switch {
	case style.Heading > 0:
		level := min(style.Heading, len(theme.Headings)) - 1
		heading := theme.Headings[level]
		baseFont.Size *= heading.Size
		baseFont.Weight = heading.Weight
		if baseFont.Weight == 0 {
			baseFont.Weight = defaults.Headings[level].Weight
		}
	case style.CodeBlock:
		baseFont.Family = defaults.CodeFont
		baseFont.Size *= defaults.CodeSize
		right = font.Size * defaults.CodeBlockPadding[0]
		left += right
	}
	for _, run := range text.Runs(start, end) {
		a := run.Attributes
		span := mygoui.Span{Text: run.Text, Font: a.Font, Size: a.Size, Weight: a.Weight, Italic: a.Italic}
		if a.Code {
			span.Font = defaults.CodeFont
			if span.Size == 0 {
				span.Size = font.Size * defaults.CodeSize
			}
		}
		if a.Tag != "" {
			w.visualTagStyle(a.Tag, &span)
		}
		if object := a.Attachment; object != nil {
			objectWidth, objectHeight := object.Width, object.Height
			if object.Image != nil && (objectWidth <= 0 || objectHeight <= 0) {
				objectWidth, objectHeight = mygoui.ImageSize(object.Image)
			}
			objectWidth, objectHeight = max(objectWidth, 1), max(objectHeight, 1)
			available := max(1, width-left-right)
			span.ObjectWidth = min(available, objectWidth)
			span.ObjectHeight = objectHeight * (span.ObjectWidth / objectWidth)
		}
		spans = append(spans, span)
	}
	return spans, baseFont, left, right
}
