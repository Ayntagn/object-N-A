// spec: EV-UI-218 / EV-UI-224 / EV-UI-227
package ui

import (
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestQuickActionsPromptReturn(t *testing.T) {
	app := workspaceWindow{quickActions: workspaceQuickActions{open: true, focusPrompt: true}}
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		app.quickActions.draw(c, true, app.runQuickAction, func(quickActionDestination) {}, app.dismissQuickActions)
	}, 640, 640)
	if !tester.Focused("Custom instruction for AI...") {
		t.Fatal("Quick Actions prompt did not take focus")
	}

	tester.Key(0, mygoui.KeyEnter)
	if !app.quickActions.open || app.quickActions.prompt != "" {
		t.Fatalf("empty Return left panel open=%t with prompt %q", app.quickActions.open, app.quickActions.prompt)
	}

	const instruction = "Use a shorter title"
	tester.Type(instruction)
	if app.quickActions.prompt != instruction {
		t.Fatalf("typed prompt = %q, want %q", app.quickActions.prompt, instruction)
	}
	tester.Key(0, mygoui.KeyEnter)
	if app.quickActions.open {
		t.Fatalf("Return kept Quick Actions open with prompt %q", app.quickActions.prompt)
	}
}

func TestQuickActionsPanelUsesPreviousSubmissionDestination(t *testing.T) {
	app := workspaceWindow{vault: &workspace.Vault{}, vaultVisible: true, workspaceTab: &workspaceTab{}}
	for _, step := range []struct {
		name string
		next quickActionDestination
		want workspaceSidebarTab
	}{
		{"first Chat", quickActionChat, workspaceFilesTab},
		{"second Chat", quickActionChat, workspaceAITab},
		{"first Insert", quickActionInsert, workspaceAITab},
		{"second Insert", quickActionInsert, workspaceFilesTab},
	} {
		app.openFilesSidebar()
		app.quickActions = workspaceQuickActions{open: true, destination: step.next}
		app.runQuickAction(quickActionSummaryInstruction)
		if app.quickActions.open || app.sidebarTab != step.want {
			t.Fatalf("%s left card open=%t and sidebar=%d, want closed and sidebar=%d",
				step.name, app.quickActions.open, app.sidebarTab, step.want)
		}
	}
}
