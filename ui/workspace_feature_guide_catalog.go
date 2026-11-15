package ui

import "strings"

type featureGuideCard struct {
	id, title    string
	summary      string
	source       string
	hints        []string
	points       []string
	keywords     string
	preview      float32
	illustration string
	inApp        bool
}

type featureGuideSection struct {
	title string
	cards []featureGuideCard
}

// EV-UI-122/123 establish the order, titles and preview slots. Examples
// and instructions are written for this implementation.
var featureGuideSections = []featureGuideSection{
	{"Markdown", []featureGuideCard{
		{id: "headings", title: "Headings", summary: "Use headings to organize sections, build the outline, and generate a table of contents.",
			source: "# Field Notes\n## Observations\n### Follow-up", preview: 220,
			hints: []string{"⌘ ⌃ 1 / 2 / 3", "Insert → Headings", "# Section title"}},
		{id: "callouts", title: "Callouts", summary: "Set a note, tip, or warning apart from the surrounding text.",
			source: "> [!NOTE] Fieldwork\n> Label every sample before storing it.\n\n> [!TIP]\n> Add the location to your notes.", preview: 220,
			hints: []string{"Source toolbar → Alert", "> [!NOTE]"}},
		{id: "tables", title: "Tables", summary: "Arrange related values in rows and columns. The first row supplies the headers.",
			source: "| Station | Count |\n| --- | ---: |\n| North | 12 |\n| South | 8 |", preview: 220,
			hints:    []string{"Insert → Table", "WYSIWYG → Table grid", "Use pipes to separate columns"},
			points:   []string{"A colon at the end of a separator aligns that column to the right.", "Use the table editor to change cell contents."},
			keywords: "table excel spreadsheet csv tsv paste grid rows columns tab convert format"},
		{id: "tasks", title: "Task Lists", summary: "Track work with Markdown checkboxes.",
			source: "- [x] Choose a route\n- [ ] Pack the notebook\n- [ ] Confirm the meeting point", preview: 220,
			hints: []string{"Insert → Lists → Task List", "- [ ] Open task"}, keywords: "checkbox todo checklist"},
		{id: "lists", title: "Lists", summary: "Group related items with bullets or put steps in order with numbers.",
			source: "- Notebook\n- Spare battery\n\n1. Set a starting point.\n2. Check the weather.", preview: 220,
			hints: []string{"Source toolbar → Bullet list", "Source toolbar → Numbered list", "Indent to nest an item"}},
		{id: "code", title: "Syntax Highlighting", summary: "Name the language after an opening fence to make the example easier to read.",
			source: "```go\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Field notes\")\n}\n```", preview: 220,
			hints: []string{"Insert → Code Block", "View → Disable Syntax Highlighting"}},
		{id: "mermaid", title: "Mermaid Diagrams", summary: "Describe a diagram in a Markdown fence. It renders offline in the preview and as a WYSIWYG card.",
			source: "```mermaid\ngraph LR\n    A[Collect notes] --> B[Review findings]\n    B --> C[Publish summary]\n```", preview: 220,
			hints: []string{"Insert → Diagrams → Flowchart", "⌘ / → Flowchart", "WYSIWYG card → Edit source"}},
		{id: "vega", title: "Charts (Vega-Lite)", summary: "Pair a dataset with a visual encoding. This bar chart uses values embedded in the note.",
			source: "```vega-lite\n{\n  \"data\": {\"values\": [{\"site\":\"North\",\"visits\":12},{\"site\":\"South\",\"visits\":8}]},\n  \"mark\": \"bar\",\n  \"encoding\": {\n    \"x\": {\"field\": \"site\", \"type\": \"nominal\"},\n    \"y\": {\"field\": \"visits\", \"type\": \"quantitative\"}\n  }\n}\n```", preview: 300,
			hints:  []string{"Start with a vega-lite fence", "Set data.values for inline data", "Use data.url for a local Vault file", "Open Markdown Preview"},
			points: []string{"CSV, TSV, and JSON sources can be resolved relative to the note or Vault."}},
		{id: "math", title: "LaTeX Math", summary: "Place small expressions inline and longer equations on their own line.",
			source: "The radius is $r$.\n\n$$\nA = \\pi r^2\n$$", preview: 220,
			hints: []string{"$expression$ for inline math", "$$ on separate lines for display math", "View → Preview Only"}},
		{id: "footnotes", title: "Footnotes", summary: "Keep references close to a passage without interrupting the sentence.",
			source: "We surveyed the site twice.[^visits]\n\n[^visits]: Morning and late afternoon.", preview: 220,
			hints: []string{"Insert → Footnote", "Use the same label in the reference and definition"}},
		{id: "highlight", title: "Highlight & Inline Code", summary: "Draw attention to a phrase or set a short command in a fixed-width font.",
			source: "The **revised date** is Thursday. Save the note before sharing.", preview: 220,
			hints: []string{"⌘ B", "Insert → Inline Code", "Select the phrase first"}},
		{id: "figures", title: "Image Captions & Figures", summary: "Attach a descriptive caption and choose a portable relative image path.",
			source:   "![Trail marker](images/trail.png \"East entrance\")",
			hints:    []string{"Insert → Image", "Use alt text for the image description"},
			keywords: "caption figure figures numbering academic report alt text resize size width"},
		{id: "wikilinks", title: "Wikilinks", summary: "Connect notes by name, use an alias for the label, or link directly to a heading.",
			source: "Start with [[Field Notes]] and continue in [[Observations#Weather|weather notes]].", preview: 220,
			hints: []string{"[[Note title]]", "Vault file menu → Copy Wikilink"},
			points: []string{"Use [[Folder/Note]] when more than one file shares a name. The part after a pipe changes the label shown in the preview and keeps the destination.",
				"Add a heading after # to open a particular section. Match the heading text used in the destination note.",
				"Place ! before the brackets to include an image. The path is relative to the current note or the Vault.",
				"Click a rendered link in the Markdown preview to open the destination note and keep the original note available.",
				"Right-click a Vault file and use Copy Wikilink to get its exact path when several folders share the same name.",
				"The links in this guide demonstrate the rendered labels. They point to example notes."},
			keywords: "autocomplete suggest complete link backlink alias picker"},
		{id: "tags", title: "Tags", summary: "Use a tag in the body or in frontmatter to find related notes.",
			source: "# Survey log\n\nRecord the counts under #fieldwork and #project/survey.", preview: 220,
			hints: []string{"#project/survey", "Properties → Tags"}},
		{id: "css", title: "Inline CSS Styling", summary: "A style attribute can supply color or alignment in HTML. The example shows a locally styled sentence.",
			source: "<span style=\"color: #56a5eb\">Water level rising</span>", preview: 220,
			hints: []string{"Use a span to style a phrase", "Use a div for a block", "Preview HTML to check the result"}, keywords: "color html style"},
	}},
	{"Custom Commands", []featureGuideCard{
		{id: "pagebreak", title: "Page Break", summary: "Start a new page in PDF and Word exports at a standalone page break.",
			source: "[pagebreak]", hints: []string{"Insert → Page Break", "GLOW → /page-break"},
			points: []string{"Place the marker on a line of its own. The outline and table of contents keep using the document headings."}},
		{id: "mermaid-bg", title: "Mermaid Card Background", summary: "Set the background of a Mermaid card without editing the graph.",
			source: "```mermaid[bg:#24354a]\ngraph LR\n    A[Draft] --> B[Ready]\n```", preview: 320,
			hints:  []string{"Mermaid card → Background", "mermaid[bg:#24354a]", "mermaid[bg:none]"},
			points: []string{"Theme removes the override. None blends the card into the page."}, illustration: "diagram"},
		{id: "api", title: "Live API Data", summary: "Run an HTTP request from a note and display its response.",
			source: "```api[title: Station reading]\nGET https://example.invalid/reading.json\n```",
			hints:  []string{"Add → Live Data → API Request", "WYSIWYG → /api", "⌘ / → API History"},
			points: []string{"Run sends the request. The response appears beneath the block and is saved with the note.", "Built-in formatting displays JSON records as a table. Use API History to review and copy previous calls."}},
		{id: "query", title: "Query Block", summary: "List notes or display their properties in a table.",
			source: queryTemplate, hints: []string{"⌘ / → Query Block", "Add → Live Data → Query Block", "WYSIWYG → /query"},
			points: []string{"Filter by folder, tag, or property. Use SORT to order results and LIMIT to cap them.", "Click a result to open its note. In WYSIWYG, Edit source opens the query inside the card."}},
		{id: "fold", title: "Fold Text Blocks", summary: "Collapse selected lines in source mode to keep a long section out of the way.",
			hints:  []string{"Edit → Fold Selection", "⌘ .", "⌘ / → Fold Selection"},
			points: []string{"Select multiple lines and fold them. Click the fold marker to expand the same lines."}},
		{id: "embeds", title: "Media Embeds", summary: "Prefix a wikilink with ! to embed a Vault image or note. The original also supports media and tables such as CSV.",
			source: "![[images/route.png]]\n\n![[Notes/Observations]]",
			hints:  []string{"![[Vault path]]", "Insert → Image", "Use a relative filename"},
			points: []string{"Use Markdown Preview to check Vault images and included note text."}},
	}},
	{"Present & Export", []featureGuideCard{
		{id: "export", title: "Export - PDF, DOCX & HTML", summary: "Export the current draft and choose a format, paper settings, and destination.",
			hints:  []string{"⌘ E → Quick Export PDF", "⌘ ⇧ E → Export options", "Preview toolbar → Share"},
			points: []string{"PDF can include a page range and the current theme. Word preserves headings, paragraphs, and lists.", "HTML can contain Mermaid diagrams, with a background set in the document theme."}},
		{id: "custom-css", title: "Custom Stylesheet", summary: "Apply document styles to Markdown previews and exports. The built-in themes provide a stable starting point.",
			source:   "h1 { letter-spacing: 0.02em; }\nblockquote { border-inline-start-width: 3px; }",
			hints:    []string{"Settings → Themes", "Custom Stylesheet"},
			points:   []string{"Edit the stylesheet in Settings or open its file from the same section.", "The switch controls whether the CSS is applied. The file remains available while the switch is off.", "PDF uses screen media at the document's printable width. Reopen the note to pick up external stylesheet changes in Quick Export PDF."},
			keywords: "css stylesheet style snippet obsidian typora theme custom heading color line length margins print", inApp: true},
		{id: "themes", title: "Themes", summary: "Choose the appearance of the application and the style of the rendered document.",
			hints: []string{"Settings → Themes", "Titlebar → Preview Theme"}, keywords: "color customize dark mode light mode css stylesheet", illustration: "themes",
			points: []string{"Choose App Appearance for the window and Preview Theme for the rendered note. The live sample updates as you change the default theme."}},
		{id: "toc", title: "Table of Contents", summary: "Generate links to the headings in the active note.",
			source: "# Field Notes\n## Observations\n## Follow-up",
			hints:  []string{"⌘ ⇧ T", "Insert → Table of Contents", "GLOW → /table-of-contents"}},
		{id: "slides", title: "Slides", summary: "Present a note in the same window, using headings or separators to divide it.",
			hints:  []string{"Editor toolbar → Slideshow", "Play as Slides"},
			points: []string{"Switch layout or zoom while presenting. Slides can contain the same code, formulas, and tables as the note."}, illustration: "slides"},
		{id: "flashcards", title: "Flash Cards", summary: "Review a prompt, reveal the answer, and rate how well you recalled it.",
			source: "# What did we measure?\n\nThe number of visits at each station.",
			hints:  []string{"Editor toolbar → Slideshow", "Flash Cards"}, illustration: "flashcards",
			points: []string{"Reveal the answer before rating a card. The review stores progress for the current note."}},
		{id: "focus", title: "Focus Mode", summary: "Hide editing controls and keep your attention on the document.",
			hints:  []string{"⌘ ⇧ F", "Titlebar → Focus"},
			points: []string{"Use the same shortcut or the exit control to restore the editing toolbar."}},
	}},
	{"Vault", []featureGuideCard{
		{id: "terminal", title: "GLOW Terminal", summary: "Use local commands to inspect the Vault and invoke editor actions.",
			source: "pwd\nls\n/features", hints: []string{"Bottom bar → GLOW Terminal", "⌃ `", "Enter / to list app commands"},
			keywords: "terminal console shell cli command line plato prompt ai apply plan grep pipe redirect drawer shade quake glow", illustration: "glow",
			points: []string{"The terminal can dock in the workspace, open below the title bar, or float. Its local commands accept wikilinks as note paths."}},
		{id: "wikibuilder", title: "Unlinked Mentions & Vault Health", summary: "Find mentions that could become a wikilink and inspect whether existing links resolve.",
			hints:    []string{"⌘ / → Vault Health Report", "GLOW → /health"},
			points:   []string{"Vault Health.md lists broken wikilinks, orphan notes, unlinked mentions, and duplicate titles."},
			keywords: "wiki mentions unlinked health lint broken orphan duplicate links backlinks audit alias"},
		{id: "drawings", title: "Drawings (Excalidraw)", summary: "Sketch a diagram on the canvas and save it as an Excalidraw scene.",
			hints: []string{"Titlebar → Drawing → New Drawing", "Open Drawing…", "Map Vault"}, keywords: "excalidraw drawing sketch whiteboard canvas draw diagram freehand",
			points: []string{"A drawing opens in its own tab. Save it to keep the shapes, labels, and connections in the Vault."}},
		{id: "properties", title: "Properties & Frontmatter", summary: "Edit document metadata in a sidebar while keeping the underlying YAML or TOML in the note.",
			source: "---\ntitle: Field Notes\ntags: [fieldwork, survey]\n---",
			hints:  []string{"Vault → Properties", "Edit frontmatter at the top of the note"}, illustration: "properties",
			points: []string{"Choose the field type when you add a property. Saved keys and values remain part of the Markdown file."}},
		{id: "graph", title: "Knowledge Graph", summary: "Explore how notes connect through their wikilinks and open a note from a node.",
			hints: []string{"Vault → Graph", "Open Graph in Window"}, illustration: "graph",
			points: []string{"Select a shape for the 3D layout or use 2D mode. Search or apply a tag to reduce the nodes in view."}},
		{id: "concept-search", title: "Chat with Your Vault", summary: "The original can include relevant Vault passages in a conversation.",
			hints:  []string{"Search Vault notes by text here", "Open a result to check its source"},
			points: []string{"A model-backed Vault conversation is pending in this replica."}},
		{id: "vault-embeddings", title: "Vault Embeddings", summary: "An embedding index groups text by semantic similarity for use in searches and model context.",
			hints:    []string{"Use the ordinary Vault search to find text", "Properties → Tags"},
			points:   []string{"The local full-text index is available. Embedding providers and semantic indexing are pending."},
			keywords: "embeddings index semantic context depth rag build index"},
		{id: "backlinks", title: "Backlinks", summary: "See which notes contain incoming wikilinks for the document you have open.",
			hints:  []string{"Open the backlinks section in the workspace"},
			points: []string{"Pick an incoming link to open its note. Outgoing links are visible in the same document context."}},
		{id: "quick-switcher", title: "Quick Switcher", summary: "Jump to a note or drawing by filename or its containing folder.",
			hints:    []string{"⌘ P", "Type a filename, then press Return"},
			points:   []string{"Use the result's path to tell equally named notes apart."},
			keywords: "jump to file switcher open file fuzzy", illustration: "switcher"},
		{id: "outline", title: "Outline Sidebar", summary: "Use the current note's headings as a map and jump to a section.",
			hints:  []string{"⌘ ⇧ O", "View → Toggle Outline"},
			points: []string{"Select a heading in the outline to reveal its position in the current document."}},
	}},
	{"Editor", []featureGuideCard{
		{id: "smartpaste", title: "Smart Paste", summary: "Paste plain text or formatted material, then check the Markdown the editor produced.",
			hints:    []string{"⌘ V", "Switch to WYSIWYG for rich clipboard text", "Review the source after pasting"},
			points:   []string{"For tables, use the table editor when the clipboard format needs correction."},
			keywords: "paste clipboard markdown rich html rtf word excel spreadsheet table convert plain"},
		{id: "selectiontoolbar", title: "Selection Toolbar", summary: "The original shows formatting tools next to a selected passage.",
			hints:    []string{"Select text", "Use the editor toolbar to format it"},
			points:   []string{"The regular toolbar supports bold, italic, links, and table entry. The floating selection toolbar is pending."},
			keywords: "bubble floating toolbar selection format bold italic quick crepe"},
		{id: "hex", title: "Hex Colour Swatches", summary: "Hex colors describe a shade with three or six digits.",
			hints:    []string{"Type #4f86d9", "Use the color picker in a diagram card"},
			points:   []string{"The original displays inline swatches. This replica exposes color editing for supported cards."},
			keywords: "color color picker swatch"},
		{id: "wysiwyg", title: "WYSIWYG Editing", summary: "Edit headings, lists, links, and Mermaid cards visually while retaining Markdown.",
			hints:  []string{"⌘ 4", "View → WYSIWYG", "Switch to source to check the Markdown"},
			points: []string{"Use the table editor for cell changes and source mode for constructs the rich editor cannot project."}, illustration: "wysiwyg"},
		{id: "templates", title: "Templates", summary: "Start a new note from a template or reuse a whole document as a template.",
			hints: []string{"File → New from Template", "⌘ / → Templates", "File → Save as Template…"}, illustration: "templates",
			points: []string{"The File menu opens a draft. Choosing a template in the command palette inserts it at the current selection."}},
	}},
	{"UI & AI", []featureGuideCard{
		{id: "palette", title: "Command Palette", summary: "Search available actions from the keyboard and run the selected command.",
			hints: []string{"⌘ /", "Type a command, then press Return"}, illustration: "palette",
			points: []string{"Move with the arrow keys to review matches. Escape closes the panel and returns to the editor."}},
		{id: "quick-actions", title: "AI Quick Actions", summary: "The original can send the selection to a model for revision or explanation.",
			hints:  []string{"Select a passage", "AI Quick Actions in the original editor"},
			points: []string{"Model-backed selection actions are pending in this replica."}},
		{id: "ai-panel", title: "AI Workspace", summary: "Ask a model about your note from a conversation beside the editor.",
			hints:    []string{"Title bar → AI", "Settings → AI"},
			points:   []string{"Attach the current note or selection, choose whether to insert the reply into the editor, and use the history button to return to earlier conversations. The Aa button switches between formatted replies and Markdown source."},
			keywords: "chat table llm ollama lm studio claude openrouter mlx vllm embeddings rag context system prompt stream model provider openai apple intelligence"},
		{id: "voice", title: "Voice Notes & Dictation", summary: "The original editor can record an attachment, transcribe a recording, or insert dictated text.",
			hints:  []string{"Original source toolbar → Microphone", "Record a voice note", "Start dictation"},
			points: []string{"Recording and transcription are pending in this replica."}},
	}},
}

var featureGuideCards = func() map[string]*featureGuideCard {
	cards := make(map[string]*featureGuideCard)
	for _, section := range featureGuideSections {
		for i := range section.cards {
			card := &section.cards[i]
			cards[card.id] = card
		}
	}
	return cards
}()

func (card *featureGuideCard) searchableText() string {
	return strings.ToLower(strings.Join([]string{
		card.title, card.summary, card.source, strings.Join(card.points, " "),
		strings.Join(card.hints, " "), card.keywords,
	}, " "))
}
