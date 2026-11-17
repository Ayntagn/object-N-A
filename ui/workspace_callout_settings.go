package ui

import (
	"errors"
	"slices"
	"strings"

	"cyberwriter/internal/core/callout"
	mygoui "github.com/egoist/mygo/ui"
)

type calloutStyleForm struct {
	draft    callout.Style
	adding   bool
	feedback string
	invalid  bool
}

func (app *workspaceWindow) calloutStyleSettings(c *mygoui.Context) {
	tr := app.tr
	styles := app.calloutStyles()
	app.settingsSection(c, tr("Callout Styles", "提示块样式"), "exclamationmark.bubble", func() {
		card := app.settingsCard(c.Key("callout styles")).Gap(10)
		form := mygoui.Local(card, "style form", func() calloutStyleForm {
			return calloutStyleForm{draft: calloutStyleDraft("note", styles)}
		})
		card.Children(func() {
			app.calloutStylePicker(c, form, styles)
			changed := func() { form.feedback = "" }
			if form.adding {
				calloutStyleInput(c, tr("Type Identifier", "类型标识"), "research", &form.draft.Kind, changed).AutoFocus()
			}
			draft, validation := callout.Normalize(form.draft)
			inherited := callout.Resolve(strings.TrimSpace(form.draft.Kind), nil)
			calloutStyleInput(c, tr("Display Name", "显示名称"), inherited.Name, &form.draft.Name, changed).
				Description(tr("Default title when the Callout has no title.", "提示块未指定标题时使用的默认名称。"))
			mygoui.Row(c.Key("icon and color")).FillWidth().Gap(10).Children(func() {
				calloutStyleInput(c, tr("Icon", "图标"), inherited.Icon, &form.draft.Icon, changed)
				calloutStyleInput(c, tr("Accent Color", "主题色"), inherited.Color, &form.draft.Color, changed).
					Description(tr("Enter a six-digit hex color, such as #A9C7FF.", "输入六位十六进制颜色，例如 #A9C7FF。"))
			})
			sample := inherited
			if validation == nil {
				sample = callout.Resolve(draft.Kind, append([]callout.Style{draft}, styles...))
			}
			app.calloutStyleSample(c, sample)
			mygoui.Row(c.Key("callout style actions")).FillWidth().Gap(8).Wrap().Children(func() {
				mygoui.Button(c, tr("Save Style", "保存样式")).Height(28).OnClick(func() {
					form.setFeedback(app.saveCalloutStyle(form.draft), tr("Style saved", "样式已保存"), app.calloutStyleError)
					if !form.invalid {
						saved, _ := callout.Normalize(form.draft)
						form.draft, form.adding = saved, false
					}
				})
				if !form.adding && slices.ContainsFunc(styles, func(style callout.Style) bool { return style.Kind == form.draft.Kind }) {
					mygoui.Button(c, tr("Restore Default", "恢复默认样式")).Height(28).OnClick(func() {
						kind := form.draft.Kind
						form.setFeedback(app.removeCalloutStyle(kind), tr("Default style restored", "已恢复默认样式"), app.calloutStyleError)
						if form.invalid {
							return
						}
						if !slices.ContainsFunc(callout.Menu(app.calloutStyles()), func(style callout.Style) bool { return style.Kind == kind }) {
							kind = "note"
						}
						form.draft = callout.Style{Kind: kind}
					})
				}
			})
			if form.feedback != "" {
				role, ink := mygoui.RoleStatus, c.Theme().TextMuted
				if form.invalid {
					role, ink = mygoui.RoleAlert, c.Theme().Danger
				}
				mygoui.Text(c, form.feedback).Role(role).FontSize(11).TextColor(ink).Wrap()
			}
		})
	})
}

func (app *workspaceWindow) calloutStylePicker(c *mygoui.Context, form *calloutStyleForm, styles []callout.Style) {
	tr := app.tr
	label := tr("Callout Type", "提示块类型")
	value := tr("New Callout Type", "新提示块类型")
	if !form.adding {
		value = calloutStyleChoice(callout.Resolve(form.draft.Kind, styles))
	}
	theme := c.Theme()
	mygoui.Row(c.Key("callout type chooser")).FillWidth().Gap(8).AlignItems(mygoui.Center).Children(func() {
		mygoui.ButtonBase(c.Key("callout type")).Role(mygoui.RoleMenuButton).Label(label).Value(value).
			Menu(func(menu *mygoui.Menu) {
				for _, style := range callout.Menu(styles) {
					if menu.Item(calloutStyleChoice(style)).Checked(!form.adding && form.draft.Kind == style.Kind).Chosen() {
						*form = calloutStyleForm{draft: calloutStyleDraft(style.Kind, styles)}
					}
				}
			}).
			Basis(0).Grow(1).MinWidth(0).Height(28).Padding(0, 9).Radius(6).Gap(8).
			AlignItems(mygoui.Center).Justify(mygoui.SpaceBetween).FocusRing(true).
			Background(theme.Background).Border(1, theme.Border).Children(func() {
			mygoui.Text(c, value).MinWidth(0).Shrink(1).FontSize(12).FontWeight(600).
				SingleLine().Ellipsis("…").Role(mygoui.RoleNone)
			chromeIconColor(c, "chevron.up.chevron.down", 9, theme.TextMuted).Shrink(0)
		})
		mygoui.Button(c, tr("Add Type…", "添加类型…")).Height(28).Shrink(0).OnClick(func() {
			*form = calloutStyleForm{adding: true}
		})
	})
}

func calloutStyleChoice(style callout.Style) string {
	return style.Name + " [!" + style.Kind + "]"
}

func calloutStyleDraft(kind string, styles []callout.Style) callout.Style {
	for _, style := range styles {
		if style.Kind == kind {
			return style
		}
	}
	return callout.Style{Kind: kind}
}

func calloutStyleInput(c *mygoui.Context, label, hint string, value *string, changed func()) mygoui.Element {
	var field mygoui.Element
	mygoui.Column(c.Key(label)).MinWidth(0).Grow(1).Gap(4).Children(func() {
		mygoui.Text(c, label).FontSize(12).FontWeight(600).Role(mygoui.RoleNone).Wrap()
		theme := c.Theme()
		field = mygoui.TextInputBase(c.Key("input"), value).Label(label).Placeholder(hint).FillWidth().MinWidth(0).
			Height(28).Padding(0, 8).FontSize(13).Radius(6).Background(theme.Background).Border(1, theme.Border).
			OnChange(changed)
		if field.Focused() {
			field.BorderColor(theme.Accent)
		}
	})
	return field
}

func (app *workspaceWindow) calloutStyleSample(c *mygoui.Context, style callout.Style) {
	accent := mygoui.Hex(style.Color)
	mygoui.Column(c.Key("callout style sample")).Role(mygoui.RoleGroup).Label(app.tr("Callout Style Preview", "提示块样式预览")).
		FillWidth().MinHeight(34).Padding(8, 12).Radius(6).Clip().Background(c.Theme().Background.Mix(accent, .12)).
		Children(func() {
			mygoui.Box(c.Key("accent")).Absolute().Left(0).Top(0).Bottom(0).Width(4).Background(accent).Role(mygoui.RoleNone)
			mygoui.Text(c, style.Icon+" "+style.Name).FontSize(13).FontWeight(650).Wrap()
		})
}

func (form *calloutStyleForm) setFeedback(err error, success string, describeError func(error) string) {
	form.invalid = err != nil
	form.feedback = success
	if form.invalid {
		form.feedback = describeError(err)
	}
}

func (app *workspaceWindow) calloutStyleError(err error) string {
	switch {
	case errors.Is(err, callout.ErrKind):
		return app.tr("Start the type identifier with a letter. Use up to 32 letters, digits, hyphens, or underscores.",
			"类型标识需以英文字母开头，最多 32 个字符，可用英文字母、数字、连字符和下划线。")
	case errors.Is(err, callout.ErrName):
		return app.tr("Enter a single-line name of up to 64 characters.", "显示名称需为单行，最多 64 个字符。")
	case errors.Is(err, callout.ErrIcon):
		return app.tr("Enter an emoji or short text of up to 12 characters.", "输入表情或简短图标文字，最多 12 个字符。")
	case errors.Is(err, callout.ErrColor):
		return app.tr("Enter a six-digit hex color, such as #A9C7FF.", "输入六位十六进制颜色，例如 #A9C7FF。")
	default:
		return app.tr("Could not save Callout styles.", "无法保存提示块样式。")
	}
}
