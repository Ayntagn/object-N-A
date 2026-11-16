package ui

import mygoui "github.com/egoist/mygo/ui"

const vaultEmbeddingOfferTitle = "Let the AI read this vault?"

// EV-UI-195 / EV-UI-300 / EV-UI-302. This sits between the search section
// and Recents. Keeping focus lets a dismissal return space to the file list.
func (app *workspaceWindow) drawVaultEmbeddingOffer(c *mygoui.Context) {
	mode := app.vaultEmbeddingOfferMode()
	if mode == vaultEmbeddingOfferHidden {
		return
	}
	vault := app.vault
	label := "Build index"
	detail := "Builds an index once so the AI can pull in related notes you didn't mention. It stays on your Mac."
	if mode == vaultEmbeddingOfferDownload {
		label = "Get the model"
		detail = "Needs Apple's embedding model first. Then the AI can pull in related notes you didn't mention."
	}

	theme := c.Theme()
	fill := theme.Surface.Mix(theme.Accent, 0.07)
	outline := theme.Border.Mix(theme.Accent, 0.25)
	titleInk, bodyInk, subdued := theme.Text, theme.TextMuted, theme.TextMuted
	iconInk, actionFill, actionInk := theme.Accent, theme.Accent, theme.AccentText
	if app.blackout {
		fill, outline = mygoui.Hex("#160E19"), mygoui.Hex("#533752")
		titleInk, bodyInk, subdued = mygoui.Hex("#E3E2E6"), mygoui.Hex("#C4C6D0"), mygoui.Hex("#96959F")
		iconInk, actionFill, actionInk = mygoui.Hex("#FFAAF7"), mygoui.Hex("#A9C7FF"), mygoui.Hex("#003063")
	}

	card := mygoui.Column(c.Key("Vault embedding offer")).AlignSelf(mygoui.Stretch).MinWidth(0).
		Shrink(0).Margin(6, 16, 10).Padding(8).Gap(6).Background(fill).Radius(8).
		DrawOver(func(p *mygoui.Painter, frame mygoui.Rect) { p.Stroke(frame, outline, 8, 1) })
	card.Children(func() {
		mygoui.Row(c).FillWidth().MinWidth(0).Gap(6).AlignItems(mygoui.Center).Children(func() {
			mygoui.Row(c).Size(12, 15).Shrink(0).Center().Children(func() {
				// The SF Symbol raster includes padding around the visible glyph.
				chromeIconColor(c, "sparkles", 16.5, iconInk)
			})
			mygoui.Text(c, vaultEmbeddingOfferTitle).MinWidth(0).FontSize(12).FontWeight(600).
				FixedLineHeight(15).TextColor(titleInk).Wrap()
		})
		mygoui.Text(c, detail).AlignSelf(mygoui.Stretch).MinWidth(0).FontSize(11).FixedLineHeight(14).
			TextColor(bodyInk).Wrap()
		mygoui.Row(c).FillWidth().MinWidth(0).Margin(2, 0, 0).AlignItems(mygoui.Center).Gap(8).Wrap().Children(func() {
			primary := mygoui.ButtonBase(c.Key("primary")).Height(22).MinWidth(80.5).Padding(0, 10).Shrink(0).
				Center().Radius(6).Background(actionFill).Label(label).KeepFocus().Disabled(app.modalOpen).
				OnClick(func() { app.activateVaultEmbeddingOffer(vault) })
			if primary.Hovered() {
				primary.Background(actionFill.Mix(actionInk, 0.08))
			}
			primary.Children(func() {
				mygoui.Text(c, label).SingleLine().FontSize(11).FontWeight(600).
					FixedLineHeight(14).TextColor(actionInk).Role(mygoui.RoleNone)
			})
			app.vaultEmbeddingDismissButton(c, "Not now", "", 43.5, bodyInk, func() {
				app.dismissVaultEmbeddingOffer(vault, false)
			})
			app.vaultEmbeddingDismissButton(c, "Don't ask", "Never offer this for this vault", 50, subdued, func() {
				app.dismissVaultEmbeddingOffer(vault, true)
			})
		})
	})
}

func (app *workspaceWindow) vaultEmbeddingDismissButton(c *mygoui.Context, title, help string, width float32, ink mygoui.Color, action func()) {
	button := mygoui.ButtonBase(c.Key(title)).Size(width, 14).Shrink(0).Center().Radius(3).Label(title).
		KeepFocus().Disabled(app.modalOpen).OnClick(action)
	if help != "" {
		button.Tooltip(help)
	}
	if button.Hovered() {
		button.Background(ink.Alpha(0.11))
	}
	button.Children(func() {
		mygoui.Text(c, title).SingleLine().FontSize(11).FixedLineHeight(14).
			TextColor(ink).Role(mygoui.RoleNone)
	})
}
