package ui

import (
	"math"
	"strconv"
	"strings"

	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
)

type visualImageEditor struct {
	open, invalid bool
	scale         string
}

func transformedVisualImageSize(picture *wysiwyg.Image, naturalW, naturalH, available float32,
	drag visualImageResize) (base, maxHeight, width, height float32) {
	transform := picture.ImageTransform
	base = naturalH
	if transform.SwapsAxes() {
		base = naturalW
	}
	limit := transform
	limit.Ratio = math.Inf(1)
	_, maxH := limit.DisplaySize(float64(naturalW), float64(naturalH), float64(available))
	if drag.picture == picture {
		transform.Ratio = float64(drag.height / base)
	}
	x, y := transform.DisplaySize(float64(naturalW), float64(naturalH), float64(available))
	return base, float32(maxH), float32(x), float32(y)
}

func drawTransformedVisualBitmap(p *mygoui.Painter, bounds mygoui.Rect, bitmap *mygoui.Bitmap, transform wysiwyg.ImageTransform) {
	cx, cy := bounds.X+bounds.W/2, bounds.Y+bounds.H/2
	width, height := bounds.W, bounds.H
	if transform.SwapsAxes() {
		width, height = height, width
	}
	local := mygoui.Rect{X: cx - width/2, Y: cy - height/2, W: width, H: height}
	p.Clip(bounds, 0, func() {
		p.Transform(mygoui.Rotation(float32(transform.Rotation)), cx, cy, func() {
			p.Image(bitmap, local, mygoui.Contain)
		})
	})
}

func (w *workspaceWYSIWYG) drawImageTransformControls(c *mygoui.Context, app *workspaceWindow, state *visualImageInput,
	offset int, right float32) {
	form := &state.editor
	anchor := chromeButtonIcon(c.Key("edit image"), "slider.horizontal.3", "Edit image", func() {
		if !w.editable(app) {
			return
		}
		if form.open {
			form.open = false
			return
		}
		w.commitImageCaption(app, offset, state)
		w.session.Document.SetSelection(offset, offset+1)
		*form = visualImageEditor{open: true, scale: strconv.FormatFloat(state.picture.Ratio*100, 'f', -1, 64)}
	}, 20, w.math.appearance.surface).Absolute().Right(right+38).Top(12).Size(32, 32).Radius(16).
		Background(w.math.appearance.ink.Alpha(.6)).KeepFocus()

	commit := func(transform wysiwyg.ImageTransform) {
		form.open = false
		if !w.editable(app) {
			return
		}
		if next, ok := w.session.TransformImage(offset, state.picture, transform); ok {
			state.picture = next
			w.sync(app)
		}
		w.focus = true
	}
	rotate := func(degrees int) {
		transform := state.picture.ImageTransform
		transform.Rotation = (transform.Rotation + degrees + 360) % 360
		commit(transform)
	}
	apply := func() {
		percent, err := strconv.ParseFloat(strings.TrimSpace(form.scale), 64)
		if err != nil || !(percent >= 1 && percent <= 1000) {
			form.invalid = true
			return
		}
		transform := state.picture.ImageTransform
		transform.Ratio = percent / 100
		commit(transform)
	}

	mygoui.PopoverBase(c.Key("image edit panel"), anchor, &form.open, func(panel mygoui.Element) {
		panel.AttachTo(anchor, mygoui.AnchorBottomEnd, mygoui.AnchorTopEnd).Margin(6, 0, 0).
			Width(228).Padding(10).Radius(8).Role(mygoui.RolePopup).Label("Edit image").
			Background(c.Theme().Surface).Border(1, c.Theme().TextMuted.Alpha(.2)).
			Shadow(0, 2, 6, 0, mygoui.RGBA(0, 0, 0, .18))
		mygoui.Column(c).FillWidth().Gap(8).Children(func() {
			mygoui.Row(c).FillWidth().Gap(6).Children(func() {
				mygoui.Button(c, "Rotate Left").Grow(1).OnClick(func() { rotate(-90) })
				mygoui.Button(c, "Rotate Right").Grow(1).OnClick(func() { rotate(90) })
			})
			mygoui.Text(c, "Scale (%)").Role(mygoui.RoleNone).IgnoreInput(true)
			mygoui.Row(c).FillWidth().Gap(6).Children(func() {
				mygoui.TextInput(c, &form.scale).Label("Scale (%)").Width(70).OnChange(func() { form.invalid = false }).OnSubmit(apply)
				mygoui.Button(c, "Apply Size").Grow(1).OnClick(apply)
			})
			if form.invalid {
				mygoui.Text(c, "Enter a percentage from 1 to 1000.").FontSize(11).FillWidth().TextColor(c.Theme().TextMuted)
			}
			mygoui.Button(c, "Reset Image").FillWidth().OnClick(func() { commit(wysiwyg.ImageTransform{Ratio: 1}) })
		})
	})
}
