package ui

import (
	"math"
	"time"

	mygoui "github.com/egoist/mygo/ui"
)

const (
	splashWordmarkFont  = "Space Grotesk"
	splashWordmarkStart = 3.77
	splashEraseStart    = 6.02
	splashExitStart     = 8.15
	splashExitEnd       = 8.70
)

type splashSceneColors struct {
	shade, accent, ink mygoui.Color
	particles          [4]mygoui.Color
}

// EV-UI-335. The view is a function of playback time. The overlay window owns
// its own creation, event routing, and completion.
func drawWorkspaceSplash(c *mygoui.Context, elapsed time.Duration) {
	width, height := c.Size()
	seconds := max(0, elapsed.Seconds())
	reduced := c.Preferences().ReduceMotion
	colors := workspaceSplashSceneColors(c.Theme())
	fontSize := min(128, max(44, min(.09*width, .20*height)))
	centerY := height/2 - 18
	loadBundledFont(splashWordmarkFont)
	alpha := float32(1 - splashSceneExitProgress(elapsed))
	mygoui.Box(c).Fill().Clip().Opacity(alpha).Draw(func(p *mygoui.Painter, bounds mygoui.Rect) {
		shade := float32(splashPhase(seconds, .08, .62))
		if reduced {
			shade = 1
		}
		// Compose the opaque background before applying the overlay's fade.
		p.Fill(bounds, mygoui.RGB(0, 0, 0).Mix(colors.shade, shade), 0)
		if !reduced && seconds < 4.06 {
			paintSplashParticles(p, bounds, seconds, colors.particles)
		}
	}).Children(func() {
		if reduced || seconds >= splashWordmarkStart {
			drawSplashWordmark(c, width, centerY, fontSize, seconds, reduced, colors)
		}
	})
}

func workspaceSplashSceneColors(theme *mygoui.Theme) splashSceneColors {
	if theme.Accent == appearancePalettes[appearancePLATO].accent {
		// EV-UI-335. The PLATO palette is also visible in the recorded rose.
		return splashSceneColors{
			shade: mygoui.RGB(23, 16, 8), accent: mygoui.RGB(255, 196, 107), ink: mygoui.RGB(255, 236, 206),
			particles: [4]mygoui.Color{
				mygoui.RGB(255, 196, 107), mygoui.RGB(255, 140, 31), mygoui.RGB(232, 179, 107), mygoui.RGB(255, 217, 161),
			},
		}
	}
	ink, accent := mygoui.RGB(233, 235, 243), mygoui.RGB(253, 177, 244)
	return splashSceneColors{
		shade: mygoui.RGB(11, 15, 24), accent: accent, ink: ink,
		particles: [4]mygoui.Color{accent, theme.Accent, theme.Accent.Mix(ink, .4), ink},
	}
}

func paintSplashParticles(p *mygoui.Painter, bounds mygoui.Rect, seconds float64, palette [4]mygoui.Color) {
	const petals, rings, points = 4, 10, 34
	visibility := splashPhase(seconds, .16, .66) * (1 - splashPhase(seconds, 3.37, 4.06))
	if visibility == 0 {
		return
	}
	radius := float64(min(260, max(110, .30*min(bounds.W, bounds.H))))
	centerX, centerY := bounds.X+bounds.W/2, bounds.Y+bounds.H/2-18
	rotation := .31 + (seconds-.9)*.95
	converge := splashPhase(seconds, 3.37, 4.06)
	for petal := range petals {
		axis := rotation + math.Pi/2*float64(petal)
		depth := .67 + .33*math.Abs(math.Cos(axis))
		for ring := range rings {
			fraction := float64(ring) / (rings - 1)
			arc := radius * (.27 + .73*fraction)
			twist := .22 * (1 - fraction)
			for step := range points {
				u := (float64(step) + .45) / points
				theta := axis + twist + (u-.5)*math.Pi/2
				distance := arc * math.Sin(math.Pi*u)
				x := centerX + float32(distance*math.Cos(theta)*(1+.25*converge))
				y := centerY + float32(distance*math.Sin(theta)*(1-.84*converge))
				size := float32((2.1 + 3*fraction) * depth * min(1, radius/220))
				color := palette[1].Mix(palette[(petal+step/9)%len(palette)], float32(.45+.48*fraction))
				p.Fill(mygoui.Rect{X: x - size/2, Y: y - size/2, W: size, H: size}, color.Alpha(float32(visibility*depth)), size/2)
			}
		}
	}
}

func drawSplashWordmark(c *mygoui.Context, width, centerY, size float32, seconds float64, reduced bool, colors splashSceneColors) {
	const first = "object"
	accent := mygoui.Span{Text: first, Font: splashWordmarkFont, Size: size, Weight: 700, Color: colors.accent}
	ink := accent
	ink.Text, ink.Color = applicationName[len(first):], colors.ink
	wordmarkWidth, wordmarkHeight := c.MeasureText(0, accent, ink)
	left, top := (width-wordmarkWidth)/2, centerY-wordmarkHeight/2
	visibleWidth := wordmarkWidth
	if !reduced {
		visibleWidth *= float32(splashPhase(seconds, splashWordmarkStart, 4.41))
		if seconds >= splashEraseStart {
			remaining := max(0, len(applicationName)-int((seconds-splashEraseStart)/.18))
			prefix, suffix := accent, ink
			prefix.Text = first[:min(remaining, len(first))]
			suffix.Text = ink.Text[:max(0, remaining-len(first))]
			visibleWidth, _ = c.MeasureText(0, prefix, suffix)
		}
	}
	if visibleWidth > 0 {
		mygoui.Box(c.Key("splash wordmark")).Absolute().Top(top).Left(left).Size(visibleWidth, wordmarkHeight).Clip().Children(func() {
			mygoui.RichText(c, accent, ink).Width(wordmarkWidth).Shrink(0).Label(applicationName).
				Role(mygoui.RoleHeading).Level(1)
		})
	}
	if !reduced && seconds < splashExitStart && (seconds < 4.56 || seconds >= splashEraseStart) {
		mygoui.Box(c.Key("splash caret")).Absolute().Left(left+visibleWidth+5).Top(centerY-size*.27).
			Size(max(2, size*.028), size*.56).Radius(2).Background(colors.accent).Role(mygoui.RoleNone)
	}
	taglineOpacity := 1.0
	if !reduced {
		taglineOpacity = splashPhase(seconds, 4.25, 5.0) * (1 - splashPhase(seconds, 6.04, 6.50))
	}
	if taglineOpacity > 0 {
		mygoui.Text(c.Key("splash tagline"), "Markdown workspace").Absolute().Left(0).Right(0).Top(top + wordmarkHeight + 6).
			TextAlign(mygoui.Center).Font("monospace").FontSize(13).TextColor(colors.ink).Opacity(float32(taglineOpacity))
	}
}

func splashSceneExitProgress(elapsed time.Duration) float64 {
	return splashPhase(elapsed.Seconds(), splashExitStart, splashExitEnd)
}

func splashPhase(seconds, start, end float64) float64 {
	progress := min(1, max(0, (seconds-start)/(end-start)))
	return progress * progress * (3 - 2*progress)
}
