package glow

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	"github.com/go-text/typesetting/segmenter"
)

type commandInfo struct{ name, usage, detail string }

var commandHelp = [...]commandInfo{
	{"ls", "ls [path] [-t]", "list folder (-t: by modified)"},
	{"cd", "cd [path]", "change folder (cd .. to go up)"},
	{"pwd", "pwd", "print current folder"},
	{"tree", "tree [path] [-d n]", "folder tree (-d: max depth)"},
	{"open", "open <note>", "open a note in the editor"},
	{"cat", "cat <note>", "print a note's contents"},
	{"new", "new <path> [Title…]", "create + open a note"},
	{"mkdir", "mkdir <path>", "create a folder"},
	{"mv", "mv <src> <dst>", "move / rename"},
	{"rm", "rm <path>", "move to Trash (undo in Finder)"},
	{"grep", "grep <term> [path]", "search note contents"},
	{"find", "find <query>", "fuzzy-find notes by name"},
	{"recent", "recent", "recently opened notes"},
	{"backlinks", "backlinks [note]", "notes linking to a note"},
	{"links", "links [note]", "notes a note links to"},
	{"tags", "tags", "list all tags"},
	{"tag", "tag <name>", "notes carrying a tag"},
	{"wc", "wc [note]", "word count (default: current note)"},
	{"stats", "stats", "vault totals"},
	{"this", "this", "print current doc's filename"},
	{"run", "run <note>", "execute the note's cw blocks"},
	{"theme", "theme [plato|mono|matrix]", "switch terminal theme"},
	{"clear", "clear", "clear the screen"},
	{"dump", "dump [name]", "save this session to a note"},
	{"demo", "demo", "play the guided tour"},
	{"help", "help", "list commands"},
}

func (s *Session) run(command string, args []string, env Environment) Result {
	switch command {
	case "help":
		s.text("COMMANDS", Accent)
		for _, item := range commandHelp {
			s.text(fmt.Sprintf("  %-34s %s", item.usage, item.detail), Plain)
		}
		s.text(`quotes group words → grep "orange glow"   ·   TAB completes paths and @notes   ·   this = the active note`, Muted)
		s.text("backlinks this > related.md   ·   <command> > this inserts into the active note", Muted)
		return Result{}
	case "clear":
		if s.script != nil {
			return Result{ScriptEffects: []ScriptEffect{{kind: scriptClear}}}
		}
		s.lines = nil
		return Result{}
	case "pwd":
		s.text(s.WorkingDirectory(env.Name), Plain)
		return Result{}
	case "theme":
		if len(args) == 0 {
			s.text("themes → plato, mono, matrix", Muted)
			return Result{}
		}
		theme := strings.ToLower(args[0])
		switch theme {
		case "plato", "mono", "matrix":
			s.lines = append(s.lines, Line{{Text: "theme → ", Style: Muted}, {Text: theme, Style: Accent}})
			return Result{Theme: theme}
		default:
			s.text("unknown theme: "+theme+" (plato, mono, matrix)", Error)
			return Result{}
		}
	case "demo":
		if s.script != nil {
			return Result{ScriptEffects: []ScriptEffect{{kind: scriptDemo}}}
		}
		return Result{DemoDelay: s.startDemo(env)}
	case "run":
		return s.runScript(strings.Join(args, " "), env)
	}

	if env.Vault == nil {
		s.text("no vault open", Error)
		return Result{}
	}
	arg := strings.Join(args, " ")
	switch command {
	case "cd":
		if arg == "" {
			arg = "/"
		}
		relative := vaultPath(s.cwd, arg)
		entry, exists := entryAt(env.Entries, relative)
		isDirectory := exists && entry.IsDir
		if s.script != nil && relative != "" {
			info, err := env.Vault.StatEntry(relative)
			isDirectory = err == nil && info.IsDir()
		}
		if !isDirectory {
			if s.script != nil {
				s.text("no such folder: "+arg, Error)
			} else {
				s.text("folder not found: "+arg, Error)
			}
			return Result{}
		}
		s.cwd = relative
		s.text(s.WorkingDirectory(env.Name), Plain)
	case "ls", "tree":
		s.list(command, args, env)
	case "new", "mkdir":
		return s.create(command, args, env)
	case "mv":
		return s.move(args, env)
	case "rm":
		return s.trash(args, env)
	case "dump":
		return s.dump(args, env)
	case "find":
		if arg == "" {
			s.text("usage: find <query>", Error)
			return Result{}
		}
		matches := matchingNotes(env.Entries, arg)
		for _, entry := range matches {
			s.file("", entry.Path, "")
		}
		if len(matches) == 0 {
			s.text("no matches for: "+arg, Muted)
		}
	case "grep":
		s.grep(args, env)
	case "cat", "open", "wc", "this", "links", "backlinks":
		return Result{OpenPath: s.noteCommand(command, arg, env)}
	case "tags", "tag":
		s.tags(command, arg, env)
	case "recent":
		for _, relative := range env.Recent {
			if entry, exists := entryAt(env.Entries, relative); exists && !entry.IsDir {
				s.file("", relative, "")
			}
		}
	case "stats":
		var files int
		var err error
		if s.script != nil {
			files, err = s.script.files, s.script.filesErr
		} else {
			files, err = countVaultFiles(env.Vault)
		}
		if err != nil {
			s.text("stats failed: "+err.Error(), Error)
			return Result{}
		}
		stats := vaultStats(env)
		s.text(vaultName(env.Name), Accent)
		s.text(fmt.Sprintf("  %d notes · %d files · %d links · %d tags", stats.notes, files, stats.links, stats.tags), Plain)
	default:
		if s.script != nil {
			s.text("unknown command: "+command, Error)
			s.text("HELP → type help", Muted)
		} else {
			s.text("unknown command: "+command+". Type help for available commands.", Error)
		}
	}
	return Result{}
}

func countVaultFiles(vault *workspace.Vault) (int, error) {
	entries, err := vault.ReportEntries()
	var count int
	walkFiles(entries, func(workspace.Entry) { count++ })
	return count, err
}

type stats struct{ notes, links, tags int }

func vaultStats(env Environment) stats {
	if env.Index != nil {
		graph := env.Index.Graph()
		return stats{notes: len(graph.Notes), links: graph.Links, tags: len(graph.Tags)}
	}
	var result stats
	walkFiles(env.Entries, func(entry workspace.Entry) {
		if workspace.IsMarkdownPath(entry.Path) {
			result.notes++
		}
	})
	return result
}

func vaultPath(cwd, relative string) string {
	if !strings.HasPrefix(relative, "/") {
		relative = "/" + path.Join(cwd, relative)
	}
	return strings.TrimPrefix(path.Clean(relative), "/")
}

func entryAt(entries []workspace.Entry, relative string) (workspace.Entry, bool) {
	if relative == "" {
		return workspace.Entry{IsDir: true, Children: entries}, true
	}
	var entry workspace.Entry
	for part := range strings.SplitSeq(relative, "/") {
		at := slices.IndexFunc(entries, func(candidate workspace.Entry) bool { return candidate.Name == part })
		if at < 0 {
			return workspace.Entry{}, false
		}
		entry = entries[at]
		entries = entry.Children
	}
	return entry, true
}

func walkFiles(entries []workspace.Entry, visit func(workspace.Entry)) {
	for _, entry := range entries {
		if entry.IsDir {
			walkFiles(entry.Children, visit)
		} else {
			visit(entry)
		}
	}
}

func (s *Session) list(command string, args []string, env Environment) {
	var folderWords []string
	var byTime bool
	var depth int
	for at := 0; at < len(args); at++ {
		switch {
		case command == "ls" && args[at] == "-t":
			byTime = true
		case command == "tree" && args[at] == "-d":
			at++
			if at == len(args) {
				s.text("usage: tree [path] [-d n]", Error)
				return
			}
			n, err := strconv.Atoi(args[at])
			if err != nil || n < 1 {
				s.text("tree depth must be a positive integer", Error)
				return
			}
			depth = n
		default:
			folderWords = append(folderWords, args[at])
		}
	}
	arg := strings.Join(folderWords, " ")
	relative := vaultPath(s.cwd, arg)
	scanDepth := depth
	if command == "tree" && depth == 0 {
		scanDepth = -1
	}
	entries, err := env.Vault.TerminalEntries(relative, scanDepth)
	if err != nil {
		if command == "ls" && arg == "" {
			s.text("no such folder: .", Error)
			return
		}
		s.text("folder not found: "+arg, Error)
		return
	}
	if command == "tree" {
		label := path.Base(relative)
		if relative == "" {
			label = vaultName(env.Name)
		}
		s.text(label, Accent)
		folders, files := s.tree(entries, "", depth)
		s.text(fmt.Sprintf("%d folders · %d files", folders, files), Muted)
		return
	}
	if byTime {
		slices.SortStableFunc(entries, func(a, b workspace.Entry) int { return b.ModTime().Compare(a.ModTime()) })
	} else {
		slices.SortStableFunc(entries, func(a, b workspace.Entry) int {
			if a.IsDir != b.IsDir {
				if a.IsDir {
					return -1
				}
				return 1
			}
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})
	}
	now := time.Now()
	for _, child := range entries {
		if child.IsDir {
			s.text(child.Name+"/", Prompt)
			continue
		}
		age := ""
		if byTime && !child.ModTime().IsZero() {
			age = "   " + timeAgo(now.Sub(child.ModTime()))
		}
		s.link(Span{}, child.Name, child.Path, age)
	}
}

func (s *Session) tree(entries []workspace.Entry, prefix string, depth int) (folders, files int) {
	for i, entry := range entries {
		branch, indent := "├── ", "│   "
		if i == len(entries)-1 {
			branch, indent = "└── ", "    "
		}
		if !entry.IsDir {
			files++
			s.link(Span{Text: prefix + branch}, entry.Name, entry.Path, "")
			continue
		}
		folders++
		s.text(prefix+branch+entry.Name+"/", Accent)
		if len(entry.Children) == 0 {
			continue
		}
		if depth == 1 {
			s.text(prefix+indent+"…", Muted)
			continue
		}
		childFolders, childFiles := s.tree(entry.Children, prefix+indent, max(0, depth-1))
		folders += childFolders
		files += childFiles
	}
	return folders, files
}

func timeAgo(age time.Duration) string {
	switch {
	case age >= 24*time.Hour:
		return fmt.Sprintf("%dd ago", age/(24*time.Hour))
	case age >= time.Hour:
		return fmt.Sprintf("%dh ago", age/time.Hour)
	case age >= time.Minute:
		return fmt.Sprintf("%dm ago", age/time.Minute)
	default:
		return "just now"
	}
}

func (s *Session) grep(args []string, env Environment) {
	if len(args) == 0 || args[0] == "" {
		s.text("usage: grep <term> [path]", Error)
		return
	}
	entries := env.Entries
	if len(args) > 1 {
		arg := strings.Join(args[1:], " ")
		entry, exists := entryAt(entries, vaultPath(s.cwd, arg))
		if !exists {
			s.text("path not found: "+arg, Error)
			return
		}
		entries = []workspace.Entry{entry}
	}
	term := strings.ToLower(args[0])
	matches := 0
	walkFiles(entries, func(entry workspace.Entry) {
		if !workspace.IsMarkdownPath(entry.Path) {
			return
		}
		doc, err := env.Vault.ReadDocument(entry.Path)
		if err != nil {
			return
		}
		number, fileMatches := 0, 0
		for line := range strings.SplitSeq(doc.Text, "\n") {
			number++
			if !strings.Contains(strings.ToLower(line), term) {
				continue
			}
			if fileMatches == 0 {
				s.file("", entry.Path, "")
			}
			fileMatches++
			matches++
			s.text(fmt.Sprintf("  %d: %s", number, strings.TrimSpace(line)), Plain)
			if fileMatches == 5 {
				break
			}
		}
	})
	s.text(fmt.Sprintf("%d matches", matches), Muted)
}

func (s *Session) noteCommand(command, arg string, env Environment) string {
	if command == "this" || arg == "" && (command == "wc" || command == "links" || command == "backlinks") {
		arg = "this"
	}
	if arg == "" {
		s.text("usage: "+command+" <note>", Error)
		return ""
	}
	relative := s.notePath(arg, env)
	if relative == "" {
		if command == "this" {
			s.text("no current note - open one first", Error)
		} else if arg == "this" || arg == "@this" {
			s.text("no note open", Error)
		} else if command == "cat" {
			s.text("no match for: "+arg, Error)
		} else {
			s.text("note not found: "+arg, Error)
		}
		return ""
	}
	if command != "this" && (strings.EqualFold(arg, "this") || strings.EqualFold(arg, "@this")) {
		if _, err := env.Vault.StatEntry(relative); errors.Is(err, fs.ErrNotExist) {
			if command == "open" || command == "cat" {
				s.text("no match for: "+relative, Error)
			} else {
				s.text("no match", Error)
			}
			return ""
		}
	}
	switch command {
	case "open":
		s.link(Span{Text: "→ ", Style: Muted}, relative, relative, "")
		return relative
	case "this":
		s.file("", relative, "")
	case "links", "backlinks":
		s.connections(command, relative, env)
	case "cat", "wc":
		doc, err := env.Vault.ReadTextDocument(relative)
		if err != nil {
			s.text("could not read "+relative, Error)
			return ""
		}
		if command == "cat" {
			s.lines = append(s.lines, Line{{Text: "── " + relative + " ", Path: relative, Style: Accent}})
			for line := range strings.SplitSeq(doc.Text, "\n") {
				line = strings.TrimSuffix(line, "\r")
				if line == "" {
					line = " "
				}
				s.text(line, Plain)
			}
			return ""
		}
		var segments segmenter.Segmenter
		_ = segments.InitWithString(doc.Text)
		characters := segments.GraphemeIterator()
		count := 0
		for characters.Next() {
			count++
		}
		s.file("", relative, fmt.Sprintf("   %d words · %d chars · %d lines", len(strings.Fields(doc.Text)), count, strings.Count(doc.Text, "\n")+1))
	}
	return ""
}

func (s *Session) notePath(arg string, env Environment) string {
	if strings.EqualFold(arg, "this") || strings.EqualFold(arg, "@this") {
		return env.Active
	}
	name, rootReference := strings.CutPrefix(arg, "@")
	cwd := s.cwd
	if rootReference {
		cwd = ""
	}
	relative := vaultPath(cwd, name)
	if s.script != nil {
		if found := exactVaultFile(env.Vault, relative); found != "" {
			return found
		}
	}
	for _, candidate := range []string{relative, relative + ".md", relative + ".markdown"} {
		if entry, exists := entryAt(env.Entries, candidate); exists && !entry.IsDir {
			return entry.Path
		}
	}
	if path.Ext(relative) != "" {
		parent := path.Dir(relative)
		if parent == "." {
			parent = ""
		}
		if entries, err := env.Vault.TerminalEntries(parent, 0); err == nil {
			if entry, ok := entryAt(entries, path.Base(relative)); ok && !entry.IsDir {
				return entry.Path
			}
		}
	}
	if rootReference || strings.Contains(name, "/") {
		return ""
	}
	if matches := matchingNotes(env.Entries, name); len(matches) > 0 {
		return matches[0].Path
	}
	return ""
}

func noteTitle(relative string) string {
	return strings.TrimSuffix(path.Base(relative), path.Ext(relative))
}

func matchingNotes(entries []workspace.Entry, query string) []workspace.Entry {
	type match struct {
		entry workspace.Entry
		rank  int
	}
	var matches []match
	query = strings.ToLower(query)
	walkFiles(entries, func(entry workspace.Entry) {
		if !workspace.IsMarkdownPath(entry.Path) {
			return
		}
		name := strings.ToLower(noteTitle(entry.Path))
		var rank int
		switch {
		case name == query || strings.ToLower(entry.Name) == query:
			rank = 0
		case strings.HasPrefix(name, query):
			rank = 1
		case strings.Contains(name, query):
			rank = 2
		case subsequence(name, query):
			rank = 3
		default:
			return
		}
		matches = append(matches, match{entry: entry, rank: rank})
	})
	slices.SortFunc(matches, func(a, b match) int {
		if a.rank != b.rank {
			return a.rank - b.rank
		}
		return strings.Compare(a.entry.Path, b.entry.Path)
	})
	result := make([]workspace.Entry, 0, len(matches))
	for _, item := range matches {
		result = append(result, item.entry)
	}
	return result
}

func subsequence(text, query string) bool {
	for _, char := range query {
		at := strings.IndexRune(text, char)
		if at < 0 {
			return false
		}
		text = text[at+utf8.RuneLen(char):]
	}
	return true
}

func (s *Session) connections(command, relative string, env Environment) {
	if env.Index == nil {
		s.text("vault index is loading", Muted)
		return
	}
	if command == "backlinks" {
		entries := env.Index.WikiBacklinks(relative)
		s.link(Span{Text: fmt.Sprintf("%d → ", len(entries))}, noteTitle(relative), relative, "")
		for _, entry := range entries {
			s.file("  ", entry.Path, "")
		}
		return
	}
	targets := env.Index.WikiLinksFrom(relative)
	s.link(Span{}, noteTitle(relative), relative, fmt.Sprintf(" → %d", len(targets)))
	for _, target := range targets {
		if target.Path != "" {
			s.file("  ", target.Path, "")
		} else {
			s.text("  "+target.Name, Muted)
		}
	}
}

func (s *Session) tags(command, arg string, env Environment) {
	if env.Index == nil {
		s.text("vault index is loading", Muted)
		return
	}
	graph := env.Index.Graph()
	if command == "tags" {
		tags := slices.Clone(graph.Tags)
		if len(tags) == 0 {
			s.text("no tags in this vault", Muted)
			return
		}
		slices.SortStableFunc(tags, func(a, b vaultindex.GraphTag) int { return b.Count - a.Count })
		for _, tag := range tags {
			s.text(fmt.Sprintf("#%s  %d", tag.Name, tag.Count), Plain)
		}
		return
	}
	tag := strings.ToLower(strings.TrimPrefix(arg, "#"))
	if tag == "" {
		s.text("usage: tag <name>", Error)
		return
	}
	var notes []string
	for _, note := range graph.Notes {
		if slices.Contains(note.Tags, tag) {
			notes = append(notes, note.Path)
		}
	}
	s.text(fmt.Sprintf("#%s  %d notes", tag, len(notes)), Accent)
	for _, relative := range notes {
		s.file("  ", relative, "")
	}
}
