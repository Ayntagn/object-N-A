package ui

import (
	"cyberwriter/internal/core/markdowndetails"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type nativeDetailsProjection struct {
	source, rendered string
	extensions       parser.Extensions
	openers          map[int]markdowndetails.Block
}

func nativeMarkdownWithDetails(c *mygoui.Context, owner mygoui.Element, source string, options *markdown.Options) mygoui.Element {
	state := mygoui.Local(owner, "Markdown details projection", func() nativeDetailsProjection { return nativeDetailsProjection{} })
	extensions := options.Extensions
	if extensions == 0 {
		extensions = parser.Default
	}
	if state.source != source || state.extensions != extensions {
		rendered, regions := markdowndetails.Project(source, extensions)
		*state = nativeDetailsProjection{source: source, rendered: rendered, extensions: extensions}
		if len(regions) > 0 {
			state.openers = make(map[int]markdowndetails.Block, len(regions))
			for _, region := range regions {
				state.openers[region.Start] = region
			}
		}
	}
	if len(state.openers) == 0 {
		return markdown.View(c, source, options)
	}
	projectedOptions := *options
	projectedOptions.CopySource = source
	projectedOptions.Disclosure = state.disclosure
	return markdown.View(c, state.rendered, &projectedOptions)
}

func (state *nativeDetailsProjection) disclosure(doc *parser.Document, index int) (markdown.Disclosure, bool) {
	block := doc.Block(index)
	region, ok := state.openers[block.Start]
	if !ok || block.Kind != parser.HTMLBlock || block.Parent != 0 {
		return markdown.Disclosure{}, false
	}
	for i := block.Next; i < doc.Len(); i = doc.Block(i).Next {
		sibling := doc.Block(i)
		if sibling.Start > region.Close {
			break
		}
		if sibling.Start == region.Close && sibling.Kind == parser.HTMLBlock {
			return markdown.Disclosure{Title: region.Summary, Close: i, Open: region.Open}, true
		}
	}
	return markdown.Disclosure{}, false
}
