package ui

import mygoui "github.com/egoist/mygo/ui"

// EV-UI-283. Connection has a leading Saved picker when profiles exist,
// a Trash button beside a selected item, and New at the trailing edge.
func (app *workspaceWindow) drawAIProfileControls(c *mygoui.Context) {
	tr := app.tr
	profiles := app.preferences.AIProfiles
	height := float32(20)
	if len(profiles) > 0 {
		height = 24
	}
	mygoui.Row(c.Key("AI profiles")).FillWidth().Height(height).Margin(0, 0, 30).
		AlignItems(mygoui.Center).Gap(8).Children(func() {
		if len(profiles) > 0 {
			mygoui.Text(c, tr("Saved:", "已保存：")).FontSize(12).SingleLine().Shrink(0)
			app.drawAIProfilePicker(c, profiles)
			id := app.aiConnectionForm.ProfileID
			if app.preferences.profileIndex(id) >= 0 {
				chromeControl(c.Key("delete AI profile"), tr("Trash", "删除配置"), false).
					Size(18, 20).OnClick(func() { app.deleteAIProfile(id) }).Children(func() {
					aiWorkspaceIcon(c, "trash", "trash", 12, c.Theme().TextMuted)
				})
			}
		}
		mygoui.Spacer(c)
		mygoui.Button(c, tr("New", "新建")).Size(63.5, 20).Shrink(0).FontSize(11).OnClick(app.newAIConnection)
	})
}

func (app *workspaceWindow) drawAIProfilePicker(c *mygoui.Context, profiles []workspaceAIProfile) {
	theme := c.Theme()
	newLabel := app.tr("New connection", "新建连接")
	selection := app.aiConnectionForm.ProfileID
	label := newLabel
	if index := app.preferences.profileIndex(selection); index >= 0 {
		label = profiles[index].label()
	} else {
		selection = ""
	}
	picker := mygoui.SelectBase(c.Key("pick saved AI profile"), &selection)
	picker.Trigger.Size(257.5, 24).MinWidth(0).Shrink(1).Padding(0, 8).Radius(5).
		Background(theme.SurfacePressed).Border(1, theme.Border).Row().AlignItems(mygoui.Center).
		Label(label).OnChange(func() { app.selectAIProfile(selection) }).Children(func() {
		mygoui.Text(c, label).FontSize(12).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").Role(mygoui.RoleNone)
		aiWorkspaceIcon(c, "chevron.up.chevron.down", "chevron.down", 10, theme.TextMuted)
	})
	picker.Popup(func(panel mygoui.Element) {
		panel.Padding(4).Radius(6).Gap(2).Background(theme.Surface).Border(1, theme.Border)
		item := func(id, name string) {
			option := picker.Item(id).Padding(4, 8).Radius(4).Label(name)
			if option.Highlighted() {
				option.Background(theme.SurfaceHover)
			}
			option.Children(func() {
				mygoui.Text(c, name).FontSize(12).SingleLine().Ellipsis("…").Role(mygoui.RoleNone)
			})
		}
		item("", newLabel)
		for _, profile := range profiles {
			item(profile.ID, profile.label())
		}
	})
}
