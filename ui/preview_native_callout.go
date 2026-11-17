package ui

import (
	"strings"

	"cyberwriter/internal/core/callout"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func (app *workspaceWindow) withNativeMarkdownCallouts(options *markdown.Options) *markdown.Options {
	options.Callout = app.nativeCallout
	options.HardBreak = nativeQuoteBreak
	return options
}

func (app *workspaceWindow) nativeCallout(doc *parser.Document, index int) (markdown.Callout, bool) {
	header, _, end, ok := callout.ReadQuoteHeader(doc.Source(), doc.Block(index).Start)
	if !ok {
		return markdown.Callout{}, false
	}
	style := callout.Resolve(header.Kind, app.calloutStyles())
	title := style.Name
	if header.Title != "" {
		titleDoc := parser.Parse("a " + header.Title)
		title = strings.TrimPrefix(parser.PlainText(titleDoc.Inlines(1)), "a ")
	}
	return markdown.Callout{Title: title, Icon: style.Icon, Color: mygoui.Hex(style.Color), HeaderEnd: end, Fold: header.Fold}, true
}

func nativeQuoteBreak(doc *parser.Document, index int, inline parser.Inline) bool {
	depth := 0
	for parent := doc.Block(index).Parent; parent > 0; parent = doc.Block(parent).Parent {
		if doc.Block(parent).Kind == parser.Quote {
			depth++
		}
	}
	return callout.ExplicitQuoteBreak(doc.Source()[inline.Start:inline.End], depth)
}
