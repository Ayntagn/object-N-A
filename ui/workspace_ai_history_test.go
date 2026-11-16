// spec: EV-UI-250 / EV-UI-257 / EV-UI-268 / EV-UI-270, AI history and per-reply statistics.
package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"cyberwriter/internal/core/aiworkspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestAIHistoryRestoresReplyMetrics(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ai-conversations.json")
	const saved = `[{"id":"timed","title":"Fixture","model":"fixture","updatedAt":"2026-11-15T12:00:00Z","turns":[
		{"prompt":"Old history","model":"fixture","reply":"Old reply","keepInHistory":true},
		{"prompt":"New history","model":"fixture","reply":"Timed reply","keepInHistory":true,
		 "metrics":{"timeToFirstTokenNs":500000000,"tokensPerSecond":2}}]}]`
	if err := os.WriteFile(file, []byte(saved), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := aiworkspace.LoadHistory(file)
	if err != nil {
		t.Fatal(err)
	}
	app := &workspaceWindow{}
	app.selectAIConversation(store, "timed")
	for _, raw := range []bool{false, true} {
		view := mygoui.NewTester(func(c *mygoui.Context) {
			app.ai.drawAIConversation(c, aiWorkspaceColors(c, true), nil, aiComposerOptions{rawText: raw})
		}, vaultPanelWidth, 600)
		rate, rateFound := view.Access("2.0 tok/s")
		delay, delayFound := view.Access("0.50s")
		if !rateFound || !delayFound || rate.Bounds.Y != delay.Bounds.Y {
			t.Fatalf("raw=%t, missing reply metrics on the same row: rate=%+v, delay=%+v", raw, rate, delay)
		}
		if _, found := view.Access("0.0 tok/s"); found {
			t.Fatal("history with no measurement displayed a rate")
		}
	}
}

func TestAIHistoryContinuesAcrossWindowsAndReload(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CYBERWRITER_CONFIG_DIR", config)
	application := &workspaceApplication{}
	alpha, beta := workspaceWindow{application: application}, workspaceWindow{application: application}
	application.windows = []*workspaceWindow{&alpha, &beta}
	makeTurn := func(prompt string) workspaceAITurn {
		return workspaceAITurn{
			prompt: prompt, model: "fixture", reply: "reply to " + prompt,
			contextHint: "window-only attachment", previewHint: "preview theme", keepInHistory: true,
		}
	}

	alpha.ai.rememberConversationDocument("notes/Project.md")
	alpha.ai.submitted = []workspaceAITurn{makeTurn("alpha")}
	metrics := aiworkspace.ReplyMetrics{TimeToFirstToken: 180 * time.Millisecond, TokensPerSecond: 1.2}
	alpha.ai.submitted[0].metrics = metrics
	alpha.saveAIConversation()
	beta.selectAIConversation(application.aiHistory, alpha.ai.conversationID)
	if len(beta.ai.submitted) != 1 || beta.ai.submitted[0].reply != "reply to alpha" ||
		beta.ai.submitted[0].contextHint != "preview theme" || !beta.ai.submitted[0].keepInHistory ||
		beta.ai.submitted[0].metrics != metrics {
		t.Fatalf("history selection loaded %+v", beta.ai.submitted)
	}
	alpha.ai.submitted = append(alpha.ai.submitted, makeTurn("alpha continuation"))
	alpha.saveAIConversation()
	beta.ai.submitted = append(beta.ai.submitted, makeTurn("beta continuation"))
	beta.saveAIConversation()
	if len(alpha.ai.submitted) != 2 || alpha.ai.submitted[1].prompt != "alpha continuation" {
		t.Fatalf("another window changed the current transcript: %+v", alpha.ai.submitted)
	}

	beta.workspaceTab = &workspaceTab{aiContext: &workspaceAIContext{notes: "keep on this document"}}
	beta.ai.newConversation()
	beta.ai.submitted = []workspaceAITurn{makeTurn("new beta conversation")}
	beta.saveAIConversation()
	if beta.aiContext.notes != "keep on this document" || beta.ai.conversationID == alpha.ai.conversationID {
		t.Fatalf("New Chat reused the old ID or cleared the document notes")
	}

	store, err := aiworkspace.LoadHistory(filepath.Join(config, "ai-conversations.json"))
	if err != nil {
		t.Fatal(err)
	}
	records := store.Snapshot()
	if len(records) != 2 || records[0].Title != "new beta conversation" ||
		records[1].Title != "alpha" || records[1].Document != "Project" ||
		len(records[1].Turns) != 3 || records[1].Turns[0].Metrics != metrics || records[1].Turns[1].Prompt != "alpha continuation" ||
		records[1].Turns[2].Prompt != "beta continuation" {
		t.Fatalf("reopened history lost or reordered turns: %+v", records)
	}
}

func TestAIHistoryRowMenuActions(t *testing.T) {
	store, err := aiworkspace.LoadHistory(filepath.Join(t.TempDir(), "ai-conversations.json"))
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"older", "newer"} {
		record := aiworkspace.Conversation{
			ID: id, Title: id, UpdatedAt: time.Date(2026, 11, 15, 12, i, 0, 0, time.UTC),
			Turns: []aiworkspace.ConversationTurn{{Prompt: id + " question", Reply: id + " reply"}},
		}
		if err := store.Upsert(record); err != nil {
			t.Fatal(err)
		}
	}
	app := &workspaceWindow{}
	app.selectAIConversation(store, "newer")
	closed := false
	view := mygoui.NewTester(func(c *mygoui.Context) {
		app.drawAIHistory(c, store, func() { closed = true })
	}, aiHistoryWidth, aiHistoryHeight)
	openRowMenu := func(id, firstItem string) {
		t.Helper()
		for _, conversation := range store.Snapshot() {
			if conversation.ID != id {
				continue
			}
			label := conversation.Title + ", " + conversation.UpdatedAt.Local().Format("15:04")
			if err := view.RightClick(label); err != nil {
				t.Fatal(err)
			}
			if got := view.Menu(); !slices.Equal(got, []string{firstItem, "-", "Delete"}) {
				t.Fatalf("history row menu = %q", got)
			}
			return
		}
		t.Fatalf("missing history row %q", id)
	}
	for _, action := range []string{"Pin", "Unpin"} {
		openRowMenu("older", action)
		if err := view.ChooseMenuItem(action); err != nil {
			t.Fatal(err)
		}
		if closed || app.ai.conversationID != "newer" {
			t.Fatal("pin action closed history or selected a different conversation")
		}
	}
	openRowMenu("older", "Pin")
	if err := view.ChooseMenuItem("Delete"); err != nil {
		t.Fatal(err)
	}
	if rows := store.Snapshot(); len(rows) != 1 || rows[0].ID != "newer" || app.ai.conversationID != "newer" || closed {
		t.Fatalf("deleting an inactive conversation changed the selection or closed the popup: %+v", rows)
	}
	record := store.Snapshot()[0]
	label := record.Title + ", " + record.UpdatedAt.Local().Format("15:04")
	row, found := view.AccessByRole(mygoui.RoleButton, label)
	if !found {
		t.Fatal("active history row is missing")
	}
	x, y := row.Bounds.X+row.Bounds.W-18, row.Bounds.Y+row.Bounds.H/2
	view.Move(x, y)
	// The hover control lives inside the leaf AX row, as in the original.
	if _, found := view.Find("Delete conversation"); !found {
		t.Fatal("hover did not expose the row's Delete conversation action")
	}
	cancelled, lateSave := false, false
	app.ai.request = &workspaceAIRequest{
		cancel: func() { cancelled = true }, saveTurn: func() { lateSave = true },
	}
	view.ClickAt(x, y)
	if len(store.Snapshot()) != 0 || app.ai.conversationID != "" || len(app.ai.submitted) != 0 || closed || !cancelled || lateSave {
		t.Fatal("deleting the active conversation failed to clear its transcript, or affected the history popup")
	}
}
