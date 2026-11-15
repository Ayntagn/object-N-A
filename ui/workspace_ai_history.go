package ui

import (
	"crypto/rand"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"cyberwriter/internal/core/aiworkspace"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

func (app *workspaceWindow) loadAIHistory() (*aiworkspace.History, error) {
	if app.application.aiHistory == nil {
		settings, err := preferenceFile()
		if err != nil {
			return nil, err
		}
		store, err := aiworkspace.LoadHistory(filepath.Join(filepath.Dir(settings), "ai-conversations.json"))
		if err != nil {
			return nil, err
		}
		app.application.aiHistory = store
	}
	return app.application.aiHistory, nil
}

// EV-UI-245/250. The transcript belongs to the window and the list to the app.
func (s *workspaceAI) newConversation() {
	s.stopResponse()
	s.prompt = ""
	s.submitted = nil
	s.conversationID = ""
	s.conversationDocument = ""
	s.savedTurns = 0
	s.scroll.Y = math.MaxFloat32
}

func (s *workspaceAI) rememberConversationDocument(path string) {
	if path == "" || s.conversationID != "" || s.conversationDocument != "" {
		return
	}
	for _, turn := range s.submitted {
		if turn.model != "" {
			return
		}
	}
	name := filepath.Base(path)
	s.conversationDocument = strings.TrimSuffix(name, filepath.Ext(name))
}

func (app *workspaceWindow) saveAIConversation() {
	if app.application == nil {
		return
	}
	s := &app.ai
	var additions []aiworkspace.ConversationTurn
	for _, turn := range s.submitted[s.savedTurns:] {
		if turn.model != "" {
			additions = append(additions, aiworkspace.ConversationTurn{
				Prompt: turn.prompt, Model: turn.model, Reply: turn.reply, Problem: turn.problem,
				PreviewHint: turn.previewHint, KeepInHistory: turn.keepInHistory,
			})
		}
	}
	if len(additions) == 0 {
		return
	}
	store, err := app.loadAIHistory()
	if err != nil {
		app.problem = fmt.Sprintf("Could not load AI conversations. %v", err)
		return
	}
	now := time.Now()
	if s.conversationID == "" {
		record := aiworkspace.Conversation{
			ID: rand.Text(), Title: additions[0].Prompt, Model: additions[0].Model, Document: s.conversationDocument,
			UpdatedAt: now, Turns: additions,
		}
		if err = store.Upsert(record); err == nil {
			s.conversationID = record.ID
		}
	} else {
		err = store.AppendTurns(s.conversationID, additions, now)
	}
	if err != nil {
		app.problem = fmt.Sprintf("Could not save AI conversation. %v", err)
		return
	}
	s.savedTurns = len(s.submitted)
	app.invalidateAIHistory()
}

func (app *workspaceWindow) setAIConversationPinned(store *aiworkspace.History, id string, pinned bool) {
	if err := store.SetPinned(id, pinned, time.Now()); err != nil {
		app.problem = fmt.Sprintf("Could not update AI conversation. %v", err)
		app.invalidateAIChat()
		return
	}
	app.invalidateAIHistory()
}

func (app *workspaceWindow) deleteAIConversation(store *aiworkspace.History, id string) {
	if err := store.Delete(id); err != nil {
		app.problem = fmt.Sprintf("Could not delete AI conversation. %v", err)
		app.invalidateAIChat()
		return
	}
	for _, window := range app.documentWindows() {
		if window.ai.conversationID != id {
			continue
		}
		if request := window.ai.request; request != nil {
			// Stopping a request normally saves its partial reply. The
			// conversation was just deleted, so discard that history write.
			request.saveTurn = nil
		}
		window.ai.newConversation()
		window.invalidateAIChat()
	}
	app.invalidateAIHistory()
}

func (app *workspaceWindow) invalidateAIHistory() {
	for _, window := range app.documentWindows() {
		if flyout := window.ai.historyFlyout; flyout != nil && !flyout.IsDestroyed() {
			flyout.Invalidate()
		}
	}
}

func (app *workspaceWindow) selectAIConversation(store *aiworkspace.History, id string) {
	s := &app.ai
	if s.conversationID == id {
		return
	}
	for _, record := range store.Snapshot() {
		if record.ID != id {
			continue
		}
		s.newConversation()
		for _, turn := range record.Turns {
			s.submitted = append(s.submitted, workspaceAITurn{
				prompt: turn.Prompt, model: turn.Model, reply: turn.Reply, problem: turn.Problem,
				contextHint: turn.PreviewHint, previewHint: turn.PreviewHint, keepInHistory: turn.KeepInHistory,
			})
		}
		s.conversationID, s.savedTurns = id, len(s.submitted)
		app.invalidateAIChat()
		return
	}
}

func (app *workspaceWindow) showAIConversationHistory(anchor mygoui.Rect) {
	if app.application == nil || app.win == nil || app.win.IsDestroyed() {
		return
	}
	s := &app.ai
	if s.historyFlyout != nil {
		s.historyFlyout.Close()
		return
	}
	store, err := app.loadAIHistory()
	if err != nil {
		app.problem = fmt.Sprintf("Could not open AI conversation history. %v", err)
		app.invalidateAIChat()
		return
	}
	var flyout *mygo.Window
	flyout = mygo.NewFlyout(mygo.FlyoutOptions{
		Parent: app.win, Anchor: documentTitleAnchor(anchor),
		Placement: mygo.PlacementBottom, Width: aiHistoryWidth, Height: aiHistoryHeight,
		Popover: true, Focusable: true, Shadow: true,
		Content: mygoui.View(func(c *mygoui.Context) {
			app.applyAppearance(c)
			c.Root().Background(mygoui.Transparent)
			app.drawAIHistory(c, store, func() { flyout.Close() })
		}),
	})
	if flyout == nil {
		return
	}
	s.historyFlyout = flyout
	flyout.OnClosed(func() {
		if s.historyFlyout == flyout {
			s.historyFlyout = nil
		}
	})
}

func (app *workspaceWindow) drawAIHistory(c *mygoui.Context, store *aiworkspace.History, closePopup func()) {
	s := &app.ai
	records := store.Snapshot()
	rows := make([]workspaceAIHistoryRow, len(records))
	for i, record := range records {
		rows[i] = workspaceAIHistoryRow{
			ID: record.ID, Title: record.Title, Model: record.Model, UpdatedAt: record.UpdatedAt,
			Active: record.ID == s.conversationID, Document: record.Document, Pinned: record.Pinned,
		}
	}
	s.drawConversationHistory(c, app.blackout, rows, aiHistoryActions{
		selectRow: func(id string) {
			app.selectAIConversation(store, id)
			closePopup()
		},
		setPinned: func(id string, pinned bool) { app.setAIConversationPinned(store, id, pinned) },
		deleteRow: func(id string) { app.deleteAIConversation(store, id) },
		newChat: func() {
			s.newConversation()
			closePopup()
			app.invalidateAIChat()
		},
	})
}
