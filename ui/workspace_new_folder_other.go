//go:build !darwin

package ui

import (
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

func showWorkspaceNamePrompt(origin *mygo.Window, options workspaceNamePrompt, done func(string, bool)) {
	name, confirmed, initial := options.initial, false, true
	height := 232
	if options.detail != "" {
		height = 259
	}
	var prompt *mygo.Window
	submit := func() {
		confirmed = true
		prompt.Close()
	}
	prompt = mygo.NewWindow(mygo.WindowOptions{
		Title: options.title, Width: 312, Height: height, Hidden: true, UseContentSize: true,
		Parent: origin, Modal: true, DisableResize: true, DisableMinimize: true, DisableMaximize: true,
		TitleBarStyle: mygo.TitleBarHidden, HideWindowButtons: true,
		Content: mygoui.View(func(c *mygoui.Context) {
			c.OnWindowShortcut(0, mygoui.KeyEscape, func() { prompt.Close() })
			mygoui.Column(c).Fill().Padding(19, 15, 15).Gap(10).Children(func() {
				if options.icon != "" {
					chromeIcon(c, options.icon, 40)
				}
				mygoui.Text(c, options.heading).Role(mygoui.RoleHeading).FontSize(14).Bold().Wrap()
				if options.detail != "" {
					mygoui.Text(c, options.detail).FontSize(13).Wrap()
				}
				input := mygoui.TextInputBase(c, &name).Label(options.field).FillWidth().
					Height(26).OnSubmit(submit).OnTextKey(func(key mygoui.TextKey) bool {
					if key.Mods == 0 && key.Key == mygoui.KeyEscape {
						prompt.Close()
						return true
					}
					return false
				})
				if initial {
					input.Focus().SetTextSelection(0, len(options.initial))
					initial = false
				}
				mygoui.Spacer(c)
				mygoui.Row(c).Gap(6).Justify(mygoui.Center).Children(func() {
					mygoui.Button(c, "Cancel").Size(112, 30).OnClick(func() { prompt.Close() })
					mygoui.PrimaryButton(c, options.action).Size(112, 30).OnClick(submit)
				})
			})
		}),
	})
	prompt.OnClosed(func() { done(name, confirmed) })
	prompt.Show()
}
