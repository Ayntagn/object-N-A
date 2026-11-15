package ui

import mygoui "github.com/egoist/mygo/ui"

// EV-UI-022 / EV-UI-247. The typography, paper settings, Word note and location form four sections.
func (app *workspaceWindow) exportSettings(c *mygoui.Context) {
	prefs := &app.preferences.Export
	spacing := exportSpacingOptions[prefs.spacingIndex()]
	spacingDetail := spacing
	if spacing == exportSpacingOptions[0] {
		spacingDetail = "Use each theme's default"
	}
	paper, _ := findExportPaper(prefs.Paper)
	numberPages := !prefs.OmitPageNumbers
	theme := *c.Theme()
	if app.blackout && !c.Preferences().HighContrast {
		theme.TextMuted = mygoui.Hex("#c1c2cc")
		theme.Border = mygoui.Hex("#353539")
	}
	// The shared card adds one point of border on either side of its content.
	mygoui.Themed(c, &theme).FillWidth().Gap(20).Children(func() {
		app.settingsSection(c, "Preview & Export Typography", "text.alignleft", func() {
			app.settingsCard(c).Padding(0, 15).Children(func() {
				app.exportSettingsMenu(c, "Line Spacing", spacing, spacingDetail, "arrow.up.and.down.text.horizontal", 78, func(menu *mygoui.Menu) {
					for _, choice := range exportSpacingOptions {
						if menu.Item(choice).Radio(spacing == choice).Chosen() {
							prefs.LineSpacing = choice
							app.preferencesChanged()
						}
					}
				})
			})
		})
		app.settingsSection(c, "PDF Export", "doc.fill", func() {
			app.settingsCard(c).Padding(0, 15).Children(func() {
				app.exportSettingsMenu(c, "Paper Size", paper.description, paper.description, "doc.plaintext", 79, func(menu *mygoui.Menu) {
					for _, group := range exportPapers {
						for _, choice := range group.papers {
							if menu.Item(choice.label).Radio(paper.label == choice.label).Chosen() {
								prefs.Paper = choice.label
								app.writePreferences()
							}
						}
					}
				})
				mygoui.Divider(c).Height(1)
				app.exportSettingsOrientation(c)
				mygoui.Divider(c).Height(1)
				app.settingsSwitchOnChange(c, "Page Numbers", "Include page numbers in footer", "number", 79, &numberPages, func() {
					prefs.OmitPageNumbers = !numberPages
					app.preferencesChanged()
				})
			})
		})
		mygoui.Column(c).FillWidth().PaddingX(23.5).Children(func() {
			app.settingsSection(c, "Word Export", "doc.text.fill", func() {
				app.settingsCard(c).Padding(15).Children(func() {
					mygoui.Row(c).FillWidth().MinHeight(30).Gap(15).AlignItems(mygoui.Center).Children(func() {
						infoInk := c.Theme().Accent
						if app.blackout && !c.Preferences().HighContrast {
							infoInk = mygoui.Hex("#009dff")
						}
						mygoui.Box(c).Size(16, 16).Center().Children(func() {
							chromeIconColor(c, "info.circle", 21, infoInk)
						})
						mygoui.Text(c, "Word documents (.docx) are exported with standard formatting optimized for Microsoft Word and compatible applications.").
							FontSize(12).Grow(1).MinWidth(0).TextColor(c.Theme().TextMuted)
					})
				})
			})
		})
		app.settingsSection(c, "File Location", "folder", func() {
			app.settingsCard(c).Padding(0, 15).Children(func() {
				app.settingsSwitchOnChange(c, "Remember Location", "Use last export folder as default", "clock.arrow.circlepath", 78, &prefs.RememberLocation, func() {
					if !prefs.RememberLocation {
						prefs.LastDirectory = ""
					}
					app.preferencesChanged()
				})
			})
		})
	})
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
	labels := [...]string{"Portrait", "Landscape"}
	selected := 0
	if prefs.Landscape {
		selected = 1
	}
	settingsRow(c, "Orientation", labels[selected], "rectangle.portrait.arrowtriangle.2.outward", 80, func() {
		theme := c.Theme()
		trackFill, selectedFill, selectedInk := theme.SurfaceHover, theme.Accent, theme.AccentText
		if app.blackout && !c.Preferences().HighContrast {
			trackFill, selectedFill, selectedInk = mygoui.Hex("#2a2a2d"), mygoui.Hex("#444447"), theme.Text
			if c.WindowActive() {
				selectedFill, selectedInk = mygoui.Hex("#4598fa"), mygoui.RGB(255, 255, 255)
			}
		}
		parts := mygoui.SegmentedBase(c, &selected, len(labels))
		parts.Track.Label("Orientation").Size(181, 24).MinWidth(128).Shrink(1).Radius(6).Background(trackFill).
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
