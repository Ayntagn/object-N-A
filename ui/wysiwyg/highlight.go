package wysiwyg

import (
	"crypto/rand"
	"slices"
	"strings"
	"unicode/utf8"

	"cyberwriter/internal/core/markdownhighlight"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

// web.wys.host.highlight.mark in re/ui/design-tokens.json.
var highlightBackground = mygoui.RGBA(255, 214, 92, .32)

type highlightDelimiters struct{ open, close string }

func newHighlightDelimiters(source string) highlightDelimiters {
	for {
		// Like '=', § is punctuation to CommonMark, but cannot start syntax.
		stem := "§" + rand.Text()
		if !strings.Contains(source, stem) {
			return highlightDelimiters{stem + "A§", stem + "B§"}
		}
	}
}

func projectHighlights(doc *parser.Document) (*parser.Document, highlightDelimiters) {
	if !strings.Contains(doc.Source(), "==") {
		return doc, highlightDelimiters{}
	}
	delimiters := newHighlightDelimiters(doc.Source())
	return projectHighlightDelimiters(doc, delimiters), delimiters
}

func projectHighlightDelimiters(doc *parser.Document, delimiters highlightDelimiters) *parser.Document {
	source := markdownhighlight.Rewrite(doc, delimiters.open, delimiters.close)
	if source == doc.Source() {
		return doc
	}
	projected := parser.New(doc.Extensions())
	projected.Set(source)
	return projected
}

func applyHighlightDelimiters(text richtext.Text, marks highlightDelimiters) (richtext.Text, bool) {
	var paragraphs []richtext.Paragraph
	marked := false
	for _, paragraph := range text.Paragraphs() {
		p := richtext.Paragraph{Style: paragraph.Style}
		for _, run := range paragraph.Runs {
			for remaining := run.Text; remaining != ""; {
				token, unexpected := marks.open, marks.close
				if marked {
					token, unexpected = marks.close, marks.open
				}
				value, tail, found := strings.Cut(remaining, token)
				if strings.Contains(value, unexpected) {
					return richtext.Text{}, false
				}
				if value != "" {
					part := richtext.Run{Text: value, Attributes: run.Attributes}
					if marked {
						part.Attributes.Background = highlightBackground
					}
					p.Runs = append(p.Runs, part)
				}
				if found {
					marked = !marked
				}
				remaining = tail
			}
		}
		if marked {
			return richtext.Text{}, false
		}
		paragraphs = append(paragraphs, p)
	}
	return richtext.TextOf(paragraphs...), true
}

// ToggleHighlight changes only the background of the selected rich text.
func ToggleHighlight(doc *richtext.Document) bool {
	anchor, caret := doc.Selection()
	text := doc.Text()
	start, end := min(anchor, caret), max(anchor, caret)
	if start == end {
		attributes := doc.Formatting().Attributes
		if text.ParagraphStyleAt(start).CodeBlock || attributes.Code {
			return false
		}
		doc.Format(func(a *richtext.Attributes) {
			if a.Background == highlightBackground {
				a.Background = mygoui.Transparent
			} else {
				a.Background = highlightBackground
			}
		})
		return true
	}
	type span struct{ start, end int }
	var ranges []span
	allMarked := true
	for paragraph := start; paragraph < end; {
		_, paragraphEnd := text.ParagraphRange(paragraph)
		if !text.ParagraphStyleAt(paragraph).CodeBlock {
			last := min(end, paragraphEnd)
			selected := text.Slice(paragraph, last).String()
			if core := strings.TrimSpace(selected); core != "" {
				leading := strings.Index(selected, core)
				first := paragraph + utf8.RuneCountInString(selected[:leading])
				last = first + utf8.RuneCountInString(core)
				for offset, run := range text.Runs(first, last) {
					a := run.Attributes
					if a.Attachment == nil && (!a.Code || a.Background == highlightBackground) {
						ranges = append(ranges, span{offset, offset + utf8.RuneCountInString(run.Text)})
						allMarked = allMarked && a.Background == highlightBackground
					}
				}
			}
		}
		paragraph = paragraphEnd + 1
	}
	if len(ranges) == 0 {
		return false
	}
	doc.Edit(func(edit *richtext.Edit) {
		for _, selected := range ranges {
			edit.Format(selected.start, selected.end, func(a *richtext.Attributes) {
				if allMarked {
					a.Background = mygoui.Transparent
				} else {
					a.Background = highlightBackground
				}
			})
		}
		edit.Select(anchor, caret)
	})
	return true
}

func markdownWithHighlights(text richtext.Text, paragraphs []richtext.Paragraph) string {
	var marks highlightDelimiters
	var projected []richtext.Paragraph
	for p, paragraph := range paragraphs {
		if paragraph.Style.CodeBlock {
			continue
		}
		var runs []richtext.Run
		for i := 0; i < len(paragraph.Runs); {
			if paragraph.Runs[i].Attributes.Background != highlightBackground {
				if runs != nil {
					runs = append(runs, paragraph.Runs[i])
				}
				i++
				continue
			}
			if projected == nil {
				marks = newHighlightDelimiters(text.String())
				projected = slices.Clone(paragraphs)
			}
			if runs == nil {
				runs = append(runs, paragraph.Runs[:i]...)
			}
			first := i
			for i < len(paragraph.Runs) && paragraph.Runs[i].Attributes.Background == highlightBackground {
				i++
			}
			marked := paragraph.Runs[first:i]
			a := sharedHighlightAttributes(marked)
			runs = append(runs, richtext.Run{Text: marks.open, Attributes: a})
			runs = append(runs, marked...)
			runs = append(runs, richtext.Run{Text: marks.close, Attributes: a})
		}
		if runs != nil {
			projected[p].Runs = runs
		}
	}
	if projected == nil {
		return text.Markdown()
	}
	return marks.writeMarkdown(richtext.TextOf(projected...).Markdown())
}

// Keep formatting shared by the whole highlight outside the delimiters.
// Marks belonging to only part of the range stay inside the highlight.
func sharedHighlightAttributes(runs []richtext.Run) richtext.Attributes {
	shared := runs[0].Attributes
	shared.Code, shared.Attachment = false, nil
	for _, run := range runs[1:] {
		a := run.Attributes
		if a.Bold() != shared.Bold() {
			shared.Weight = 0
		}
		shared.Italic = shared.Italic && a.Italic
		shared.Strikethrough = shared.Strikethrough && a.Strikethrough
		shared.Underline = shared.Underline && a.Underline
		if shared.Link != a.Link {
			shared.Link = ""
		}
	}
	return shared
}

func (marks highlightDelimiters) writeMarkdown(markdown string) string {
	var out strings.Builder
	for {
		before, remaining, exists := strings.Cut(markdown, marks.open)
		if !exists {
			out.WriteString(before)
			return out.String()
		}
		body, after, paired := strings.Cut(remaining, marks.close)
		if !paired {
			out.WriteString(before + remaining)
			return out.String()
		}
		core := strings.TrimSpace(body)
		if core == "" {
			out.WriteString(before + body)
			markdown = after
			continue
		}
		leading := strings.Index(body, core)
		before += body[:leading]
		trailing := body[leading+len(core):]
		// Keep literal equals next to the delimiters from forming a run of
		// three or more equals, which isn't a highlight delimiter.
		before = escapeFinalHighlightEqual(before)
		if suffix, ok := strings.CutPrefix(core, "="); ok {
			core = "&#61;" + suffix
		}
		core = escapeFinalHighlightEqual(core)
		out.WriteString(before)
		out.WriteString("==")
		out.WriteString(core)
		out.WriteString("==")
		out.WriteString(trailing)
		if trailing == "" && strings.HasPrefix(after, "=") {
			out.WriteString("&#61;")
			after = after[1:]
		}
		markdown = after
	}
}

func escapeFinalHighlightEqual(source string) string {
	prefix, ok := strings.CutSuffix(source, "=")
	if !ok {
		return source
	}
	// MyGo escapes a leading equals when it starts a paragraph.
	slashes := len(prefix) - len(strings.TrimRight(prefix, `\`))
	return prefix[:len(prefix)-slashes%2] + "&#61;"
}
