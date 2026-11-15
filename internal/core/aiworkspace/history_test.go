// spec: EV-UI-250 / EV-UI-242 / EV-UI-268 / EV-UI-269, AI conversation history.
package aiworkspace

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestHistoryOrdersAndRestoresConversations(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "profile", "ai-conversations.json")
	store, err := LoadHistory(filename)
	if err != nil || len(store.Snapshot()) != 0 {
		t.Fatalf("fresh history: %v", err)
	}
	at := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	first := Conversation{ID: "first", Title: "First conversation", Model: "fixture", UpdatedAt: at}
	newest := Conversation{ID: "newest", Title: "New conversation", Model: "fixture", UpdatedAt: at.Add(time.Minute),
		Turns: []ConversationTurn{{Prompt: "开篇", Model: "fixture", Reply: "部分回答", KeepInHistory: true}}}
	for _, conversation := range []Conversation{newest, first} {
		if err := store.Upsert(conversation); err != nil {
			t.Fatal(err)
		}
	}
	opened, err := LoadHistory(filename)
	if err != nil {
		t.Fatal(err)
	}
	if got := opened.Snapshot(); !reflect.DeepEqual(got, []Conversation{newest, first}) {
		t.Fatalf("history loaded out of order: %+v", got)
	}

	first.UpdatedAt = at.Add(2 * time.Minute)
	first.Turns = []ConversationTurn{{Prompt: "retry me", Model: "fixture", Problem: "Connection failed"}}
	if err := opened.Upsert(first); err != nil {
		t.Fatal(err)
	}
	first.Turns[0].Prompt = "caller mutation"
	snapshot := opened.Snapshot()
	snapshot[0].Turns[0].Problem = "reader mutation"
	want := ConversationTurn{Prompt: "retry me", Model: "fixture", Problem: "Connection failed"}
	if got := opened.Snapshot(); len(got) != 2 || got[0].ID != first.ID || !slices.Equal(got[0].Turns, []ConversationTurn{want}) {
		t.Fatalf("updated history changed aliases or duplicated a conversation: %+v", got)
	}
	reopened, err := LoadHistory(filename)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Snapshot(); !reflect.DeepEqual(got, opened.Snapshot()) {
		t.Fatalf("saved update differs from the in-process history: %+v", got)
	}
}

func TestHistoryDeleteRejectsAStaleContinuation(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "ai-conversations.json")
	store, err := LoadHistory(filename)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	keep := Conversation{ID: "survivor", Title: "Keep", UpdatedAt: at, Turns: []ConversationTurn{{Prompt: "Other chat"}}}
	for _, record := range []Conversation{keep, {ID: "delete", Title: "Delete this", UpdatedAt: at}} {
		if err := store.Upsert(record); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Delete("delete"); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendTurns("delete", []ConversationTurn{{Prompt: "Late result"}}, at.Add(time.Minute)); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("continuation of removed conversation: %v", err)
	}
	opened, err := LoadHistory(filename)
	if err != nil {
		t.Fatal(err)
	}
	if got := opened.Snapshot(); !reflect.DeepEqual(got, []Conversation{keep}) {
		t.Fatalf("deleting a record or rejecting old turns changed the other record: %+v", got)
	}
}

func TestHistoryFailedUpsertRetainsOriginal(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "ai-conversations.json")
	store, err := LoadHistory(filename)
	if err != nil {
		t.Fatal(err)
	}
	saved := Conversation{ID: "original", Title: "Keep this", UpdatedAt: time.Now().UTC()}
	if err := store.Upsert(saved); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	broken := saved
	broken.Title = "Uncommitted update"
	broken.UpdatedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := store.Upsert(broken); err == nil {
		t.Fatal("an unencodable timestamp was unexpectedly committed")
	}
	if err := store.SetPinned(saved.ID, true, broken.UpdatedAt); err == nil {
		t.Fatal("pinning with an unencodable timestamp unexpectedly succeeded")
	}
	after, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("failed upsert changed the history file: %v", err)
	}
	if got := store.Snapshot(); !reflect.DeepEqual(got, []Conversation{saved}) {
		t.Fatalf("failed upsert changed the live list: %+v", got)
	}
}

func TestHistoryPinAndAppendPreserveLatestRecord(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "ai-conversations.json")
	store, err := LoadHistory(filename)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	first := ConversationTurn{Prompt: "first prompt", Reply: "first reply", KeepInHistory: true}
	continuation := ConversationTurn{Prompt: "follow-up", Reply: "new reply", KeepInHistory: true}
	for _, conversation := range []Conversation{
		{ID: "older", Title: "Older conversation", UpdatedAt: at, Turns: []ConversationTurn{first}},
		{ID: "newer", Title: "Newer conversation", UpdatedAt: at.Add(time.Minute)},
	} {
		if err := store.Upsert(conversation); err != nil {
			t.Fatal(err)
		}
	}
	pinnedAt := at.Add(2 * time.Minute)
	if err := store.SetPinned("older", true, pinnedAt); err != nil {
		t.Fatal(err)
	}
	if pinned := store.Snapshot()[0]; pinned.ID != "older" || !pinned.Pinned || !pinned.UpdatedAt.Equal(pinnedAt) {
		t.Fatalf("pin did not update the flag and visible time: %+v", pinned)
	}
	if err := store.AppendTurns("older", []ConversationTurn{continuation}, at.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	opened, err := LoadHistory(filename)
	if err != nil {
		t.Fatal(err)
	}
	if pinned := opened.Snapshot()[0]; !pinned.Pinned || !slices.Equal(pinned.Turns, []ConversationTurn{first, continuation}) {
		t.Fatalf("appending a turn lost the pin or transcript: %+v", pinned)
	}
	if err := opened.SetPinned("newer", true, at.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if both := opened.Snapshot(); both[0].ID != "newer" || both[1].ID != "older" {
		t.Fatalf("two pinned records were not sorted by update time: %+v", both)
	}
	unpinnedAt := at.Add(5 * time.Minute)
	if err := opened.SetPinned("newer", false, unpinnedAt); err != nil {
		t.Fatal(err)
	}
	if final := opened.Snapshot(); final[0].ID != "older" || final[1].ID != "newer" || final[1].Pinned ||
		!final[1].UpdatedAt.Equal(unpinnedAt) {
		t.Fatalf("the newest unpinned record did not follow the pinned group: %+v", final)
	}
}
