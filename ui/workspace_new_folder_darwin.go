package ui

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

func showWorkspaceNamePrompt(_ *mygo.Window, prompt workspaceNamePrompt, done func(string, bool)) {
	defer nativeAutoreleasePool()()
	const (
		nsAlertStyleInformational = 1
		nsAlertFirstButtonReturn  = 1000
	)
	alert := objc.ID(objc.GetClass("NSAlert")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("init"))
	defer alert.Send(objc.RegisterName("release"))
	alert.Send(objc.RegisterName("setAlertStyle:"), nsAlertStyleInformational)
	alert.Send(objc.RegisterName("setMessageText:"), nativeNSString(prompt.heading))
	if prompt.detail != "" {
		alert.Send(objc.RegisterName("setInformativeText:"), nativeNSString(prompt.detail))
	}
	alert.Send(objc.RegisterName("addButtonWithTitle:"), nativeNSString(prompt.action))
	alert.Send(objc.RegisterName("addButtonWithTitle:"), nativeNSString("Cancel"))
	if prompt.icon != "" {
		icon := objc.ID(objc.GetClass("NSImage")).Send(objc.RegisterName("imageWithSystemSymbolName:accessibilityDescription:"),
			nativeNSString(prompt.icon), nativeNSString(prompt.title))
		if icon != 0 {
			alert.Send(objc.RegisterName("setIcon:"), icon)
		}
	}

	frame := previewNativeRect{Size: previewNativeSize{Width: 280, Height: 24}}
	field := objc.ID(objc.GetClass("NSTextField")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithFrame:"), frame)
	defer field.Send(objc.RegisterName("release"))
	field.Send(objc.RegisterName("setStringValue:"), nativeNSString(prompt.initial))
	alert.Send(objc.RegisterName("setAccessoryView:"), field)
	alert.Send(objc.RegisterName("window")).Send(objc.RegisterName("setInitialFirstResponder:"), field)

	if objc.Send[int](alert, objc.RegisterName("runModal")) != nsAlertFirstButtonReturn {
		done("", false)
		return
	}
	value := objc.Send[string](field.Send(objc.RegisterName("stringValue")), objc.RegisterName("UTF8String"))
	done(value, true)
}
