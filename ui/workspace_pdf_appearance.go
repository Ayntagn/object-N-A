package ui

import (
	"fmt"
	"slices"
	"sync"

	"cyberwriter/internal/core/exportdoc"
	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-133/134: these overrides are part of remembered PDF settings.
type pdfAppearance struct {
	Background     string `json:"background,omitempty"`
	CodeBackground string `json:"codeBackground,omitempty"`
	Text           string `json:"text,omitempty"`
}

type pdfColorPreset struct{ label, hex string }

const pdfThemeDefault = "Theme Default"

var pdfPaperPresets = []pdfColorPreset{
	{pdfThemeDefault, ""}, {"White", "#ffffff"}, {"Light Gray", "#f5f5f5"},
	{"Cream", "#fffef0"}, {"Sepia", "#f4ecd8"}, {"Dark Gray", "#2d2d2d"}, {"Black", "#000000"},
}
var pdfCodePresets = []pdfColorPreset{
	{pdfThemeDefault, ""}, {"White", "#ffffff"}, {"Light Gray", "#f5f5f5"},
	{"Medium Gray", "#e8e8e8"}, {"Cream", "#fffef0"}, {"Dark Gray", "#2d2d2d"}, {"Black", "#000000"},
}
var pdfTextPresets = []pdfColorPreset{
	{pdfThemeDefault, ""}, {"Black", "#000000"}, {"Dark Gray", "#333333"}, {"Gray", "#666666"},
	{"Brown", "#5b4636"}, {"Light Gray", "#cccccc"}, {"White", "#ffffff"},
}

var pdfFontChoices = []string{
	pdfThemeDefault, "System Default", "Space Grotesk", "Serif", "Sans Serif", "Georgia", "Palatino",
	"Times New Roman", "Helvetica", "Arial", "Verdana", "JetBrains Mono", "Fira Code", "Cascadia Code",
}

func pdfPresetIndex(presets []pdfColorPreset, value string) int {
	return max(0, slices.IndexFunc(presets, func(preset pdfColorPreset) bool { return preset.hex == value }))
}

func (appearance pdfAppearance) applyTo(theme documentPreviewTheme) documentPreviewTheme {
	if paper := pdfPaperPresets[pdfPresetIndex(pdfPaperPresets, appearance.Background)].hex; paper != "" {
		theme.BackgroundColor = paper
		lightPaper := lightPDFColor(paper)
		if lightPaper == lightPDFColor(theme.HeadingColor) {
			theme.HeadingColor = "#ffffff"
			if lightPaper {
				theme.HeadingColor = "#1a1a1a"
			}
		}
	}
	if code := pdfCodePresets[pdfPresetIndex(pdfCodePresets, appearance.CodeBackground)].hex; code != "" {
		theme.CodeBackgroundColor = code
	}
	if text := pdfTextPresets[pdfPresetIndex(pdfTextPresets, appearance.Text)].hex; text != "" {
		theme.BodyTextColor, theme.HeadingColor = text, text
	}
	return theme
}

func lightPDFColor(hex string) bool {
	c := mygoui.Hex(hex)
	return 0.2126*float64(c.R)+0.7152*float64(c.G)+0.0722*float64(c.B) >= 127.5
}

func pdfFontLabel(font exportdoc.FontFamily) string {
	if !slices.Contains(pdfFontChoices[1:], string(font)) {
		return pdfThemeDefault
	}
	return string(font)
}

func pdfFontChoice(label string) exportdoc.FontFamily {
	if label == pdfThemeDefault {
		return ""
	}
	return exportdoc.FontFamily(label)
}

var pdfOverrideFonts = map[exportdoc.FontFamily]func() string{
	"Space Grotesk": spaceGroteskPreviewFont,
	"JetBrains Mono": sync.OnceValue(func() string {
		return previewFontFace("JetBrainsMono-Regular", "JetBrains Mono", "normal", "400")
	}),
	"Fira Code": sync.OnceValue(func() string {
		return previewFontFace("FiraCode-Regular", "Fira Code", "normal", "400")
	}),
	"Cascadia Code": sync.OnceValue(func() string {
		return previewFontFace("CascadiaCode-Regular", "Cascadia Code", "normal", "400")
	}),
}

func (s *workspaceExportSheet) appearanceOptions(c *mygoui.Context) {
	appearance := &s.form.appearance
	s.colorOption(c.Key("background"), "Background", &appearance.Background, pdfPaperPresets, "#ffffff")
	s.colorOption(c.Key("code-color"), "Code Blocks", &appearance.CodeBackground, pdfCodePresets, "#f5f5f5")
	s.colorOption(c.Key("text-color"), "Text Color", &appearance.Text, pdfTextPresets, "#000000")
	mygoui.Row(c.Key("font-scale")).FillWidth().Gap(8).AlignItems(mygoui.Center).Children(func() {
		mygoui.Text(c, "Font Size").Width(56.5)
		mygoui.StepSlider(c, &s.form.layout.FontScale, pdfFontMin, pdfFontMax, pdfFontStep).Label("Font Size").Size(140, 20).
			MinWidth(65).Shrink(1).Disabled(s.busy)
		mygoui.Text(c, fmt.Sprintf("%.0f%%", s.form.layout.FontScale)).Width(47).MarginStart(12)
	})
	font := pdfFontLabel(s.form.layout.FontFamily)
	mygoui.Row(c.Key("pdf-font")).AlignItems(mygoui.Center).Height(31).Gap(8).Children(func() {
		mygoui.Text(c, "Font").SingleLine()
		mygoui.Select(c, &font, pdfFontChoices).Size(160, 24).Label("Font").
			Background(c.Theme().SurfaceHover).Disabled(s.busy).OnChange(func() {
			s.form.layout.FontFamily = pdfFontChoice(font)
		})
	})
}

func (s *workspaceExportSheet) colorOption(c *mygoui.Context, label string, value *string, presets []pdfColorPreset, defaultSwatch string) {
	selected := presets[pdfPresetIndex(presets, *value)]
	swatch := selected.hex
	if swatch == "" {
		swatch = defaultSwatch
	}
	choices := make([]string, len(presets))
	for index, preset := range presets {
		choices[index] = preset.label
	}
	mygoui.Row(c).AlignItems(mygoui.Center).Height(31).Gap(9).Children(func() {
		mygoui.Text(c, label).SingleLine().Width(76)
		mygoui.Box(c).Size(24, 24).Radius(4).Background(mygoui.Hex(swatch)).
			Border(1, c.Theme().TextMuted.Alpha(0.5)).Role(mygoui.RoleNone)
		mygoui.Select(c, &selected.label, choices).Size(136.5, 24).MinWidth(136.5).Label(label).
			Background(c.Theme().SurfaceHover).Disabled(s.busy).OnChange(func() {
			*value = presets[max(0, slices.Index(choices, selected.label))].hex
		})
	})
}
