package wysiwyg

import (
	"crypto/rand"
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

const bashFencePrefix = "bash-fence:"
const compactFencePrefix = "compact-fence:"

// tagCodeFences gives the paragraphs in a fence one identity so adjacent
// fences retain their boundaries. IDs also carry Bash and empty fence styles.
func tagCodeFences(doc *parser.Document, visual richtext.Text) (richtext.Text, bool) {
	var styles []string
	for i := 1; i < doc.Len(); i++ {
		block := doc.Block(i)
		if block.Kind != parser.CodeBlock {
			continue
		}
		id := rand.Text()
		if block.Info() == "bash" {
			id = bashFencePrefix + id
		} else if block.Parent == 0 && block.Fenced && !block.Open && doc.LineCount(i) == 0 {
			id = compactFencePrefix + id
		}
		for range max(1, doc.LineCount(i)) {
			styles = append(styles, id)
		}
	}
	if len(styles) == 0 {
		return visual, true
	}
	var paragraphs []richtext.Paragraph
	index := 0
	for _, paragraph := range visual.Paragraphs() {
		if paragraph.Style.CodeBlock {
			if index >= len(styles) {
				return richtext.Text{}, false
			}
			paragraph.Style.ID = styles[index]
			index++
		}
		paragraphs = append(paragraphs, paragraph)
	}
	return richtext.TextOf(paragraphs...), index == len(styles)
}

// MyGo parses a bare ATX marker as an empty heading, but adds a space when
// writing it. Give only empty top-level headings the explicit empty-block ID.
func tagEmptyHeadings(visual richtext.Text) richtext.Text {
	for offset, paragraph := range visual.Paragraphs() {
		style := paragraph.Style
		if style.Heading > 0 && style.List == richtext.NoList && !style.Quote && len(paragraph.Runs) == 0 {
			visual = visual.FormatParagraphs(offset, offset, func(p *richtext.ParagraphStyle) { p.ID = tableExitID })
		}
	}
	return visual
}

// visualMarkdown delegates ordinary blocks to MyGo. An app ID on an empty
// paragraph preserves its Markdown even though it has no text runs.
func visualMarkdown(visual richtext.Text) string {
	var paragraphs []richtext.Paragraph
	var hasFences, hasEmpty, hasCallouts bool
	for _, paragraph := range visual.Paragraphs() {
		hasCallouts = hasCallouts || calloutID(paragraph.Style) != ""
		if !hasCallouts {
			for _, run := range paragraph.Runs {
				if calloutObject(run.Attributes) != nil {
					hasCallouts = true
					break
				}
			}
		}
		paragraphs = append(paragraphs, paragraph)
	}
	if hasCallouts {
		if markdown, ok := markdownWithCallouts(paragraphs); ok {
			return markdown
		}
	}
	for i, paragraph := range paragraphs {
		hasFences = hasFences || paragraph.Style.CodeBlock && !paragraph.Style.Quote
		if isTableExit(paragraph.Style) && len(paragraph.Runs) == 0 {
			hasEmpty = true
			paragraph.Runs = []richtext.Run{{
				Text:       string(richtext.ObjectReplacement),
				Attributes: richtext.Attributes{Attachment: tableExitMarkdownObject},
			}}
		}
		paragraphs[i] = paragraph
	}
	if !hasFences && !hasEmpty {
		return markdownWithoutFences(visual, paragraphs)
	}
	markdown := markdownWithFences(paragraphs)
	if !hasEmpty {
		return markdown
	}
	marker := attachmentMarkdown(tableExitMarkdownObject)
	return emptyBlockMarkdown(markdown, marker)
}

func markdownWithoutFences(text richtext.Text, paragraphs []richtext.Paragraph) string {
	// MyGo escapes both brackets in literal text. At a quote header the
	// resulting \[ ... \] would be mistaken for display math on reload.
	var parts []string
	start := 0
	for i, paragraph := range paragraphs {
		if !paragraph.Style.Quote || paragraph.Style.CodeBlock || paragraph.Style.List != richtext.NoList ||
			paragraph.Style.Heading != 0 || len(paragraph.Runs) == 0 ||
			!strings.HasPrefix(paragraph.Runs[0].Text, "[!") {
			continue
		}
		markdown := richtext.TextOf(paragraph).Markdown()
		after, ok := strings.CutPrefix(markdown, `> \[!`)
		if !ok {
			continue
		}
		if i > start {
			parts = append(parts, richtext.TextOf(paragraphs[start:i]...).Markdown())
		}
		parts = append(parts, "> [!"+after)
		start = i + 1
	}
	if len(parts) == 0 {
		return text.Markdown()
	}
	if start < len(paragraphs) {
		parts = append(parts, richtext.TextOf(paragraphs[start:]...).Markdown())
	}
	return strings.Join(parts, "\n\n")
}

var tableExitMarkdownObject = &richtext.Attachment{Name: "Empty paragraph", URL: "cyberwriter-empty-paragraph:" + rand.Text()}

// Empty markers emitted for menu blocks are private attachments, so these
// replacements cannot affect ordinary headings, list items or user HTML.
func emptyBlockMarkdown(markdown, marker string) string {
	lines := strings.Split(markdown, "\n")
	var out strings.Builder
	starList := false
	previousList := false
	for i, line := range lines {
		prefix, empty := strings.CutSuffix(line, marker)
		starItem := empty && strings.HasPrefix(prefix, "- ")
		listItem := strings.HasPrefix(line, "- ")
		if i > 0 {
			out.WriteByte('\n')
			if previousList && listItem && starItem != starList {
				// Switching between '-' and '*' starts a new Markdown list.
				out.WriteByte('\n')
			}
		}
		starList, previousList = starItem, listItem
		if !empty {
			out.WriteString(line)
			continue
		}
		heading := strings.TrimSuffix(prefix, " ")
		switch {
		case heading != "" && len(heading) <= 6 && strings.Trim(heading, "#") == "":
			out.WriteString(heading)
		case starItem:
			out.WriteString("*" + prefix[1:] + tableEmptyParagraphMarkdown)
		default:
			out.WriteString(prefix + tableEmptyParagraphMarkdown)
		}
	}
	return out.String()
}

func markdownWithFences(paragraphs []richtext.Paragraph) string {
	var parts []string
	start := 0
	for i := 0; i < len(paragraphs); {
		if !paragraphs[i].Style.CodeBlock || paragraphs[i].Style.Quote {
			i++
			continue
		}
		end := i
		id := ""
		for end < len(paragraphs) && paragraphs[end].Style.CodeBlock && !paragraphs[end].Style.Quote {
			nextID := paragraphs[end].Style.ID
			if id != "" && nextID != "" && nextID != id {
				break
			}
			if nextID != "" {
				id = nextID
			}
			end++
		}
		if start < i {
			parts = append(parts, markdownWithoutFences(richtext.TextOf(paragraphs[start:i]...), paragraphs[start:i]))
		}
		fence := richtext.TextOf(paragraphs[i:end]...).Markdown()
		if strings.HasPrefix(id, bashFencePrefix) {
			fence = strings.Replace(fence, "\n", "bash\n", 1)
		} else if strings.HasPrefix(id, compactFencePrefix) && end == i+1 && len(paragraphs[i].Runs) == 0 {
			fence = "```\n```"
		}
		parts = append(parts, fence)
		start = end
		i = end
	}
	if start < len(paragraphs) {
		parts = append(parts, markdownWithoutFences(richtext.TextOf(paragraphs[start:]...), paragraphs[start:]))
	}
	return strings.Join(parts, "\n\n")
}
