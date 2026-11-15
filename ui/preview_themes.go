package ui

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

const systemPreviewThemeID = "system"

type documentPreviewTheme struct {
	ID, DisplayName                                   string
	BackgroundColor, BodyTextColor, HeadingColor      string
	LinkColor, LinkHoverColor                         string
	CodeBackgroundColor, CodeTextColor                string
	BlockquoteBorderColor, BlockquoteTextColor        string
	TableBorderColor, TableHeaderBackground           string
	BodyFontFamily, HeadingFontFamily, CodeFontFamily string
	BaseFontSize, ParagraphSpacing, HeadingSpacing    int
	LineHeight                                        float64
}

// The palette data comes from re/ui/design-tokens.json (EV-UI-063).
//
//go:embed themes/preview.json
var previewThemeData []byte

var previewThemes = func() []documentPreviewTheme {
	var themes []documentPreviewTheme
	if err := json.Unmarshal(previewThemeData, &themes); err != nil {
		panic(err)
	}
	return themes
}()

func findPreviewTheme(id string) (documentPreviewTheme, bool) {
	for _, theme := range previewThemes {
		if theme.ID == id {
			return theme, true
		}
	}
	return documentPreviewTheme{}, false
}

func normalizedPreviewThemeID(id string) string {
	if id == "" || id == systemPreviewThemeID {
		return systemPreviewThemeID
	}
	if _, ok := findPreviewTheme(id); ok {
		return id
	}
	return "synthetic-editorial"
}

func (theme documentPreviewTheme) dark() bool {
	switch theme.ID {
	case "synthetic-editorial", "synthetic-editorial-black", "night", "plato":
		return true
	default:
		return false
	}
}

func (app *workspaceWindow) resolvePreviewTheme(id string) documentPreviewTheme {
	id = normalizedPreviewThemeID(id)
	if id == systemPreviewThemeID {
		switch app.appAppearance() {
		case appearanceBlackout:
			id = "synthetic-editorial-black"
		case appearancePLATO:
			id = "plato"
		case appearanceLight:
			id = "modern"
		case appearanceCyber, appearanceDark:
			id = "synthetic-editorial"
		default:
			id = "modern"
			if app.win != nil && !app.win.IsDestroyed() && mygo.Theme.IsDark() {
				id = "synthetic-editorial"
			}
		}
	}
	theme, _ := findPreviewTheme(id)
	return theme
}

func (app *workspaceWindow) previewThemeSelection() string {
	if app.workspaceTab != nil && app.previewThemeID != "" {
		return app.previewThemeID
	}
	return normalizedPreviewThemeID(app.preferences.PreviewThemeID)
}

func (app *workspaceWindow) previewThemeLabel() string {
	if id := app.previewThemeSelection(); id != systemPreviewThemeID {
		return app.resolvePreviewTheme(id).DisplayName
	}
	return "System"
}

func (app *workspaceWindow) previewThemeButton(c *mygoui.Context) mygoui.Element {
	ink := c.Theme().TextMuted
	if app.blackout {
		ink = blackoutTitleInk
	}
	return chromeMenuControl(c, "Preview Theme", app.previewThemeMenu).Tooltip("Preview Theme · " + app.previewThemeLabel()).Children(func() {
		chromeIconColor(c, "swatchpalette", 20, ink)
	})
}

func (app *workspaceWindow) previewThemeMenu(m *mygoui.Menu) {
	current := app.previewThemeSelection()
	choose := func(id, label string, colorTheme documentPreviewTheme) {
		if m.Item(label).Icon(previewThemeMenuIcons()[colorTheme.ID]).Checked(current == id).Chosen() && !app.modalOpen {
			if app.draft != nil {
				app.previewThemeID = id
			}
			app.preferences.PreviewThemeID = id
			app.preferencesChanged()
		}
	}
	choose(systemPreviewThemeID, "System", app.resolvePreviewTheme(systemPreviewThemeID))
	for _, theme := range previewThemes {
		choose(theme.ID, theme.DisplayName, theme)
	}
	m.Separator()
	if m.Item("Custom Theme...").Icon(customPreviewThemeIcon()).Chosen() {
		app.settingsTab = "Themes"
		if app.settings != nil && !app.settings.IsDestroyed() {
			app.settings.Invalidate()
		}
		app.showSettings()
	}
}

var customPreviewThemeIcon = sync.OnceValue(func() mygoui.MenuIcon {
	if pngData := nativeMenuSymbol("plus.circle"); len(pngData) != 0 {
		if icon, err := mygo.NewMenuIcon(pngData, true); err == nil {
			return icon
		}
	}
	icon, _ := mygo.NewMenuIconSVG(chromeIconFallbacks["plus.circle"], true)
	return icon
})

var previewThemeMenuIcons = sync.OnceValue(func() map[string]mygoui.MenuIcon {
	icons := make(map[string]mygoui.MenuIcon, len(previewThemes))
	for _, theme := range previewThemes {
		fill := mygoui.Hex(theme.BackgroundColor)
		stroke := fill.Mix(mygoui.Hex(theme.BodyTextColor), 0.28)
		dot := image.NewNRGBA(image.Rect(0, 0, 32, 32))
		for y := range 32 {
			for x := range 32 {
				distance := math.Hypot(float64(x)-15.5, float64(y)-15.5)
				if distance >= 12.5 {
					continue
				}
				ink := fill.Mix(stroke, float32(min(1, max(0, distance-10))))
				alpha := uint8(math.Round(255 * min(1, 12.5-distance)))
				dot.SetNRGBA(x, y, color.NRGBA{R: ink.R, G: ink.G, B: ink.B, A: alpha})
			}
		}
		var pngData bytes.Buffer
		if png.Encode(&pngData, dot) == nil {
			if icon, err := mygo.NewMenuIcon(pngData.Bytes(), false); err == nil {
				icons[theme.ID] = icon
			}
		}
	}
	return icons
})

func (app *workspaceWindow) previewThemeSettings(c *mygoui.Context) {
	current := normalizedPreviewThemeID(app.preferences.PreviewThemeID)
	mygoui.Column(c).FillWidth().Gap(20).Children(func() {
		app.settingsSection(c, "App Appearance", "paintpalette", func() {
			app.settingsCard(c).Gap(7).Children(func() {
				app.appearanceChoices(c)
				mygoui.Text(c, "Cyber is the original Synthetic Editorial dark theme. Blackout is pure black. "+
					"PLATO is the phosphor-orange terminal homage. System Light and System Dark match the platform appearance. "+
					"Document themes (Preview / PDF) are independent of this setting.").
					FontSize(11).TextColor(c.Theme().TextMuted)
			})
		})
		app.settingsSection(c, "Preview Theme", "doc.richtext", func() {
			app.settingsCard(c).Gap(10).Children(func() {
				app.previewThemeGrid(c, current)
			})
		})
		app.settingsSection(c, "Live Preview", "eye", func() {
			app.settingsCard(c).Children(func() {
				app.previewThemeSample(c, app.resolvePreviewTheme(current))
			})
		})
		app.settingsSection(c, "Sidebar Appearance", "sidebar.left", func() {
			app.settingsCard(c).Padding(8, 16).Children(func() {
				app.settingsSwitch(c, "Color Tags by Category", "Give each tag a hue based on its name",
					"paintpalette.fill", 48, &app.preferences.TagsColoredByHash)
				app.settingsSwitch(c, "Color Graph by Tag", "Tint knowledge-graph nodes by their primary tag",
					"point.3.connected.trianglepath.dotted", 48, &app.preferences.GraphColorNodesByTag)
			})
		})
		app.customStylesheetSettings(c)
	})
}

func (app *workspaceWindow) previewThemeGrid(c *mygoui.Context, current string) {
	columns := 3
	width, _ := c.Size()
	if width < 700 {
		columns = 2
	}
	for first := 0; first <= len(previewThemes); first += columns {
		mygoui.Row(c.Key(fmt.Sprint(first))).FillWidth().Gap(6).Children(func() {
			for index := first; index < first+columns; index++ {
				if index > len(previewThemes) {
					mygoui.Spacer(c).Basis(0).Grow(1)
					continue
				}
				id, label, theme := systemPreviewThemeID, "System", app.resolvePreviewTheme(systemPreviewThemeID)
				if index > 0 {
					theme = previewThemes[index-1]
					id, label = theme.ID, theme.DisplayName
				}
				app.previewThemeCard(c, id, label, theme, current == id)
			}
		})
	}
}

func (app *workspaceWindow) previewThemeSample(c *mygoui.Context, theme documentPreviewTheme) {
	mygoui.Column(c).FillWidth().MinHeight(136).Padding(16).Gap(7).Radius(8).
		Border(1, mygoui.Hex(theme.TableBorderColor)).Background(mygoui.Hex(theme.BackgroundColor)).
		Children(func() {
			mygoui.Text(c, "Heading").FontSize(13).Bold().TextColor(mygoui.Hex(theme.HeadingColor))
			markdown.View(c, "Body text with **bold** and *italic* formatting.", &markdown.Options{Theme: theme.markdownTheme(100, 0)})
			mygoui.Text(c, "Link text").FontSize(13).TextColor(mygoui.Hex(theme.LinkColor))
			mygoui.Row(c).Children(func() {
				mygoui.Text(c, "Code block").Font("monospace").FontSize(12).Padding(7, 8).Radius(5).
					TextColor(mygoui.Hex(theme.CodeTextColor)).Background(mygoui.Hex(theme.CodeBackgroundColor))
			})
		})
}

func (app *workspaceWindow) previewThemeCard(c *mygoui.Context, id, label string, theme documentPreviewTheme, selected bool) {
	kind := "Light"
	if theme.dark() {
		kind = "Dark"
	}
	colors := c.Theme()
	accent := colors.Accent
	if app.blackout && !c.Preferences().HighContrast {
		accent = blackoutSettingsAccent
	}
	card := mygoui.ButtonBase(c.Key(id)).Basis(0).Grow(1).MinWidth(0).Height(54).
		Padding(0, 10).Gap(10).AlignItems(mygoui.Center).Justify(mygoui.Start).Radius(8).
		Label(label).Tooltip(kind + " preview theme").AccessSelected(selected).OnClick(func() {
		app.preferences.PreviewThemeID = id
		app.preferencesChanged()
	})
	if selected {
		card.Background(accent.Alpha(0.14)).Border(2, accent)
	} else if card.Hovered() {
		card.Background(colors.SurfaceHover)
	}
	card.Children(func() {
		mygoui.Box(c).Size(32, 32).Shrink(0).Center().Radius(6).
			Border(1, mygoui.Hex(theme.TableBorderColor)).Background(mygoui.Hex(theme.BackgroundColor)).
			Role(mygoui.RoleNone).Children(func() {
			mygoui.Text(c, "Aa").FontSize(12).Bold().TextColor(mygoui.Hex(theme.BodyTextColor)).Role(mygoui.RoleNone)
		})
		mygoui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			mygoui.Text(c, label).FontSize(12).FontWeight(600).SingleLine().Ellipsis("…").Role(mygoui.RoleNone)
			mygoui.Text(c, kind).FontSize(10).TextColor(colors.TextMuted).Role(mygoui.RoleNone)
		})
		if selected {
			chromeIconColor(c, "checkmark.circle.fill", 13, accent).Shrink(0)
		}
	})
}

func (theme documentPreviewTheme) markdownTheme(zoom int, spacing float64) *markdown.Theme {
	if spacing <= 0 {
		spacing = theme.LineHeight
	}
	codeBackground := mygoui.Hex(theme.CodeBackgroundColor)
	style := &markdown.Theme{
		FontSize:   float32(theme.BaseFontSize) * 4 / 3 * float32(zoom) / 100,
		LineHeight: float32(spacing), Text: mygoui.Hex(theme.BodyTextColor), Link: mygoui.Hex(theme.LinkColor),
		Code: mygoui.Hex(theme.CodeTextColor), CodeBackground: codeBackground, CodeBlockBackground: codeBackground,
		QuoteBar: mygoui.Hex(theme.BlockquoteBorderColor), QuoteText: mygoui.Hex(theme.BlockquoteTextColor),
		TableBorder: mygoui.Hex(theme.TableBorderColor), TableHeader: mygoui.Hex(theme.TableHeaderBackground),
	}
	if font, _, _ := strings.Cut(theme.BodyFontFamily, ","); !strings.HasPrefix(font, "-") {
		style.Font = strings.Trim(font, `"' `)
	}
	for i := range style.Headings {
		style.Headings[i].Color = mygoui.Hex(theme.HeadingColor)
	}
	return style
}
