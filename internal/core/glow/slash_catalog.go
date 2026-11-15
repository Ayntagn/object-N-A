package glow

// EV-UI-078 / EV-UI-079: the GLOW app command order and search terms.
var appCommands = [...]AppCommand{
	{"editor", "Editor Only", "View", []string{"editor", "write"}},
	{"split", "Split View", "View", []string{"split", "dual", "both"}},
	{"preview", "Preview Only", "View", []string{"preview", "read"}},
	{"wys", "WYSIWYG", "View", []string{"wysiwyg", "visual", "rich", "what you see"}},
	{"focus", "Focus Mode", "View", []string{"focus", "zen", "distraction", "fullscreen"}},
	{"slides", "Play as Slides", "View", []string{"present", "slides", "play", "slideshow"}},
	{"flashcards", "Flash Cards", "View", []string{"flash", "cards", "study", "quiz", "flashcard", "question", "answer"}},
	{"outline", "Toggle Outline", "View", []string{"outline", "sidebar", "headings", "nav"}},
	{"pdf-preview", "Toggle PDF Preview", "View", []string{"pdf", "export", "preview"}},
	{"fold", "Fold Selection", "View", []string{"fold", "collapse", "hide", "selection", "block"}},
	{"snippets", "Toggle Snippets", "View", []string{"snippets", "notes", "scissors", "clipboard", "reference"}},
	{"terminal", "Toggle GLOW Terminal", "View", []string{"terminal", "console", "shell", "command", "plato", "cli", "glow"}},

	{"ai-panel", "Toggle AI Panel", "AI", []string{"ai", "chat", "llm", "assistant"}},
	{"ai-actions", "AI Quick Actions", "AI", []string{"ai", "quick", "summarize", "rewrite"}},

	{"export-pdf", "Quick Export PDF", "Export", []string{"pdf", "export", "save"}},
	{"export", "Export with Options", "Export", []string{"export", "docx", "html", "word"}},
	{"draw", "New Drawing", "Export", []string{"drawing", "excalidraw", "sketch", "draw", "whiteboard", "canvas", "diagram"}},
	{"new-folder", "New Folder", "Export", []string{"folder", "directory", "new", "organize", "vault", "create"}},
	{"map-vault", "Map Vault", "Export", []string{"map", "vault", "graph", "excalidraw", "atlas", "links", "visualize", "drawing"}},
	{"health", "Vault Health Report", "Export", []string{"vault", "health", "lint", "broken", "orphan", "mentions", "duplicate", "report", "wiki", "audit"}},
	{"save-to-vault", "Save to Vault", "Export", []string{"vault", "save", "copy", "import"}},
	{"save-template", "Save as Template", "Export", []string{"template", "save", "custom", "reuse"}},
	{"voice", "Record Voice Note", "Export", []string{"voice", "record", "audio", "microphone", "note"}},
	{"transcribe", "Record & Transcribe", "Export", []string{"voice", "record", "transcribe", "speech", "text"}},
	{"dictate", "Live Dictation", "Export", []string{"dictation", "speech", "voice", "live", "speak"}},

	{"custom-stylesheet", "Custom Stylesheet", "Settings", []string{"css", "stylesheet", "style", "custom", "theme", "snippet", "obsidian", "typora", "font", "margins", "heading color"}},
	{"find", "Find", "Settings", []string{"find", "search"}},
	{"replace", "Find & Replace", "Settings", []string{"replace", "find", "swap"}},
	{"font-up", "Increase Font Size", "Settings", []string{"font", "bigger", "zoom", "larger"}},
	{"font-down", "Decrease Font Size", "Settings", []string{"font", "smaller", "zoom"}},
	{"font-reset", "Reset Font Size", "Settings", []string{"font", "reset", "default"}},
	{"line-numbers", "Toggle Line Numbers", "Settings", []string{"line", "numbers", "gutter"}},
	{"sync", "Toggle Scroll Sync", "Settings", []string{"scroll", "sync", "lock"}},
	{"spell", "Toggle Spell Check", "Settings", []string{"spell", "check", "grammar"}},
	{"hex-swatches", "Toggle Collapse Hex Colors", "Settings", []string{"hex", "color", "collapse", "swatch"}},

	{"features", "Feature Guide", "Help", []string{"help", "feature", "guide", "learn", "how", "cards", "tour"}},
	{"shortcuts", "Keyboard Shortcuts", "Help", []string{"keyboard", "shortcuts", "keys", "hotkey"}},
	{"manual", "User Guide", "Help", []string{"help", "guide", "manual", "docs", "markdown"}},

	{"readme", "README", "Templates", []string{"template"}},
	{"meeting-notes", "Meeting Notes", "Templates", []string{"template"}},
	{"blog-post", "Blog Post", "Templates", []string{"template"}},
	{"project-plan", "Project Plan", "Templates", []string{"template"}},
	{"changelog", "Changelog", "Templates", []string{"template"}},

	{"page-break", "Page Break", "Blocks", []string{"break", "page", "newpage"}},
	{"table-of-contents", "Table of Contents", "Blocks", []string{"toc", "contents", "outline", "nav"}},
}
