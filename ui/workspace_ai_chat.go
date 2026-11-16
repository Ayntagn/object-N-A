package ui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

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
	metrics                    aiworkspace.ReplyMetrics
}

type workspaceAIRequest struct {
	cancel          context.CancelFunc
	turn            int
	reply           strings.Builder
	progress        aiworkspace.StreamProgress
	writeback       *workspaceAIWriteback
	visualWriteback *workspaceAIVisualWriteback
	finishVisual    func(string)
	saveTurn        func()
}

func (s *workspaceAI) stopResponse() {
	if request := s.request; request != nil {
		stoppedAt := time.Now()
		request.cancel()
		s.request = nil
		// EV-UI-242: a stopped turn's received text remains in the next request.
		turn := &s.submitted[request.turn]
		turn.keepInHistory = turn.reply != ""
		turn.metrics = request.progress.MetricsAt(stoppedAt)
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
	if prompt == "" || s.request != nil || s.referenceLoad != nil || saved.Model == "" {
		return false
	}
	owner := app.activeAIContext()
	previewHint := app.aiChatPreviewHint()
	hint := app.aiChatContextHint(previewHint)
	requestHint := hint
	if len(s.references) > 0 {
		requestHint = app.aiChatContextHint(previewHint, s.references...)
	}
	messages := app.chatMessages(owner, requestHint, prompt)
	vaultQuery := app.aiVaultContextQuery(saved)
	s.clearAIReferences()
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
		deliverDelta := func(delta string, progress aiworkspace.StreamProgress) {
			mygo.RunOnMain(func() {
				if s.request == request {
					request.reply.WriteString(delta)
					request.progress = progress
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
		}
		var reply aiworkspace.ChatReply
		err := vaultQuery.appendToMessages(ctx, prompt, previewHint, messages)
		if err == nil {
			reply, err = aiworkspace.Chat(ctx, cfg, messages, deliverDelta)
		}
		mygo.RunOnMain(func() {
			if s.request != request {
				return
			}
			turn := &s.submitted[turnIndex]
			turn.reply, turn.metrics, turn.keepInHistory = reply.Text, reply.Metrics, err == nil
			if request.finishVisual != nil {
				if err != nil {
					request.finishVisual("")
				} else {
					request.finishVisual(reply.Text)
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

func (app *workspaceWindow) chatMessages(owner *workspaceAIContext, hint, prompt string) []aiworkspace.Message {
	messages := make([]aiworkspace.Message, 0, 3*len(app.ai.submitted)+3)
	messages = append(messages, aiworkspace.Message{Role: "system", Content: app.preferences.aiSystemPrompt()})
	for _, turn := range app.ai.submitted {
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
	messages = append(messages, aiworkspace.Message{Role: "system", Content: hint},
		aiworkspace.Message{Role: "user", Content: prompt})
	// EV-UI-282. The current turn counts toward the limit. The service's default
	// system instructions precede the retained messages and are counted separately.
	if limit := app.preferences.contextMessageLimit(); len(messages)-1 > limit {
		messages = append(messages[:1], messages[len(messages)-limit:]...)
	}
	return messages
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
	owner.characterLimit = app.preferences.documentContextLimit()
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

// EV-UI-231 / EV-UI-242 / EV-UI-277. References belong to this request's context.
func (app *workspaceWindow) aiChatContextHint(previewHint string, references ...workspaceAIReference) string {
	owner := app.activeAIContext()
	var source string
	if owner != nil {
		source = owner.snapshot
	}
	if source == "" {
		source = app.aiSourceSelection()
	}
	parts := make([]string, 0, len(references)+3)
	if source != "" {
		parts = append(parts, "Use the following Markdown as source material:\n"+source)
	}
	for _, reference := range references {
		parts = append(parts, fmt.Sprintf("Attached file \"%s\":\n\n%s", reference.name, reference.content))
	}
	if owner != nil && owner.hasNotes() {
		parts = append(parts, "Additional notes for this conversation:\n"+owner.notes)
	}
	parts = append(parts, previewHint)
	return strings.Join(parts, "\n\n---\n\n")
}

func (app *workspaceWindow) invalidateAIChat() {
	if app.win != nil && !app.win.IsDestroyed() {
		app.win.Invalidate()
	}
}
