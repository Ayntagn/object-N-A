package ui

import (
	"slices"
	"strings"
	"unicode/utf8"

	"cyberwriter/ui/wysiwyg"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/richtext"
)

type workspaceAIVisualWriteback struct {
	tab                         *workspaceTab
	draft                       *editor.Buffer
	session                     *wysiwyg.Session
	document, preview           *richtext.Document
	draftVersion, visualVersion uint64
	start, end                  int
}

// EV-UI-246: streamed text stays literal until the visual editor commits
// the received Markdown as one undo step, including the prefix on Stop.
func (app *workspaceWindow) beginAIVisualWriteback() *workspaceAIVisualWriteback {
	if !app.preferences.AIStreamIntoEditor {
		return nil
	}
	target := app.aiVisualWritebackAtSelection(app.ai.replaceSelection)
	if target != nil {
		text := target.document.Text()
		if target.start != target.end {
			text = text.Delete(target.start, target.end)
		}
		target.preview = richtext.NewDocument(text)
		target.preview.SetSelection(target.start, target.start)
	}
	return target
}

func (app *workspaceWindow) aiVisualWritebackAtSelection(replace bool) *workspaceAIVisualWriteback {
	if app.workspaceTab == nil || app.draft == nil || app.mode != wysiwygMode || !app.wys.editable(app) {
		return nil
	}
	session := app.wys.session
	session.Sync(app.draft)
	doc := session.Document
	anchor, caret := doc.Selection()
	start := min(anchor, caret)
	end := start
	if replace {
		end = max(anchor, caret)
	}
	return &workspaceAIVisualWriteback{
		tab: app.workspaceTab, draft: app.draft, session: session, document: doc,
		draftVersion: app.draft.Version(), visualVersion: doc.Version(), start: start, end: end,
	}
}

func (target *workspaceAIVisualWriteback) current(app *workspaceWindow) bool {
	tab := target.tab
	return !app.modalOpen && (app.win == nil || !app.win.IsDestroyed()) &&
		tab.mode == wysiwygMode && slices.Contains(app.tabs, tab) && tab.draft == target.draft &&
		tab.wys.session == target.session && target.session.Document == target.document &&
		target.draft.Version() == target.draftVersion && target.document.Version() == target.visualVersion
}

func (target *workspaceAIVisualWriteback) previewFor(app *workspaceWindow, document *richtext.Document) *richtext.Document {
	if target.preview != nil && app.workspaceTab == target.tab && document == target.document && target.current(app) {
		return target.preview
	}
	return nil
}

func (target *workspaceAIVisualWriteback) update(app *workspaceWindow, reply string) bool {
	if target.preview == nil || !target.current(app) {
		return false
	}
	reply = strings.ReplaceAll(reply, "\r\n", "\n")
	text := target.document.Text().ReplaceString(target.start, target.end, reply, richtext.Attributes{})
	target.preview.SetText(text)
	caret := target.start + utf8.RuneCountInString(reply)
	target.preview.SetSelection(caret, caret)
	return true
}

func (target *workspaceAIVisualWriteback) commit(app *workspaceWindow, reply string) bool {
	target.preview = nil
	if reply == "" || !target.current(app) {
		return false
	}
	reply = strings.ReplaceAll(reply, "\r\n", "\n")
	if strings.TrimSpace(reply) == "" || !target.session.InsertMarkdownAt(target.start, target.end, reply) {
		attributes := target.document.TypingAttributes()
		target.document.Edit(func(edit *richtext.Edit) {
			edit.ReplaceString(target.start, target.end, reply, attributes)
			caret := target.start + utf8.RuneCountInString(reply)
			edit.Select(caret, caret)
		})
	}
	target.session.Sync(target.draft)
	if app.workspaceTab == target.tab {
		tab := target.tab
		tab.wys.focus = true
	}
	return true
}
