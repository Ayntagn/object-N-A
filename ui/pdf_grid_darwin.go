package ui

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var openPDFKit = sync.OnceValue(func() error {
	_, err := purego.Dlopen("/System/Library/Frameworks/PDFKit.framework/PDFKit", purego.RTLD_LAZY|purego.RTLD_LOCAL)
	return err
})

func renderPlatformPDFGrid(ctx context.Context, w io.Writer, pdf []byte, style pdfThumbnailStyle) error {
	if len(pdf) == 0 {
		return errors.New("PDF preview is empty")
	}
	if err := openPDFKit(); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer nativeAutoreleasePool()()
	data := objc.ID(objc.GetClass("NSData")).Send(objc.RegisterName("dataWithBytes:length:"), &pdf[0], uint(len(pdf)))
	runtime.KeepAlive(pdf)
	doc := objc.ID(objc.GetClass("PDFDocument")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithData:"), data)
	if doc == 0 {
		return errors.New("PDFKit could not open the PDF preview")
	}
	defer doc.Send(objc.RegisterName("release"))
	if err := writePDFGridHeader(w, "", style); err != nil {
		return err
	}
	if _, err := io.WriteString(w, `<main id="pages" aria-label="PDF pages">`); err != nil {
		return err
	}
	quality := objc.ID(objc.GetClass("NSNumber")).Send(objc.RegisterName("numberWithInt:"), int32(2))
	options := objc.ID(objc.GetClass("NSDictionary")).Send(objc.RegisterName("dictionaryWithObject:forKey:"), quality, nativeNSString("NSImageCompressionFactor"))
	count := min(objc.Send[uint](doc, objc.RegisterName("pageCount")), pdfGridMaxPages)
	shown := 0
	for index := range count {
		if err := ctx.Err(); err != nil {
			return err
		}
		jpeg := pdfKitPageThumbnail(doc, index, options, float64(style.maxSide()))
		if len(jpeg) == 0 {
			continue
		}
		shown++
		if _, err := fmt.Fprintf(w, `<div class="page-card"><img alt="Page %d" src="data:image/jpeg;base64,%s"><span class="page-number">%d</span></div>`,
			shown, base64.StdEncoding.EncodeToString(jpeg), shown); err != nil {
			return err
		}
	}
	if shown == 0 {
		return errors.New("PDFKit could not create page thumbnails")
	}
	_, err := io.WriteString(w, `</main></body></html>`)
	return err
}

func pdfKitPageThumbnail(doc objc.ID, index uint, options objc.ID, maxSide float64) []byte {
	defer nativeAutoreleasePool()()
	page := doc.Send(objc.RegisterName("pageAtIndex:"), index)
	if page == 0 {
		return nil
	}
	const mediaBox = int64(0)
	bounds := objc.Send[previewNativeRect](page, objc.RegisterName("boundsForBox:"), mediaBox)
	size := bounds.Size
	if size.Width <= 0 || size.Height <= 0 {
		return nil
	}
	fit := maxSide / max(size.Width, size.Height)
	size = previewNativeSize{Width: size.Width * fit, Height: size.Height * fit}
	image := page.Send(objc.RegisterName("thumbnailOfSize:forBox:"), size, mediaBox)
	return nativeBitmapBytes(image, nativeBitmapJPEG, options)
}
