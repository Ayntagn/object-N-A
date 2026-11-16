package ui

import (
	"context"
	"errors"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

func readAIReferencePDFPages(ctx context.Context, pdf []byte) ([]string, error) {
	if err := openPDFKit(); err != nil {
		return nil, err
	}
	defer nativeAutoreleasePool()()
	data := objc.ID(objc.GetClass("NSData")).Send(objc.RegisterName("dataWithBytes:length:"), &pdf[0], uint(len(pdf)))
	runtime.KeepAlive(pdf)
	doc := objc.ID(objc.GetClass("PDFDocument")).Send(objc.RegisterName("alloc")).
		Send(objc.RegisterName("initWithData:"), data)
	if doc == 0 {
		return nil, errUnreadableWorkspacePDF
	}
	defer doc.Send(objc.RegisterName("release"))
	if objc.Send[bool](doc, objc.RegisterName("isLocked")) {
		return nil, errors.New("the PDF is password-protected")
	}
	count := objc.Send[uint](doc, objc.RegisterName("pageCount"))
	pages := make([]string, 0, count)
	for index := range count {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if text, ok := aiReferencePDFPageText(doc, index); ok {
			pages = append(pages, text)
		}
	}
	return pages, nil
}

func aiReferencePDFPageText(doc objc.ID, index uint) (string, bool) {
	defer nativeAutoreleasePool()()
	page := doc.Send(objc.RegisterName("pageAtIndex:"), index)
	text := page.Send(objc.RegisterName("string"))
	if text == 0 {
		return "", false
	}
	const utf8Encoding = uint(4)
	length := objc.Send[uint](text, objc.RegisterName("lengthOfBytesUsingEncoding:"), utf8Encoding)
	data := objc.Send[*byte](text, objc.RegisterName("UTF8String"))
	if data == nil || length == 0 {
		return "", true
	}
	return string(unsafe.Slice(data, length)), true
}
