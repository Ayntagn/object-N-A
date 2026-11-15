package glow

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"
)

func (s *Session) move(args []string, env Environment) Result {
	if len(args) != 2 {
		s.text("usage: mv <src> <dst>", Error)
		return Result{}
	}
	from, sourceOK := outputPath(s.cwd, currentFileArgument(args[0], env.Active))
	to, destOK := outputPath(s.cwd, currentFileArgument(args[1], env.Active))
	if !sourceOK || !destOK || from == "" || to == "" {
		s.text("path escapes the vault", Error)
		return Result{}
	}
	if _, err := env.Vault.StatEntry(from); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.text("no such file: "+from, Error)
		} else {
			s.text("mv failed: "+err.Error(), Error)
		}
		return Result{}
	}
	info, err := env.Vault.StatEntry(to)
	switch {
	case err == nil && info.IsDir():
		to = path.Join(to, path.Base(from))
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		s.text("mv failed: "+err.Error(), Error)
		return Result{}
	case fileExtension(to) == "":
		to += fileExtension(from)
	}
	if err := env.Vault.MoveEntry(from, to); err != nil {
		if errors.Is(err, fs.ErrExist) {
			s.text("destination exists: "+to, Error)
		} else {
			s.text("mv failed: "+err.Error(), Error)
		}
		return Result{}
	}
	s.link(Span{Text: from + " → ", Style: Muted}, to, to, "")
	return Result{VaultChanged: true}
}

func currentFileArgument(arg, active string) string {
	if active != "" && strings.EqualFold(arg, "this") {
		return active
	}
	return arg
}

func (s *Session) trash(args []string, env Environment) Result {
	if len(args) == 0 {
		s.text("usage: rm <path>", Error)
		return Result{}
	}
	relative, valid := outputPath(s.cwd, currentFileArgument(args[0], env.Active))
	switch {
	case !valid:
		s.text("path escapes the vault", Error)
		return Result{}
	case relative == "":
		s.text("refusing to trash the vault root", Error)
		return Result{}
	}
	destination, err := env.Vault.TrashEntry(relative, env.TrashItem)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.text("no such file: "+relative, Error)
		} else {
			s.text("trash failed: "+err.Error(), Error)
		}
		return Result{}
	}
	s.trashReceipt(relative, destination)
	return Result{VaultChanged: true}
}

func (s *Session) trashReceipt(relative, destination string) {
	s.lines = append(s.lines, Line{{Text: relative, Style: Accent}, {Text: " → " + destination + "  (recoverable)", Style: Muted}})
}

var archiveFilename = strings.NewReplacer("/", "-", "\\", "-", ":", "-")

func (s *Session) dump(args []string, env Environment) Result {
	for i := range args {
		args[i] = currentFileArgument(args[i], env.Active)
	}
	name := archiveFilename.Replace(strings.TrimSpace(strings.Join(args, " ")))
	name = strings.TrimSuffix(name, ".md")
	if s.script != nil {
		return Result{ScriptEffects: []ScriptEffect{{kind: scriptDump, value: name}}}
	}
	return s.saveTranscript(name, env)
}

func (s *Session) saveTranscript(name string, env Environment) Result {
	date := time.Now().Format(time.DateOnly)
	if name == "" {
		name = "GLOW session " + date
	}
	transcript, commands := s.transcript()
	fence := transcriptFence(transcript)
	body := "---\ntags: [glow, session]\ncreated: " + date + "\n---\n\n" +
		"# GLOW session - " + vaultName(env.Name) + "\n\n" +
		"*Saved from the GLOW terminal on " + date + " - every command and its output.*\n\n" +
		fence + "text\n" + transcript + "\n" + fence
	for number := 1; ; number++ {
		filename := name + ".md"
		if number > 1 {
			filename = fmt.Sprintf("%s %d.md", name, number)
		}
		_, err := env.Vault.CreateTextDocument(filename, body)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			s.text("dump failed: "+err.Error(), Error)
			return Result{}
		}
		s.link(Span{Text: "✓ saved ", Style: Prompt}, filename, filename, fmt.Sprintf("  ·  %d commands", commands))
		return Result{OpenPath: filename, VaultChanged: true}
	}
}

func (s *Session) transcript() (string, int) {
	var out strings.Builder
	commands := 0
	for _, line := range s.lines {
		if len(line) > 1 && line[0].Style == Prompt {
			if directory, ok := strings.CutSuffix(line[0].Text, "> "); ok {
				out.WriteString(directory + " ▸ ")
				line = line[1:]
				commands++
			}
		}
		for _, span := range line {
			out.WriteString(span.Text)
		}
		out.WriteByte('\n')
	}
	return strings.TrimSpace(out.String()), commands
}

func transcriptFence(transcript string) string {
	run, longest := 0, 0
	for _, char := range transcript {
		if char == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}
