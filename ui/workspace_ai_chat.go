package ui

import (
	"context"
	"math"
	"strings"

	"cyberwriter/internal/core/aiworkspace"
	"github.com/egoist/mygo"
	"github.com/go-text/typesetting/segmenter"
)

type workspaceAITurn struct {
	prompt, model, contextHint string
	reply, problem             string
	previewHint                string
	keepInHistory              bool
	contextOwner               *workspaceAIContext
}

type workspaceAIRequest struct {
	cancel          context.CancelFunc
	turn            int
	reply           strings.Builder
	writeback       *workspaceAIWriteback
	visualWriteback *workspaceAIVisualWriteback
	finishVisual    func(string)
	saveTurn        func()
}

func (s *workspaceAI) stopResponse() {
	if request := s.request; request != nil {
		request.cancel()
		s.request = nil
		// EV-UI-242: a stopped turn's received text remains in the next request.
		turn := &s.submitted[request.turn]
		turn.keepInHistory = turn.reply != ""
		if request.finishVisual != nil {
			request.finishVisual(turn.reply)
		}
		if request.saveTurn != nil {
			request.saveTurn()
		}
	}
}

func (app *workspaceWindow) submitAIChat(prompt string) bool {
	s := &app.ai
	prompt = strings.TrimSpace(prompt)
	saved := app.preferences.AIConnection
	if prompt == "" || s.request != nil || saved.Model == "" {
		return false
	}
	owner := app.activeAIContext()
	previewHint := app.aiChatPreviewHint()
	hint := app.aiChatContextHint(previewHint)
	messages := s.chatMessages(owner, hint, prompt)
	app.clearAIContext()
	if app.workspaceTab != nil {
		s.rememberConversationDocument(app.document.Path)
	}
	s.appendQuickAction(prompt)
	s.prompt = ""
	turnIndex := len(s.submitted) - 1
	s.submitted[turnIndex].model = saved.Model
	s.submitted[turnIndex].contextHint = hint
	s.submitted[turnIndex].previewHint = previewHint
	s.submitted[turnIndex].contextOwner = owner
	ctx, cancel := context.WithCancel(context.Background())
	request := &workspaceAIRequest{cancel: cancel, turn: turnIndex, writeback: app.beginAIWriteback(),
		saveTurn: app.saveAIConversation}
	if target := app.beginAIVisualWriteback(); target != nil {
		request.visualWriteback = target
		request.finishVisual = func(reply string) {
			if request.visualWriteback != nil {
				request.visualWriteback = nil
				target.commit(app, reply)
			}
		}
	}
	s.request = request
	cfg, err := saved.config()
	if s.request != request {
		cancel()
		return true
	}
	if err != nil {
		s.submitted[turnIndex].problem = err.Error()
		s.stopResponse()
		return true
	}
	go func() {
		defer cancel()
		reply, err := aiworkspace.Chat(ctx, cfg, messages, func(delta string) {
			mygo.RunOnMain(func() {
				if s.request == request {
					request.reply.WriteString(delta)
					reply := request.reply.String()
					if request.writeback != nil && !request.writeback.apply(app, reply) {
						request.writeback = nil
					}
					if request.visualWriteback != nil && !request.visualWriteback.update(app, reply) {
						request.visualWriteback = nil
					}
					s.submitted[request.turn].reply = reply
					s.scroll.Y = math.MaxFloat32
					app.invalidateAIChat()
				}
			})
		})
		mygo.RunOnMain(func() {
			if s.request != request {
				return
			}
			turn := &s.submitted[turnIndex]
			turn.reply, turn.keepInHistory = reply, err == nil
			if request.finishVisual != nil {
				if err != nil {
					request.finishVisual("")
				} else {
					request.finishVisual(reply)
				}
			}
			if err != nil {
				turn.problem = err.Error()
			}
			s.request = nil
			request.saveTurn()
			s.scroll.Y = math.MaxFloat32
			app.invalidateAIChat()
		})
	}()
	return true
}

func (s *workspaceAI) chatMessages(owner *workspaceAIContext, hint, prompt string) []aiworkspace.Message {
	messages := make([]aiworkspace.Message, 0, 3*len(s.submitted)+3)
	messages = append(messages, aiworkspace.Message{Role: "system", Content: workspaceAIWritingInstructions})
	for _, turn := range s.submitted {
		if !turn.keepInHistory {
			continue
		}
		// History remains with the window. Include a previous document hint
		// only when it belongs to the document composing this request.
		previousHint := turn.previewHint
		if owner != nil && turn.contextOwner == owner {
			previousHint = turn.contextHint
		}
		if previousHint != "" {
			messages = append(messages, aiworkspace.Message{Role: "system", Content: previousHint})
		}
		messages = append(messages, aiworkspace.Message{Role: "user", Content: turn.prompt},
			aiworkspace.Message{Role: "assistant", Content: turn.reply})
	}
	return append(messages, aiworkspace.Message{Role: "system", Content: hint},
		aiworkspace.Message{Role: "user", Content: prompt})
}

const workspaceAIWritingInstructions = "You are the writing assistant in object-N/A, a Markdown editor. " +
	"Help the user draft, revise, summarize, and explain text in the format they request. " +
	"Use Markdown for structured prose, tables for tabular data, and Mermaid code fences when asked for diagrams. " +
	"Keep any HTML suitable for embedding in the document preview."

// EV-UI-242: Attach snapshots the selection or, with a caret, the current note.
func (app *workspaceWindow) attachAIContext() {
	owner := app.activeAIContext()
	if owner == nil {
		return
	}
	var source string
	switch {
	case app.sourceEditing():
		source = app.aiSourceSelection()
	case app.displayedMode() == wysiwygMode && app.wys.editable(app):
		source = app.wys.session.SelectionMarkdown()
	default:
		return
	}
	if source == "" {
		source = app.workingText()
	}
	var segments segmenter.Segmenter
	segments.InitWithString(source)
	characters := 0
	for graphemes := segments.GraphemeIterator(); graphemes.Next(); {
		characters++
	}
	owner.snapshot, owner.characters = source, characters
}

func (app *workspaceWindow) clearAIContext() {
	if tab := app.workspaceTab; tab != nil && tab.aiContext != nil {
		tab.aiContext.snapshot, tab.aiContext.characters = "", 0
	}
	app.closeAIContextReader()
}

func (app *workspaceWindow) aiSourceSelection() string {
	if app.workspaceTab == nil || app.draft == nil || !app.sourceEditing() {
		return ""
	}
	selection := app.code.Selection()
	return app.draft.Slice(selection.Start(), selection.End())
}

func (app *workspaceWindow) aiChatPreviewHint() string {
	theme := app.resolvePreviewTheme(app.previewThemeSelection())
	contrast := "dark text and lines against a light background"
	if theme.dark() {
		contrast = "light text and lines against a dark background"
	}
	return "The current document preview uses background " + theme.BackgroundColor + ". Prefer " +
		contrast + ". Keep generated HTML readable when the preview theme changes. Let the editor theme Mermaid diagrams."
}

// EV-UI-231 / EV-UI-242: each turn has a preview hint and one optional note context.
func (app *workspaceWindow) aiChatContextHint(hint string) string {
	owner := app.activeAIContext()
	var source string
	if owner != nil {
		source = owner.snapshot
	}
	if source == "" {
		source = app.aiSourceSelection()
	}
	if source != "" {
		hint += "\n\nUse the following Markdown as source material:\n" + source
	}
	if owner != nil && owner.hasNotes() {
		hint += "\n\nAdditional notes for this conversation:\n" + owner.notes
	}
	return hint
}

func (app *workspaceWindow) invalidateAIChat() {
	if app.win != nil && !app.win.IsDestroyed() {
		app.win.Invalidate()
	}
}
