package ui

import (
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

var exportCheckmark = mygoui.MustParseSVG([]byte(`<svg viewBox="0 0 56 56">
<path fill-rule="evenodd" d="M28 0a28 28 0 1 1 0 56 28 28 0 0 1 0-56Z
M15.5 27.8a2.4 2.4 0 0 1 3.4.1l5.2 6.2L36.6 17a2.4 2.4 0 1 1 3.9 2.8L26.2 39.4a2.4 2.4 0 0 1-3.8.1l-7.2-8.3a2.4 2.4 0 0 1 .3-3.4Z"/>
</svg>`))

func revealExportLabel() string {
	if runtime.GOOS == "darwin" {
		return "Show in Finder"
	}
	return "Show in File Manager"
}

func (app *workspaceWindow) showQuickExportResult(win *mygo.Window, format exportFormat, filename string) {
	options := mygo.MessageOptions{
		Parent: win, Title: "Word Export", Message: "Export Successful", Detail: "Saved to " + filepath.Base(filename),
		Buttons: []string{"Done", revealExportLabel()},
	}
	revealButton := 1
	if format == exportHTML {
		// EV-UI-137: Finder appears first and Done is the default button.
		options.Title = "HTML Export"
		options.Buttons = []string{revealExportLabel(), "Done"}
		options.DefaultButton, options.CancelButton = 1, 1
		revealButton = 0
	}
	result, err := mygo.Dialog.Message(options)
	if err == nil && result.Button == revealButton {
		mygo.Shell.ShowItemInFolder(filename)
	}
	win.Update(func() { app.modalOpen = false })
}

func (s *workspaceExportSheet) exportResult(c *mygoui.Context) {
	if s.resultPath == "" && s.resultError == "" {
		return
	}
	open := true
	finish := func() {
		s.resultPath, s.resultError = "", ""
		s.win.Close()
	}
	retry := func() {
		s.resultError = ""
		if s.format == exportDOCX {
			s.start()
		}
	}
	mygoui.DialogBase(c.Key("export-result"), &open, func(backdrop, panel mygoui.Element) {
		theme := c.Theme()
		backdrop.Background(mygoui.RGBA(0, 0, 0, 0.35))
		panel.Column().AlignItems(mygoui.Center).Background(theme.Surface)
		html := s.format == exportHTML
		if html {
			// EV-UI-137: the status icon and actions align across success and cancellation.
			panel.Width(364).MinHeight(250).Padding(37.5, 28, 32, 28).Radius(16).Label("HTML Export")
			if s.app.blackout && !c.Preferences().HighContrast {
				panel.Background(mygoui.Hex("#18181c"))
			}
		} else {
			panel.Width(340).Padding(20).Gap(12).Radius(12).Border(1, theme.Border).Label("Word Export")
		}
		if s.resultError != "" {
			if html {
				chromeIconColor(c, "exclamationmark.triangle.fill", 56, theme.Warning).Size(57.5, 56).
					Role(mygoui.RoleImage).Label("Warning").Margin(0, 0, 63.25)
			} else {
				mygoui.Text(c, "Could Not Export Word Document").FontWeight(600).Role(mygoui.RoleHeading)
			}
			message := mygoui.Text(c, s.resultError).TextColor(theme.TextMuted).TextAlign(mygoui.Center).Wrap()
			if html {
				message.FontSize(12).FixedLineHeight(15).Margin(0, 0, 20.25)
			}
			row := mygoui.Row(c).Gap(10)
			if html {
				row.Gap(16)
			}
			row.Children(func() {
				cancel := mygoui.Button(c, "Cancel").OnClick(finish)
				tryAgain := mygoui.PrimaryButton(c, "Try Again").OnClick(retry)
				if html {
					cancel.Size(66, 24).PaddingX(8)
					tryAgain.Size(81, 24).PaddingX(8)
				}
			})
			return
		}
		green := mygoui.RGB(52, 199, 89)
		if html {
			mygoui.Icon(c, exportCheckmark).Size(56, 56).TextColor(green).
				Role(mygoui.RoleImage).Label("Selected").Margin(0, 0, 24.75)
		} else {
			chromeIconColor(c, "checkmark.circle", 44, green).Size(44, 44)
		}
		heading := mygoui.Text(c, "Export Successful!").FontSize(17).FontWeight(600).Role(mygoui.RoleHeading)
		filename := mygoui.Text(c, filepath.Base(s.resultPath)).TextColor(theme.TextMuted).TextAlign(mygoui.Center).Wrap()
		if html {
			heading.FixedLineHeight(20.5).Margin(0, 0, 20)
			filename.FontSize(12).FixedLineHeight(15).Margin(0, 0, 20.25)
		}
		mygoui.Row(c).Gap(16).Children(func() {
			reveal := mygoui.Button(c, revealExportLabel()).MinWidth(113.5).Height(24).OnClick(func() {
				mygo.Shell.ShowItemInFolder(s.resultPath)
				finish()
			})
			done := mygoui.PrimaryButton(c, "Done").Size(56, 24).OnClick(finish)
			if html {
				reveal.Size(113.5, 24).PaddingX(8).FontSize(13)
				done.PaddingX(8).FontSize(13)
				if s.app.blackout && !c.Preferences().HighContrast {
					reveal.Background(mygoui.Hex("#29292d")).BorderColor(mygoui.Transparent)
				}
			}
		})
	})
	if !open {
		finish()
	}
}
