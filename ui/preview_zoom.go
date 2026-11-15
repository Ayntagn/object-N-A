package ui

import "github.com/egoist/mygo"

const (
	previewZoomDefault = 100
	previewZoomMin     = 25
	previewZoomMax     = 400
	previewZoomStep    = 25
)

func normalizedPreviewZoom(percentage int) int {
	if percentage <= 0 {
		return previewZoomDefault
	}
	return min(previewZoomMax, max(previewZoomMin, percentage))
}

func (app *workspaceWindow) setPreviewZoom(percentage int) {
	zoom := normalizedPreviewZoom(percentage)
	if zoom == app.previewZoom {
		return
	}
	app.previewZoom = zoom
	app.preferences.Session.PreviewZoom = zoom
	app.writePreferences()
}

type previewViewport struct {
	*mygo.WebView
	magnify func(float64)
}

func newPreviewViewport(win *mygo.Window, options mygo.WebViewOptions) (*previewViewport, error) {
	var view *mygo.WebView
	var magnify func(float64)
	var err error
	options.Page.PinchZoom = true
	mygo.RunOnMain(func() {
		connect := observeNewPreviewMagnifier(win)
		view, err = win.NewWebView(options)
		if err == nil && connect != nil {
			magnify = connect()
		}
	})
	if err != nil {
		return nil, err
	}
	return &previewViewport{WebView: view, magnify: magnify}, nil
}

func (v *previewViewport) setZoom(percentage int) {
	zoom := float64(normalizedPreviewZoom(percentage)) / 100
	if v.magnify == nil {
		v.Page().SetZoomFactor(zoom)
		return
	}
	mygo.RunOnMain(func() {
		if !v.IsDestroyed() {
			v.magnify(zoom)
		}
	})
}
