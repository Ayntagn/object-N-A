package ui

import (
	"path"
	"strconv"

	"cyberwriter/internal/core/vaultindex"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

type vaultSectionColors struct {
	headerFill, headerText, secondary, accent, border, buttonFill mygoui.Color
}

func (app *workspaceWindow) vaultSectionColors(c *mygoui.Context) vaultSectionColors {
	theme := c.Theme()
	colors := vaultSectionColors{headerFill: theme.Surface, headerText: theme.Text,
		secondary: theme.TextMuted, accent: theme.Accent, border: theme.Border, buttonFill: theme.SurfaceHover}
	if c.Preferences().HighContrast {
		return colors
	}
	switch app.appAppearance() {
	case appearanceBlackout:
		colors.headerFill = mygoui.Hex("#18181c").Alpha(.5)
		colors.headerText = mygoui.Hex("#e3e2e6")
		colors.secondary = mygoui.Hex("#c4c6d0")
		colors.accent = blackoutTitleAccent
		colors.border = mygoui.Hex("#45474b")
		colors.buttonFill = mygoui.Hex("#19191a")
	case appearanceCyber:
		colors.headerFill = mygoui.Hex("#262a31").Alpha(.5)
		colors.headerText = mygoui.Hex("#e3e2e6")
		colors.secondary = theme.Text
		colors.border = mygoui.Hex("#45474b")
	case appearancePLATO:
		colors.headerFill = mygoui.Hex("#241809").Alpha(.5)
		colors.headerText = mygoui.Hex("#e8b36a")
		colors.secondary = theme.Text
		colors.border = mygoui.Hex("#4a3319")
	}
	return colors
}

func vaultSectionHeading(c *mygoui.Context, title, count, symbol string, expanded, disabled bool, colors vaultSectionColors, toggle func()) {
	mygoui.ButtonBase(c.Key("toggle")).FillWidth().Height(27).Padding(0, 12).Shrink(0).
		Gap(4).Justify(mygoui.Start).AlignItems(mygoui.Center).Background(colors.headerFill).
		Label(title + ", " + count).Expanded(expanded).Disabled(disabled).OnClick(toggle).Children(func() {
		chevron := sfsymbols.Symbol(c, "chevron.right").Size(10).Weight(sfsymbols.Semibold).
			Color(colors.secondary.Alpha(.5)).Fallback(chromeIconFallbacks["chevron.right"]).Role(mygoui.RoleNone)
		if expanded {
			chevron.Rotate(90)
		}
		sfsymbols.Symbol(c, symbol).Size(14).Color(colors.accent.Alpha(.7)).
			Fallback(chromeIconFallbacks["link"]).Role(mygoui.RoleNone)
		mygoui.Text(c, title).FontSize(12).FontWeight(600).TextColor(colors.headerText).Role(mygoui.RoleNone)
		mygoui.Text(c, count).Font("monospace").FontSize(11).TextColor(colors.secondary.Alpha(.5)).Role(mygoui.RoleNone)
	})
}

func (app *workspaceWindow) drawUnlinkedMentions(c *mygoui.Context, result vaultindex.UnlinkedMentionResult) {
	if len(result.Hits) == 0 {
		return
	}
	const title = "Unlinked Mentions"
	colors := app.vaultSectionColors(c)
	expanded := app.preferences.VaultMentionsExpanded
	count := strconv.Itoa(result.Total)
	if result.Truncated {
		count += "+"
	}
	mygoui.Column(c.Key("unlinked mentions")).Margin(0, 3, 0, 4).Shrink(0).Children(func() {
		mygoui.Box(c).FillWidth().Height(1).Shrink(0).Background(colors.border.Alpha(.2))
		header := mygoui.Row(c.Key("heading")).FillWidth().Height(27).Shrink(0)
		header.Children(func() {
			vaultSectionHeading(c, title, count, "link.badge.plus", expanded, app.modalOpen, colors, app.toggleUnlinkedMentions)
			if expanded && len(result.Hits) >= 2 {
				all := mygoui.ButtonBase(c.Key("link all")).Absolute().Right(6).Top(6).Height(15).Padding(0, 6).
					Label("Link All").Tooltip("Convert every mention below into a wikilink").KeepFocus().
					Disabled(app.modalOpen || app.mentions.refreshing).OnClick(func() { app.linkUnlinkedMentions(result.Hits) })
				all.Children(func() {
					mygoui.Text(c, "Link All").FontSize(11).FontWeight(600).TextColor(colors.accent).Role(mygoui.RoleNone)
				})
			}
		})
		if !expanded {
			return
		}
		mygoui.Scroll(c.Key("rows")).FillWidth().MaxHeight(220).Shrink(0).Label(title + " list").Children(func() {
			mygoui.Column(c).FillWidth().Children(func() {
				for _, hit := range result.Hits {
					app.drawUnlinkedMentionRow(c, hit, colors)
				}
			})
		})
	})
}

func (app *workspaceWindow) drawUnlinkedMentionRow(c *mygoui.Context, hit vaultindex.UnlinkedMentionHit, colors vaultSectionColors) {
	key := struct {
		path       string
		start, end int
	}{hit.Path, hit.Start, hit.End}
	row := mygoui.Row(c.Key(key)).FillWidth().Padding(4, 12, 3).Gap(6).AlignItems(mygoui.Start).
		BorderWidth(0, 0, 1, 0).BorderColor(colors.border.Alpha(.2))
	row.Children(func() {
		mygoui.Column(c).Grow(1).MinWidth(0).Gap(1).AlignItems(mygoui.Start).Children(func() {
			name := app.noteLabel(path.Base(hit.Path))
			source := mygoui.ButtonBase(c.Key("source")).Height(14).MaxWidthPercent(100).Padding(0).
				Justify(mygoui.Start).AlignItems(mygoui.Center).Label(name).Tooltip(hit.Path).
				Disabled(app.modalOpen).OnClick(func() { app.openUnlinkedMention(hit.Path) })
			source.Children(func() {
				mygoui.Text(c, name).SingleLine().MinWidth(0).FontSize(11.5).FontWeight(500).
					TextColor(colors.accent).Role(mygoui.RoleNone)
			})
			mygoui.Text(c, hit.Snippet).FillWidth().MinWidth(0).FontSize(11).FixedLineHeight(14).
				MaxLines(2).TextColor(colors.secondary.Alpha(.8))
		})
		link := mygoui.ButtonBase(c.Key("link")).Size(34.5, 13).Margin(1, 0, 0).Shrink(0).Padding(0, 6).
			Radius(4).Label("Link").Tooltip("Replace this mention with a wikilink").KeepFocus().
			Background(colors.buttonFill).Disabled(app.modalOpen || app.mentions.refreshing).
			OnClick(func() { app.linkUnlinkedMentions([]vaultindex.UnlinkedMentionHit{hit}) })
		if link.Hovered() {
			link.Background(colors.buttonFill.Mix(colors.headerText, .12))
		}
		link.Children(func() {
			mygoui.Text(c, "Link").FontSize(11).FontWeight(600).TextColor(colors.headerText).Role(mygoui.RoleNone)
		})
	})
}
