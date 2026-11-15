package aiworkspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Conversation is the locally stored transcript of one chat.
type Conversation struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Model     string             `json:"model"`
	Document  string             `json:"document,omitempty"`
	Pinned    bool               `json:"pinned,omitempty"`
	UpdatedAt time.Time          `json:"updatedAt"`
	Turns     []ConversationTurn `json:"turns"`
}

// ConversationTurn contains the visible turn and its eligibility for follow-up requests.
type ConversationTurn struct {
	Prompt        string `json:"prompt"`
	Model         string `json:"model"`
	Reply         string `json:"reply,omitempty"`
	Problem       string `json:"problem,omitempty"`
	PreviewHint   string `json:"previewHint,omitempty"`
	KeepInHistory bool   `json:"keepInHistory,omitempty"`
}

// History stores transcripts for one configuration. Windows share one History.
type History struct {
	mu            sync.Mutex
	filename      string
	conversations []Conversation
}

// ErrConversationNotFound identifies an update to a conversation that no longer exists.
var ErrConversationNotFound = errors.New("AI conversation not found")

// LoadHistory reads existing conversations. A missing file is an empty history.
func LoadHistory(filename string) (*History, error) {
	if filename == "" {
		return nil, errors.New("missing AI conversation history path")
	}
	history := &History{filename: filename}
	data, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return history, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &history.conversations); err != nil {
		return nil, errors.New("invalid AI conversation history")
	}
	seen := make(map[string]struct{}, len(history.conversations))
	for _, conversation := range history.conversations {
		if err := validateConversation(conversation); err != nil {
			return nil, err
		}
		if _, exists := seen[conversation.ID]; exists {
			return nil, errors.New("duplicate AI conversation ID")
		}
		seen[conversation.ID] = struct{}{}
	}
	sortConversations(history.conversations)
	return history, nil
}

// Snapshot returns detached transcripts, with pinned records first and each
// group newest to oldest.
func (h *History) Snapshot() []Conversation {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := slices.Clone(h.conversations)
	for i := range result {
		result[i].Turns = slices.Clone(result[i].Turns)
	}
	return result
}

// Upsert replaces a transcript with the same ID or adds a new one.
// A failed save leaves both the last file and the published snapshot intact.
func (h *History) Upsert(conversation Conversation) error {
	if err := validateConversation(conversation); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	conversation.Turns = slices.Clone(conversation.Turns)
	next := slices.Clone(h.conversations)
	index := conversationIndex(next, conversation.ID)
	if index < 0 {
		next = append(next, conversation)
	} else {
		next[index] = conversation
	}
	return h.commit(next)
}

// AppendTurns updates the latest saved record, preserving its current pin state.
func (h *History) AppendTurns(id string, turns []ConversationTurn, at time.Time) error {
	return h.update(id, at, func(record *Conversation) {
		record.Turns = append(slices.Clone(record.Turns), turns...)
	})
}

// SetPinned changes the flag and the displayed update time in one commit.
func (h *History) SetPinned(id string, pinned bool, at time.Time) error {
	return h.update(id, at, func(record *Conversation) { record.Pinned = pinned })
}

func (h *History) update(id string, at time.Time, change func(*Conversation)) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	index := conversationIndex(h.conversations, id)
	if index < 0 {
		return ErrConversationNotFound
	}
	record := h.conversations[index]
	record.UpdatedAt = at
	if err := validateConversation(record); err != nil {
		return err
	}
	change(&record)
	next := slices.Clone(h.conversations)
	next[index] = record
	return h.commit(next)
}

// Delete removes a saved transcript in a single commit.
func (h *History) Delete(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	index := conversationIndex(h.conversations, id)
	if index < 0 {
		return ErrConversationNotFound
	}
	next := slices.Clone(h.conversations)
	return h.commit(slices.Delete(next, index, index+1))
}

func conversationIndex(conversations []Conversation, id string) int {
	return slices.IndexFunc(conversations, func(conversation Conversation) bool { return conversation.ID == id })
}

// Call with the history locked. Readers keep seeing the old list if the save fails.
func (h *History) commit(next []Conversation) error {
	sortConversations(next)
	if err := writeConversations(h.filename, next); err != nil {
		return err
	}
	h.conversations = next
	return nil
}

func validateConversation(conversation Conversation) error {
	if conversation.ID == "" || conversation.UpdatedAt.IsZero() {
		return errors.New("AI conversation requires an ID and update time")
	}
	return nil
}

func sortConversations(conversations []Conversation) {
	slices.SortStableFunc(conversations, func(a, b Conversation) int {
		if a.Pinned != b.Pinned {
			if a.Pinned {
				return -1
			}
			return 1
		}
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
}

func writeConversations(filename string, conversations []Conversation) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	pending, err := os.CreateTemp(dir, ".ai-conversations-*")
	if err != nil {
		return err
	}
	defer os.Remove(pending.Name())
	writeErr := json.NewEncoder(pending).Encode(conversations)
	if writeErr == nil {
		writeErr = pending.Sync()
	}
	if err := errors.Join(writeErr, pending.Close()); err != nil {
		return err
	}
	return os.Rename(pending.Name(), filename)
}
