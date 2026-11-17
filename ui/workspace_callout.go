package ui

import (
	"slices"
	"strings"

	"cyberwriter/internal/core/callout"
	mygoui "github.com/egoist/mygo/ui"
)

func (app *workspaceWindow) calloutMenu(m *mygoui.Menu) {
	for _, style := range callout.Menu(app.calloutStyles()) {
		if m.Item(style.Icon + " " + style.Name).Chosen() {
			app.insertCallout(style)
		}
	}
}

func (app *workspaceWindow) insertCallout(style callout.Style) {
	if app.sourceEditing() {
		prefix := "> [!" + style.Kind + "] " + style.Name + "\n> "
		body := app.code.SelectedText()
		if body == "" {
			app.insertSourceSnippetAt(prefix+"Content\n", len(prefix)+1)
		} else {
			markdown := prefix + strings.ReplaceAll(body, "\n", "\n> ") + "\n"
			app.insertSourceSnippetAt(markdown, len(markdown))
		}
		return
	}
	w := &app.wys
	if app.displayedMode() != wysiwygMode || !w.editable(app) {
		return
	}
	if w.session.WrapCalloutSelection(style.Kind, style.Name) {
		w.sync(app)
		w.focus = true
	}
}

func (app *workspaceWindow) calloutStyles() []callout.Style {
	if app.preferences == nil {
		return nil
	}
	return app.preferences.CalloutStyles
}

func upsertCalloutStyle(styles []callout.Style, style callout.Style) []callout.Style {
	if existing := slices.IndexFunc(styles, func(row callout.Style) bool { return row.Kind == style.Kind }); existing >= 0 {
		styles[existing] = style
		return styles
	}
	return append(styles, style)
}

func (app *workspaceWindow) saveCalloutStyle(style callout.Style) error {
	clean, err := callout.Normalize(style)
	if err != nil {
		return err
	}
	return app.commitCalloutStyles(upsertCalloutStyle(slices.Clone(app.calloutStyles()), clean))
}

func (app *workspaceWindow) removeCalloutStyle(kind string) error {
	styles := app.calloutStyles()
	if index := slices.IndexFunc(styles, func(style callout.Style) bool { return style.Kind == kind }); index >= 0 {
		return app.commitCalloutStyles(slices.Delete(slices.Clone(styles), index, index+1))
	}
	return nil
}

func (app *workspaceWindow) commitCalloutStyles(styles []callout.Style) error {
	pending := *app.preferences
	pending.CalloutStyles = styles
	if err := pending.write(); err != nil {
		return err
	}
	*app.preferences = pending
	for _, window := range app.documentWindows() {
		window.previewPane.refresh()
		if window.win != nil && !window.win.IsDestroyed() {
			window.win.Invalidate()
		}
		if window.settings != nil && !window.settings.IsDestroyed() {
			window.settings.Invalidate()
		}
	}
	return nil
}
