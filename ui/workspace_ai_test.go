// spec: EV-UI-225 / EV-UI-226 / EV-UI-231 / EV-UI-242 / EV-UI-261
package ui

import (
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestAIWorkspaceWithoutModel(t *testing.T) {
	var panel workspaceAI
	var settingsOpens, closes int
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		mygoui.Column(c).Fill().Clip().BorderWidth(0, 1, 0, 0).Children(func() {
			panel.draw(c, true, func() {}, func() { settingsOpens++ }, func() { closes++ }, aiComposerOptions{})
		})
	}, vaultPanelWidth, 940)
	if !tester.HasText("No AI model connected") || !tester.HasText("Ask about what you're writing.") {
		t.Fatal("AI Workspace lacks the no-model notice or empty conversation")
	}
	if settings, ok := tester.AccessByRole(mygoui.RoleButton, "Open AI Settings"); !ok ||
		settings.Bounds.X != 108.5 || settings.Bounds.W != 131 || settings.Bounds.H != 22 {
		t.Fatalf("model settings button not centered: %+v", settings.Bounds)
	}
	if conversation, ok := tester.AccessByRole(mygoui.RoleScroll, "AI Conversation"); !ok ||
		conversation.Bounds.X != 4 || conversation.Bounds.W != 340 {
		t.Fatalf("conversation does not fit the sidebar: %+v", conversation.Bounds)
	}
	prompt, ok := tester.AccessByRole(mygoui.RoleTextField, "AI prompt")
	if !ok || !prompt.Multiline || prompt.Focused {
		t.Fatalf("unexpected composer: %+v", prompt)
	}
	tester.ClickAt(prompt.Bounds.X+20, prompt.Bounds.Y+10)
	tester.Type("A local draft")
	if panel.prompt != "A local draft" {
		t.Fatalf("typed prompt = %q", panel.prompt)
	}
	if send, ok := tester.AccessByRole(mygoui.RoleButton, "Send"); !ok || !send.Disabled || send.Offscreen ||
		send.Bounds.W != 22 || send.Bounds.H != 22 || send.Bounds.X+send.Bounds.W > 332 {
		t.Fatalf("unexpected no-model Send button: %+v", send)
	}
	panel.appendQuickAction("Summarize the first section")
	panel.appendQuickAction("Explain the conclusion")
	tester.Frame()
	if !tester.HasText("Summarize the first section") || !tester.HasText("Explain the conclusion") ||
		tester.HasText("Ask about what you're writing.") || !tester.HasText("No AI model connected") {
		t.Fatal("AI Workspace lacks the submitted conversation or model notice")
	}

	click := func(label string) {
		t.Helper()
		button, ok := tester.AccessByRole(mygoui.RoleButton, label)
		if !ok || button.Disabled {
			t.Fatalf("missing enabled button %q", label)
		}
		tester.ClickAt(button.Bounds.X+button.Bounds.W/2, button.Bounds.Y+button.Bounds.H/2)
	}
	click("New conversation")
	if panel.prompt != "" || len(panel.submitted) != 0 || tester.HasText("Explain the conclusion") ||
		!tester.HasText("No AI model connected") {
		t.Fatal("New conversation did not restore the empty conversation")
	}
	if panel.scroll.Y != panel.scroll.MaxY {
		t.Fatalf("new conversation scroll = %g, end = %g", panel.scroll.Y, panel.scroll.MaxY)
	}
	click("Open AI Settings")
	click("Close AI sidebar")
	if settingsOpens != 1 || closes != 1 {
		t.Fatalf("settings opened %d times, sidebar closed %d times", settingsOpens, closes)
	}
}

func TestAIComposerWriteBackAndStop(t *testing.T) {
	panel := workspaceAI{prompt: "Draft a short reply"}
	var documentContext workspaceAIContext
	writeBack := true
	cancelled := 0
	var inserted string
	var readerAnchor mygoui.Rect
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		mygoui.Column(c).Fill().Children(func() {
			panel.draw(c, true, func() {}, nil, nil, aiComposerOptions{
				model: "fixture", streamToEditor: writeBack,
				context:         &documentContext,
				toggleWriteBack: func() { writeBack = !writeBack },
				attachContext: func() {
					documentContext.snapshot = "SELECTION118 👩🏽‍💻 alpha"
					documentContext.characters = 20
				},
				clearContext: func() { documentContext.snapshot = "" },
				showContext:  func(anchor mygoui.Rect) { readerAnchor = anchor },
				insertReply:  func(reply string) { inserted = reply },
				send: func(prompt string) bool {
					panel.appendQuickAction(prompt)
					panel.submitted[0].reply = "Partial reply β"
					panel.prompt = ""
					panel.request = &workspaceAIRequest{turn: 0, cancel: func() { cancelled++ }}
					return true
				},
			})
		})
	}, vaultPanelWidth, 940)

	mode, ok := tester.AccessByRole(mygoui.RoleButton, "Insert Text")
	if !ok || mode.Disabled || !mode.Checked {
		t.Fatalf("write-back button = %+v", mode)
	}
	if err := tester.Click("Replace Selection"); err != nil {
		t.Fatal(err)
	}
	if replace, ok := tester.AccessByRole(mygoui.RoleButton, "Replace Selection"); !ok || !replace.Checked || !panel.replaceSelection {
		t.Fatalf("Replace Selection did not enable the mode: %+v", replace)
	}
	if err := tester.Click("Insert Text"); err != nil {
		t.Fatal(err)
	}
	if chat, ok := tester.AccessByRole(mygoui.RoleButton, "Chat only"); !ok || chat.Checked || chat.Bounds != mode.Bounds {
		t.Fatalf("chat-only button did not replace write-back in place: %+v", chat)
	}
	if _, ok := tester.AccessByRole(mygoui.RoleButton, "Replace Selection"); ok {
		t.Fatal("chat-only mode retained the Replace Selection button")
	}
	prompt, _ := tester.AccessByRole(mygoui.RoleTextField, "AI prompt")
	conversation, _ := tester.AccessByRole(mygoui.RoleScroll, "AI Conversation")
	if err := tester.Click("Attach current note"); err != nil {
		t.Fatal(err)
	}
	chip, ok := tester.AccessByRole(mygoui.RoleButton, "+Context 20")
	if !ok || chip.Bounds.H != 15 || chip.Bounds.Y != prompt.Bounds.Y-31 {
		t.Fatalf("attached context chip = %+v", chip)
	}
	if attached, _ := tester.AccessByRole(mygoui.RoleTextField, "AI prompt"); attached.Bounds != prompt.Bounds {
		t.Fatal("attaching context moved the prompt")
	}
	if attached, _ := tester.AccessByRole(mygoui.RoleScroll, "AI Conversation"); attached.Bounds.H != conversation.Bounds.H-15 {
		t.Fatal("attaching context did not reserve 15 points above the composer")
	}
	if err := tester.Click("+Context 20"); err != nil || readerAnchor != chip.Bounds {
		t.Fatalf("context chip passed anchor %+v: %v", readerAnchor, err)
	}
	documentContext.notes = "Keep the example β."
	tester.Frame()
	if _, ok := tester.AccessByRole(mygoui.RoleButton, "+Context 20 +notes"); !ok {
		t.Fatal("Notes were missing from the attachment badge")
	}
	if err := tester.Click("Remove context"); err != nil {
		t.Fatal(err)
	}
	notesChip, ok := tester.AccessByRole(mygoui.RoleButton, "+Notes")
	if !ok || notesChip.Bounds.W != 57 || notesChip.Bounds.H != 15 || notesChip.Bounds.Y != chip.Bounds.Y {
		t.Fatalf("standalone Notes chip = %+v", notesChip)
	}
	if err := tester.Click("+Notes"); err != nil || readerAnchor != notesChip.Bounds || panel.request != nil {
		t.Fatalf("Notes chip passed anchor %+v, request %v: %v", readerAnchor, panel.request, err)
	}
	if _, ok := tester.AccessByRole(mygoui.RoleButton, "Remove context"); ok {
		t.Fatal("standalone Notes shows an attachment removal button")
	}
	documentContext.notes = ""
	tester.Frame()
	if _, ok := tester.AccessByRole(mygoui.RoleButton, "+Notes"); ok {
		t.Fatal("clearing Notes did not remove the chip")
	}

	send, ok := tester.AccessByRole(mygoui.RoleButton, "Send")
	if !ok || send.Disabled {
		t.Fatal("a filled connected composer cannot send")
	}
	if err := tester.Click("Send"); err != nil {
		t.Fatal(err)
	}
	if stop, ok := tester.AccessByRole(mygoui.RoleButton, "Stop"); !ok || stop.Disabled || stop.Bounds != send.Bounds {
		t.Fatalf("composer Stop did not replace Send in place: %+v", stop)
	}
	if stop, ok := tester.AccessByRole(mygoui.RoleButton, "Stop response"); !ok || stop.Bounds.W != 14 || stop.Bounds.H != 14 {
		t.Fatalf("conversation Stop = %+v", stop)
	}
	if err := tester.Click("Stop"); err != nil {
		t.Fatal(err)
	}
	if cancelled != 1 || panel.request != nil {
		t.Fatalf("Stop cancelled %d requests and left request %v", cancelled, panel.request)
	}
	if idle, ok := tester.AccessByRole(mygoui.RoleButton, "Send"); !ok || !idle.Disabled || idle.Bounds != send.Bounds {
		t.Fatalf("empty composer did not restore its disabled Send: %+v", idle)
	}
	if insert, ok := tester.AccessByRole(mygoui.RoleButton, "Insert into Editor"); !ok || insert.Bounds.W != 136 || insert.Bounds.H != 22 {
		t.Fatalf("chat-only reply insertion = %+v", insert)
	}
	if err := tester.Click("Insert into Editor"); err != nil || inserted != "Partial reply β" {
		t.Fatalf("Insert into Editor dispatched %q: %v", inserted, err)
	}
}

func TestAIContextReaderCopiesWholeSnapshot(t *testing.T) {
	const snapshot = "# Note\n\nSelected β 👩🏽‍💻\n"
	var notes string
	closed := false
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		drawAIContextReader(c, true, snapshot, 22, &notes, func() { closed = true }, nil)
	}, aiContextWidth, aiContextHeight)
	if body, ok := tester.AccessByRole(mygoui.RoleTextField, "Attached context"); !ok ||
		body.Bounds.W != 404 || body.Bounds.H != 249 || !body.ReadOnly {
		t.Fatalf("context reader = %+v", body)
	}
	if field, ok := tester.AccessByRole(mygoui.RoleTextField, "Context notes"); !ok ||
		field.Bounds.W != 384 || field.Bounds.H != 60 || field.ReadOnly {
		t.Fatalf("context notes = %+v", field)
	}
	if err := tester.Click("Copy"); err != nil || tester.Clipboard() != snapshot {
		t.Fatalf("Copy changed the Markdown snapshot %q: %v", tester.Clipboard(), err)
	}
	if err := tester.Click("Context notes"); err != nil {
		t.Fatal(err)
	}
	tester.Type("Keep the example β.")
	if notes != "Keep the example β." {
		t.Fatalf("edited notes = %q", notes)
	}
	if err := tester.Click("Done"); err != nil || !closed {
		t.Fatalf("Done did not dismiss the context reader: %v", err)
	}

	tester = mygoui.NewTester(func(c *mygoui.Context) {
		drawAIContextReader(c, true, "", 0, &notes, nil, nil)
	}, aiContextWidth, aiContextHeight)
	if tester.HasText("Document") || !tester.HasText("0 chars") {
		t.Fatal("the standalone Notes reader showed attachment metadata")
	}
	if body, ok := tester.AccessByRole(mygoui.RoleTextField, "Attached context"); !ok ||
		body.Bounds.W != 404 || body.Bounds.H != 249 || !body.ReadOnly || !body.Focused || body.Value != "" {
		t.Fatalf("empty context reader = %+v", body)
	}
	if field, ok := tester.AccessByRole(mygoui.RoleTextField, "Context notes"); !ok || field.Value != "Keep the example β." {
		t.Fatalf("standalone context notes = %+v", field)
	}
	if err := tester.Click("Context notes"); err != nil {
		t.Fatal(err)
	}
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	tester.Key(0, mygoui.KeyBackspace)
	if notes != "" {
		t.Fatalf("clearing the standalone Notes left %q", notes)
	}
}
