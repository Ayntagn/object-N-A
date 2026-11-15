package ui

import (
	"cyberwriter/internal/core/audioinput"
	mygoui "github.com/egoist/mygo/ui"
)

func (app *workspaceWindow) microphoneControl(c *mygoui.Context) mygoui.Element {
	ink := c.Theme().Text
	if app.blackout && !c.Preferences().HighContrast {
		ink = blackoutToolbarMenuInk
	}
	return chromeMenuControl(c, "Microphone", app.microphoneMenu).
		Size(20, 16).KeepFocus().Disabled(app.modalOpen).Children(func() {
		chromeIconColor(c, "mic", 17, ink)
	})
}

// EV-UI-209 / EV-UI-212. Input Source is available independently of recording.
func (app *workspaceWindow) microphoneMenu(m *mygoui.Menu) {
	m.Item("Record Voice Note").Disabled(true)
	m.Item("Record & Transcribe").Disabled(true)
	m.Separator()
	m.Item("Live Dictation").Disabled(true)
	m.Separator()
	m.Submenu("Input Source", app.inputSourceMenu)
}

func (app *workspaceWindow) inputSourceMenu(m *mygoui.Menu) {
	m.Item("Input Source").Disabled(true)
	selected := app.preferences.PreferredInputDeviceUID
	if m.Item("System Microphone").ID("system-input").Checked(selected == "").Chosen() {
		app.setPreferredInputDevice("")
	}
	devices, err := audioinput.List()
	if err != nil || len(devices) == 0 {
		return
	}
	m.Separator()
	for _, device := range devices {
		if m.Item(device.Name).ID("input-device:" + device.UID).Checked(device.UID == selected).Chosen() {
			app.setPreferredInputDevice(device.UID)
		}
	}
}

func (app *workspaceWindow) setPreferredInputDevice(uid string) {
	if app.preferences.PreferredInputDeviceUID != uid {
		app.preferences.PreferredInputDeviceUID = uid
		app.writePreferences()
	}
}
