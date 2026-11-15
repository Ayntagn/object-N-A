package ui

import (
	"strings"
	"unicode/utf16"

	"cyberwriter/internal/core/drawing"
	mygoui "github.com/egoist/mygo/ui"
)

type drawingTextDraft struct {
	index    int // -1 when adding a new element.
	position drawing.Point
	value    string
}

var drawingTextNewlines = strings.NewReplacer("\r\n", "\n", "\r", "\n")

func (d *workspaceDrawing) editTextAt(position drawing.Point) {
	index := d.hit(position, false)
	if index < 0 || d.scene.Elements()[index].Type != drawing.Text {
		index = -1
	}
	d.startTextEdit(position, index)
}

func (d *workspaceDrawing) startTextEdit(position drawing.Point, index int) {
	edit := &drawingTextDraft{index: index, position: position}
	if index >= 0 {
		edit.value = drawingTextNewlines.Replace(d.scene.Elements()[index].SourceText())
	}
	d.textEdit = edit
}

func (d *workspaceDrawing) textKey(event mygoui.InputEvent) bool {
	if event.Key != mygoui.KeyEnter || event.Mods != 0 || event.Repeat || d.gesture.active {
		return false
	}
	if d.tool == drawing.Text {
		center := drawing.Point{d.viewport[0] / 2, d.viewport[1] / 2}
		d.startTextEdit(d.scenePoint(center), -1)
		return true
	}
	if d.tool == drawingSelection && d.selected >= 0 && d.scene.Elements()[d.selected].Type == drawing.Text {
		d.startTextEdit(drawing.Point{}, d.selected)
		return true
	}
	return false
}

func (app *workspaceWindow) drawingTextEditor(c *mygoui.Context, d *workspaceDrawing) {
	edit := d.textEdit
	title, action := "Add Drawing Text", "Add text"
	if edit.index >= 0 {
		title, action = "Edit Drawing Text", "Apply"
	}
	closeEdit := func() {
		app.modalOpen = false
		d.textEdit = nil
	}
	apply := func(value string) {
		value = drawingTextNewlines.Replace(value)
		if strings.TrimSpace(value) == "" {
			return
		}
		d.applyText(edit, value)
		closeEdit()
	}
	mygoui.Modal(c.Key("drawing text editor"), &app.modalOpen, func() {
		mygoui.Text(c, title).Role(mygoui.RoleHeading).Bold()
		// Let the modal focus its first control after it records the canvas as its opener.
		mygoui.TextArea(c, &edit.value).Label("Drawing text").FillWidth().Lines(4, 8).
			OnTextKey(func(key mygoui.TextKey) bool {
				if key.Key == mygoui.KeyEnter && key.Mods == mygoui.Cmd {
					apply(key.Text())
					return true
				}
				return false
			})
		mygoui.Row(c).Gap(8).Justify(mygoui.End).Children(func() {
			mygoui.Button(c, "Cancel").OnClick(closeEdit)
			mygoui.PrimaryButton(c, action).Disabled(strings.TrimSpace(edit.value) == "").
				OnClick(func() { apply(edit.value) })
		})
	}).Label(title).Width(400)
	if !app.modalOpen {
		d.textEdit = nil
	}
}

func (d *workspaceDrawing) applyText(edit *drawingTextDraft, value string) {
	scene := d.scene
	if edit.index >= 0 {
		element := scene.Elements()[edit.index]
		if value != drawingTextNewlines.Replace(element.SourceText()) {
			displayed, width, height := layoutDrawingText(element, value)
			scene = scene.EditText(edit.index, value, displayed, width, height)
		}
		d.selected = edit.index
	} else {
		element := drawing.NewText(edit.position, value, 0, 0)
		_, element.Width, element.Height = layoutDrawingText(element, value)
		d.selected = len(scene.Elements())
		scene = scene.Add(element)
	}
	d.commit(scene)
	if !d.keepTool {
		d.tool = drawingSelection
	}
}

func layoutDrawingText(element drawing.Element, source string) (displayed string, width, height float64) {
	font := drawingTextFont(element)
	wrap := float32(0)
	if !element.AutoResize && element.Width > 0 {
		wrap = float32(element.Width)
	}
	layout := mygoui.ShapeText(source, font, wrap)
	w, _ := layout.Size()
	displayed = source
	lineCount := strings.Count(source, "\n") + 1
	if wrap > 0 {
		w = wrap
		// ShapeText reports offsets in UTF-16. Save the wrapped lines in
		// text and keep the typed content in originalText for future edits.
		units := utf16.Encode([]rune(source))
		lines := layout.Lines()
		wrapped := make([]string, len(lines))
		for index, line := range lines {
			wrapped[index] = string(utf16.Decode(units[line.Start:line.End]))
		}
		displayed = strings.Join(wrapped, "\n")
		lineCount = len(wrapped)
	}
	return displayed, float64(max(1, w)), float64(max(1, lineCount)) * float64(font.Size) * float64(font.LineHeight)
}

func drawingTextFont(element drawing.Element) mygoui.Font {
	size, lineHeight := element.FontSize, element.LineHeight
	if size <= 0 {
		size = drawing.DefaultTextFontSize
	}
	if lineHeight <= 0 {
		lineHeight = drawing.DefaultTextLineHeight
	}
	font := mygoui.Font{Size: float32(min(size, 512)), LineHeight: float32(min(lineHeight, 8))}
	if element.FontFamily == drawing.HelveticaFontFamily {
		font.Family = "Helvetica, Arial, sans-serif"
	} else if element.FontFamily == drawing.ExcalifontFamily {
		font.Family = "Chalkboard SE, Comic Sans MS, sans-serif"
	}
	return font
}

func paintDrawingText(p *mygoui.Painter, element drawing.Element, left, top, width float32, zoom float64, ink mygoui.Color) {
	font := drawingTextFont(element)
	font.Size = min(512, font.Size*float32(zoom))
	for index, line := range strings.Split(element.Text, "\n") {
		layout := mygoui.ShapeText(line, font, 0)
		x := left
		w, _ := layout.Size()
		switch element.TextAlign {
		case "center":
			x += (width - w) / 2
		case "right":
			x += width - w
		}
		p.TextLayout(layout, x, top+float32(index)*font.Size*font.LineHeight, ink)
	}
}
