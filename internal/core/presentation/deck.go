// Package presentation extracts slides and study cards from Markdown notes.
package presentation

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui/markdown/parser"
	"go.yaml.in/yaml/v3"
)

type SlideDeck struct {
	Title    string
	Subtitle string
	Pages    []string // Markdown pages after the cover.
}

func Slides(body, frontmatter, fallback string, date time.Time) SlideDeck {
	doc := parser.New(parser.GFM)
	doc.Set(body)
	title, author, when, tags := coverMetadata(frontmatter)
	if title == "" {
		title = firstHeading(doc)
		if title == "" {
			title = fallback
		}
	}
	if when == "" && !date.IsZero() {
		when = date.Format("January 2, 2006")
	}
	var subtitle []string
	for _, text := range []string{author, when, tags} {
		if text != "" {
			subtitle = append(subtitle, text)
		}
	}
	pages, divided := sections(doc, parser.ThematicBreak)
	if !divided {
		pages, _ = sections(doc, parser.Heading)
	}
	return SlideDeck{Title: title, Subtitle: strings.Join(subtitle, " · "), Pages: pages}
}

func firstHeading(doc *parser.Document) string {
	for i := range doc.Children(0) {
		if doc.Block(i).Kind != parser.Heading {
			continue
		}
		var label strings.Builder
		for _, inline := range doc.Inlines(i) {
			switch inline.Kind {
			case parser.Text, parser.Code:
				label.WriteString(inline.Text)
			case parser.SoftBreak, parser.LineBreak:
				label.WriteByte(' ')
			}
		}
		if title := strings.TrimSpace(label.String()); title != "" {
			return title
		}
	}
	return ""
}

func coverMetadata(frontmatter string) (title, author, date, tags string) {
	var doc yaml.Node
	if frontmatter == "" || yaml.Unmarshal([]byte(frontmatter), &doc) != nil || len(doc.Content) == 0 {
		return
	}
	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i].Value, mapping.Content[i+1]
		text := strings.TrimSpace(value.Value)
		if value.Kind == yaml.ScalarNode && value.Tag != "!!null" {
			switch key {
			case "title":
				title = text
				if title != "" && value.Style == yaml.DoubleQuotedStyle {
					title = "“" + title + "”"
				}
				if title != "" && value.Style == yaml.SingleQuotedStyle {
					title = "‘" + title + "’"
				}
			case "author":
				author = text
			case "date":
				date = text
			case "tags":
				tags = text
			}
		}
		if key == "tags" && value.Kind == yaml.SequenceNode {
			var names []string
			for _, tag := range value.Content {
				if tag.Kind == yaml.ScalarNode && tag.Tag != "!!null" && strings.TrimSpace(tag.Value) != "" {
					names = append(names, strings.TrimSpace(tag.Value))
				}
			}
			tags = strings.Join(names, ", ")
		}
	}
	return
}

func sections(doc *parser.Document, divider parser.Kind) (pages []string, divided bool) {
	text := doc.Source()
	start := 0
	for i := range doc.Children(0) {
		block := doc.Block(i)
		if block.Kind != divider || divider == parser.Heading && block.Level != 2 && block.Level != 3 {
			continue
		}
		if page := strings.TrimSpace(text[start:block.Start]); page != "" {
			pages = append(pages, page)
		}
		start = block.Start
		if divider == parser.ThematicBreak {
			start = block.End
		}
		divided = true
	}
	if page := strings.TrimSpace(text[start:]); page != "" {
		pages = append(pages, page)
	}
	return
}

type Card struct {
	Question string
	Answer   string
}

func Flashcards(body string) []Card {
	doc := parser.New(parser.GFM)
	doc.Set(body)
	pages, _ := sections(doc, parser.ThematicBreak)
	cards := make([]Card, 0, len(pages))
	for _, page := range pages {
		doc.Set(page)
		question, answer := splitAnswer(doc)
		if question != "" {
			cards = append(cards, Card{Question: question, Answer: answer})
		}
	}
	return cards
}

func splitAnswer(doc *parser.Document) (question, answer string) {
	source := doc.Source()
	for i := range doc.Children(0) {
		block := doc.Block(i)
		if block.Kind != parser.Paragraph {
			continue
		}
		offset := block.Start
		for line := range strings.SplitSeq(source[block.Start:block.End], "\n") {
			if strings.TrimSpace(line) == "???" {
				return strings.TrimSpace(source[:offset]), strings.TrimSpace(source[offset+len(line):])
			}
			offset += len(line) + 1
		}
	}
	return source, ""
}
