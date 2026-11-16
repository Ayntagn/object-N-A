package ui

import (
	"slices"
	"time"

	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-281: both the panel and its composer accept file URLs. MyGo sends
// each file drop to the innermost registered element.
func aiReferenceDrop(zone mygoui.Element, accept func([]string)) bool {
	if accept == nil {
		return false
	}
	if files := zone.DroppedFiles(); len(files) > 0 {
		accept(files)
	}
	return zone.FileDragOver()
}

func aiComposerReferenceDrop(card mygoui.Element, radius float32, color mygoui.Color, accept func([]string)) {
	target := float32(0)
	if aiReferenceDrop(card, accept) {
		target = 1
	}
	if highlight := card.Animate("reference drop border", target, 150*time.Millisecond); highlight > 0 {
		card.DrawOver(func(p *mygoui.Painter, bounds mygoui.Rect) {
			p.Stroke(bounds, color.Alpha(highlight), radius, 1+0.5*highlight)
		})
	}
}

func (app *workspaceWindow) dropAIReferences(paths []string) {
	win, chat := app.win, &app.ai
	if win == nil || win.IsDestroyed() || app.modalOpen || len(paths) == 0 {
		return
	}
	if load := chat.referenceLoad; load != nil {
		load.waiting = append(load.waiting, paths...)
		return
	}
	app.readAIReferenceBatch(chat.beginAIReferenceLoad(), slices.Clone(paths))
}
