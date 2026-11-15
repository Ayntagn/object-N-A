package ui

import (
	"runtime"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

const (
	blackoutTextCSS   = "#ecebef"
	blackoutAccentCSS = "#a9c7ff"
)

// EV-UI-036, sampled in the archived dark window after ICC conversion.
var (
	blackoutSidebarFill = mygoui.RGB(6, 6, 8)
	blackoutSidebarEdge = mygoui.RGB(26, 27, 30)
	blackoutSearchFill  = mygoui.RGB(21, 21, 24)
	blackoutPaneEdge    = mygoui.RGB(22, 23, 24)
	blackoutTitleRule   = mygoui.RGB(34, 36, 47)
	blackoutTitleInk    = mygoui.RGB(158, 168, 196)
	blackoutTitleAccent = mygoui.Hex(blackoutAccentCSS)
	blackoutSourceHead  = mygoui.RGB(255, 170, 247)
)

// EV-UI-038: colors fitted to the visible toolbar in the archived dark window.
var (
	blackoutPaneToolInk        = mygoui.RGB(145, 147, 175)
	blackoutPreviewLabelInk    = mygoui.RGB(228, 228, 228)
	blackoutPreviewIconInk     = mygoui.RGB(202, 202, 202)
	blackoutPreviewZoomSurface = mygoui.RGB(11, 11, 11)
	blackoutPreviewZoomInk     = mygoui.RGB(150, 150, 150)
	blackoutPreviewPDFSurface  = mygoui.RGB(32, 32, 32)
	blackoutPreviewPDFEdge     = mygoui.RGB(56, 56, 56)
	blackoutPreviewPDFInk      = mygoui.RGB(102, 102, 102)
)

func applyWorkspaceTheme(c *mygoui.Context) {
	theme := *c.Theme()
	black, white := mygoui.RGB(0, 0, 0), mygoui.RGB(255, 255, 255)
	// Round the sRGB AccentColor renditions to MyGo's 8-bit channels (EV-UI-003).
	if theme.Dark {
		theme.Accent = mygoui.RGB(64, 149, 250)
		theme.AccentHover = theme.Accent.Mix(white, 0.2)
		theme.AccentPressed = theme.Accent.Mix(black, 0.15)
		theme.AccentText = black
		theme.Selection = theme.Accent.Alpha(0.4)
	} else {
		theme.Accent = mygoui.RGB(0, 112, 221)
		theme.AccentHover = theme.Accent.Mix(black, 0.12)
		theme.AccentPressed = theme.Accent.Mix(black, 0.25)
		theme.AccentText = white
		theme.Selection = theme.Accent.Alpha(0.25)
	}
	theme.Focus = theme.Accent
	c.SetTheme(&theme)
	if workspaceChromeSurface(c) == mygoui.Transparent {
		c.Root().Background(mygoui.Transparent)
	}
}

func workspaceVibrancy() mygo.Vibrancy {
	// Sidebar material is provisional while the original AppKit material is unknown.
	if runtime.GOOS == "darwin" {
		return mygo.VibrancySidebar
	}
	return mygo.VibrancyNone
}

func workspaceChromeSurface(c *mygoui.Context) mygoui.Color {
	prefs := c.Preferences()
	if c.Vibrancy() && !prefs.ReduceTransparency && !prefs.HighContrast {
		return mygoui.Transparent
	}
	return c.Theme().Surface
}
