package ui

import (
	"errors"
	"runtime"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

func (viewer *workspacePDFWindow) mount(pdf []byte) error {
	if err := openPDFKit(); err != nil {
		return err
	}
	win := viewer.win
	var err error
	mygo.RunOnMain(func() {
		defer nativeAutoreleasePool()()
		window := objc.ID(win.NativeHandle())
		if window == 0 {
			err = errors.New("the PDF window closed")
			return
		}
		content := window.Send(objc.RegisterName("contentView"))
		bounds := objc.Send[previewNativeRect](content, objc.RegisterName("bounds"))
		var bytes *byte
		if len(pdf) > 0 {
			bytes = &pdf[0]
		}
		data := objc.ID(objc.GetClass("NSData")).Send(objc.RegisterName("dataWithBytes:length:"), bytes, uint(len(pdf)))
		runtime.KeepAlive(pdf)
		doc := objc.ID(objc.GetClass("PDFDocument")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithData:"), data)
		if doc == 0 {
			err = errUnreadableWorkspacePDF
			return
		}
		defer doc.Send(objc.RegisterName("release"))
		view := objc.ID(objc.GetClass("PDFView")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithFrame:"), bounds)
		if view == 0 {
			err = errors.New("PDFKit could not create its viewer")
			return
		}
		defer view.Send(objc.RegisterName("release"))
		view.Send(objc.RegisterName("setAutoresizingMask:"), uint(2|16)) // Sizable width and height.
		view.Send(objc.RegisterName("setAutoScales:"), true)
		view.Send(objc.RegisterName("setDisplayMode:"), int64(1))      // Single-page continuous.
		view.Send(objc.RegisterName("setDisplayDirection:"), int64(0)) // Vertical.
		view.Send(objc.RegisterName("setDisplaysPageBreaks:"), true)
		background := objc.ID(objc.GetClass("NSColor")).Send(objc.RegisterName("windowBackgroundColor"))
		view.Send(objc.RegisterName("setBackgroundColor:"), background)
		view.Send(objc.RegisterName("setDocument:"), doc)
		content.Send(objc.RegisterName("addSubview:"), view)
		window.Send(objc.RegisterName("makeFirstResponder:"), view)
		viewer.zoom = func(action workspacePDFZoom) { zoomPDFKitView(view, action) }
		// PDFKit can center an oversized first page while the window is
		// hidden. Navigate to the top edge once the real viewport is shown.
		var off func()
		off = win.OnShow(func() {
			off()
			defer nativeAutoreleasePool()()
			page := view.Send(objc.RegisterName("document")).Send(objc.RegisterName("pageAtIndex:"), uint(0))
			if page == 0 {
				return
			}
			box := objc.Send[previewNativeRect](page, objc.RegisterName("boundsForBox:"), int64(1)) // Crop box.
			top := previewNativePoint{X: box.Origin.X, Y: box.Origin.Y + box.Size.Height}
			destination := objc.ID(objc.GetClass("PDFDestination")).Send(objc.RegisterName("alloc")).
				Send(objc.RegisterName("initWithPage:atPoint:"), page, top)
			if destination != 0 {
				view.Send(objc.RegisterName("goToDestination:"), destination)
				destination.Send(objc.RegisterName("release"))
			}
		})
	})
	return err
}

func zoomPDFKitView(view objc.ID, action workspacePDFZoom) {
	defer nativeAutoreleasePool()()
	switch action {
	case workspacePDFZoomIn:
		view.Send(objc.RegisterName("zoomIn:"), objc.ID(0))
	case workspacePDFZoomOut:
		view.Send(objc.RegisterName("zoomOut:"), objc.ID(0))
	case workspacePDFActualSize:
		view.Send(objc.RegisterName("setAutoScales:"), false)
		view.Send(objc.RegisterName("setScaleFactor:"), float64(1))
	case workspacePDFFitWidth:
		view.Send(objc.RegisterName("setAutoScales:"), true)
	}
}
