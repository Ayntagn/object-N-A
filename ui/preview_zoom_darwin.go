package ui

import (
	"slices"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

type previewNativePoint struct{ X, Y float64 }
type previewNativeSize struct{ Width, Height float64 }
type previewNativeRect struct {
	Origin previewNativePoint
	Size   previewNativeSize
}

// MyGo mounts a new WKWebView in its own window during NewWebView. Compare
// the view trees on the main thread to identify that specific instance.
func observeNewPreviewMagnifier(win *mygo.Window) func() func(float64) {
	window := objc.ID(win.NativeHandle())
	if window == 0 {
		return nil
	}
	root := window.Send(objc.RegisterName("contentView"))
	before := previewNativeWebViews(root, nil)
	return func() func(float64) {
		var added objc.ID
		for _, view := range previewNativeWebViews(root, nil) {
			if slices.Contains(before, view) {
				continue
			}
			if added != 0 {
				return nil
			}
			added = view
		}
		if added == 0 {
			return nil
		}
		boundsSelector := objc.RegisterName("bounds")
		zoomSelector := objc.RegisterName("setMagnification:centeredAtPoint:")
		return func(zoom float64) {
			bounds := objc.Send[previewNativeRect](added, boundsSelector)
			center := previewNativePoint{bounds.Origin.X + bounds.Size.Width/2, bounds.Origin.Y + bounds.Size.Height/2}
			objc.Send[objc.ID](added, zoomSelector, zoom, center)
		}
	}
}

func previewNativeWebViews(view objc.ID, matches []objc.ID) []objc.ID {
	if view == 0 {
		return matches
	}
	if class := objc.GetClass("WKWebView"); class != 0 && objc.Send[bool](view, objc.RegisterName("isKindOfClass:"), class) {
		return append(matches, view)
	}
	children := view.Send(objc.RegisterName("subviews"))
	count := objc.Send[uint](children, objc.RegisterName("count"))
	for index := range count {
		child := children.Send(objc.RegisterName("objectAtIndex:"), index)
		matches = previewNativeWebViews(child, matches)
	}
	return matches
}
