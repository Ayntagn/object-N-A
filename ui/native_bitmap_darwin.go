package ui

import (
	"runtime"

	"github.com/ebitengine/purego/objc"
)

const nativeBitmapJPEG = 3
const nativeBitmapPNG = 4

func nativeAutoreleasePool() func() {
	// Foundation drains an autorelease pool on the thread that created it.
	runtime.LockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("init"))
	return func() {
		defer runtime.UnlockOSThread()
		pool.Send(objc.RegisterName("drain"))
	}
}

func nativeNSString(value string) objc.ID {
	return objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), value)
}

func nativeBitmapBytes(image objc.ID, format uint, options objc.ID) []byte {
	tiff := image.Send(objc.RegisterName("TIFFRepresentation"))
	if tiff == 0 {
		return nil
	}
	bitmap := objc.ID(objc.GetClass("NSBitmapImageRep")).Send(objc.RegisterName("imageRepWithData:"), tiff)
	data := bitmap.Send(objc.RegisterName("representationUsingType:properties:"), format, options)
	length := objc.Send[uint](data, objc.RegisterName("length"))
	if length == 0 {
		return nil
	}
	result := make([]byte, length)
	data.Send(objc.RegisterName("getBytes:length:"), &result[0], length)
	return result
}

func nativeMenuSymbol(symbol string) []byte {
	defer nativeAutoreleasePool()()
	image := objc.ID(objc.GetClass("NSImage")).Send(objc.RegisterName("imageWithSystemSymbolName:accessibilityDescription:"), nativeNSString(symbol), objc.ID(0))
	return nativeBitmapBytes(image, nativeBitmapPNG, 0)
}
