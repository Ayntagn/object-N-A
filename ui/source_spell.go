package ui

import (
	"slices"
	"time"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// Coalesce edits before submitting a snapshot to the system spell checker.
const sourceSpellPause = 300 * time.Millisecond

type sourceSpellCheck struct {
	owner   *workspaceWindow
	draft   *editor.Buffer
	version uint64
	due     time.Time
}

type sourceSpellingRange struct{ Location, Length uint }

func (app *workspaceWindow) sourceSpellSettings(c *mygoui.Context) {
	label := app.tr("Spell Check", "拼写检查")
	if sourceSpellAvailable {
		app.settingsSwitch(c, label, app.tr("Check as you type", "输入时检查拼写"), "", 60, &app.preferences.SpellCheckEnabled)
		return
	}
	settingsRow(c, label, app.tr("Unavailable on this platform", "当前平台不可用"), "", 60, func() {
		var off bool
		mygoui.Switch(c, &off).Label(label).Size(36, 20).Disabled(true).Shrink(0)
	})
}

func (app *workspaceWindow) applySourceSpellCheck(c *mygoui.Context, tab *workspaceTab) {
	version := tab.draft.Version()
	check := tab.sourceSpell
	if !sourceSpellAvailable || !app.preferences.SpellCheckEnabled {
		if check != nil {
			tab.sourceSpell = nil
			tab.code.SetDiagnostics(version, nil)
		}
		return
	}
	now := c.Now()
	if check == nil || check.owner != app || check.draft != tab.draft || check.version != version {
		check = &sourceSpellCheck{owner: app, draft: tab.draft, version: version, due: now.Add(sourceSpellPause)}
		tab.sourceSpell = check
		tab.code.SetDiagnostics(version, nil)
	}
	if check.due.IsZero() {
		return
	}
	if remaining := check.due.Sub(now); remaining > 0 {
		c.After(remaining)
		return
	}
	check.due = time.Time{}
	win := app.win
	if win == nil || win.IsDestroyed() || check.draft.Len() == 0 {
		return
	}
	snapshot := check.draft.Snapshot()
	if snapshot.Version() != check.version {
		return
	}
	text := snapshot.String()
	requestSystemSourceSpelling(text, func(ranges []sourceSpellingRange) {
		diagnostics := sourceSpellingDiagnostics(text, ranges)
		win.Update(func() {
			if !win.IsDestroyed() {
				app.publishSourceSpelling(tab, check, diagnostics)
			}
		})
	})
}

func (app *workspaceWindow) publishSourceSpelling(tab *workspaceTab, check *sourceSpellCheck, diagnostics []editor.Diagnostic) {
	if !app.preferences.SpellCheckEnabled || tab.sourceSpell != check || check.owner != app ||
		!slices.Contains(app.tabs, tab) || tab.draft != check.draft || check.version != tab.draft.Version() {
		return
	}
	tab.code.SetDiagnostics(check.version, diagnostics)
}

// The system returns disjoint spelling results in UTF-16 order. Advance
// through UTF-8 once and keep the committed byte offsets for MyGo.
func sourceSpellingDiagnostics(text string, ranges []sourceSpellingRange) []editor.Diagnostic {
	var diagnostics []editor.Diagnostic
	var byteOffset int
	var units uint
	length := uint(mygoui.UTF16Len(text))
	for _, span := range ranges {
		if span.Length == 0 || span.Location < units || span.Location >= length || span.Length > length-span.Location {
			continue
		}
		start := byteOffset + mygoui.UTF16ByteOffset(text[byteOffset:], int(span.Location-units))
		end := start + mygoui.UTF16ByteOffset(text[start:], int(span.Length))
		if uint(mygoui.UTF16Len(text[byteOffset:start])) != span.Location-units ||
			uint(mygoui.UTF16Len(text[start:end])) != span.Length {
			continue
		}
		diagnostics = append(diagnostics, editor.Diagnostic{
			Start: start, End: end, Severity: editor.SeverityError, Message: "Misspelled word", Source: "Spelling",
		})
		byteOffset, units = end, span.Location+span.Length
	}
	return diagnostics
}
