package ui

import (
	"strings"

	mygoui "github.com/egoist/mygo/ui"
)

type calloutKind struct {
	name, icon, border string
}

// EV-UI-155. Order matches the original Alert toolbar menu.
var calloutKinds = [...]calloutKind{
	{"Note", "📝", "#a9c7ff"}, {"Tip", "💡", "#10b981"}, {"Info", "ℹ️", "#3b82f6"},
	{"Warning", "⚠️", "#f59e0b"}, {"Danger", "🔴", "#ef4444"}, {"Success", "✅", "#10b981"},
	{"Question", "❓", "#f59e0b"}, {"Example", "📋", "#8b5cf6"}, {"Quote", "💬", "#7f7f7f80"},
	{"Bug", "🐛", "#ef4444"}, {"Abstract", "📄", "#06b6d4"},
}

func calloutAppearance(kind string) calloutKind {
	kind = strings.ToLower(kind)
	switch kind {
	case "important":
		return calloutKind{"Important", "❗", "#8b5cf6"}
	case "hint":
		kind = "tip"
	case "caution", "attention":
		kind = "warning"
	case "error":
		kind = "danger"
	case "check", "done":
		kind = "success"
	case "help", "faq":
		kind = "question"
	case "cite":
		kind = "quote"
	case "summary", "tldr":
		kind = "abstract"
	}
	for _, style := range calloutKinds {
		if strings.EqualFold(style.name, kind) {
			return style
		}
	}
	return calloutKinds[0]
}

func (app *workspaceWindow) calloutMenu(m *mygoui.Menu) {
	for _, kind := range calloutKinds {
		if m.Item(kind.icon + " " + kind.name).Chosen() {
			app.insertCallout(kind)
		}
	}
}

func (app *workspaceWindow) insertCallout(kind calloutKind) {
	name := strings.ToLower(kind.name)
	if app.sourceEditing() {
		prefix := "> [!" + name + "] " + kind.name + "\n> "
		body := app.code.SelectedText()
		if body == "" {
			app.insertSourceSnippetAt(prefix+"Content\n", len(prefix)+1)
		} else {
			markdown := prefix + body + "\n"
			app.insertSourceSnippetAt(markdown, len(markdown))
		}
		return
	}
	w := &app.wys
	if app.displayedMode() != wysiwygMode || !w.editable(app) {
		return
	}
	if w.session.WrapCalloutSelection(name, kind.name) {
		w.sync(app)
		w.focus = true
	}
}
