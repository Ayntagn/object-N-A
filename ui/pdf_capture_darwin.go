package ui

import (
	"errors"
	"fmt"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

const pdfInitialViewportHeight = pdfMaximumViewportHeight

func capturePDFContent(window *mygo.Window, width, height float64) ([]pdfSource, error) {
	const chunkHeight = 14000 // EV-UI-132: WebKit's long-document capture threshold.
	var sources []pdfSource
	for top := 0.0; top < height; top += chunkHeight {
		h := min(chunkHeight, height-top)
		data, err := capturePDFRect(window, width, h, top)
		if err != nil {
			return nil, err
		}
		sources = append(sources, pdfSource{Top: top, Height: h, Data: data})
	}
	return sources, nil
}

func capturePDFRect(window *mygo.Window, width, height, top float64) ([]byte, error) {
	type capture struct {
		data []byte
		err  error
	}
	done := make(chan capture, 1)
	mygo.RunOnMain(func() {
		native := objc.ID(window.NativeHandle())
		if native == 0 {
			done <- capture{err: errors.New("the PDF capture window closed")}
			return
		}
		root := native.Send(objc.RegisterName("contentView"))
		views := previewNativeWebViews(root, nil)
		method := objc.RegisterName("createPDFWithConfiguration:completionHandler:")
		if len(views) != 1 || !objc.Send[bool](views[0], objc.RegisterName("respondsToSelector:"), method) {
			done <- capture{err: errors.New("the system WebView cannot capture a PDF")}
			return
		}
		configuration := objc.ID(objc.GetClass("WKPDFConfiguration")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("init"))
		defer configuration.Send(objc.RegisterName("release"))
		rect := previewNativeRect{Origin: previewNativePoint{Y: top}, Size: previewNativeSize{Width: width, Height: height}}
		configuration.Send(objc.RegisterName("setRect:"), rect)
		if selector := objc.RegisterName("setAllowTransparentBackground:"); objc.Send[bool](configuration, objc.RegisterName("respondsToSelector:"), selector) {
			configuration.Send(selector, true)
		}
		completion := objc.NewBlock(func(_ objc.Block, data, errID objc.ID) {
			if errID != 0 {
				message := errID.Send(objc.RegisterName("localizedDescription"))
				done <- capture{err: fmt.Errorf("WebKit PDF capture failed: %s", objc.Send[string](message, objc.RegisterName("UTF8String")))}
				return
			}
			length := objc.Send[uint](data, objc.RegisterName("length"))
			if length == 0 {
				done <- capture{err: errors.New("WebKit PDF capture returned no data")}
				return
			}
			pdf := make([]byte, length)
			data.Send(objc.RegisterName("getBytes:length:"), &pdf[0], length)
			done <- capture{data: pdf}
		})
		defer completion.Release()
		views[0].Send(method, configuration, completion)
	})
	select {
	case result := <-done:
		return result.data, result.err
	case <-time.After(15 * time.Second):
		return nil, errors.New("timed out capturing PDF content")
	}
}
