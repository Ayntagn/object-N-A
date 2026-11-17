package ui

import (
	"io/fs"
	"net/url"
	"strings"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const nativeWikiImageScheme = "cyberwriter-wiki-image:"

type nativeMarkdownImageState struct {
	origin  previewResourceOrigin
	options *markdown.Options
}

func nativeWikiAssets(owner mygoui.Element, source string, extensions parser.Extensions, origin previewResourceOrigin) string {
	if !strings.Contains(source, "[[") {
		return source
	}
	type projection struct {
		source, rendered string
		origin           previewResourceOrigin
		extensions       parser.Extensions
	}
	state := mygoui.Local(owner, "Markdown Wiki assets", func() projection { return projection{} })
	if source != state.source || origin != state.origin || extensions != state.extensions {
		state.source = source
		state.origin = origin
		state.extensions = extensions
		state.rendered = projectNativeWikiAssets(source, extensions, origin)
	}
	return state.rendered
}

// The native viewer accepts standard Markdown links and images. Project only
// actual prose spans and use private destinations for Vault files.
func projectNativeWikiAssets(source string, extensions parser.Extensions, origin previewResourceOrigin) string {
	doc := parser.New(extensions)
	doc.Set(source)
	var output strings.Builder
	start := 0
	for block := 1; block < doc.Len(); block++ {
		inlines := doc.Inlines(block)
		for index := 0; index < len(inlines); index++ {
			inline := inlines[index]
			if inline.Kind == parser.Link || inline.Kind == parser.Image {
				index = inline.Next - 1
				continue
			}
			if inline.Kind != parser.Text || inline.Start < start || inline.End > len(source) || !strings.Contains(inline.Text, "[[") {
				continue
			}
			for _, link := range vaultindex.Wikilinks(source[inline.Start:inline.End]) {
				var token string
				if link.Embed && workspace.IsRasterImagePath(link.Target) {
					token = "!" + droppedMarkdownLink(link.Label, nativeWikiImageScheme+url.PathEscape(link.Target))
				} else if !link.Embed {
					if relative, page, ok := origin.resolveWikiPDF(link.Target); ok {
						token = droppedMarkdownLink(link.Label, previewPDFDestination(relative, page))
					}
				}
				if token == "" {
					continue
				}
				output.WriteString(source[start : inline.Start+link.Start])
				output.WriteString(token)
				start = inline.Start + link.End
			}
		}
	}
	if start == 0 {
		return source
	}
	output.WriteString(source[start:])
	return output.String()
}

func (origin previewResourceOrigin) loadMarkdownImage(destination string) (mygoui.ImageSource, error) {
	var candidates []string
	if encoded, wiki := strings.CutPrefix(destination, nativeWikiImageScheme); wiki {
		target, err := url.PathUnescape(encoded)
		if err != nil {
			return nil, err
		}
		candidates = origin.wikiImageCandidates(target)
	} else if relative, ok := workspace.ResolveImagePath(origin.note, destination); ok {
		candidates = append(candidates, relative)
	}
	for _, relative := range candidates {
		data, _, err := origin.vault.ReadPreviewImage(relative, 4<<20)
		if err == nil {
			return mygoui.DecodeBitmap(data)
		}
	}
	return nil, fs.ErrNotExist
}
