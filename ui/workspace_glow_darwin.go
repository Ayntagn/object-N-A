package ui

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

func configureGlowPanel(win *mygo.Window) {
	if panel := objc.ID(win.NativeHandle()); panel != 0 {
		panel.Send(objc.RegisterName("setFloatingPanel:"), true)
		panel.Send(objc.RegisterName("setAccessibilitySubrole:"), nativeNSString("AXFloatingWindow"))
	}
}
