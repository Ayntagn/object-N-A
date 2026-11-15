package glow

import "strings"

// AppCommand is a workspace action advertised by the terminal.
type AppCommand struct {
	Slug, Title, Category string
	keywords              []string
}

// SlashMenu is the visible set of suggestions above the terminal input.
type SlashMenu struct {
	Rows []AppCommand
	More int
	Open bool
}

// SlashSuggestions ranks results in the order observed in EV-UI-078 and EV-UI-079.
func SlashSuggestions(input string) SlashMenu {
	query, ok := strings.CutPrefix(input, "/")
	if !ok || strings.ContainsRune(query, ' ') {
		return SlashMenu{}
	}
	query = strings.ToLower(query)
	var prefix, text, keyword []AppCommand
	for _, command := range appCommands {
		switch {
		case strings.HasPrefix(command.Slug, query):
			prefix = append(prefix, command)
		case command.matchesText(query):
			text = append(text, command)
		case command.matchesKeyword(query):
			keyword = append(keyword, command)
		}
	}
	matches := append(append(prefix, text...), keyword...)
	shown := min(6, len(matches))
	return SlashMenu{Rows: matches[:shown], More: len(matches) - shown, Open: true}
}

func (command AppCommand) matchesText(query string) bool {
	return strings.Contains(command.Slug, query) || strings.Contains(strings.ToLower(command.Title), query)
}

func (command AppCommand) matchesKeyword(query string) bool {
	for _, keyword := range command.keywords {
		if strings.Contains(keyword, query) {
			return true
		}
	}
	return false
}

func lookupAppCommands(query string) []AppCommand {
	var prefix, partial []AppCommand
	for _, command := range appCommands {
		switch {
		case command.Slug == query:
			return []AppCommand{command}
		case strings.HasPrefix(command.Slug, query):
			prefix = append(prefix, command)
		case command.matchesText(query) || command.matchesKeyword(query):
			partial = append(partial, command)
		}
	}
	if len(prefix) > 0 {
		return prefix
	}
	return partial
}

// SubmitApp records a slash invocation and returns an action for the owning
// workspace. The terminal engine does not call into the UI itself.
func (s *Session) SubmitApp(input string, env Environment, available func(AppCommand) bool) AppCommand {
	s.Start(env)
	input = strings.TrimSpace(input)
	s.recordInput(input, env)
	return s.resolveAppCommand(input, available)
}

func (s *Session) resolveAppCommand(input string, available func(AppCommand) bool) AppCommand {
	words := lexWords(strings.TrimPrefix(input, "/"))
	if len(words) == 0 {
		s.text("type / for the menu", Muted)
		return AppCommand{}
	}
	query := strings.ToLower(words[0].value)
	matches := lookupAppCommands(query)
	switch len(matches) {
	case 0:
		s.text("no app command matches /"+query+"   ·   type / for the menu", Error)
	case 1:
		command := matches[0]
		if available != nil && !available(command) {
			s.text("app command unavailable /"+command.Slug, Error)
			return AppCommand{}
		}
		s.text("→ "+command.Title, Accent)
		if len(words) > 1 {
			s.text("(app commands take no arguments yet - extra words ignored)", Muted)
		}
		return command
	default:
		s.text("which one?", Muted)
		for _, command := range matches[:min(10, len(matches))] {
			s.lines = append(s.lines, Line{{Text: "  /" + command.Slug, Style: Accent}, {Text: "  " + command.Title}})
		}
	}
	return AppCommand{}
}
