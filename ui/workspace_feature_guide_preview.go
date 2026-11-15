package ui

import (
	"html"
	"strings"

	"cyberwriter/internal/core/vaultindex"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type featureGuidePreview struct {
	view      *mygo.WebView
	ready     bool
	card      *featureGuideCard
	theme     string
	customCSS string
	language  workspaceLanguage
	revision  uint64
	document  uint64
	mermaid   bool
	vega      bool
}

var featureGuidePage = previewHTMLShell("Feature example", "",
	strings.Replace(previewContentPolicy, "connect-src https: http: data:", "connect-src 'none'", 1),
	"", 0, "", "")

func (preview *featureGuidePreview) init(win *mygo.Window) {
	if !mygo.WebViewAvailable() {
		return
	}
	view, err := win.NewWebView(mygo.WebViewOptions{
		ScrollToContainer: true, Page: mygo.PageOptions{Untrusted: true, PreloadScript: previewScript()},
	})
	if err != nil {
		return
	}
	preview.view = view
	page := view.Page()
	page.OnPageTitleUpdated(func(e *mygo.TitleEvent) { e.PreventDefault() })
	page.OnWillNavigate(func(e *mygo.NavigateEvent) { e.PreventDefault() })
	page.OnDOMReady(func() {
		if preview.view == view {
			preview.ready, preview.card, preview.theme = true, nil, ""
			preview.mermaid, preview.vega = false, false
			win.Invalidate()
		}
	})
	page.OnRenderProcessGone(func(string) {
		if preview.view == view {
			preview.ready = false
			page.LoadHTML(featureGuidePage, "")
		}
	})
	page.LoadHTML(featureGuidePage, "")
}

func (preview *featureGuidePreview) draw(c *mygoui.Context, card *featureGuideCard, language workspaceLanguage, customCSS string) {
	themeID := "modern"
	if c.Theme().Dark {
		themeID = "synthetic-editorial"
	}
	theme, _ := findPreviewTheme(themeID)
	title := card.localizedCopy(language).title
	label := title + language.text(" example", "示例")
	mygoui.Box(c.Key("guide-preview")).FillWidth().Height(card.preview).Shrink(0).
		Radius(9).Background(mygoui.Hex(theme.BackgroundColor)).Clip().Children(func() {
		if preview.view == nil || !preview.ready {
			mygoui.Scroll(c).Fill().Padding(16).Children(func() {
				markdown.View(c, card.source, &markdown.Options{Extensions: webPreviewExtensions, Theme: theme.markdownTheme(100, 0)})
			})
			return
		}
		page := preview.view.Page()
		contentChanged := preview.card != card || preview.theme != themeID || preview.customCSS != customCSS
		if contentChanged || preview.language != language {
			preview.revision++
			document := preview.document
			if contentChanged {
				document = preview.revision
			}
			body := featureGuideExampleHTML(card)
			frame := previewFrame{Revision: preview.revision, Document: document,
				Title: title, Label: label, HTML: body, Theme: themeID, CustomCSS: customCSS}
			if preview.theme != themeID {
				frame.ThemeFonts = previewThemeFonts(themeID)
			}
			if previewChanged.EmitPage(page, frame) == nil {
				preview.card, preview.theme, preview.customCSS = card, themeID, customCSS
				preview.language, preview.document = language, document
				if !preview.mermaid && strings.Contains(body, mermaidBlockStart) {
					preview.mermaid = true
					go loadPreviewAsset(page, "Mermaid", mermaidLoader)
				}
				if !preview.vega && strings.Contains(body, vegaBlockMarker) {
					preview.vega = true
					go loadPreviewAsset(page, "Vega", vegaLoader)
				}
			}
		}
		mygoui.WebView(c, preview.view).Fill().PassThrough().Label(label)
	})
}

func featureGuideExampleHTML(card *featureGuideCard) string {
	source := card.source
	if card.id == "css" {
		// This particular example is a fixed application string.
		return "<p>" + source + "</p>"
	}
	if card.id == "wikilinks" {
		var out strings.Builder
		start := 0
		for _, link := range vaultindex.Wikilinks(source) {
			out.WriteString(html.EscapeString(source[start:link.Start]))
			out.WriteString(`<a class="wikilink" href="https://example.invalid/example" style="text-decoration-style:dashed">` +
				html.EscapeString(link.Label) + "</a>")
			start = link.End
		}
		out.WriteString(html.EscapeString(source[start:]))
		return "<p>" + out.String() + "</p>"
	}
	body, _ := renderPreviewDocument(parser.New(webPreviewExtensions), source, card.title, nil, "", true, false, nil)
	return body
}
