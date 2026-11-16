//go:build !darwin

package ui

import "github.com/egoist/mygo"

func splashHostWindows() []*mygo.Window {
	return mygo.Windows()
}

func installSplashKeyMonitor(*mygo.Window, func()) {}
