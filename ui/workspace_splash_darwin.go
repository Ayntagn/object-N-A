//go:build darwin

package ui

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

func splashHostWindows() []*mygo.Window {
	const titledWindow = 1 << 0
	windows := make(map[objc.ID]*mygo.Window)
	for _, window := range mygo.Windows() {
		if !window.IsDestroyed() {
			windows[objc.ID(window.NativeHandle())] = window
		}
	}
	application := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	native := application.Send(objc.RegisterName("windows"))
	count := objc.Send[uint](native, objc.RegisterName("count"))
	styleMask := objc.RegisterName("styleMask")
	at := objc.RegisterName("objectAtIndex:")
	var ordered []*mygo.Window
	for i := uint(0); i < count; i++ {
		handle := native.Send(at, i)
		if window := windows[handle]; window != nil && objc.Send[uint](handle, styleMask)&titledWindow != 0 {
			ordered = append(ordered, window)
		}
	}
	return ordered
}

func installSplashKeyMonitor(window *mygo.Window, finish func()) {
	application := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	keyWindow := objc.RegisterName("keyWindow")
	handle := objc.ID(window.NativeHandle())
	eventClass := objc.ID(objc.GetClass("NSEvent"))
	handler := objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
		if application.Send(keyWindow) != handle {
			return event
		}
		finish()
		return 0
	})
	const keyDownMask = 1 << 10
	monitor := eventClass.Send(objc.RegisterName("addLocalMonitorForEventsMatchingMask:handler:"), uint(keyDownMask), handler)
	if monitor == 0 {
		handler.Release()
		return
	}
	window.OnClosed(func() {
		eventClass.Send(objc.RegisterName("removeMonitor:"), monitor)
		handler.Release()
	})
}
