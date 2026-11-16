// spec: EV-UI-277 / EV-UI-281 / EV-MYGO-187. AI reference files in the picker and sidebar.
package ui

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestAIReferenceTextUsesFoundationUTF8Decoding(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "Windows.csv")
	const body = "title,value\r\nref,👩🏽‍💻\r\n"
	if err := os.WriteFile(filename, []byte("\uFEFF"+body), 0o600); err != nil {
		t.Fatal(err)
	}
	reference, ok := loadAIReference(context.Background(), filename)
	if !ok || reference.content != body {
		t.Fatalf("decoded reference = %q", reference.content)
	}
}

func TestAIAttachedFileChipsFollowConversation(t *testing.T) {
	var panel workspaceAI
	var app workspaceWindow
	view := mygoui.NewTester(func(c *mygoui.Context) {
		panel.draw(c, true, func() {}, nil, nil, aiComposerOptions{
			pickReferences: func() {
				panel.addAIReferences([]workspaceAIReference{
					{name: "Reference.md", content: "first reference\n"},
					{name: "Reference.md", content: "duplicate content"},
					{name: "Examples.json", content: "{\"example\":true}"},
				})
			},
		})
	}, vaultPanelWidth, 940)

	click := func(label string) {
		t.Helper()
		button, ok := view.AccessByRole(mygoui.RoleButton, label)
		if !ok || button.Disabled {
			t.Fatalf("missing enabled button %s", label)
		}
		view.ClickAt(button.Bounds.X+button.Bounds.W/2, button.Bounds.Y+button.Bounds.H/2)
	}
	click("Attachments")
	want := "Attached file \"Reference.md\":\n\nfirst reference\n" +
		"\n\n---\n\nAttached file \"Examples.json\":\n\n{\"example\":true}\n\n---\n\npreview"
	if got := app.aiChatContextHint("preview", panel.references...); got != want {
		t.Fatalf("attachment context = %q, want %q", got, want)
	}
	click("Remove Reference.md")
	if got := app.aiChatContextHint("preview", panel.references...); strings.Contains(got, "Reference.md") || !strings.Contains(got, "Examples.json") {
		t.Fatalf("removed reference left context %q", got)
	}
	click("New conversation")
	if len(panel.references) != 0 || view.HasText("Examples.json") {
		t.Fatal("new conversation retained a pending attachment")
	}
}

func TestAIReferenceDropTargetsStayInsideSidebar(t *testing.T) {
	app, view, _ := snippetsWorkspace(t, "Active.md", "# Active\n", sourceMode)
	app.sidebarTab, app.vaultVisible = workspaceAITab, true
	filename := filepath.Join(t.TempDir(), "Attachment.txt")
	if err := os.WriteFile(filename, []byte("Synthetic reference"), 0o600); err != nil {
		t.Fatal(err)
	}
	view.Frame()
	prompt, ok := view.Access("AI prompt")
	if !ok || prompt.Offscreen {
		t.Fatal("missing visible AI prompt")
	}
	view.ClickAt(prompt.Bounds.X+prompt.Bounds.W/2, prompt.Bounds.Y+prompt.Bounds.H/2)
	view.Type("Keep this draft")
	dropOn := func(label string) bool {
		t.Helper()
		item, ok := view.Access(label)
		if !ok || item.Offscreen {
			t.Fatalf("missing visible %q", label)
		}
		return view.DropFiles(item.Bounds.X+item.Bounds.W/2, item.Bounds.Y+item.Bounds.H/2, filename)
	}
	if !dropOn("AI Workspace tools") || !dropOn("AI prompt") {
		t.Fatal("the AI toolbar or composer did not accept its file drop")
	}
	after, _ := view.Access("AI prompt")
	if dropOn("Source editor") || len(app.ai.submitted) != 0 || app.ai.prompt != "Keep this draft" ||
		!after.Focused || after.Bounds != prompt.Bounds {
		t.Fatal("the file drop escaped the sidebar, sent a message or disturbed the draft")
	}
}

func TestAIReferenceBatchesDiscardOldConversation(t *testing.T) {
	write := func(name, text string) string {
		t.Helper()
		filename := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(filename, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return filename
	}
	first := write("Reference.md", "first reference")
	duplicate := write("Reference.md", "some other content")
	second := write("Extra.txt", "next reference")
	var panel workspaceAI
	t.Cleanup(panel.clearAIReferences)
	load := panel.beginAIReferenceLoad()
	load.waiting = []string{duplicate, second}
	initial := loadAIReferenceFiles(load.ctx, []string{first}, make(map[string]bool))
	next := panel.completeAIReferenceBatch(load, initial)
	if !slices.Equal(next, []string{duplicate, second}) || panel.referenceLoad != load ||
		len(panel.references) != 1 || panel.references[0].content != "first reference" {
		t.Fatalf("first batch lost its place or queued work: %+v", panel.references)
	}
	later := loadAIReferenceFiles(load.ctx, next, map[string]bool{"Reference.md": true})
	if len(later) != 1 || later[0].content != "next reference" {
		t.Fatalf("queued references = %+v", later)
	}
	panel.newConversation()
	current := panel.beginAIReferenceLoad()
	if wait := panel.completeAIReferenceBatch(load, later); len(wait) != 0 || load.ctx.Err() == nil ||
		panel.referenceLoad != current || len(panel.references) != 0 {
		t.Fatal("an old import completed in the new conversation")
	}
	fresh := loadAIReferenceFiles(current.ctx, []string{second}, make(map[string]bool))
	panel.completeAIReferenceBatch(current, fresh)
	if len(panel.references) != 1 || panel.references[0].name != "Extra.txt" || panel.referenceLoad != nil {
		t.Fatal("the new conversation did not finish its own import")
	}
}
