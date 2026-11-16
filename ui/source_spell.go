package ui

import (
	"slices"
	"strings"
	"time"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// Coalesce edits before submitting a snapshot to the system spell checker.
const sourceSpellPause = 300 * time.Millisecond

type sourceSpellCheck struct {
	owner    *workspaceWindow
	draft    *editor.Buffer
	version  uint64
	markdown bool
	due      time.Time
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
	markdown := tab.draft.Language() != nil
	now := c.Now()
	if check == nil || check.owner != app || check.draft != tab.draft || check.version != version || check.markdown != markdown {
		check = &sourceSpellCheck{owner: app, draft: tab.draft, version: version, markdown: markdown, due: now.Add(sourceSpellPause)}
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
	go func() {
		text := snapshot.String()
		input := text
		if markdown {
			input = sourceSpellingInput(text)
		}
		if strings.TrimSpace(input) == "" {
			return
		}
		mygo.RunOnMain(func() {
			if win.IsDestroyed() || !app.sourceSpellingCurrent(tab, check) {
				return
			}
			requestSystemSourceSpelling(input, func(ranges []sourceSpellingRange) {
				diagnostics := sourceSpellingDiagnostics(text, ranges)
				win.Update(func() {
					if !win.IsDestroyed() {
						app.publishSourceSpelling(tab, check, diagnostics)
					}
				})
			})
		})
	}()
}

func (app *workspaceWindow) sourceSpellingCurrent(tab *workspaceTab, check *sourceSpellCheck) bool {
	return app.preferences.SpellCheckEnabled && tab.sourceSpell == check && check.owner == app &&
		slices.Contains(app.tabs, tab) && tab.draft == check.draft && check.version == tab.draft.Version()
}

func (app *workspaceWindow) publishSourceSpelling(tab *workspaceTab, check *sourceSpellCheck, diagnostics []editor.Diagnostic) {
	if app.sourceSpellingCurrent(tab, check) {
		tab.code.SetDiagnostics(check.version, diagnostics)
	}
}

// Give the spell checker whitespace in place of Markdown code and math.
// Preserve line endings and UTF-16 offsets into the original source.
func sourceSpellingInput(text string) string {
	prefix, _, body := workspace.SplitFrontmatter(text)
	// Code and math need a delimiter or at least four columns of indent.
	// Ordinary prose can go straight to the checker.
	if !strings.ContainsAny(body, "`~$\\\t") && !strings.Contains(body, "    ") {
		return text
	}
	doc := parser.Parse(body)
	var input strings.Builder
	next := 0
	mask := func(start, end int) {
		start += len(prefix)
		end += len(prefix)
		if next == 0 {
			input.Grow(len(text))
		}
		input.WriteString(text[next:start])
		for _, r := range text[start:end] {
			switch {
			case r == '\r' || r == '\n':
				input.WriteByte(byte(r))
			case r > '\uffff':
				input.WriteString("  ")
			default:
				input.WriteByte(' ')
			}
		}
		next = end
	}
	for i := 1; i < doc.Len(); i++ {
		block := doc.Block(i)
		switch block.Kind {
		case parser.CodeBlock, parser.MathBlock:
			mask(block.Start, block.End)
		case parser.Paragraph, parser.Heading, parser.TableCell:
			if !strings.ContainsAny(body[block.Start:block.End], "`$\\") {
				continue
			}
			for _, span := range doc.Inlines(i) {
				if span.Kind == parser.Code || span.Kind == parser.Math {
					mask(span.Start, span.End)
				}
			}
		}
	}
	if next == 0 {
		return text
	}
	input.WriteString(text[next:])
	return input.String()
}

// Advance through the source once for ordered, non-overlapping UTF-16
// results and keep the committed byte offsets for MyGo.
func sourceSpellingDiagnostics(text string, ranges []sourceSpellingRange) []editor.Diagnostic {
	var diagnostics []editor.Diagnostic
	var byteOffset int
	var units uint
	// Each UTF-16 unit needs at least one UTF-8 byte. Use that cheap upper
	// bound here and validate the exact lengths as each range is decoded.
	limit := uint(len(text))
	for _, span := range ranges {
		if span.Length == 0 || span.Location < units || span.Location >= limit || span.Length > limit-span.Location {
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
