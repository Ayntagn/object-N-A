package ui

import (
	"math"

	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-187. VaultTagPalette, quantized to MyGo's RGB channel precision.
var graphTagPalette = [...]graphRGB{
	{169, 199, 255}, {255, 170, 247}, {153, 230, 153}, {255, 204, 115}, {140, 217, 230},
	{230, 166, 128}, {191, 179, 255}, {230, 191, 242}, {128, 230, 191}, {255, 179, 179},
	{217, 217, 128}, {128, 191, 255}, {255, 140, 191}, {115, 230, 140}, {242, 179, 217},
	{153, 179, 230}, {204, 153, 115}, {179, 230, 217}, {230, 128, 140}, {166, 217, 166},
	{217, 166, 255}, {140, 204, 191}, {255, 166, 115}, {191, 217, 140}, {204, 153, 204},
	{128, 217, 217}, {242, 191, 153}, {153, 166, 217}, {217, 140, 179}, {179, 204, 128},
}

type graphRGB struct{ r, g, b uint8 }

func (rgb graphRGB) color() mygoui.Color { return mygoui.RGB(rgb.r, rgb.g, rgb.b) }

func graphTagSlot(tag string) int {
	var hash int64
	for i := range len(tag) {
		hash = 31*hash + int64(tag[i])
	}
	remainder := int(hash % int64(len(graphTagPalette)))
	return max(remainder, -remainder)
}

// The graph's flat renderer blends in sRGB. Preblend the translucent
// node ink over the canvas in linear light to match SceneKit.
func graphMixNodeColor(ink, background mygoui.Color, opacity float64) mygoui.Color {
	mix := func(fg, bg uint8) uint8 {
		linear := graphLinearChannel[fg]*opacity + graphLinearChannel[bg]*(1-opacity)
		if linear <= .0031308 {
			return uint8(math.Round(255 * 12.92 * linear))
		}
		return uint8(math.Round(255 * (1.055*math.Pow(linear, 1/2.4) - .055)))
	}
	return mygoui.RGB(mix(ink.R, background.R), mix(ink.G, background.G), mix(ink.B, background.B))
}

type graphFlatColors struct {
	base, background mygoui.Color
	opacity          float64
	bright, dim      [len(graphTagPalette) + 1]mygoui.Color
}

func (colors *graphFlatColors) update(base, background mygoui.Color, opacity float64, blackout bool) {
	if colors.base == base && colors.background == background && colors.opacity == opacity {
		return
	}
	colors.base, colors.background, colors.opacity = base, background, opacity
	colors.bright[0], colors.dim[0] = base, graphMixNodeColor(base, background, opacity)
	for slot, rgb := range graphTagPalette {
		bright := rgb.color()
		if blackout {
			// Approximate the lightness of sampled Blackout node materials.
			bright = bright.Mix(mygoui.RGB(255, 255, 255), .12)
		}
		colors.bright[slot+1] = bright
		colors.dim[slot+1] = graphMixNodeColor(bright, background, opacity)
	}
}

var graphLinearChannel = func() [256]float64 {
	var values [256]float64
	for channel := range values {
		v := float64(channel) / 255
		if v <= .04045 {
			values[channel] = v / 12.92
		} else {
			values[channel] = math.Pow((v+.055)/1.055, 2.4)
		}
	}
	return values
}()
