package glow

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"cyberwriter/internal/core/workspace"
)

const (
	ExampleVaultLink = "cwterm-open-example-vault"
	demoFolder       = "_demo"
	demoRoster       = "_demo/crew.md"
)

type demoState struct {
	vault   *workspace.Vault
	chapter int
	action  int
	waiting bool
	typing  int
	draft   Line

	createdFolder fs.FileInfo
	createdRoster workspace.Document
}

type demoAction struct {
	command string
	work    string
	output  []string
}

type demoChapter struct {
	title, summary string
	rule           int
	actions        []demoAction
}

var demoChapters = []demoChapter{
	{
		title: "the commands", rule: 40,
		summary: "A command line for your vault. What it does, in five moves:\n \n" +
			"  look      ls · tree · stats · cat · open\n" +
			"  connect   grep · find · tag · backlinks · links\n" +
			"  make      new · mkdir · mv · rm · >  (send output to a note)\n" +
			"  ask       ai - chat, read the vault, and draft notes as plans\n" +
			"  keep      dump · theme · run  (save a session, reskin, run scripts)\n \n" +
			"We'll walk through these on the real notes in this vault -\n" +
			"one short example at a time.",
	},
	{
		title: "look around", rule: 35,
		summary: "GLOW is a command line for your vault - not your computer. Let's see what's here, then size it up.",
		actions: []demoAction{{command: "ls", work: "reading"}, {command: "stats", work: "counting"}},
	},
	{
		title: "read a note", rule: 35,
		summary: "Open any note by a fuzzy name - no need to type it in full. Here's a crew file.",
		actions: []demoAction{{command: "cat marcus", work: "reading"}},
	},
	{
		title: "search and connect", rule: 28,
		summary: "Find a word across every note, then trace who links into one.",
		actions: []demoAction{{command: "grep thermal", work: "searching"}, {command: "backlinks system architecture", work: "tracing links"}},
	},
	{
		title: "make a note from a search", rule: 21,
		summary: "Send a command's output to a new note - the crew, as a roster. File names become [[wikilinks]] on the way.",
		actions: []demoAction{
			{command: "tag person > " + demoRoster, work: "writing"},
			{command: "open _demo/crew", work: "opening"},
			{output: []string{"- that file name is a link - click it to open the note in the editor."}},
		},
	},
	{
		title: "ask your model", rule: 32,
		summary: "Ask in plain language. GLOW reads the vault to answer - and remembers the thread.",
		actions: []demoAction{
			{command: `ai "who's on the crew and what does each own?" @briefing`, work: "thinking", output: []string{
				"Marcus Kim owns power, thermal, and comms. Zara Okafor is",
				"mission director - structures and the habitat shell. Dr. Elena",
				"Vasquez leads life-support and the water loop.",
			}},
			{command: `ai "which of them is stretched thinnest?"`, work: "thinking", output: []string{
				`⌁ grep "owns"`, "⌁ backlinks Marcus Kim",
				"Marcus Kim - three systems to the others' one or two, and the",
				"most backlinks. If anyone's overcommitted, it's him.",
			}},
			{output: []string{"- no @ref that time - it remembered the conversation."}},
		},
	},
	{
		title: "let it write - safely", rule: 25,
		summary: "Ask it to write and you get a plan, not files. Nothing lands without apply; undo takes it back.",
		actions: []demoAction{
			{command: `ai "draft a docking-procedures note, house style"`, work: "thinking", output: []string{"▣ plan - 1 file   _demo/Docking Procedures.md"}},
			{command: "show 1", output: []string{
				"Docking checklist for the Perihelion approach corridor.", "## Sequence",
				"1. Match [[Orbital Mechanics|phasing orbit]], hold at 200 m.", "#inbox",
			}},
			{command: "apply", work: "writing", output: []string{"✓ wrote _demo/Docking Procedures.md"}},
			{command: "undo", work: "reverting", output: []string{"↩ reverted - file removed. Your call, always."}},
		},
	},
	{
		title: "make it yours", rule: 33,
		summary: "Themes are your call - and it stays one session as you move between notes.",
		actions: []demoAction{{command: "theme matrix"}, {command: "theme plato"}},
	},
	{
		title: "your turn", rule: 37,
		summary: "The sandbox goes to the Trash. dump keeps a cheat-sheet of everything you just saw.",
		actions: []demoAction{{command: "rm _demo", work: "trashing"}, {command: "dump"}},
	},
}

func (s *Session) DemoRunning() bool { return s.demo != nil }

// DemoPreview is a transient typed command or work line displayed below scrollback.
func (s *Session) DemoPreview() Line {
	if s.demo == nil {
		return nil
	}
	return s.demo.draft
}

func (s *Session) startDemo(env Environment) time.Duration {
	if env.Vault == nil || !hasDemoNotes(env.Entries) {
		s.text(" ", Plain)
		s.text("demo runs on the cyberWriter Example Vault - this vault doesn't have its notes.", Muted)
		s.link(Span{Text: "→ ", Style: Muted}, "open the Example Vault", ExampleVaultLink, "   then run demo again.")
		return 0
	}
	if _, err := env.Vault.StatEntry(demoFolder); !errors.Is(err, fs.ErrNotExist) {
		s.text("demo needs an unused _demo folder", Error)
		return 0
	}
	s.demo = &demoState{vault: env.Vault}
	s.demoWelcome(env)
	s.demoHeading(demoChapters[0])
	return 750 * time.Millisecond
}

func hasDemoNotes(entries []workspace.Entry) bool {
	var marcus, architecture bool
	walkFiles(entries, func(entry workspace.Entry) {
		if workspace.IsMarkdownPath(entry.Path) {
			switch noteTitle(entry.Path) {
			case "Marcus Kim":
				marcus = true
			case "System Architecture":
				architecture = true
			}
		}
	})
	return marcus && architecture
}

func (s *Session) demoWelcome(env Environment) {
	s.text("UNCSOFT TERMLINK  ·  the Glowing Logical Operator Window", Accent)
	hour := time.Now().Hour()
	greeting := "good evening, operator."
	switch {
	case hour < 5:
		greeting = "burning the midnight oil, operator."
	case hour < 12:
		greeting = "good morning, operator."
	case hour < 18:
		greeting = "good afternoon, operator."
	}
	s.text(greeting, Plain)
	s.text(" ", Plain)
	s.text("▸ system check", Prompt)
	stats := vaultStats(env)
	s.text(fmt.Sprintf("  vault ............. %s   (%d notes · %d tags)   [ ok ]", vaultName(env.Name), stats.notes, stats.tags), Plain)
	s.text("  ai core ........... none configured   [ offline ]", Muted)
	active := "none"
	if env.Active != "" {
		active = path.Base(env.Active)
	}
	s.text("  open document ..... "+active, Plain)
	s.text("  phosphor .......... nominal   [ glowing ]", Plain)
	s.text("all systems glowing. here's what i can do -", Plain)
}

func (s *Session) demoHeading(chapter demoChapter) {
	s.text(" ", Plain)
	s.lines = append(s.lines, Line{
		{Text: "▸ " + chapter.title + " ", Style: Accent},
		{Text: strings.Repeat("─", chapter.rule), Style: Muted},
	})
	for line := range strings.SplitSeq(chapter.summary, "\n") {
		if keyword, rest, ok := strings.Cut(strings.TrimPrefix(line, "  "), " "); strings.HasPrefix(line, "  ") && ok {
			s.lines = append(s.lines, Line{{Text: "  " + keyword, Style: Prompt}, {Text: " " + rest}})
		} else if strings.HasPrefix(line, "We'll walk") || strings.HasPrefix(line, "one short example") {
			s.text(line, Muted)
		} else {
			s.text(line, Plain)
		}
	}
}

func (s *Session) resumeDemo() time.Duration {
	if !s.demo.waiting {
		return 0
	}
	s.demo.waiting = false
	s.demo.chapter++
	s.demo.action = 0
	s.demoHeading(demoChapters[s.demo.chapter])
	return 500 * time.Millisecond
}

func (s *Session) StopDemo() {
	if s.demo != nil {
		s.demo = nil
		s.text("- demo stopped · type help -", Muted)
	}
}

// TickDemo advances the guide on the workspace event loop. Zero delay waits
// for input or ends the tour.
func (s *Session) TickDemo(env Environment) (Result, time.Duration) {
	demo := s.demo
	if demo == nil || demo.waiting {
		return Result{}, 0
	}
	if env.Vault != demo.vault {
		s.StopDemo()
		return Result{}, 0
	}
	chapter := demoChapters[demo.chapter]
	if demo.action == len(chapter.actions) {
		if demo.chapter == len(demoChapters)-1 {
			s.demo = nil
			s.text(" ", Plain)
			s.text("that's the tour. type help for the full command list.", Accent)
		} else {
			demo.waiting = true
			s.lines = append(s.lines, Line{
				{Text: "- press ", Style: Muted}, {Text: "⏎", Style: Prompt},
				{Text: " for next   ·   ", Style: Muted}, {Text: "⌃C", Style: Prompt},
				{Text: " to stop -", Style: Muted},
			})
		}
		return Result{}, 0
	}
	action := chapter.actions[demo.action]
	runes := []rune(action.command)
	if demo.typing < len(runes) {
		demo.typing++
		demo.draft = Line{{Text: vaultName(env.Name) + "> ", Style: Prompt}, {Text: string(runes[:demo.typing])}}
		return Result{}, 38 * time.Millisecond
	}
	if action.work != "" && demo.typing == len(runes) {
		demo.typing++
		demo.draft = Line{{Text: "⌁ " + action.work + "…", Style: Muted}}
		return Result{}, 180 * time.Millisecond
	}
	demo.action++
	demo.typing, demo.draft = 0, nil
	return s.runDemoAction(action, env), 700 * time.Millisecond
}

func (s *Session) runDemoAction(action demoAction, env Environment) Result {
	if action.output != nil {
		if action.command != "" {
			s.echo(action.command, vaultName(env.Name)+">")
		}
		for _, line := range action.output {
			s.text(line, Plain)
		}
		return Result{}
	}
	cwd := s.cwd
	s.cwd = ""
	defer func() { s.cwd = cwd }()
	switch action.command {
	case "tag person > " + demoRoster:
		if err := env.Vault.CreateFolder(demoFolder); err != nil {
			s.echo(action.command, s.Prompt(env.Name))
			s.text("could not create the demo folder: "+err.Error(), Error)
			return Result{}
		}
		demo := s.demo
		demo.createdFolder, _ = env.Vault.StatEntry(demoFolder)
		result := s.execute(action.command, env, false)
		if result.VaultChanged {
			demo.createdRoster, _ = env.Vault.ReadDocument(demoRoster)
		}
		result.VaultChanged = true
		return result
	case "rm _demo":
		s.echo(action.command, s.Prompt(env.Name))
		demo := s.demo
		destination, err := env.Vault.TrashGeneratedFolder(demoFolder, demo.createdFolder, demo.createdRoster, env.TrashItem)
		if err != nil {
			s.text("could not move the demo folder to Trash: "+err.Error(), Error)
			return Result{}
		}
		s.trashReceipt(demoFolder, destination)
		return Result{VaultChanged: true}
	}
	result := s.execute(action.command, env, false)
	// The guide exposes result links while leaving the active document alone.
	result.OpenPath = ""
	return result
}
