package ui

import (
	"strings"

	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// The title attachment also reserves the extra line space in the card.
// MyGo's paragraph boxes exclude the spacing between paragraphs.
const calloutTopInset, calloutBottomInset = float32(15), float32(10.5)

func (w *workspaceWYSIWYG) visualTagStyle(tag string, span *mygoui.Span) {
	switch tag {
	case wysiwyg.CalloutIconTag:
		span.ObjectDescent = float32(w.fontSize()) * .55
	case wysiwyg.CalloutTitleTag:
		span.Size, span.Weight, span.Color = float32(w.fontSize())*.95, 700, w.math.appearance.ink
	case wysiwyg.CalloutBodyTag:
		span.Color = w.math.appearance.ink
	default:
		w.mathTagStyle(tag, span)
	}
}

// The callout is editable in the parent RichText. These noninteractive
// siblings paint its rounded background and mark the same range in AX.
func (w *workspaceWYSIWYG) drawCallouts(c *mygoui.Context, viewport mygoui.Rect, padding float32,
	text richtext.Text, boxes []richtext.ParagraphBox) {
	if viewport.W <= 2*padding {
		return
	}
	ink := w.math.appearance.ink
	for i := 0; i < len(boxes); i++ {
		box := boxes[i]
		paragraph := text.ParagraphStyleAt(box.Start)
		id := wysiwyg.CalloutParagraphID(text, box.Start)
		title := wysiwyg.CalloutAt(text, box.Start)
		if id == "" {
			if title != nil {
				id = title.ID()
			} else if paragraph.Quote {
				w.drawQuoteRail(c, text, padding, box, paragraph)
			}
			if id == "" {
				continue
			}
		}
		card := title
		if card == nil {
			// Scrolling may leave only a body paragraph visible.
			for start := box.Start; start > 0 && wysiwyg.CalloutParagraphID(text, start-1) == id; {
				start, _ = text.ParagraphRange(start - 1)
				if card = wysiwyg.CalloutAt(text, start); card != nil {
					break
				}
			}
			if card == nil {
				w.drawQuoteRail(c, text, padding, box, paragraph)
				continue
			}
		}
		last := i
		for last+1 < len(boxes) && wysiwyg.CalloutParagraphID(text, boxes[last+1].Start) == id &&
			wysiwyg.CalloutAt(text, boxes[last+1].Start) == nil {
			last++
		}
		style := calloutAppearance(card.Kind)
		label := style.name
		if !strings.EqualFold(card.Kind, label) {
			label = card.Kind
		}
		const corner = float32(6)
		top := box.Top + calloutTopInset
		if title == nil {
			top = -corner
		}
		bottom := boxes[last].Top + boxes[last].Height + calloutBottomInset
		_, end := text.ParagraphRange(boxes[last].Start)
		if end < text.Len() && wysiwyg.CalloutParagraphID(text, end+1) == id {
			bottom = max(bottom, viewport.H+corner)
		}
		background := mygoui.RGBA(127, 127, 127, .08)
		border := mygoui.Hex(style.border)
		mygoui.Box(c.Key(id).Key(box.Start)).Absolute().Left(padding).Right(padding).Top(top).Height(max(1, bottom-top)).
			ZIndex(-1).PassThrough().Role(mygoui.RoleGroup).Label(label + " callout").
			Draw(func(p *mygoui.Painter, r mygoui.Rect) {
				p.Fill(r, background, corner)
				p.Clip(r, corner, func() {
					p.Fill(mygoui.Rect{X: r.X, Y: r.Y, W: 4, H: r.H}, border, 0)
				})
			})
		if title != nil {
			size := float32(w.fontSize())
			icon := mygoui.ShapeText(style.icon, mygoui.Font{Size: size * .95}, 0)
			width, _ := icon.Size()
			baseline := icon.Lines()[0].Baseline
			descent := size * .55
			ascent := max(16, size) + 22
			if card.SetIcon(max(21, width+4), ascent, func(p *mygoui.Painter, r mygoui.Rect) {
				p.TextLayout(icon, r.X, r.Y+r.H-descent-baseline, ink)
			}) {
				w.calloutLayout = !w.calloutLayout
				c.Invalidate()
			}
		}
		i = last
	}
}

// RichText uses one quote rail color for the whole editor. Paint ordinary
// rails here so the title attachment's extra space stays outside the card.
func (w *workspaceWYSIWYG) drawQuoteRail(c *mygoui.Context, text richtext.Text, padding float32,
	box richtext.ParagraphBox, style richtext.ParagraphStyle) {
	size := float32(w.fontSize())
	top := box.Top
	if box.Start > 0 && style.List == richtext.NoList && style.Heading == 0 && !style.CodeBlock {
		prev := text.ParagraphStyleAt(box.Start - 1)
		if prev.Quote && prev.List == richtext.NoList && prev.Heading == 0 && !prev.CodeBlock {
			top -= size * .75
		}
	}
	mygoui.Box(c.Key(box.Start).Key("quote rail")).Absolute().
		Left(padding + float32(style.Indent)*size*1.7).Top(top).
		Width(3).Height(box.Top + box.Height - top).ZIndex(-1).Radius(1.5).
		Background(c.Theme().Border).Role(mygoui.RoleNone).IgnoreInput(true)
}
