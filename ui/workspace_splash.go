package ui

import (
	"log"
	"time"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

const (
	splashFadeIn      = 450 * time.Millisecond
	splashDuration    = 8700 * time.Millisecond
	splashSkipFadeOut = 225 * time.Millisecond
)

type workspaceSplash struct {
	win, host   *mygo.Window
	cold        bool
	hostOpacity float64
	started     time.Time
	skipStarted time.Time
	closing     bool
}

func (a *workspaceApplication) replaySplashMenuItem(language workspaceLanguage) *mygo.MenuItem {
	return &mygo.MenuItem{
		Label:    language.text("Replay Splash", "重播启动动画"),
		Disabled: a == nil,
		Click: func(*mygo.MenuItem, *mygo.Window) {
			if a != nil && a.splash == nil {
				a.playSplash(a.splashHost(), false, nil)
			}
		},
	}
}

// EV-UI-264: the original considers titled windows in native window order,
// including windows behind the frontmost document.
func (a *workspaceApplication) splashHost() *mygo.Window {
	for _, window := range splashHostWindows() {
		if !window.IsDestroyed() && validSplashFrame(window.Bounds()) {
			return window
		}
	}
	return nil
}

func validSplashFrame(frame mygo.Rectangle) bool {
	return frame.Width >= 400 && frame.Height >= 300
}

func (a *workspaceApplication) splashFrame(host *mygo.Window) mygo.Rectangle {
	if host != nil && !host.IsDestroyed() {
		frame := host.Bounds()
		if validSplashFrame(frame) {
			if frame != a.preferences.SplashLastHostFrame {
				a.preferences.SplashLastHostFrame = frame
				if err := a.preferences.write(); err != nil {
					log.Printf("Save splash window position: %v", err)
				}
			}
			return frame
		}
	}
	if frame := a.preferences.SplashLastHostFrame; validSplashFrame(frame) {
		display := mygo.Screen.DisplayMatching(frame).Bounds
		if frame.X < display.X+display.Width && frame.X+frame.Width > display.X &&
			frame.Y < display.Y+display.Height && frame.Y+frame.Height > display.Y {
			return frame
		}
	}
	area := mygo.Screen.PrimaryDisplay().WorkArea
	if area.Width <= 0 || area.Height <= 0 {
		return mygo.Rectangle{Width: 1100, Height: 760}
	}
	width, height := min(1100, area.Width), min(760, area.Height)
	return mygo.Rectangle{X: area.X + (area.Width-width)/2, Y: area.Y + (area.Height-height)/2, Width: width, Height: height}
}

func (a *workspaceApplication) playSplash(host *mygo.Window, cold bool, done func()) {
	if a.splash != nil {
		return
	}
	splash := &workspaceSplash{host: host, cold: cold}
	a.splash = splash
	if host != nil && cold {
		splash.hostOpacity = host.Opacity()
		host.SetOpacity(0)
		host.ShowInactive()
	}
	bounds := a.splashFrame(host)
	title := a.preferences.InterfaceLanguage.text("Launch animation", "启动动画")
	win := mygo.NewWindow(mygo.WindowOptions{
		Width: bounds.Width, Height: bounds.Height, X: bounds.X, Y: bounds.Y,
		Hidden: true, Frameless: true, Transparent: true, AlwaysOnTop: true,
		DisableResize: true, DisableMove: true, DisableMinimize: true,
		DisableMaximize: true, DisableFullScreen: true, DisableShadow: true, SkipTaskbar: true,
		Content: mygoui.View(func(c *mygoui.Context) {
			if palette, ok := appearancePalettes[a.preferences.Appearance]; ok {
				theme := *c.Theme()
				theme.Accent, theme.Text = palette.accent, palette.text
				c.SetTheme(&theme)
			}
			splash.view(c, title)
		}),
	})
	splash.win = win
	win.SetBounds(bounds)
	win.SetTitle("")
	win.SetOpacity(0)
	watchdog := time.AfterFunc(20*time.Second, func() { win.Update(splash.finish) })
	win.OnClosed(func() {
		watchdog.Stop()
		a.splash = nil
		if target := splash.host; target != nil && !target.IsDestroyed() {
			if cold {
				target.SetOpacity(splash.hostOpacity)
			}
			if splash.closing {
				if target.IsMinimized() {
					target.Restore()
				}
				target.Show()
			}
		}
		if splash.closing && done != nil {
			done()
		}
	})
	installSplashKeyMonitor(win, splash.skip)
	splash.started = time.Now()
	win.Show()
}

func (s *workspaceSplash) skip() {
	if s.skipStarted.IsZero() && !s.closing {
		s.skipStarted = time.Now()
		s.win.Invalidate()
	}
}

func (s *workspaceSplash) finish() {
	if s.win != nil && !s.win.IsDestroyed() && !s.closing {
		s.closing = true
		s.win.Update(s.win.Close)
	}
}

func (s *workspaceSplash) view(c *mygoui.Context, title string) {
	if s.started.IsZero() {
		drawWorkspaceSplash(c, 0)
		return
	}
	elapsed := time.Since(s.started)
	opacity := min(1, float64(elapsed)/float64(splashFadeIn))
	if len(c.KeysDown()) != 0 {
		s.skip()
	}
	exit := splashSceneExitProgress(elapsed)
	var skipProgress float64
	if !s.skipStarted.IsZero() {
		skipProgress = min(1, float64(time.Since(s.skipStarted))/float64(splashSkipFadeOut))
		exit = max(exit, skipProgress)
		if skipProgress == 1 {
			s.finish()
		}
	}
	s.win.SetOpacity(opacity)
	if s.cold && s.host != nil && !s.host.IsDestroyed() && exit > 0 {
		s.host.SetOpacity(s.hostOpacity * exit)
	}
	if elapsed >= splashDuration {
		s.finish()
	}
	mygoui.Box(c).Fill().Opacity(float32(1 - skipProgress)).Label(title).Role(mygoui.RoleGroup).OnClick(s.skip).Children(func() {
		drawWorkspaceSplash(c, elapsed)
	})
	if !s.closing {
		c.After(16 * time.Millisecond)
	}
}
