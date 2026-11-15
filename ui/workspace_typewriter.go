package ui

import (
	"fmt"
	"math"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type typewriterSourceInput struct {
	version    uint64
	selection  editor.Selection
	navigation bool
}

func (app *workspaceWindow) sourceTypewriter(_ *mygoui.Context, source mygoui.Element, state *editor.State) {
	if !app.preferences.TypewriterMode || app.modalOpen || state.ReadOnly {
		return
	}
	version := state.Buffer.Version()
	input := mygoui.Local(source, "typewriter input", func() typewriterSourceInput {
		return typewriterSourceInput{version: version}
	})
	source.ObserveInput(func(event mygoui.InputEvent) {
		if event.Kind == mygoui.InputPointerDown || event.Kind == mygoui.InputPointerUp {
			input.navigation = false
		}
		if event.Kind != mygoui.InputKeyDown || input.navigation {
			return
		}
		switch event.Key {
		case mygoui.KeyUp, mygoui.KeyDown, mygoui.KeyLeft, mygoui.KeyRight,
			mygoui.KeyHome, mygoui.KeyEnd, mygoui.KeyPageUp, mygoui.KeyPageDown:
			input.selection = state.Selection()
			input.navigation = true
		}
	})

	// Cocoa's IME sends committed text straight to the editor. Its buffer
	// revision changes even when the input observer sees no text event.
	edited := input.version != version
	moved := input.navigation && input.selection != state.Selection()
	input.version, input.navigation = version, false
	if !source.Focused() || !moved && !edited {
		return
	}
	line := state.CursorPos().Line
	y := state.LineTop(line)
	height := state.LineTop(line+1) - y
	viewport := state.ViewportHeight()
	if height <= 0 || height >= viewport {
		return
	}
	state.SetScrollY(y - (viewport-height)*typewriterPosition(app.preferences.TypewriterPosition))
}

func typewriterPosition(position float64) float64 {
	return min(1, max(0, position))
}

func (app *workspaceWindow) typewriterSettings(c *mygoui.Context) {
	prefs := app.preferences
	tr := app.tr
	app.settingsSwitch(c, tr("Typewriter Mode", "打字机模式"), tr("Keep the active line at a fixed height", "将正在编辑的行保持在固定高度"), "text.cursor", 54, &prefs.TypewriterMode)
	app.settingsSwitch(c, tr("Auto-Indent Paragraphs", "自动缩进段落"), tr("Insert two full-width spaces in new source paragraphs", "在源码的新段落开头插入两个全角空格"),
		"text.alignleft", 54, &prefs.AutoIndentParagraphs)
	if !prefs.TypewriterMode {
		return
	}
	theme := c.Theme()
	mygoui.Row(c.Key("typewriter position settings")).FillWidth().Wrap().AlignItems(mygoui.Center).Gap(16).
		Padding(10, 12).Radius(8).Background(theme.Surface).Border(1, theme.Border).
		Children(func() {
			mygoui.Column(c).Basis(188).Grow(1).MinWidth(0).MaxWidthPercent(100).Gap(4).Children(func() {
				mygoui.Text(c, tr("Active Line Position", "当前行的位置")).FontSize(13).FontWeight(600)
				mygoui.Text(c, tr("Click in the page to choose where the active line stays while typing.", "点击页面缩略图，设置输入时当前行的位置。")).
					FontSize(11).TextColor(theme.TextMuted)
			})
			app.typewriterPositionPreview(c)
		})
}

func (app *workspaceWindow) typewriterPositionPreview(c *mygoui.Context) {
	const inset, border = 14, 1
	prefs, theme := app.preferences, c.Theme()
	// A vertical slider increases upward. The saved position increases downward.
	value := 100 * (1 - typewriterPosition(prefs.TypewriterPosition))
	page := mygoui.SliderBase(c.Key("typewriter position"), &value, 0, 100).
		Vertical().Step(1).Size(192, 232).Shrink(0).MaxWidthPercent(100).Padding(inset-border, 12).
		Radius(6).Background(theme.Background).Border(border, theme.Border).Label(app.tr("Typewriter Position", "打字机位置"))
	page.Value(fmt.Sprintf(app.tr("%.0f%% from top", "距顶部 %.0f%%"), 100-value))
	setPosition := func(position float64) {
		if prefs.TypewriterPosition != position {
			prefs.TypewriterPosition = position
			app.preferencesChanged()
		}
	}
	page.OnChange(func() { setPosition((100 - value) / 100) })
	page.OnClick(func() {
		// A fast click can end before the slider paints a drag frame.
		_, y, over := page.PointerPosition()
		if height := page.Bounds().H - 2*inset; over && height > 0 {
			position := typewriterPosition(float64((y - inset) / height))
			setPosition(math.Round(position*100) / 100)
		}
	})
	page.Draw(func(p *mygoui.Painter, r mygoui.Rect) {
		marker := r.Y + inset + (r.H-2*inset)*float32(1-value/100)
		p.Fill(mygoui.Rect{X: r.X + 6, Y: marker - 6, W: r.W - 12, H: 12}, theme.Accent.Alpha(0.15), 3)

		margin := min(21, r.W/6)
		left, textWidth := r.X+margin, r.W-2*margin
		p.Fill(mygoui.Rect{X: left, Y: r.Y + r.H*0.11, W: textWidth * 0.67, H: 5}, theme.TextMuted.Alpha(0.8), 2)
		p.Fill(mygoui.Rect{X: left, Y: r.Y + r.H*0.18, W: textWidth * 0.45, H: 2}, theme.TextMuted.Alpha(0.55), 1)
		y := r.Y + r.H*0.27
		for _, paragraph := range [...][]float32{
			{0.93, 1, 0.85, 0.52}, {0.98, 0.76, 0.92, 0.65}, {0.94, 0.8, 0.58},
		} {
			for _, lineWidth := range paragraph {
				p.Fill(mygoui.Rect{X: left, Y: y, W: textWidth * lineWidth, H: 2}, theme.TextMuted.Alpha(0.48), 1)
				y += r.H * 0.047
			}
			y += r.H * 0.045
		}
		p.Fill(mygoui.Rect{X: r.X + 9, Y: marker - 1, W: r.W - 18, H: 2}, theme.Accent, 1)
	})
}
