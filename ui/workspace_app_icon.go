package ui

import (
	_ "embed"
	"log"
	"runtime"

	"github.com/egoist/mygo"
)

//go:embed app_icon.png
var workspaceAppIcon []byte

func setWorkspaceAppIcon() {
	// MyGo's Linux Dock adapter sets GTK's default icon for every window.
	if runtime.GOOS == "linux" {
		if err := mygo.App.Dock.SetIcon(workspaceAppIcon); err != nil {
			log.Printf("Set object-N/A icon: %v", err)
		}
	}
}
