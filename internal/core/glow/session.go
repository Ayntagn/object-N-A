// Package glow implements GLOW's Vault terminal.
package glow

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"time"
	"unicode"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
)

// Environment is the active workspace snapshot for one terminal action.
type Environment struct {
	Vault   *workspace.Vault
	Name    string
	Entries []workspace.Entry
	Index   *vaultindex.Index
	Active  string
	Recent  []string
	// InsertMarkdown applies a redirect to the active editor.
	InsertMarkdown func(string) bool
	// AppCommandAvailable reports which slash commands the workspace supports.
	AppCommandAvailable func(AppCommand) bool
	// TrashItem moves a file or folder to the system trash.
	TrashItem func(string) error
}

// Result describes workspace changes to apply after a command.
type Result struct {
	OpenPath      string
	VaultChanged  bool
	Theme         string
	DemoDelay     time.Duration
	AppCommand    AppCommand
	ScriptEffects []ScriptEffect
}

type Style uint8

const (
	Plain Style = iota
	Prompt
	Accent
	Muted
	Error
	ScriptText
	ScriptMarker
	ScriptMuted
	ScriptError
)

// Span optionally links its text to a Vault-relative path.
type Span struct {
	Text  string
	Path  string
	Style Style
}

type Line []Span

// Session holds the terminal state shared by all document windows and hosts.
type Session struct {
	started bool
	cwd     string
	lines   []Line
	history []string
	demo    *demoState
	script  *scriptContext
}

// HistoryCursor is local to a terminal input field. Its zero value is at
// the end of the shared history.
type HistoryCursor struct {
	browsing bool
	index    int
	draft    string
}

// Start initializes the session once. Commands use the current Vault snapshot.
func (s *Session) Start(env Environment) {
	if s.started {
		return
	}
	s.started = true
	s.text("G L O W   the Glowing Logical Operator Window", Accent)
	timeOfDay := "morning"
	switch hour := time.Now().Hour(); {
	case hour >= 18:
		timeOfDay = "evening"
	case hour >= 12:
		timeOfDay = "afternoon"
	}
	s.text("good "+timeOfDay+", operator.", Plain)
	if env.Vault == nil {
		s.text("no vault open", Muted)
	} else {
		stats := vaultStats(env)
		s.text(fmt.Sprintf("vault %s  ·  %d notes  ·  %d tags", vaultName(env.Name), stats.notes, stats.tags), Muted)
	}
	editing := "no note open"
	if env.Active != "" {
		editing = "editing " + path.Base(env.Active)
	}
	s.text("ai none configured  ·  "+editing, Muted)
	s.text("type help to see everything, or demo for a tour", Muted)
}

// Lines is the current scrollback. Its slices are read-only.
func (s *Session) Lines() []Line { return s.lines }

func (s *Session) Prompt(name string) string {
	if s.cwd != "" {
		return s.cwd + ">"
	}
	return vaultName(name) + ">"
}

func (s *Session) WorkingDirectory(name string) string {
	if s.cwd != "" {
		return vaultName(name) + "/" + s.cwd + "/"
	}
	return vaultName(name) + "/"
}

func vaultName(name string) string {
	if name == "" {
		return "vault"
	}
	return name
}

// Submit executes a terminal command using the current workspace.
func (s *Session) Submit(input string, env Environment) Result {
	s.Start(env)
	if s.demo != nil {
		if s.demo.vault == env.Vault && strings.TrimSpace(input) == "" {
			return Result{DemoDelay: s.resumeDemo()}
		}
		s.StopDemo()
		return Result{}
	}
	return s.execute(input, env, true)
}

func (s *Session) execute(input string, env Environment, remember bool) Result {
	tokens := commandTokens(input)
	if len(tokens) == 0 {
		return Result{}
	}
	if remember {
		s.recordInput(input, env)
	} else {
		s.echo(input, s.Prompt(env.Name))
	}
	return s.dispatch(tokens, env)
}

func (s *Session) dispatch(tokens []string, env Environment) Result {
	if len(tokens) == 0 {
		return Result{}
	}
	if s.script != nil {
		if pipe := slices.Index(tokens, "|"); pipe >= 0 {
			if pipe+1 < len(tokens) && strings.EqualFold(tokens[pipe+1], "ai") {
				s.text("ai unavailable", Error)
			} else {
				s.text(`| only feeds ai → grep term | ai "summarize"`, Error)
			}
			return Result{}
		}
	}
	command, args := strings.ToLower(tokens[0]), tokens[1:]
	if operator := slices.Index(tokens, ">"); operator >= 0 {
		if operator == 0 || operator != len(tokens)-2 {
			s.text("usage: <command> > file.md   ·   <command> > this", Error)
			return Result{}
		}
		args = tokens[1:operator]
		if slices.ContainsFunc(commandHelp[:], func(item commandInfo) bool { return item.name == command }) {
			return s.redirect(command, args, tokens[operator+1], env)
		}
	}
	return s.run(command, args, env)
}

func commandTokens(input string) []string {
	words := lexWords(input)
	tokens := make([]string, 0, len(words))
	for _, word := range words {
		if word.value != "" {
			tokens = append(tokens, word.value)
		}
	}
	return tokens
}

func (s *Session) recordInput(input string, env Environment) {
	if n := len(s.history); n == 0 || s.history[n-1] != input {
		s.history = append(s.history, input)
	}
	s.echo(input, s.Prompt(env.Name))
}

func (s *Session) echo(input, prompt string) {
	if len(s.lines) != 0 {
		s.lines = append(s.lines, nil)
	}
	s.lines = append(s.lines, Line{{Text: prompt + " ", Style: Prompt}, {Text: input}})
}

// RecallFrom moves one pane through the history and restores its own draft.
func (s *Session) RecallFrom(cursor *HistoryCursor, input string, direction int) string {
	if len(s.history) == 0 || direction == 0 {
		return input
	}
	if !cursor.browsing {
		if direction > 0 {
			return input
		}
		*cursor = HistoryCursor{browsing: true, index: len(s.history), draft: input}
	}
	cursor.index = max(0, cursor.index+direction)
	if cursor.index >= len(s.history) {
		draft := cursor.draft
		*cursor = HistoryCursor{}
		return draft
	}
	return s.history[cursor.index]
}

func (s *Session) text(text string, style Style) {
	s.lines = append(s.lines, Line{{Text: text, Style: style}})
}

func (s *Session) file(prefix, relative, suffix string) {
	s.link(Span{Text: prefix}, relative, relative, suffix)
}

func (s *Session) link(prefix Span, label, relative, suffix string) {
	line := Line{}
	if prefix.Text != "" {
		line = append(line, prefix)
	}
	line = append(line, Span{Text: label, Path: relative, Style: Accent})
	if suffix != "" {
		line = append(line, Span{Text: suffix, Style: Muted})
	}
	s.lines = append(s.lines, line)
}

type commandWord struct {
	value      string
	start, end int
	quoted     bool
}

// lexWords preserves the input offsets needed to complete the last word. A
// partial quote is valid while a path is being entered.
func lexWords(input string) []commandWord {
	var words []commandWord
	var value strings.Builder
	start := -1
	var inQuote, quoted bool
	for offset, char := range input {
		if start == -1 {
			if unicode.IsSpace(char) {
				continue
			}
			start = offset
		}
		switch {
		case char == '"':
			inQuote, quoted = !inQuote, true
		case !inQuote && unicode.IsSpace(char):
			words = append(words, commandWord{value: value.String(), start: start, end: offset, quoted: quoted})
			value.Reset()
			start, quoted = -1, false
		default:
			value.WriteRune(char)
		}
	}
	if start != -1 {
		words = append(words, commandWord{value: value.String(), start: start, end: len(input), quoted: quoted})
	}
	return words
}
