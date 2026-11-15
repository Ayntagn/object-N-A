package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

// spec: EV-UI-231 / EV-UI-242
func TestAIWritebackStreamsOneUndoPerTurn(t *testing.T) {
	const prefix, suffix = "# Example 🧪\n\n", "Original paragraph.\n"
	draft := editor.NewBuffer(prefix + suffix)
	code := &editor.State{Buffer: draft}
	tab := &workspaceTab{draft: draft, code: code, mode: splitMode}
	app := workspaceWindow{
		workspaceTab: tab, tabs: []*workspaceTab{tab},
		preferences: &workspacePreferences{AIStreamIntoEditor: true},
	}
	code.Select(len(prefix), len(prefix))

	for _, turn := range []struct {
		before, first, final string
	}{{"", "Reply ", "Reply one. "}, {"Reply one. ", "Reply ", "Reply two."}} {
		writeback := app.beginAIWriteback()
		if writeback == nil {
			t.Fatal("no writeback at the source caret")
		}
		for _, reply := range []string{turn.first, turn.final} {
			if !writeback.apply(&app, reply) {
				t.Fatal("writeback stopped")
			}
			want := prefix + turn.before + reply
			if got := draft.String(); got != want+suffix || code.Cursor() != len(want) {
				t.Fatalf("source = %q, caret = %d, want source %q at %d", got, code.Cursor(), want+suffix, len(want))
			}
		}
	}

	app.preferences.AIStreamIntoEditor = false
	if app.beginAIWriteback() != nil {
		t.Fatal("chat-only turn acquired a source writeback")
	}
	for _, want := range []string{prefix + "Reply one. " + suffix, prefix + suffix} {
		if !draft.Undo() || draft.String() != want {
			t.Fatalf("undo source = %q, want %q", draft.String(), want)
		}
	}
	if draft.CanUndo() || draft.Modified() {
		t.Fatal("two undo steps should leave the original unmodified source")
	}
	code.Select(len(prefix), len(prefix))
	app.insertAIReply("Manual reply. ")
	if got, want := draft.String(), prefix+"Manual reply. "+suffix; got != want {
		t.Fatalf("source after explicit insertion = %q, want %q", got, want)
	}
	if !draft.Undo() || draft.String() != prefix+suffix {
		t.Fatal("one Undo should remove the explicit insertion")
	}
	code.Select(len(prefix), draft.Len()-1)
	app.insertAIReply("Manual replacement.")
	if got, want := draft.String(), prefix+"Manual replacement.\n"; got != want {
		t.Fatalf("source after explicit replacement = %q, want %q", got, want)
	}
}

// spec: EV-UI-242. Insert Text follows the original source tab and revision.
func TestAIWritebackFollowsSourceTabAndPreservesInterveningEdit(t *testing.T) {
	const prefix, selection, suffix = "# Example 🧪\n\n", "SELECT 👩🏽‍💻 me", "\noutside\n"
	draft := editor.NewBuffer(prefix + selection + suffix)
	code := &editor.State{Buffer: draft}
	tab := &workspaceTab{draft: draft, code: code, mode: sourceMode}
	otherDraft := editor.NewBuffer("Second note\n")
	otherTab := &workspaceTab{draft: otherDraft, code: &editor.State{Buffer: otherDraft}, mode: splitMode}
	app := workspaceWindow{
		workspaceTab: tab, tabs: []*workspaceTab{tab, otherTab},
		preferences: &workspacePreferences{AIStreamIntoEditor: true},
	}
	code.Select(len(prefix)+len(selection), len(prefix))
	if got := app.aiChatContextHint(app.aiChatPreviewHint()); !strings.Contains(got, selection) || strings.Contains(got, suffix) {
		t.Fatalf("chat context = %q, want only the selected note text", got)
	}
	writeback := app.beginAIWriteback()
	if writeback == nil || !writeback.apply(&app, "Reply β ") {
		t.Fatal("no writeback for the selected source")
	}
	app.workspaceTab = otherTab
	if !writeback.apply(&app, "Reply β continued ") || otherDraft.String() != "Second note\n" {
		t.Fatal("switching the active tab redirected or stopped the writeback")
	}
	partial := prefix + "Reply β continued " + selection + suffix
	if got := draft.String(); got != partial {
		t.Fatalf("source = %q, want %q", got, partial)
	}

	app.workspaceTab = tab
	draft.Edit(editor.Edit{Start: draft.Len(), End: draft.Len(), Text: "Author's edit\n"})
	if writeback.apply(&app, "Reply β continued after editing") {
		t.Fatal("writeback continued after the author edited the source")
	}
	if got, want := draft.String(), partial+"Author's edit\n"; got != want {
		t.Fatalf("source = %q, want %q", got, want)
	}

	writeback = app.beginAIWriteback()
	app.workspaceTab, app.tabs = otherTab, []*workspaceTab{otherTab}
	if writeback.apply(&app, "Reply after closing") {
		t.Fatal("writeback continued after the source tab closed")
	}
}

// spec: EV-UI-242. An attachment takes precedence over the live selection.
// The live selection still determines where Replace Selection writes.
func TestAIWritebackReplacesSelectionWithAttachedContext(t *testing.T) {
	const attached, selection, suffix = "👩🏽‍💻 context", "Replace α", "\noutside\n"
	draft := editor.NewBuffer(attached + "\n" + selection + suffix)
	code := &editor.State{Buffer: draft}
	tab := &workspaceTab{draft: draft, code: code, mode: splitMode}
	app := workspaceWindow{
		workspaceTab: tab, tabs: []*workspaceTab{tab},
		preferences: &workspacePreferences{AIStreamIntoEditor: true},
	}
	code.Select(0, len(attached))
	app.attachAIContext()
	if got := tab.aiContext.characters; got != 9 {
		t.Fatalf("attachment length = %d characters, want 9", got)
	}
	start := len(attached) + 1
	code.Select(start, start+len(selection))
	if got := app.aiChatContextHint(app.aiChatPreviewHint()); !strings.Contains(got, attached) || strings.Contains(got, selection) {
		t.Fatalf("chat context = %q, want only the attached text", got)
	}

	app.ai.replaceSelection = true
	writeback := app.beginAIWriteback()
	mid, final := "New β", "New β paragraph 🧪"
	for _, reply := range []string{mid, final} {
		if !writeback.apply(&app, reply) || draft.String() != attached+"\n"+reply+suffix {
			t.Fatalf("source after %q = %q", reply, draft.String())
		}
	}
	if !draft.Undo() || draft.String() != attached+"\n"+selection+suffix {
		t.Fatalf("undo source = %q, want original selection", draft.String())
	}
	app.clearAIContext()
	if tab.aiContext.snapshot != "" || tab.aiContext.characters != 0 {
		t.Fatal("attachment remains after clearing")
	}
}
