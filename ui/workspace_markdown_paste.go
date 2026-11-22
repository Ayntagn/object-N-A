package ui

import (
	"strings"

	"cyberwriter/ui/wysiwyg"
	"github.com/egoist/mygo/transfer"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

type notePasteKind uint8

const (
	plainNotePaste notePasteKind = iota
	inlineNotePaste
	blockNotePaste
)

type notePasteText struct {
	plain, markdown string
	kind            notePasteKind
	format          transfer.Format
}

func readNotePasteText(services mygoui.Services) notePasteText {
	plain := services.ReadClipboard()
	data := services.ReadClipboardData(transfer.Markdown, transfer.HTML, richtext.RTF)
	if raw, err := data.Read(transfer.Markdown); err == nil && strings.TrimSpace(string(raw)) != "" {
		return classifyNotePaste(plain, string(raw), transfer.Markdown)
	}
	text := classifyNotePaste(plain, plain, transfer.Text)
	for _, format := range []transfer.Format{transfer.HTML, richtext.RTF} {
		raw, err := data.Read(format)
		if err != nil || len(raw) == 0 {
			continue
		}
		var converted richtext.Text
		if format == transfer.HTML {
			converted = richtext.ParseHTML(string(raw))
		} else {
			converted = richtext.ParseRTF(raw)
		}
		// A rendered copy of literal Markdown can also advertise HTML.
		// Prefer the Markdown characters when the rich text just wraps them.
		if text.kind != plainNotePaste && strings.TrimSpace(converted.PlainText()) == strings.TrimSpace(text.markdown) {
			return text
		}
		if markdown := converted.Markdown(); strings.TrimSpace(markdown) != "" {
			return classifyNotePaste(plain, markdown, format)
		}
	}
	return text
}

var notePasteNewlines = strings.NewReplacer("\r\n", "\n", "\r", "\n", "\u2028", "\n", "\u2029", "\n")

func classifyNotePaste(plain, markdown string, format transfer.Format) notePasteText {
	text := notePasteText{plain: plain, markdown: notePasteNewlines.Replace(markdown), format: format}
	doc := parser.New(wysiwyg.Extensions)
	doc.Set(text.markdown)
	paragraphs := 0
	for index := range doc.Children(0) {
		block := doc.Block(index)
		switch block.Kind {
		case parser.Heading, parser.ThematicBreak, parser.Quote, parser.List, parser.Table, parser.MathBlock:
			text.kind = blockNotePaste
			return text
		case parser.CodeBlock:
			if block.Fenced && !block.Open {
				text.kind = blockNotePaste
				return text
			}
		case parser.Paragraph:
			paragraphs++
			for _, span := range doc.Inlines(index) {
				switch span.Kind {
				case parser.Code, parser.Emphasis, parser.Strong, parser.Strikethrough, parser.Image, parser.Math:
					text.kind = inlineNotePaste
				case parser.Link:
					if !span.Auto || span.Start >= 0 && span.Start < len(text.markdown) && text.markdown[span.Start] == '<' {
						text.kind = inlineNotePaste
					}
				}
			}
		}
	}
	if paragraphs > 1 {
		text.kind = blockNotePaste
	}
	return text
}
