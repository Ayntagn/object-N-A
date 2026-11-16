// spec: FEAT-001 / EV-MYGO-181, EV-MYGO-199 / Typed code fence completion.
package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"

	mygoui "github.com/egoist/mygo/ui"
)

func TestSourceCodeFenceCompletion(t *testing.T) {
	const tick = "`"
	fence := strings.Repeat(tick, 3)
	t.Run("on the first line after a BOM", func(t *testing.T) {
		const bom = "\ufeff"
		app, tester, _ := snippetsWorkspace(t, "Fence.md", bom, sourceMode)
		tester.Frame()
		tester.Key(0, mygoui.KeyRight)
		for range 3 {
			tester.TypeKey(0, mygoui.KeyBackquote, tick)
		}
		if got := app.draft.String(); got != bom+fence+"\n\n"+fence || app.code.Cursor() != len(bom)+len(fence) {
			t.Fatalf("third key produced %q with caret %d", got, app.code.Cursor())
		}
		tester.TypeKey(0, mygoui.KeyG, "g")
		tester.TypeKey(0, mygoui.KeyO, "o")
		tester.Key(0, mygoui.KeyEnter)
		tester.TypeKey(0, mygoui.KeyX, "x")
		if got := app.draft.String(); got != bom+fence+"go\nx\n"+fence {
			t.Fatalf("entering the generated body produced %q", got)
		}
	})
	t.Run("CRLF and undo", func(t *testing.T) {
		const prefix = "# Before\n\n  "
		following := "\n" + fence + "text\nliteral\n" + fence + "\nAfter\n"
		app, tester, _ := snippetsWorkspace(t, "Fence.md", strings.ReplaceAll(prefix+following, "\n", "\r\n"), splitMode)
		app.code.Select(len(prefix), len(prefix))
		tester.Frame()
		for range 3 {
			tester.TypeKey(0, mygoui.KeyBackquote, tick)
		}
		const inside = "\n  \n  "
		want := prefix + fence + inside + fence + following
		if app.draft.String() != want || app.code.Cursor() != len(prefix)+len(fence) {
			t.Fatalf("third key produced %q with caret %d", app.draft.String(), app.code.Cursor())
		}
		tester.Command("undo")
		if got := app.draft.String(); got != prefix+fence[:2]+following || app.code.Cursor() != len(prefix)+2 {
			t.Fatalf("one undo produced %q with caret %d", got, app.code.Cursor())
		}
		for range 2 {
			tester.TypeKey(0, mygoui.KeyBackquote, tick)
		}
		longFence := fence + tick
		tester.TypeKey(0, mygoui.KeyG, "g")
		tester.TypeKey(0, mygoui.KeyO, "o")
		want = prefix + longFence + "go" + inside + longFence + following
		var saved strings.Builder
		if _, err := app.draft.WriteTo(&saved); err != nil || saved.String() != strings.ReplaceAll(want, "\n", "\r\n") {
			t.Fatalf("long fence with language produced %q, error %v", saved.String(), err)
		}
		tester.Key(0, mygoui.KeyEnter)
		tester.TypeKey(0, mygoui.KeyX, "x")
		if got := app.draft.String(); got != prefix+longFence+"go\n  x\n  "+longFence+following {
			t.Fatalf("entering the generated body produced %q", got)
		}
	})
	t.Run("new ordered item inside quote", func(t *testing.T) {
		const prefix, following = "> 1.  ", "\n> 2. next\n"
		before := prefix + fence[:2] + following
		app, tester, _ := snippetsWorkspace(t, "Fence.md", before, sourceMode)
		app.code.Select(len(prefix)+2, len(prefix)+2)
		tester.Frame()
		tester.TypeKey(0, mygoui.KeyBackquote, tick)
		want := prefix + fence + "\n>     \n>     " + fence + following
		if app.draft.String() != want || app.code.Cursor() != len(prefix)+len(fence) {
			t.Fatalf("third key produced %q with caret %d", app.draft.String(), app.code.Cursor())
		}
		tester.Command("undo")
		if app.draft.String() != before || app.code.Cursor() != len(prefix)+2 {
			t.Fatalf("one undo produced %q with caret %d", app.draft.String(), app.code.Cursor())
		}
	})

	for _, test := range []struct {
		name, prefix, following string
		paste                   bool
	}{
		{name: "existing close", prefix: "  " + fence[:2], following: "\n\n  " + fence + "\nAfter"},
		{name: "existing close in list", prefix: "- first item\n  " + fence[:2], following: "\n  body\n  " + fence + "\n- second item\n"},
		{name: "existing close in quoted list", prefix: "> - " + fence[:2], following: "\n>   body\n>   " + fence + "\nOutside\n"},
		{name: "inline ticks", prefix: "Inline " + fence[:2]},
		{name: "closing fence", prefix: fence + "go\ncode\n" + fence[:2], following: "\nAfter"},
		{name: "paste single tick", prefix: fence[:2], paste: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, tester, _ := snippetsWorkspace(t, "Fence.md", test.prefix+test.following, sourceMode)
			app.code.Select(len(test.prefix), len(test.prefix))
			tester.Frame()
			if test.paste {
				tester.SetClipboard(tick)
				tester.Command("paste")
			} else {
				tester.TypeKey(0, mygoui.KeyBackquote, tick)
			}
			if got := app.draft.String(); got != test.prefix+tick+test.following {
				t.Fatalf("expected only the typed marker, got %q", got)
			}
		})
	}
}

func TestVisualTaggedCodeFenceKeepsLanguage(t *testing.T) {
	const before = "Before."
	const after = "After."
	for _, fixture := range []struct {
		language, body string
	}{
		{"go", "package main"},
		{"PowerShell", "$notes = Get-ChildItem"},
		{"my-config", "name = object"},
	} {
		t.Run(fixture.language, func(t *testing.T) {
			app, tester, file := snippetsWorkspace(t, "Fence.md", before+"\n\n"+after+"\n", wysiwygMode)
			app.wys.session.Document.SetSelection(len(before), len(before))
			tester.Frame()
			tester.Key(0, mygoui.KeyEnter)
			for range 3 {
				tester.TypeKey(0, mygoui.KeyBackquote, "`")
			}
			for _, char := range fixture.language {
				tester.Type(string(char))
			}
			literal := app.draft.String()
			tester.Key(0, mygoui.KeyEnter)
			prefix := before + "\n\n```" + fixture.language + "\n"
			suffix := "\n```\n\n" + after + "\n"
			label := fixture.language + " source"
			if got := app.draft.String(); got != prefix+suffix || !tester.Focused(label) {
				t.Fatalf("Enter produced %q without a focused %s code block", got, fixture.language)
			}
			tester.Command("undo")
			if app.draft.String() != literal || !tester.Focused("WYSIWYG editor") {
				t.Fatalf("one undo produced %q, want %q", app.draft.String(), literal)
			}
			tester.Key(0, mygoui.KeyEnter)
			tester.Type(fixture.body)
			want := prefix + fixture.body + suffix
			if got := app.draft.String(); got != want {
				t.Fatalf("typing in the new code block produced %q", got)
			}
			root := filepath.Dir(file)
			vault, err := workspace.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			defer vault.Close()
			app.document, err = vault.ReadDocument("Fence.md")
			if err != nil {
				t.Fatal(err)
			}
			app.storage, app.folderRoot = vault, root
			if !app.save() {
				t.Fatal(app.problem)
			}
			saved, err := vault.ReadDocument("Fence.md")
			if err != nil || saved.Text != want {
				t.Fatalf("saved %q, want %q, error %v", saved.Text, want, err)
			}
			app.setTabDocument(app.workspaceTab, saved)
			tester.Frame()
			if _, ok := tester.Find(label); !ok {
				t.Fatalf("reopened note has no editable %s block", fixture.language)
			}
		})
	}
}
