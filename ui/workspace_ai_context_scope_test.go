// spec: EV-UI-245 / EV-UI-267 / EV-MYGO-169, context belongs to the active document.
package ui

import (
	"strconv"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestAIContextFollowsActiveDocument(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	const snapshot, notes = "Document A attached body\n", "Document A notes BETA"
	app, view, _ := snippetsWorkspace(t, "A.md", snapshot, splitMode)
	app.openAIWorkspace()
	view.Frame()
	if err := view.Click("Attach current note"); err != nil {
		t.Fatal(err)
	}
	tabA := app.workspaceTab
	contextA := tabA.aiContext
	contextA.notes = notes
	withNotes := "+Context " + strconv.Itoa(contextA.characters) + " +notes"
	view.Frame()
	if _, found := view.AccessByRole(mygoui.RoleButton, withNotes); !found || contextA.snapshot != snapshot {
		t.Fatalf("document A attachment %q, chip %q present=%v", contextA.snapshot, withNotes, found)
	}
	app.ai.submitted = []workspaceAITurn{{
		prompt: "previous question", reply: "previous reply", keepInHistory: true,
		previewHint: "preview A", contextHint: app.aiChatContextHint("preview A"), contextOwner: contextA,
	}}

	tabB := app.blankTab()
	app.setTabDocument(tabB, workspace.Document{Path: "B.md", Text: "Document B\n"})
	app.tabs = append(app.tabs, tabB)
	app.activateTab(tabB)
	view.Frame()
	for _, label := range []string{withNotes, "+Notes", "Remove context"} {
		if _, found := view.AccessByRole(mygoui.RoleButton, label); found {
			t.Errorf("document B exposed document A's %q control", label)
		}
	}
	messages := app.ai.chatMessages(app.activeAIContext(), app.aiChatContextHint("preview B"), "current question")
	if len(messages) != 6 || messages[1].Content != "preview A" || messages[3].Content != "previous reply" {
		t.Fatalf("document B did not preserve the window conversation: %+v", messages)
	}
	for _, message := range messages {
		if strings.Contains(message.Content, notes) || strings.Contains(message.Content, snapshot) {
			t.Errorf("document B's request carried context from A: %q", message.Content)
		}
	}

	// Detach passes the same tab to the new window, with a new conversation.
	app.tabs = app.tabs[:1]
	app.activateTab(tabA)
	detached := workspaceWindow{workspaceTab: tabB, tabs: []*workspaceTab{tabB}, preferences: app.preferences}
	if hint := detached.aiChatContextHint("preview B"); hint != "preview B" {
		t.Fatalf("detached B context = %q", hint)
	}
	view.Frame()
	if _, found := view.AccessByRole(mygoui.RoleButton, withNotes); !found || app.activeAIContext() != contextA {
		t.Fatal("returning to A did not restore its attachment and Notes")
	}
	if err := view.Click("Remove context"); err != nil {
		t.Fatal(err)
	}
	if err := view.Click("New conversation"); err != nil {
		t.Fatal(err)
	}
	if _, found := view.AccessByRole(mygoui.RoleButton, "+Notes"); !found || contextA.snapshot != "" ||
		!strings.Contains(app.aiChatContextHint("preview A"), notes) || len(app.ai.submitted) != 0 {
		t.Fatal("removing the attachment and starting a new conversation did not preserve A's Notes")
	}
	app.setTabDocument(tabA, workspace.Document{Path: "Replacement.md", Text: "New document\n"})
	app.activateTab(tabA)
	view.Frame()
	if _, found := view.AccessByRole(mygoui.RoleButton, "+Notes"); found || app.aiChatContextHint("preview") != "preview" {
		t.Fatal("replacing the document in the same tab retained the old Notes")
	}
}
