package ui

import (
	"math"
	"strconv"

	mygoui "github.com/egoist/mygo/ui"
)

type sourceLineSpacing struct {
	label string
	value float64
}

// EV-UI-248: Default and an explicit 1.0 have the same geometry but
// remain distinct choices in the native General menu.
var sourceLineSpacingChoices = [...]sourceLineSpacing{
	{"Default", 0}, {"1.0", 1}, {"1.15", 1.15}, {"1.25", 1.25},
	{"1.5", 1.5}, {"1.75", 1.75}, {"2.0", 2}, {"2.5", 2.5},
}

func sourceLineSpacingChoice(value float64) sourceLineSpacing {
	for _, choice := range sourceLineSpacingChoices {
		if choice.value == value {
			return choice
		}
	}
	return sourceLineSpacingChoices[0]
}

func (app *workspaceWindow) sourceLineSpacingSettings(c *mygoui.Context) {
	choice := sourceLineSpacingChoice(app.preferences.EditorLineHeight)
	detail := app.tr("System default", "系统默认")
	if choice.value != 0 {
		detail = strconv.FormatFloat(choice.value, 'f', 2, 64)
	}
	label := func(option sourceLineSpacing) string {
		if option.value == 0 {
			return app.tr(option.label, "默认")
		}
		return option.label
	}
	settingsMenuRowWithDetail(c, app.tr("Line Spacing", "行距"), label(choice), detail,
		"arrow.up.and.down.text.horizontal", func(menu *mygoui.Menu) {
			for _, option := range sourceLineSpacingChoices {
				if menu.Item(label(option)).Checked(option.value == choice.value).Chosen() &&
					option.value != app.preferences.EditorLineHeight {
					app.preferences.EditorLineHeight = option.value
					app.preferencesChanged()
				}
			}
		})
}

// sourceEditorLineHeight converts the General multiplier to MyGo's ratio
// of line height to point size, using the selected source font's own metrics.
func (app *workspaceWindow) sourceEditorLineHeight(font mygoui.Font) float32 {
	multiplier := float32(sourceLineSpacingChoice(app.preferences.EditorLineHeight).value)
	if multiplier == 0 {
		multiplier = 1
	}
	base := float32(1.5)
	if app.blackout {
		base = 1.4
	}
	if font.Family == "" {
		font.Family = "monospace"
	}
	if font.Size > 0 {
		metrics := font.Metrics()
		height := math.Round(float64(metrics.Ascent + metrics.Descent + metrics.LineGap))
		if height > 0 {
			base = float32(height) / font.Size
		}
	}
	return base * multiplier
}
