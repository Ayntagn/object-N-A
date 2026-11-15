//go:build !darwin

package ui

import "github.com/egoist/mygo"

func (app *workspaceWindow) installWorkspaceShortcutMonitor() {}

func focusWorkspaceSurface(win *mygo.Window) {}
