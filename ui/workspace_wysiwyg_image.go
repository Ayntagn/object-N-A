package ui

import (
	"fmt"
	"image"
	"math"
	"net/url"
	"path"
	"strings"
	"time"

	"cyberwriter/internal/core/workspace"
	"cyberwriter/ui/wysiwyg"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

const visualImageMinHeight = 100
const visualImageFormHeight = 56
const visualImageCaptionHeight = 14
const visualImageCaptionGap = 4
const visualImageControlHeight = 46
const largeVisualBitmapBytes = 16 << 20

type visualImageResource struct {
	bitmap *mygoui.Bitmap
	bytes  int64
	used   uint64
}

type visualImageKey struct {
	source string
	wiki   bool
}

type visualImageCache struct {
	origin previewResourceOrigin
	images map[visualImageKey]*visualImageResource
	bytes  int64
	use    uint64
	active int
}

func (cache *visualImageCache) releaseOriginalPixels() {
	for _, resource := range cache.images {
		if resource.bytes >= largeVisualBitmapBytes {
			resource.bitmap.ReleaseOriginalPixels()
		}
	}
}

func (cache *visualImageCache) load(app *workspaceWindow, source string, wiki bool,
	previewShrink func(w, h int) float32) *mygoui.Bitmap {
	origin := app.previewOrigin()
	origin.index = nil
	if cache.origin != origin {
		*cache = visualImageCache{origin: origin}
	}
	if origin.vault == nil || source == "" {
		return nil
	}
	key := visualImageKey{source, wiki}
	var candidates []string
	if wiki {
		candidates = origin.wikiImageCandidates(source)
	} else if relative, ok := workspace.ResolveImagePath(origin.note, source); ok {
		key.source = relative
		candidates = []string{relative}
	}
	if len(candidates) == 0 {
		return nil
	}
	cache.use++
	if entry := cache.images[key]; entry != nil {
		entry.used = cache.use
		return entry.bitmap
	}
	if cache.active >= 3 {
		return nil
	}
	if cache.images == nil {
		cache.images = make(map[visualImageKey]*visualImageResource)
	}
	entry := &visualImageResource{used: cache.use}
	cache.images[key] = entry
	decode := func() *mygoui.Bitmap {
		for _, relative := range candidates {
			data, mime, err := origin.vault.ReadPreviewImage(relative, 4<<20)
			if err != nil {
				continue
			}
			var decodePreview func([]byte, int) (image.Image, error)
			switch mime {
			case "image/png":
				decodePreview = decodeVisualPNGPreview
			case "image/jpeg":
				decodePreview = decodeVisualJPEGPreview
			}
			bitmap, err := mygoui.DecodeBitmapPreview(data, previewShrink, decodePreview)
			if err == nil {
				return bitmap
			}
		}
		return nil
	}
	publish := func(bitmap *mygoui.Bitmap) {
		if cache.images[key] != entry {
			return
		}
		if bitmap != nil {
			width, height := bitmap.Size()
			entry.bitmap, entry.bytes = bitmap, int64(width)*int64(height)*4
			cache.bytes += entry.bytes
		}
		for len(cache.images) > 64 || cache.bytes > 384<<20 {
			var oldest visualImageKey
			for candidateKey, candidate := range cache.images {
				if candidateKey != key && (oldest.source == "" || candidate.used < cache.images[oldest].used) {
					oldest = candidateKey
				}
			}
			if oldest.source == "" {
				break
			}
			cache.bytes -= cache.images[oldest].bytes
			delete(cache.images, oldest)
		}
	}
	if app.win == nil {
		publish(decode())
	} else {
		win := app.win
		cache.active++
		go func() {
			bitmap := decode()
			win.Update(func() {
				if cache.origin == origin && cache.active > 0 {
					cache.active--
				}
				publish(bitmap)
			})
		}()
	}
	return entry.bitmap
}

type visualImageInput struct {
	picture                 *wysiwyg.Image
	caption, link           string
	showCaption, wasFocused bool
	deadline                time.Time
	editor                  visualImageEditor
}

type visualImageResize struct {
	picture                  *wysiwyg.Image
	handle                   mygoui.Handle
	pressY, height, original float32
}

type visualImageFrame struct {
	viewport   mygoui.Rect
	padding    float32
	pointer    mygoui.Point
	hovered    bool
	start, end int
	font       mygoui.Font
}

func (w *workspaceWYSIWYG) reserveVisualImage(c *mygoui.Context, picture *wysiwyg.Image, width, height float32,
	draw func(*mygoui.Painter, mygoui.Rect)) {
	if picture.SetVisual(width, height, draw) {
		w.imageMetricsChanged = true
		c.Invalidate()
	}
}

func imageAccessibleLabel(picture *wysiwyg.Image) string {
	if picture.Caption != "" && picture.Block {
		return picture.Caption
	}
	if picture.Alt != "" && !picture.Block {
		return picture.Alt
	}
	if picture.Wiki {
		return path.Base(picture.Source)
	}
	if u, err := url.Parse(picture.Source); err == nil && u.Path != "" {
		return path.Base(u.Path)
	}
	return "Image"
}

func (w *workspaceWYSIWYG) drawVisualImages(c *mygoui.Context, app *workspaceWindow, text richtext.Text,
	box richtext.ParagraphBox, view visualImageFrame) bool {
	start, end := text.ParagraphRange(box.Start)
	for offset, run := range text.Runs(start, end) {
		if run.Attributes.Attachment == nil {
			continue
		}
		picture, ok := wysiwyg.ImageAt(text, offset)
		if !ok {
			continue
		}
		width := max(1, view.viewport.W-2*view.padding)
		selected := view.start <= offset && offset < view.end
		block := picture.Block && start == offset && end == offset+1 && !text.ParagraphStyleAt(offset).Quote
		var shrink func(w, h int) float32
		if block {
			shrink = visualImageBlockShrink(picture, width, c.PixelScale(), w.imageResize)
		}
		bitmap := w.images.load(app, picture.Source, picture.Wiki, shrink)
		if block {
			w.drawImageCard(c, app, picture, text, box, view, width, bitmap, selected)
			return true
		}
		w.drawInlineImage(c, picture, width, bitmap, selected)
	}
	return false
}

func visualImageBlockSize(picture *wysiwyg.Image, naturalW, naturalH, width float32,
	resize visualImageResize) (base, maxHeight, imageWidth, height float32) {
	if picture.HasTransform() || picture.WikiWidth > 0 {
		return transformedVisualImageSize(picture, naturalW, naturalH, width, resize)
	}
	base = naturalH * min(1, width/naturalW)
	height = float32(min(4096, max(visualImageMinHeight, float64(base)*picture.Ratio)))
	if resize.picture == picture {
		height = resize.height
	}
	return base, 4096, min(width, height*naturalW/naturalH), height
}

func visualImageBlockShrink(picture *wysiwyg.Image, width, pixelScale float32, resize visualImageResize) func(w, h int) float32 {
	// The preview is decoded off the main thread. Only the drawing geometry
	// as it was at request time is consulted there.
	geometry := *picture
	if resize.picture == picture {
		resize = visualImageResize{picture: &geometry, height: resize.height}
	} else {
		resize = visualImageResize{}
	}
	return func(w, h int) float32 {
		if int64(w)*int64(h)*4 < largeVisualBitmapBytes {
			return 1
		}
		originalW, originalH := float32(w), float32(h)
		_, _, drawW, drawH := visualImageBlockSize(&geometry, originalW, originalH, width, resize)
		if geometry.HasTransform() {
			if geometry.SwapsAxes() {
				drawW, drawH = drawH, drawW
			}
			return (1 / min(drawW/originalW, drawH/originalH)) / pixelScale
		}
		return (1 / max(drawW/originalW, drawH/originalH)) / pixelScale
	}
}

func (w *workspaceWYSIWYG) drawInlineImage(c *mygoui.Context, picture *wysiwyg.Image, maxWidth float32, bitmap *mygoui.Bitmap, selected bool) {
	outline := w.math.appearance.accent
	if bitmap == nil {
		ink := c.Theme().TextMuted
		label := mygoui.ShapeText(imageAccessibleLabel(picture), mygoui.Font{Size: 11}, maxWidth)
		width, _ := label.Size()
		w.reserveVisualImage(c, picture, max(12, width+6), 18, func(p *mygoui.Painter, rect mygoui.Rect) {
			p.Stroke(rect, ink.Alpha(.5), 0, 1)
			p.TextLayout(label, rect.X+3, rect.Y+2, ink)
		})
		return
	}
	width, height := mygoui.ImageSize(bitmap)
	w.reserveVisualImage(c, picture, width, height, func(p *mygoui.Painter, rect mygoui.Rect) {
		p.Image(bitmap, rect, mygoui.Contain)
		if selected {
			p.Stroke(rect, outline, 0, 1)
		}
	})
}

func (w *workspaceWYSIWYG) drawImageCard(c *mygoui.Context, app *workspaceWindow, picture *wysiwyg.Image, text richtext.Text,
	box richtext.ParagraphBox, view visualImageFrame, width float32, bitmap *mygoui.Bitmap, selected bool) {
	// RichText adds body paragraph spacing above the atom and font
	// descent after it. Reserve a net four points between image cards.
	previous := text.ParagraphStyleAt(max(0, box.Start-1))
	font, padding := view.font, view.padding
	above := float32(0)
	if box.Start > 0 && !previous.CodeBlock && previous.Heading == 0 && previous.List == richtext.NoList {
		above = font.Size * .75
	}
	top := box.Top + 3 - above
	reserve := float32(4) - font.Metrics().Descent
	card := mygoui.Box(c.Key("image card").Key(box.Start)).Absolute().Left(padding).Width(width).Top(top).
		Role(mygoui.RoleGroup).Label("Image block").PassThrough()
	state := mygoui.Local(card, "image input", func() visualImageInput { return visualImageInput{} })
	if state.picture == nil || state.picture.ID != picture.ID {
		*state = visualImageInput{picture: picture, caption: picture.Caption, showCaption: picture.Caption != ""}
	} else if state.picture != picture {
		state.picture, state.caption, state.deadline = picture, picture.Caption, time.Time{}
		if picture.Source == "" {
			state.link = ""
		}
	}
	if picture.Source == "" {
		card.Height(visualImageFormHeight)
		w.reserveVisualImage(c, picture, 1, visualImageFormHeight+reserve-imageParagraphSpacing(text, box.Start, font.Size, false), nil)
		if w.editable(app) {
			// The full form is the drop target so a child field or
			// button does not change where the selected file goes.
			files := card.DroppedFiles()
			if drop, ok := mygoui.DropData(card, visualImageFileDropOptions); ok {
				files, _ = drop.Data.Files()
			}
			if len(files) == 1 {
				app.applyVisualImageFile(w.session, box.Start, files[0])
			}
		}
		card.Children(func() { w.drawImageLinkForm(c, app, state, box.Start, width, selected) })
		return
	}
	base, maxHeight, imageWidth := float32(0), float32(4096), min(width, float32(291))
	height := float32(visualImageMinHeight)
	if bitmap != nil {
		naturalW, naturalH := mygoui.ImageSize(bitmap)
		base, maxHeight, imageWidth, height = visualImageBlockSize(picture, naturalW, naturalH, width, w.imageResize)
	} else if picture.Caption == "" {
		imageWidth = min(width, 100)
	}
	wrapper := min(width, max(100, imageWidth))
	controlHeight := max(height, visualImageControlHeight)
	cardHeight := controlHeight
	if state.showCaption {
		cardHeight += visualImageCaptionGap + visualImageCaptionHeight
	}
	card.Height(cardHeight)
	w.reserveVisualImage(c, picture, 1, cardHeight+reserve-imageParagraphSpacing(text, box.Start, font.Size, true), nil)
	centered := (width - imageWidth) / 2
	hover := view.hovered && (mygoui.Rect{X: padding, Y: top, W: width, H: cardHeight}).Contains(view.pointer.X, view.pointer.Y)
	card.Children(func() {
		ink := w.math.appearance.ink
		pane := mygoui.Box(c.Key("image display")).Absolute().Top(0).Left(centered).Size(imageWidth, height).
			Role(mygoui.RoleImage).Label(imageAccessibleLabel(picture)).KeepFocus().
			OnClick(func() {
				if w.editable(app) {
					w.session.Document.SetSelection(box.Start, box.Start+1)
					w.focus = true
				}
			})
		pane.Draw(func(p *mygoui.Painter, r mygoui.Rect) {
			if bitmap == nil {
				drawMissingImage(p, r, picture.Caption, ink)
			} else if picture.HasTransform() {
				drawTransformedVisualBitmap(p, r, bitmap, picture.ImageTransform)
			} else {
				p.Image(bitmap, r, mygoui.Cover)
			}
			if selected {
				p.Fill(r, w.math.appearance.accent.Alpha(.08), 0)
			}
		})
		if w.editable(app) && (hover || selected || state.editor.open || w.imageResize.picture == picture) {
			w.drawImageActions(c, app, state, box.Start, width, wrapper, height, base, maxHeight)
		}
		if state.showCaption {
			w.drawImageCaption(c, app, state, box.Start, controlHeight)
		}
	})
}

// Loaded figures and empty forms follow the same four-point gap. The
// form's input is full width and occupies the following text spacing.
func imageParagraphSpacing(text richtext.Text, start int, fontSize float32, loaded bool) float32 {
	_, end := text.ParagraphRange(start)
	if end >= text.Len() {
		return 0
	}
	if next, ok := wysiwyg.ImageAt(text, end+1); ok && next.Block {
		if loaded && next.Source == "" {
			return fontSize * .75
		}
		return 0
	}
	style := text.ParagraphStyleAt(end + 1)
	if style.Heading == 0 && style.List == richtext.NoList && !style.CodeBlock {
		return fontSize * .75
	}
	return 0
}

func drawMissingImage(p *mygoui.Painter, r mygoui.Rect, caption string, ink mygoui.Color) {
	p.Stroke(r, ink.Alpha(.72), 0, .5)
	if caption != "" {
		layout := mygoui.ShapeText(caption, mygoui.Font{Size: 15}, max(1, r.W-4))
		p.TextLayout(layout, r.X+2, r.Y+1, ink)
	}
}

func (w *workspaceWYSIWYG) drawImageLinkForm(c *mygoui.Context, app *workspaceWindow, state *visualImageInput, offset int, width float32, selected bool) {
	ink, accent := w.math.appearance.ink, w.math.appearance.accent
	font := w.math.appearance.font
	loadBundledFont(font)
	mygoui.Box(c.Key("image import background")).Absolute().Fill().Role(mygoui.RoleNone).IgnoreInput(true).
		Background(w.math.appearance.surface)
	if selected {
		mygoui.Box(c.Key("image import selection")).Absolute().Fill().Role(mygoui.RoleNone).IgnoreInput(true).
			Background(accent.Alpha(.08))
	}
	mygoui.Box(c.Key("image import icon")).Absolute().Left(24).Top(16).Size(24, 24).Role(mygoui.RoleNone).IgnoreInput(true).
		Draw(func(p *mygoui.Painter, r mygoui.Rect) {
			color := ink.Alpha(.5)
			p.Stroke(mygoui.Rect{X: r.X + 4, Y: r.Y + 4, W: 16, H: 16}, color, 1, 2)
			var peaks mygoui.Path
			peaks.MoveTo(r.X+6, r.Y+17).LineTo(r.X+9, r.Y+13).LineTo(r.X+12, r.Y+16).
				LineTo(r.X+15, r.Y+12).LineTo(r.X+18, r.Y+17).Close()
			p.FillPath(&peaks, color)
		})
	right := float32(24)
	if state.link != "" {
		right += min(187, width*.38)
	}
	submit := func() {
		if destination := strings.TrimSpace(state.link); destination != "" {
			w.commitVisualImage(app, offset, state.picture, destination, state.picture.Caption, state.picture.Ratio)
		}
	}
	mygoui.TextInput(c.Key("image link"), &state.link).Absolute().Left(64).Right(right).Top(16).Height(24).
		Label("Image link").Font(font).FontSize(15).Padding(0).Border(0, mygoui.Transparent).Background(mygoui.Transparent).
		FocusRing(false).TextColor(ink).Disabled(!w.editable(app)).OnSubmit(submit)
	if state.link == "" {
		mygoui.Button(c.Key("upload image"), "Upload file").OnClick(func() {
			app.pickVisualImage(w.session, offset, state.picture)
		}).Absolute().Left(64).Top(16).Width(78).Height(24).Padding(0).Font(font).FontSize(15).FontWeight(600).
			Background(mygoui.Transparent).Border(0, mygoui.Transparent).TextColor(accent).
			Disabled(!w.editable(app))
		mygoui.Text(c.Key("image link prompt"), "or paste link").Absolute().Left(150).Top(17).
			Height(22).Font(font).FontSize(15).TextColor(ink.Alpha(.4)).Role(mygoui.RoleNone).IgnoreInput(true)
		return
	}
	mygoui.Button(c.Key("confirm image link"), "Confirm").Absolute().Right(24).Top(8).Width(min(101, width*.32)).Height(40).
		Font(font).FontSize(14).OnClick(submit).Disabled(!w.editable(app))
	if width >= 400 {
		if bitmap := w.images.load(app, strings.TrimSpace(state.link), false, nil); bitmap != nil {
			naturalW, naturalH := mygoui.ImageSize(bitmap)
			factor := min(1, 48/naturalH, 64/naturalW)
			mygoui.Image(c.Key("image link preview"), bitmap).Absolute().Right(135).Top((56-naturalH*factor)/2).
				Size(naturalW*factor, naturalH*factor).IgnoreInput(true).Role(mygoui.RoleNone)
		}
	}
}

func (w *workspaceWYSIWYG) drawImageActions(c *mygoui.Context, app *workspaceWindow, state *visualImageInput,
	offset int, width, wrapper, height, base, maxHeight float32) {
	right := (width-wrapper)/2 + 12
	chromeButtonIcon(c.Key("toggle image caption"), "bubble.left.and.text.bubble.right", "Toggle image caption", func() {
		w.commitImageCaption(app, offset, state)
		state.showCaption = !state.showCaption
	}, 24, w.math.appearance.surface).Absolute().Right(right).Top(12).Size(32, 32).Radius(16).
		Background(w.math.appearance.ink.Alpha(.6)).KeepFocus()
	if base <= 0 {
		return
	}
	w.drawImageTransformControls(c, app, state, offset, right)
	gripWidth := min(float32(160), wrapper)
	mygoui.Box(c.Key("image height")).Absolute().Left((width-gripWidth)/2).Top(max(height, visualImageControlHeight)-2).Size(gripWidth, 4).
		Radius(4).Background(w.math.appearance.ink.Alpha(.5)).Cursor(mygoui.CursorResizeRow).
		KeepFocus().Role(mygoui.RoleButton).Label("Resize image").Bind(&w.imageResize.handle).
		HandleInput(func(event mygoui.InputEvent) bool {
			return w.resizeVisualImage(c, app, state.picture, offset, event, height, base, maxHeight)
		})
}

func (w *workspaceWYSIWYG) resizeVisualImage(c *mygoui.Context, app *workspaceWindow, picture *wysiwyg.Image,
	offset int, event mygoui.InputEvent, height, base, maxHeight float32) bool {
	drag := &w.imageResize
	y := drag.handle.Bounds(c).Y + event.Y
	switch event.Kind {
	case mygoui.InputPointerDown:
		if event.Button != 0 || !w.editable(app) {
			return false
		}
		drag.picture, drag.pressY, drag.original, drag.height = picture, y, height, height
		w.session.Document.SetSelection(offset, offset+1)
	case mygoui.InputPointerMove, mygoui.InputPointerUp:
		if drag.picture != picture {
			return false
		}
		minHeight := float32(visualImageMinHeight)
		if picture.HasTransform() || picture.WikiWidth > 0 {
			minHeight = min(drag.original, base*.01)
		}
		drag.height = min(maxHeight, max(minHeight, drag.original+y-drag.pressY))
		if event.Kind == mygoui.InputPointerUp {
			drag.picture = nil
			if !event.Canceled && drag.height != drag.original {
				ratio := max(.01, math.Round(float64(drag.height/base)*100)/100)
				w.commitVisualImage(app, offset, picture, picture.Source, picture.Caption, ratio)
			}
		}
	default:
		return false
	}
	return true
}

func (w *workspaceWYSIWYG) drawImageCaption(c *mygoui.Context, app *workspaceWindow, state *visualImageInput, offset int, height float32) {
	input := mygoui.TextInput(c.Key("image caption"), &state.caption).Absolute().Top(height+visualImageCaptionGap).
		Left(0).Right(0).Height(visualImageCaptionHeight).MinHeight(0).Padding(0).
		Font(w.math.appearance.font).FontSize(11).TextAlign(mygoui.Center).FixedLineHeight(14).FocusRing(false).
		TextColor(w.math.appearance.ink).Background(mygoui.Transparent).Border(0, mygoui.Transparent).
		Placeholder("Write Image Caption").Label("Write Image Caption").Disabled(!w.editable(app))
	now := c.Now()
	input.OnChange(func() { state.deadline = now.Add(time.Second) })
	focused := input.Focused()
	if state.wasFocused && !focused {
		if w.commitImageCaption(app, offset, state) {
			c.Invalidate()
		}
	} else if !state.deadline.IsZero() {
		if remaining := state.deadline.Sub(now); remaining > 0 {
			c.After(remaining)
		} else {
			if w.commitImageCaption(app, offset, state) {
				c.Invalidate()
			}
		}
	}
	state.wasFocused = focused
	if focused {
		w.imageCaption = func() {
			if w.commitImageCaption(app, offset, state) {
				c.Invalidate()
			}
		}
		input.OnShortcut(mygoui.Cmd, mygoui.KeyS, func() {
			w.commitImageCaption(app, offset, state)
			app.save()
		})
	}
}

func (w *workspaceWYSIWYG) commitImageCaption(app *workspaceWindow, offset int, state *visualImageInput) bool {
	state.deadline = time.Time{}
	picture := state.picture
	if !w.editable(app) || picture.Caption == state.caption {
		return false
	}
	if next, ok := w.session.ChangeImage(offset, picture, picture.Source, state.caption, picture.Ratio); ok {
		state.picture = next
		w.sync(app)
		return true
	}
	return false
}

func (w *workspaceWYSIWYG) commitVisualImage(app *workspaceWindow, offset int, picture *wysiwyg.Image, source, caption string, ratio float64) {
	if w.editable(app) {
		if _, ok := w.session.ChangeImage(offset, picture, source, caption, ratio); ok {
			w.session.Document.SetSelection(offset, offset+1)
			w.focus = true
			w.sync(app)
		}
	}
}

func (app *workspaceWindow) pickVisualImage(session *wysiwyg.Session, offset int, picture *wysiwyg.Image) {
	win := app.win
	if app.modalOpen || win == nil || win.IsDestroyed() {
		return
	}
	root := app.root
	if app.storage != nil {
		root = app.folderRoot
	}
	app.modalOpen = true
	options := mygo.OpenDialogOptions{Parent: win, Title: "Upload file", DefaultPath: root,
		Filters: []mygo.FileFilter{{Name: "Images", Extensions: []string{"png", "jpg", "jpeg", "gif", "webp"}}}}
	go func() {
		files, err := mygo.Dialog.Open(options)
		win.Update(func() {
			app.modalOpen = false
			if err != nil {
				app.problem = fmt.Sprintf("Could not show the image picker. %v", err)
			} else if len(files) > 0 && session == app.wys.session {
				if current, ok := wysiwyg.ImageAt(session.Document.Text(), offset); ok && current == picture {
					app.applyVisualImageFile(session, offset, files[0])
				}
			}
		})
	}()
}

func (app *workspaceWindow) applyVisualImageFile(session *wysiwyg.Session, offset int, absolute string) {
	w := &app.wys
	if app.displayedMode() != wysiwygMode || w.session != session || !w.editable(app) {
		return
	}
	picture, ok := wysiwyg.ImageAt(session.Document.Text(), offset)
	if !ok || !picture.Block || picture.Source != "" {
		return
	}
	origin := app.previewOrigin()
	if origin.vault == nil || origin.note == "" {
		app.problem = "Save the note before importing an image."
		return
	}
	destination, err := origin.vault.ImportImage(origin.note, absolute)
	if err != nil {
		app.problem = fmt.Sprintf("Could not import the image. %v", err)
		return
	}
	token := importedImageMarkdown(origin.note, destination, "", picture.Caption)
	if strings.HasPrefix(token, "![[") && session.FillWikiImage(offset, picture, token) {
		session.Document.SetSelection(offset, offset+1)
		w.focus = true
		w.sync(app)
		return
	}
	w.commitVisualImage(app, offset, picture, destination, picture.Caption, picture.Ratio)
}
