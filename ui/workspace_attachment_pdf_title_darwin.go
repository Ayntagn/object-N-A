package ui

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ebitengine/purego/objc"
)

const (
	pdfTitleFileURLKey    = "fileURL"
	pdfFeatureUnsupported = 3328 // NSFeatureUnsupportedError.
)

type pdfTitleClasses struct {
	document objc.Class
	observer objc.Class
}

// The association lives for the native window's lifetime. KVO is allowed
// to deliver a file URL change away from the UI thread.
var pdfTitleObservers sync.Map // objc.ID → *pdfTitleObservation

type pdfTitleObservation struct {
	viewer *workspacePDFWindow
	app    *workspaceApplication
}

var nativePDFTitleClasses = sync.OnceValues(func() (pdfTitleClasses, error) {
	document, err := objc.RegisterClass("CWPDFAttachmentDocument", objc.GetClass("NSDocument"), nil, nil,
		[]objc.MethodDef{
			{Cmd: objc.RegisterName("writeSafelyToURL:ofType:forSaveOperation:error:"), Fn: rejectPDFContentWrite},
		})
	if err != nil {
		return pdfTitleClasses{}, err
	}
	meta := objc.ID(document).Class()
	for _, setting := range []struct {
		name  string
		value bool
	}{
		{"autosavesInPlace", true}, // AppKit uses this to provide document actions.
		{"preservesVersions", false},
		{"usesUbiquitousStorage", false},
	} {
		if !meta.AddMethod(objc.RegisterName(setting.name),
			objc.NewIMP(func(objc.ID, objc.SEL) bool { return setting.value }), "B@:") {
			return pdfTitleClasses{}, fmt.Errorf("could not register PDF document class method %s", setting.name)
		}
	}
	observer, err := objc.RegisterClass("CWPDFAttachmentURLObserver", objc.GetClass("NSObject"), nil, nil,
		[]objc.MethodDef{
			{Cmd: objc.RegisterName("observeValueForKeyPath:ofObject:change:context:"), Fn: observePDFTitleURL},
		})
	return pdfTitleClasses{document: document, observer: observer}, err
})

func rejectPDFContentWrite(_ objc.ID, _ objc.SEL, _ objc.ID, _ objc.ID, _ int64, out *objc.ID) bool {
	if out != nil {
		*out = objc.ID(objc.GetClass("NSError")).Send(objc.RegisterName("errorWithDomain:code:userInfo:"),
			nativeNSString("NSCocoaErrorDomain"), int64(pdfFeatureUnsupported), objc.ID(0))
	}
	return false
}

// EV-UI-180/181. Keep the existing NSWindow delegate and PDFKit view. The
// native document controller supplies the title popover and metadata edits.
func bindPDFDocumentTitle(viewer *workspacePDFWindow, app *workspaceApplication) {
	defer nativeAutoreleasePool()()
	win := viewer.win
	window := objc.ID(win.NativeHandle())
	if window == 0 {
		return
	}
	classes, err := nativePDFTitleClasses()
	if err != nil {
		log.Printf("PDF title actions: %v", err)
		return
	}
	doc := objc.ID(classes.document).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("init"))
	controller := objc.ID(objc.GetClass("NSWindowController")).Send(objc.RegisterName("alloc")).
		Send(objc.RegisterName("initWithWindow:"), window)
	observer := objc.ID(classes.observer).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("init"))
	if doc == 0 || controller == 0 || observer == 0 {
		controller.Send(objc.RegisterName("setWindow:"), objc.ID(0))
		controller.Send(objc.RegisterName("release"))
		doc.Send(objc.RegisterName("release"))
		observer.Send(objc.RegisterName("release"))
		log.Print("PDF title actions: could not create the document controller")
		return
	}
	url := objc.ID(objc.GetClass("NSURL")).Send(objc.RegisterName("fileURLWithPath:"), nativeNSString(viewer.absolute))
	doc.Send(objc.RegisterName("setFileURL:"), url)
	doc.Send(objc.RegisterName("setFileType:"), nativeNSString("com.adobe.pdf"))
	doc.Send(objc.RegisterName("addWindowController:"), controller)
	pdfTitleObservers.Store(observer, &pdfTitleObservation{viewer: viewer, app: app})
	doc.Send(objc.RegisterName("addObserver:forKeyPath:options:context:"),
		observer, nativeNSString(pdfTitleFileURLKey), uint(1), uintptr(0)) // NSKeyValueObservingOptionNew.
	win.OnClosed(func() {
		defer nativeAutoreleasePool()()
		pdfTitleObservers.Delete(observer)
		doc.Send(objc.RegisterName("removeObserver:forKeyPath:context:"), observer, nativeNSString(pdfTitleFileURLKey), uintptr(0))
		doc.Send(objc.RegisterName("removeWindowController:"), controller)
		controller.Send(objc.RegisterName("setWindow:"), objc.ID(0))
		observer.Send(objc.RegisterName("release"))
		controller.Send(objc.RegisterName("release"))
		doc.Send(objc.RegisterName("release"))
	})
}

func observePDFTitleURL(observer objc.ID, _ objc.SEL, _ objc.ID, _ objc.ID, change objc.ID, _ uintptr) {
	value, ok := pdfTitleObservers.Load(observer)
	if !ok {
		return
	}
	defer nativeAutoreleasePool()()
	url := change.Send(objc.RegisterName("objectForKey:"), nativeNSString("new"))
	if !objc.Send[bool](url, objc.RegisterName("isKindOfClass:"), objc.GetClass("NSURL")) ||
		!objc.Send[bool](url, objc.RegisterName("isFileURL")) {
		return
	}
	path := url.Send(objc.RegisterName("path"))
	absolute := strings.Clone(objc.Send[string](path, objc.RegisterName("UTF8String")))
	if !filepath.IsAbs(absolute) {
		return
	}
	absolute = filepath.Clean(absolute)
	obs := value.(*pdfTitleObservation)
	obs.viewer.win.Update(func() { obs.updatePath(absolute) })
}

func (obs *pdfTitleObservation) updatePath(absolute string) {
	viewer, a := obs.viewer, obs.app
	if viewer.win.IsDestroyed() || a.pdfWindows[viewer.absolute] != viewer {
		return
	}
	old := viewer.absolute
	if !a.relocatePDFWindow(old, absolute) {
		return
	}
	for _, workspace := range a.windows {
		_, wasInside := workspace.relativeToVault(old)
		_, isInside := workspace.relativeToVault(absolute)
		if wasInside || isInside {
			workspace.win.Update(workspace.refreshVault)
		}
	}
}

func setPDFDocumentURL(viewer *workspacePDFWindow) {
	defer nativeAutoreleasePool()()
	window := objc.ID(viewer.win.NativeHandle())
	doc := window.Send(objc.RegisterName("windowController")).Send(objc.RegisterName("document"))
	if doc == 0 {
		return
	}
	url := objc.ID(objc.GetClass("NSURL")).Send(objc.RegisterName("fileURLWithPath:"), nativeNSString(viewer.absolute))
	if !objc.Send[bool](doc.Send(objc.RegisterName("fileURL")), objc.RegisterName("isEqual:"), url) {
		doc.Send(objc.RegisterName("setFileURL:"), url)
	}
}
