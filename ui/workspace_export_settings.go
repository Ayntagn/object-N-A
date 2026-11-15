package ui

import (
	"strings"

	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-022 / EV-UI-247. The typography, paper settings, Word note and location form four sections.
func (app *workspaceWindow) exportSettings(c *mygoui.Context) {
	prefs := &app.preferences.Export
	tr := app.tr
	spacing := exportSpacingOptions[prefs.spacingIndex()]
	spacingLabel, spacingDetail := spacing, spacing
	if spacing == exportSpacingOptions[0] {
		spacingLabel = tr(spacing, "主题默认")
		spacingDetail = tr("Use each theme's default", "使用各主题的默认行距")
	}
	paper, _ := findExportPaper(prefs.Paper)
	paperDescription := app.exportPaperDescription(paper)
	numberPages := !prefs.OmitPageNumbers
	theme := *c.Theme()
	if app.blackout && !c.Preferences().HighContrast {
		theme.TextMuted = mygoui.Hex("#c1c2cc")
		theme.Border = mygoui.Hex("#353539")
	}
	// The shared card adds one point of border on either side of its content.
	mygoui.Themed(c, &theme).FillWidth().Gap(20).Children(func() {
		app.settingsSection(c, tr("Preview & Export Typography", "预览和导出的排版"), "text.alignleft", func() {
			app.settingsCard(c).Padding(0, 15).Children(func() {
				app.exportSettingsMenu(c, tr("Line Spacing", "行距"), spacingLabel, spacingDetail, "arrow.up.and.down.text.horizontal", 78, func(menu *mygoui.Menu) {
					for _, choice := range exportSpacingOptions {
						label := choice
						if choice == exportSpacingOptions[0] {
							label = tr(choice, "主题默认")
						}
						if menu.Item(label).Radio(spacing == choice).Chosen() {
							prefs.LineSpacing = choice
							app.preferencesChanged()
						}
					}
				})
			})
		})
		app.settingsSection(c, tr("PDF Export", "PDF 导出"), "doc.fill", func() {
			app.settingsCard(c).Padding(0, 15).Children(func() {
				app.exportSettingsMenu(c, tr("Paper Size", "纸张大小"), paperDescription, paperDescription, "doc.plaintext", 79, func(menu *mygoui.Menu) {
					for _, group := range exportPapers {
						for _, choice := range group.papers {
							if menu.Item(app.exportPaperLabel(choice)).Radio(paper.label == choice.label).Chosen() {
								prefs.Paper = choice.label
								app.writePreferences()
							}
						}
					}
				})
				mygoui.Divider(c).Height(1)
				app.exportSettingsOrientation(c)
				mygoui.Divider(c).Height(1)
				app.settingsSwitchOnChange(c, tr("Page Numbers", "页码"), tr("Include page numbers in footer", "在页脚显示页码"), "number", 79, &numberPages, func() {
					prefs.OmitPageNumbers = !numberPages
					app.preferencesChanged()
				})
			})
		})
		mygoui.Column(c).FillWidth().PaddingX(23.5).Children(func() {
			app.settingsSection(c, tr("Word Export", "Word 导出"), "doc.text.fill", func() {
				app.settingsCard(c).Padding(15).Children(func() {
					mygoui.Row(c).FillWidth().MinHeight(30).Gap(15).AlignItems(mygoui.Center).Children(func() {
						infoInk := c.Theme().Accent
						if app.blackout && !c.Preferences().HighContrast {
							infoInk = mygoui.Hex("#009dff")
						}
						mygoui.Box(c).Size(16, 16).Center().Children(func() {
							chromeIconColor(c, "info.circle", 21, infoInk)
						})
						mygoui.Text(c, tr("Word documents (.docx) are exported with standard formatting optimized for Microsoft Word and compatible applications.",
							"Word 文档（.docx）使用适合 Microsoft Word 及兼容应用的标准格式导出。")).
							FontSize(12).Grow(1).MinWidth(0).TextColor(c.Theme().TextMuted)
					})
				})
			})
		})
		app.settingsSection(c, tr("File Location", "文件位置"), "folder", func() {
			app.settingsCard(c).Padding(0, 15).Children(func() {
				app.settingsSwitchOnChange(c, tr("Remember Location", "记住位置"), tr("Use last export folder as default", "默认使用上次导出的文件夹"), "clock.arrow.circlepath", 78, &prefs.RememberLocation, func() {
					if !prefs.RememberLocation {
						prefs.LastDirectory = ""
					}
					app.preferencesChanged()
				})
			})
		})
	})
}

func (app *workspaceWindow) exportPaperLabel(paper exportPaper) string {
	switch paper.label {
	case "Letter":
		return app.tr(paper.label, "美式信纸")
	case "Legal":
		return app.tr(paper.label, "美式法律用纸")
	case "Tabloid":
		return app.tr(paper.label, "小报用纸")
	case "Executive":
		return app.tr(paper.label, "行政用纸")
	case "Statement":
		return app.tr(paper.label, "对账单用纸")
	case "Folio":
		return app.tr(paper.label, "对开纸")
	}
	return paper.label
}

func (app *workspaceWindow) exportPaperDescription(paper exportPaper) string {
	_, size, ok := strings.Cut(paper.description, " (")
	if !ok {
		return paper.description
	}
	size = strings.TrimSuffix(size, ")")
	size = strings.ReplaceAll(size, " in", " 英寸")
	size = strings.ReplaceAll(size, " mm", " 毫米")
	return app.tr(paper.description, app.exportPaperLabel(paper)+"（"+size+"）")
}

func (app *workspaceWindow) exportSettingsMenu(c *mygoui.Context, title, value, detail, symbol string, height float32, build func(*mygoui.Menu)) {
	settingsRow(c, title, detail, symbol, height, func() {
		theme := c.Theme()
		menu := mygoui.ButtonBase(c.Key(title)).Role(mygoui.RoleMenuButton).Label(title).Value(value).Menu(build).
			Size(170, 27).MinWidth(100).Shrink(1).AlignItems(mygoui.Center).Justify(mygoui.SpaceBetween).
			PaddingX(10).Gap(10).Radius(6).FocusRing(true)
		surface, border := theme.Surface, theme.Border
		if app.blackout && !c.Preferences().HighContrast {
			surface, border = mygoui.Hex("#1f1f23"), mygoui.Hex("#38383c")
		}
		if menu.Hovered() || menu.Pressed() {
			surface = theme.SurfaceHover
		}
		menu.Background(surface).Border(1, border).Children(func() {
			mygoui.Text(c, value).FontSize(12).FontWeight(600).SingleLine().Ellipsis("…").
				MinWidth(0).MaxWidth(124).Shrink(1).Role(mygoui.RoleNone)
			chromeIconColor(c, "chevron.up.chevron.down", 9, theme.TextMuted)
		})
	})
}

func (app *workspaceWindow) exportSettingsOrientation(c *mygoui.Context) {
	prefs := &app.preferences.Export
	title := app.tr("Orientation", "页面方向")
	labels := [...]string{app.tr("Portrait", "纵向"), app.tr("Landscape", "横向")}
	selected := 0
	if prefs.Landscape {
		selected = 1
	}
	settingsRow(c, title, labels[selected], "rectangle.portrait.arrowtriangle.2.outward", 80, func() {
		theme := c.Theme()
		trackFill, selectedFill, selectedInk := theme.SurfaceHover, theme.Accent, theme.AccentText
		if app.blackout && !c.Preferences().HighContrast {
			trackFill, selectedFill, selectedInk = mygoui.Hex("#2a2a2d"), mygoui.Hex("#444447"), theme.Text
			if c.WindowActive() {
				selectedFill, selectedInk = mygoui.Hex("#4598fa"), mygoui.RGB(255, 255, 255)
			}
		}
		parts := mygoui.SegmentedBase(c, &selected, len(labels))
		parts.Track.Label(title).Size(181, 24).MinWidth(128).Shrink(1).Radius(6).Background(trackFill).
			OnChange(func() {
				prefs.Landscape = selected == 1
				app.writePreferences()
			}).Children(func() {
			for index, label := range labels {
				segment := parts.Segment(index).Label(label).Basis(0).Grow(1).MinWidth(0).Height(24).Radius(6)
				ink := theme.Text
				if index == selected {
					segment.Background(selectedFill)
					ink = selectedInk
				}
				segment.Children(func() {
					mygoui.Text(c, label).FontSize(13).SingleLine().Ellipsis("…").TextColor(ink).Role(mygoui.RoleNone)
				})
			}
		})
	})
}
