package ui

import (
	"path"
	"strings"
	"unicode"

	"cyberwriter/internal/core/vaultcontext"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-294 / EV-UI-317. The section shows one button per matched passage, in search order.
func (app *workspaceWindow) drawVaultRelatedResults(c *mygoui.Context, matches []vaultcontext.Match, colors vaultSectionColors) {
	if len(matches) == 0 {
		return
	}
	ink := colors.secondary
	if !c.Preferences().HighContrast {
		appearance := app.appAppearance()
		if palette, ok := appearancePalettes[appearance]; ok {
			ink = palette.muted
		} else if appearance == appearanceBlackout {
			ink = mygoui.RGB(138, 141, 168)
		}
	}
	mygoui.Column(c.Key("related ideas")).FillWidth().Shrink(0).Children(func() {
		mygoui.Text(c, "RELATED IDEAS").AlignSelf(mygoui.Start).Margin(8, 12, 4).
			FontSize(11).FontWeight(600).FixedLineHeight(14).LetterSpacing(.8).SingleLine().TextColor(ink)
		for _, hit := range matches {
			key := struct {
				path   string
				offset int
			}{hit.Path, hit.Offset}
			name := strings.TrimSuffix(path.Base(hit.Path), ".md")
			preview := strings.TrimFunc(strings.ReplaceAll(hit.Text, "\n", " "), func(r rune) bool {
				return r == '\t' || unicode.Is(unicode.Zs, r)
			})
			label := name
			if preview != "" {
				label += ", " + preview
			}
			row := mygoui.ButtonBase(c.Key(key)).FillWidth().Shrink(0).Padding(4, 12).
				KeepFocus().Label(label)
			row.OnClick(func() { app.openVaultConceptMatch(hit, row.ClickModifiers()) }).Children(func() {
				mygoui.Column(c).FillWidth().MinWidth(0).Gap(2).AlignItems(mygoui.Start).Children(func() {
					mygoui.Row(c).FillWidth().Gap(4).AlignItems(mygoui.Center).Children(func() {
						// The natural-size rasterizer leaves room around the 11 pt symbol.
						sfsymbols.Symbol(c, "text.viewfinder").Size(15).Natural().
							Color(ink).Fallback(chromeIconFallbacks["doc.text"]).Role(mygoui.RoleNone)
						mygoui.Text(c, name).MinWidth(0).FontSize(12).FontWeight(500).FixedLineHeight(15).SingleLine().Ellipsis("…").
							TextColor(colors.headerText).Role(mygoui.RoleNone)
					})
					mygoui.Text(c, preview).FillWidth().MinWidth(0).Font("monospace").FontSize(11).FixedLineHeight(14).
						MaxLines(2).Ellipsis("…").TextColor(colors.secondary.Alpha(.7)).Role(mygoui.RoleNone)
				})
			})
		}
	})
}
