package ui

import (
	"embed"

	mygoui "github.com/egoist/mygo/ui"
)

//go:embed feature_guide
var featureGuideImages embed.FS

func guideIllustration(c *mygoui.Context, card *featureGuideCard) {
	if card.illustration == "" {
		return
	}
	box := mygoui.Box(c.Key("illustration")).FillWidth().Height(304).Shrink(0).
		Radius(9).Clip().Center()
	source := mygoui.Local(box, card.illustration, func() mygoui.ImageSource {
		bytes, err := featureGuideImages.ReadFile("feature_guide/" + card.illustration + ".png")
		if err != nil {
			return nil
		}
		bitmap, _ := mygoui.DecodeImage(bytes)
		return bitmap
	})
	if *source != nil {
		box.Children(func() {
			mygoui.Image(c, *source).Fill().Fit(mygoui.Contain).Label(card.title + " example")
		})
	} else {
		box.Hidden(true)
	}
}
