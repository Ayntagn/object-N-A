package ui

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// Give MyGo's surface the AppKit first responder before selecting a field
// inside it when the embedded preview has the keyboard.
func focusWorkspaceSurface(win *mygo.Window) {
	if win == nil || win.IsDestroyed() {
		return
	}
	window := objc.ID(win.NativeHandle())
	content := window.Send(objc.RegisterName("contentView"))
	views := content.Send(objc.RegisterName("subviews"))
	surface := objc.GetClass("MyGoSurfaceView")
	count := objc.Send[uint](views, objc.RegisterName("count"))
	for i := range count {
		view := views.Send(objc.RegisterName("objectAtIndex:"), i)
		if objc.Send[bool](view, objc.RegisterName("isKindOfClass:"), surface) {
			window.Send(objc.RegisterName("makeFirstResponder:"), view)
			return
		}
	}
}

func (app *workspaceWindow) installWorkspaceShortcutMonitor() {
	window := objc.ID(app.win.NativeHandle())
	if window == 0 {
		return
	}
	const (
		keyDownMask = 1 << 10
		shift       = 1 << 17
		control     = 1 << 18
		option      = 1 << 19
		command     = 1 << 20
	)
	eventClass := objc.ID(objc.GetClass("NSEvent"))
	application := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	keyWindow := objc.RegisterName("keyWindow")
	eventWindow := objc.RegisterName("window")
	modifierFlags := objc.RegisterName("modifierFlags")
	characters := objc.RegisterName("charactersIgnoringModifiers")
	utf8String := objc.RegisterName("UTF8String")
	isRepeat := objc.RegisterName("isARepeat")

	// AppKit delivers these before the focused input method or embedded WebView.
	handler := objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
		flags := objc.Send[uint](event, modifierFlags) & (shift | control | option | command)
		if app.modalOpen || application.Send(keyWindow) != window {
			return event
		}
		if target := event.Send(eventWindow); target != 0 && target != window {
			return event
		}
		p := app.presentation
		if flags != shift|command && flags != command && (p == nil || flags != shift && flags != 0) {
			return event
		}
		key := objc.Send[string](event.Send(characters), utf8String)
		if flags == command {
			if key == "p" && p == nil && app.drawing == nil {
				if !objc.Send[bool](event, isRepeat) {
					app.win.Update(app.toggleQuickSwitcher)
				}
				return 0
			}
			return event
		}
		if p != nil && (flags == shift || flags == 0) {
			// MyGo's native key enum does not represent shifted punctuation.
			switch {
			case key == "?" || flags == shift && key == "/":
				if !objc.Send[bool](event, isRepeat) {
					app.win.Update(func() { p.act(app, presentationHelp) })
				}
				return 0
			case key == "+" || flags == shift && key == "=":
				app.win.Update(func() { p.act(app, presentationZoomIn) })
				return 0
			default:
				return event
			}
		}
		if key != "/" && key != "?" {
			return event
		}
		if app.quickSwitcher.open {
			return 0
		}
		if !objc.Send[bool](event, isRepeat) {
			app.win.Update(app.showShortcuts)
		}
		return 0
	})
	monitor := eventClass.Send(objc.RegisterName("addLocalMonitorForEventsMatchingMask:handler:"), uint(keyDownMask), handler)
	if monitor == 0 {
		handler.Release()
		return
	}
	app.win.OnClosed(func() {
		eventClass.Send(objc.RegisterName("removeMonitor:"), monitor)
		handler.Release()
	})
}
