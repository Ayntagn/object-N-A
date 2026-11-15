package glow

import (
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
)

type completion struct {
	value string
	path  string
	dir   bool
}

// Complete expands the last word, or lists the matching words in scrollback.
func (s *Session) Complete(input string, env Environment) string {
	s.Start(env)
	words := lexWords(input)
	word := commandWord{start: len(input), end: len(input)}
	command := len(words) == 0
	if len(words) > 0 && words[len(words)-1].end == len(input) {
		word = words[len(words)-1]
		command = len(words) == 1
	}
	var candidates []completion
	switch {
	case command:
		for _, item := range commandHelp {
			if matchesPrefix(item.name, word.value) {
				candidates = append(candidates, completion{value: item.name})
			}
		}
	case strings.HasPrefix(word.value, "@"):
		if matchesPrefix("@this", word.value) {
			candidates = append(candidates, completion{value: "@this", path: env.Active})
		}
		walkFiles(env.Entries, func(entry workspace.Entry) {
			if workspace.IsMarkdownPath(entry.Path) {
				name := "@" + entry.Path
				if ext := path.Ext(entry.Path); strings.EqualFold(ext, ".md") {
					name = strings.TrimSuffix(name, ext)
				}
				if name != "@this" && matchesPrefix(name, word.value) {
					candidates = append(candidates, completion{value: name, path: entry.Path})
				}
			}
		})
	default:
		if env.Vault != nil {
			candidates = s.completePath(word.value, env.Vault)
		}
	}
	slices.SortFunc(candidates, func(a, b completion) int { return strings.Compare(a.value, b.value) })
	if len(candidates) == 0 {
		return input
	}
	if len(candidates) == 1 {
		item := candidates[0]
		return input[:word.start] + completedWord(item.value, word.quoted, !item.dir)
	}
	common := candidates[0].value
	for _, item := range candidates[1:] {
		common = commonPrefix(common, item.value)
	}
	if len(common) > len(word.value) {
		return input[:word.start] + completedWord(common, word.quoted, false)
	}
	var line Line
	for i, item := range candidates[:min(12, len(candidates))] {
		if i > 0 {
			line = append(line, Span{Text: "   "})
		}
		line = append(line, Span{Text: item.value, Path: item.path, Style: Accent})
	}
	s.lines = append(s.lines, line)
	return input
}

func (s *Session) completePath(prefix string, vault *workspace.Vault) []completion {
	split := strings.LastIndexByte(prefix, '/') + 1
	folderPrefix, childPrefix := prefix[:split], prefix[split:]
	children, err := vault.TerminalEntries(vaultPath(s.cwd, folderPrefix), 0)
	if err != nil {
		return nil
	}
	var matches []completion
	for _, child := range children {
		if !matchesPrefix(child.Name, childPrefix) {
			continue
		}
		item := completion{value: folderPrefix + child.Name, dir: child.IsDir}
		if child.IsDir {
			item.value += "/"
		} else {
			item.path = child.Path
		}
		matches = append(matches, item)
	}
	return matches
}

func matchesPrefix(value, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix))
}

func commonPrefix(a, b string) string {
	for offset, char := range a {
		if b == "" {
			return a[:offset]
		}
		other, size := utf8.DecodeRuneInString(b)
		if unicode.ToLower(char) != unicode.ToLower(other) {
			return a[:offset]
		}
		b = b[size:]
	}
	return a
}

func completedWord(value string, quoted, finished bool) string {
	value, at := strings.CutPrefix(value, "@")
	if quoted || strings.ContainsFunc(value, unicode.IsSpace) {
		value = `"` + value
		if finished {
			value += `"`
		}
	}
	if at {
		value = "@" + value
	}
	if finished {
		value += " "
	}
	return value
}
