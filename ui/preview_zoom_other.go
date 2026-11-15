//go:build !darwin

package ui

import "github.com/egoist/mygo"

func observeNewPreviewMagnifier(*mygo.Window) func() func(float64) { return nil }
