package glow

import (
	"fmt"
	"path"
	"strings"

	"cyberwriter/internal/core/workspace"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

const scriptCommandLimit = 200

type scriptContext struct {
	files    int
	filesErr error
}

type scriptStepKind uint8

const (
	scriptCommand scriptStepKind = iota
	scriptComment
	scriptCheck
)

type scriptStep struct {
	kind scriptStepKind
	text string
}

type scriptEffectKind uint8

const (
	scriptOpen scriptEffectKind = iota
	scriptTheme
	scriptAppCommand
	scriptInsert
	scriptClear
	scriptDump
	scriptDemo
)

// ScriptEffect is a pending action to apply after the script's output.
type ScriptEffect struct {
	kind    scriptEffectKind
	value   string
	command AppCommand
}

// ApplyScriptEffect runs a deferred action with the live workspace and scrollback.
func (s *Session) ApplyScriptEffect(effect ScriptEffect, env Environment) Result {
	switch effect.kind {
	case scriptOpen:
		return Result{OpenPath: effect.value}
	case scriptTheme:
		return Result{Theme: effect.value}
	case scriptAppCommand:
		return Result{AppCommand: effect.command}
	case scriptInsert:
		if env.InsertMarkdown != nil {
			env.InsertMarkdown(effect.value)
		}
	case scriptClear:
		s.lines = nil
	case scriptDump:
		if env.Vault != nil {
			return s.saveTranscript(effect.value, env)
		}
	case scriptDemo:
		return Result{DemoDelay: s.startDemo(env)}
	}
	return Result{}
}

func readScriptSteps(source string) []scriptStep {
	var steps []scriptStep
	fence := ""
	// The native parser splits on standalone LF, leaving CRLF pairs intact.
	for start, end := 0, 0; end <= len(source); end++ {
		if end < len(source) && (source[end] != '\n' || end > 0 && source[end-1] == '\r') {
			continue
		}
		line := strings.TrimSpace(source[start:end])
		start = end + 1
		if fence == "" {
			language := strings.TrimLeft(line, "`")
			markerLength := len(line) - len(language)
			if markerLength >= 3 && strings.EqualFold(strings.TrimSpace(language), "cw") {
				fence = line[:markerLength]
			}
			continue
		}
		if line == fence {
			fence = ""
			continue
		}
		if expected, ok := strings.CutPrefix(line, "#="); ok {
			if expected = strings.TrimSpace(expected); expected != "" {
				steps = append(steps, scriptStep{scriptCheck, expected})
			}
		} else if comment, ok := strings.CutPrefix(line, "#"); ok {
			steps = append(steps, scriptStep{scriptComment, strings.TrimSpace(comment)})
		} else if line != "" {
			steps = append(steps, scriptStep{scriptCommand, line})
		}
	}
	return steps
}

func (s *Session) runScript(query string, env Environment) Result {
	switch {
	case query == "":
		s.text("usage: run <note> - executes the note's ```cw blocks", Error)
	case s.script != nil:
		s.text("scripts can't run scripts", Error)
	case env.Vault == nil:
		s.text("no vault open", Error)
	default:
		return s.runScriptNote(query, env)
	}
	return Result{}
}

func (s *Session) runScriptNote(query string, env Environment) Result {
	relative := s.scriptPath(query, env)
	if relative == "" {
		s.text("no match for: "+query, Error)
		return Result{}
	}
	if !workspace.IsMarkdownPath(relative) {
		s.text("scripts live in markdown notes", Error)
		return Result{}
	}
	note, err := env.Vault.ReadDocument(relative)
	if err != nil {
		s.text("could not read "+relative, Error)
		return Result{}
	}
	steps := readScriptSteps(note.Text)
	if len(steps) == 0 {
		s.text("no ```cw blocks in "+relative+" - wrap commands in a ```cw fence", Error)
		return Result{}
	}
	s.lines = append(s.lines, Line{{Text: "── run ", Style: ScriptText}, {Text: relative, Path: relative, Style: ScriptMarker}})
	files, filesErr := countVaultFiles(env.Vault)
	child := Session{cwd: s.cwd, script: &scriptContext{files: files, filesErr: filesErr}}
	result := Result{}
	var commands, passed, failed int
	var capped bool
	var lastOutput []Line
	for _, step := range steps {
		if step.kind == scriptCommand && commands == scriptCommandLimit {
			capped = true
			break
		}
		switch step.kind {
		case scriptComment:
			s.text("# "+step.text, ScriptMuted)
		case scriptCheck:
			if scriptContains(lastOutput, step.text) {
				passed++
				s.lines = append(s.lines, Line{{Text: "  ✓ ", Style: ScriptMarker}, {Text: `contains "` + step.text + `"`, Style: ScriptMuted}})
			} else {
				failed++
				s.text(`  ✗ expected "`+step.text+`"`, ScriptError)
			}
		case scriptCommand:
			commands++
			s.text(" ", Plain)
			s.lines = append(s.lines, Line{{Text: "» ", Style: ScriptMuted}, {Text: step.text, Style: ScriptText}})
			child.lines = nil
			var outcome Result
			if strings.HasPrefix(step.text, "/") {
				outcome.AppCommand = child.resolveAppCommand(step.text, env.AppCommandAvailable)
			} else {
				outcome = child.dispatch(commandTokens(step.text), env)
			}
			for _, line := range child.lines {
				for i, span := range line {
					if span.Style == Plain {
						line[i].Style = ScriptText
					}
				}
			}
			lastOutput = child.lines
			s.lines = append(s.lines, lastOutput...)
			result.VaultChanged = result.VaultChanged || outcome.VaultChanged
			result.ScriptEffects = appendScriptEffects(result.ScriptEffects, outcome)
		}
	}
	s.cwd = child.cwd
	s.text(" ", Plain)
	suffix := "s"
	if commands == 1 {
		suffix = ""
	}
	summary := Line{{Text: fmt.Sprintf("── done - %d command%s", commands, suffix), Style: ScriptText}}
	if passed+failed > 0 {
		summary = append(summary, Span{Text: "  ·  ", Style: ScriptMuted}, Span{Text: fmt.Sprintf("%d ✓", passed), Style: ScriptMarker})
	}
	if failed > 0 {
		summary = append(summary, Span{Text: "  ·  ", Style: ScriptMuted}, Span{Text: fmt.Sprintf("%d ✗", failed), Style: ScriptError})
	}
	if capped {
		summary = append(summary, Span{Text: "  (stopped at the 200-command cap)", Style: ScriptMuted})
	}
	s.lines = append(s.lines, summary)
	return result
}

func appendScriptEffects(effects []ScriptEffect, outcome Result) []ScriptEffect {
	switch {
	case outcome.OpenPath != "":
		return append(effects, ScriptEffect{kind: scriptOpen, value: outcome.OpenPath})
	case outcome.Theme != "":
		return append(effects, ScriptEffect{kind: scriptTheme, value: outcome.Theme})
	case outcome.AppCommand.Slug != "":
		return append(effects, ScriptEffect{kind: scriptAppCommand, command: outcome.AppCommand})
	default:
		return append(effects, outcome.ScriptEffects...)
	}
}

var scriptCaseFold = cases.Fold()

func scriptContains(lines []Line, expected string) bool {
	var output strings.Builder
	for i, line := range lines {
		if i > 0 {
			output.WriteByte('\n')
		}
		for _, span := range line {
			output.WriteString(span.Text)
		}
	}
	return strings.Contains(foldScriptText(output.String()), foldScriptText(expected))
}

func foldScriptText(text string) string {
	return norm.NFC.String(scriptCaseFold.String(norm.NFD.String(text)))
}

func (s *Session) scriptPath(query string, env Environment) string {
	if strings.EqualFold(query, "this") {
		return env.Active
	}
	if strings.HasPrefix(query, "@") {
		return ""
	}
	if root, ok := strings.CutPrefix(query, "~/"); ok {
		query = "/" + root
	}
	relative, valid := outputPath(s.cwd, query)
	if !valid {
		return ""
	}
	if exact := exactVaultFile(env.Vault, relative); exact != "" {
		return exact
	}
	rootRelative, rootValid := outputPath("", query)
	if rootValid && rootRelative != relative {
		if exact := exactVaultFile(env.Vault, rootRelative); exact != "" {
			return exact
		}
	}
	if rootRelative == "" {
		return ""
	}
	query = strings.ToLower(query)
	if !strings.ContainsRune(query, '/') {
		if matches := matchingNotes(env.Entries, query); len(matches) > 0 {
			best := matches[0].Path
			if strings.Contains(strings.ToLower(path.Base(best)), query) {
				return best
			}
		}
	}
	query = strings.ToLower(rootRelative)
	var best string
	walkFiles(env.Entries, func(entry workspace.Entry) {
		if workspace.IsMarkdownPath(entry.Path) && strings.Contains(strings.ToLower(entry.Path), query) &&
			(best == "" || len(entry.Path) < len(best) || len(entry.Path) == len(best) && entry.Path < best) {
			best = entry.Path
		}
	})
	return best
}

func exactVaultFile(vault *workspace.Vault, relative string) string {
	if relative == "" {
		return ""
	}
	parent, base := path.Split(relative)
	entries, err := vault.TerminalEntries(strings.TrimSuffix(parent, "/"), 0)
	if err != nil {
		return ""
	}
	candidates := []string{base, base + ".md", base + ".markdown"}
	for _, candidate := range candidates {
		if entry, ok := entryAt(entries, candidate); ok && !entry.IsDir {
			return entry.Path
		}
	}
	for _, candidate := range candidates {
		for _, entry := range entries {
			if !entry.IsDir && strings.EqualFold(entry.Name, candidate) {
				return entry.Path
			}
		}
	}
	return ""
}
