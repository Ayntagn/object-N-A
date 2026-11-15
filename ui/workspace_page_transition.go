package ui

import (
	"time"

	mygoui "github.com/egoist/mygo/ui"
)

const workspacePageTransitionDuration = 160 * time.Millisecond

type workspacePageTransition[T comparable] struct {
	page    T
	started time.Time
}

func revealWorkspacePage[T comparable](c *mygoui.Context, host, pageView mygoui.Element, page T, surface mygoui.Color) {
	transition := mygoui.Local(host, "page transition", func() workspacePageTransition[T] {
		return workspacePageTransition[T]{page: page}
	})
	now := c.Now()
	if transition.page != page {
		transition.page = page
		transition.started = now
	}
	if prefs := c.Preferences(); prefs.ReduceMotion || prefs.HighContrast {
		transition.started = time.Time{}
		return
	}
	started := transition.started
	if started.IsZero() || now.Sub(started) >= workspacePageTransitionDuration {
		return
	}
	// Paint one fill over the live page so the view, scrolling and hit targets
	// are ready while the reveal runs.
	pageView.DrawOver(func(p *mygoui.Painter, bounds mygoui.Rect) {
		elapsed := max(0, p.Now().Sub(started))
		if elapsed >= workspacePageTransitionDuration {
			return
		}
		progress := float32(elapsed) / float32(workspacePageTransitionDuration)
		p.Fill(bounds, surface.Alpha(.18*(1-mygoui.EaseOut(progress))), 0)
		p.AnimationFrame()
	})
}
