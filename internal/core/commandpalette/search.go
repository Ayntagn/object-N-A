// Package commandpalette describes and ranks the workspace's command catalog.
package commandpalette

import (
	"slices"
	"strings"
)

type Command struct {
	Name        string
	Description string
	Shortcut    string
	Keywords    []string
}

type commandSection struct {
	name     string
	commands []Command
}

type Match struct {
	Section string
	Command
}

// Search preserves section order and ranks the matching commands within each section.
func Search(input string) []Match {
	query := strings.ToLower(strings.TrimSpace(input))
	terms := strings.Fields(query)
	type scoredCommand struct {
		command Command
		score   int
	}
	var matches []Match
	var ranked []scoredCommand
	for _, section := range catalog {
		ranked = ranked[:0]
		for _, command := range section.commands {
			score := matchScore(command, query, terms)
			if score > 0 {
				ranked = append(ranked, scoredCommand{command, score})
			}
		}
		slices.SortStableFunc(ranked, func(a, b scoredCommand) int { return b.score - a.score })
		for _, entry := range ranked {
			matches = append(matches, Match{Section: section.name, Command: entry.command})
		}
	}
	return matches
}

// EV-UI-076: the native filter tries these ranks in descending order.
func matchScore(command Command, query string, terms []string) int {
	if query == "" {
		return 100
	}
	name := strings.ToLower(command.Name)
	description := strings.ToLower(command.Description)
	switch {
	case strings.HasPrefix(name, query):
		return 100
	case slices.ContainsFunc(command.Keywords, func(keyword string) bool { return strings.HasPrefix(keyword, query) }):
		return 80
	case strings.Contains(name, query):
		return 60
	case slices.ContainsFunc(command.Keywords, func(keyword string) bool { return strings.Contains(keyword, query) }):
		return 40
	case strings.Contains(description, query):
		return 20
	case len(terms) > 1:
		fields := name + " " + description + " " + strings.Join(command.Keywords, " ")
		for _, term := range terms {
			if !strings.Contains(fields, term) {
				return 0
			}
		}
		return 10
	default:
		return 0
	}
}
