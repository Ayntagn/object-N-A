package glow

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

func (s *Session) create(command string, args []string, env Environment) Result {
	if len(args) == 0 {
		usage := command + " <path>"
		for _, item := range commandHelp {
			if item.name == command {
				usage = item.usage
				break
			}
		}
		s.text("usage: "+usage, Error)
		return Result{}
	}
	relative, valid := outputPath(s.cwd, args[0])
	if !valid || (command == "new" && relative == "") {
		s.text("invalid Vault path: "+args[0], Error)
		return Result{}
	}
	if command == "mkdir" {
		if relative != "" {
			if err := env.Vault.EnsureFolder(relative); err != nil {
				s.text("mkdir failed: "+err.Error(), Error)
				return Result{}
			}
		}
		s.lines = append(s.lines, Line{{Text: "created ", Style: Muted}, {Text: relative + "/", Style: Prompt}})
		return Result{VaultChanged: relative != ""}
	}

	relative = withDefaultExtension(relative)
	title := strings.Join(args[1:], " ")
	if title == "" {
		title = noteTitle(relative)
	}
	if _, err := env.Vault.CreateTextDocument(relative, "# "+title+"\n\n"); err != nil {
		if errors.Is(err, fs.ErrExist) {
			s.link(Span{Text: "already exists - ", Style: Error}, relative, relative, "")
		} else {
			s.text("new failed: "+err.Error(), Error)
		}
		return Result{}
	}
	s.link(Span{Text: "created ", Style: Muted}, relative, relative, "")
	return Result{OpenPath: relative, VaultChanged: true}
}

func (s *Session) redirect(command string, args []string, target string, env Environment) Result {
	if command == "dump" || command == "demo" {
		// The effect consumes the live session after the redirect reports empty output.
		s.text("nothing to write", Muted)
		return s.run(command, args, env)
	}
	scrollback := s.lines
	s.lines = nil
	result := s.run(command, args, env)
	output := s.lines
	if command == "clear" {
		return result
	}
	s.lines = scrollback
	if len(output) == 0 {
		s.text("nothing to write", Muted)
		return result
	}
	result.OpenPath = ""
	result.Theme = ""
	result.ScriptEffects = nil
	payload := redirectMarkdown(output)
	count := strings.Count(payload, "\n") + 1
	if strings.EqualFold(target, "this") || strings.EqualFold(target, "@this") {
		if env.Active == "" {
			s.text("no current note - open one first", Error)
		} else {
			if s.script != nil {
				result.ScriptEffects = []ScriptEffect{{kind: scriptInsert, value: payload + "\n"}}
			} else if env.InsertMarkdown == nil || !env.InsertMarkdown(payload+"\n") {
				s.text("cannot insert this output in the current editor", Error)
				return result
			}
			s.text(fmt.Sprintf("→ inserted %d lines into %s", count, path.Base(env.Active)), Plain)
		}
		return result
	}
	if env.Vault == nil {
		s.text("no vault open", Error)
		return result
	}
	relative, valid := outputPath(s.cwd, target)
	if !valid || relative == "" {
		s.text("invalid Vault path: "+target, Error)
		return result
	}
	relative = withDefaultExtension(relative)
	if _, err := env.Vault.CreateTextDocument(relative, payload+"\n"); err != nil {
		if errors.Is(err, fs.ErrExist) {
			s.link(Span{Text: "won't overwrite - ", Style: Error}, relative, relative, "  (pick a new name, or use > this)")
		} else {
			s.text("write failed: "+err.Error(), Error)
		}
		return result
	}
	s.link(Span{Text: fmt.Sprintf("wrote %d lines → ", count), Style: Muted}, relative, relative, "")
	result.VaultChanged = true
	return result
}

func redirectMarkdown(lines []Line) string {
	var out strings.Builder
	for index, line := range lines {
		if index > 0 {
			out.WriteByte('\n')
		}
		for _, span := range line {
			if strings.EqualFold(path.Ext(span.Path), ".md") {
				indent := len(span.Text) - len(strings.TrimLeft(span.Text, " "))
				out.WriteString(span.Text[:indent])
				out.WriteString("[[" + span.Path[:len(span.Path)-3] + "]]")
			} else {
				out.WriteString(span.Text)
			}
		}
	}
	return out.String()
}

// outputPath resolves a GLOW write path within the current Vault.
func outputPath(cwd, input string) (string, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", false
	}
	var relative string
	switch {
	case strings.HasPrefix(input, "~/"):
		relative = path.Clean(strings.TrimLeft(input[2:], "/"))
	case strings.HasPrefix(input, "/"):
		relative = path.Clean(strings.TrimLeft(input, "/"))
	default:
		relative = path.Join(cwd, input)
	}
	if relative == ".." || strings.HasPrefix(relative, "../") {
		return "", false
	}
	if relative == "." {
		return "", true
	}
	return relative, true
}

func withDefaultExtension(relative string) string {
	if fileExtension(relative) == "" {
		return relative + ".md"
	}
	return relative
}

func fileExtension(relative string) string {
	ext := path.Ext(relative)
	if ext == "." || ext == path.Base(relative) {
		return ""
	}
	return ext
}
