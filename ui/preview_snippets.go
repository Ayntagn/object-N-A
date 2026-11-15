package ui

import (
	_ "embed"
	"fmt"
	"html"
	"net/url"
	"strings"

	"cyberwriter/internal/core/snippets"
	"cyberwriter/internal/core/vaultindex"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const snippetReferencePrefix = "snippet:"
const previewSnippetScheme = "cyberwriter-snippet"

type previewSnippetLibrary struct {
	items      []snippets.Snippet
	codeTheme  *editor.Theme
	hoverItems []snippets.Snippet
	cardIDs    map[string]int
}

func loadPreviewSnippets(text string, dark bool) *previewSnippetLibrary {
	if !strings.Contains(text, "[["+snippetReferencePrefix) {
		return nil
	}
	library := &previewSnippetLibrary{cardIDs: make(map[string]int)}
	if file, err := snippetFile(false); err == nil {
		library.items, _ = snippets.Load(file)
	}
	codeTheme := "atom-one-light"
	if dark {
		codeTheme = "night-owl"
	}
	library.codeTheme = sourceThemeForID(codeTheme).palette
	return library
}

func (app *workspaceWindow) refreshSnippetPreviews() {
	for _, window := range app.documentWindows() {
		if window.workspaceTab == nil || window.draft == nil || window.previewDocumentType() != documentMarkdown ||
			!strings.Contains(window.workingText(), "[["+snippetReferencePrefix) {
			continue
		}
		window.previewPane.refreshPending = true
		if window.win != nil && !window.win.IsDestroyed() {
			window.win.Invalidate()
		}
	}
}

func previewSnippetURL(title string) string {
	return (&url.URL{Scheme: previewSnippetScheme, Host: "open", RawQuery: url.Values{"title": {title}}.Encode()}).String()
}

func previewSnippetNavigation(raw string) (string, bool) {
	link, err := url.Parse(raw)
	if err != nil || link.Scheme != previewSnippetScheme || link.Host != "open" || link.User != nil || link.Path != "" || link.Fragment != "" {
		return "", false
	}
	query, err := url.ParseQuery(link.RawQuery)
	if err != nil || len(query) != 1 || len(query["title"]) != 1 {
		return "", false
	}
	title := query.Get("title")
	return title, title != "" && title == strings.TrimSpace(title)
}

func (p *workspacePreview) handleSnippetNavigation(app *workspaceWindow, view *previewViewport, target string) bool {
	if !strings.HasPrefix(target, previewSnippetScheme+":") {
		return false
	}
	title, ok := previewSnippetNavigation(target)
	if ok {
		win, document := app.win, p.document
		win.Update(func() {
			if win.IsDestroyed() || view.IsDestroyed() || p.view != view || !p.ready || p.document != document ||
				p.tab != app.workspaceTab || p.draft != app.draft || p.path != app.document.Path || app.previewDocumentType() != documentMarkdown {
				return
			}
			app.openPreviewSnippet(title)
		})
	}
	return true
}

// EV-UI-236: links are supported in prose and hover independently of embeds.
func (m *previewMarkup) writeSnippetLink(link vaultindex.Wikilink) bool {
	if m.snippets == nil || link.Embed {
		return false
	}
	title, ok := strings.CutPrefix(link.Target, snippetReferencePrefix)
	if !ok || title == "" {
		return false
	}
	cardID := m.snippets.cardIDs[title]
	if cardID == 0 {
		if item, found := snippets.FindByTitle(m.snippets.items, title); found {
			m.snippets.hoverItems = append(m.snippets.hoverItems, item)
			cardID = len(m.snippets.hoverItems)
			m.snippets.cardIDs[title] = cardID
		}
	}
	fmt.Fprintf(&m.out, `<a class="wikilink snippet-link" href="%s" data-snippet-card="%d">✂️ %s</a>`,
		html.EscapeString(previewSnippetURL(title)), cardID, html.EscapeString(title))
	return true
}

func (m *previewMarkup) writeSnippetEmbed(in []parser.Inline, anchor string) bool {
	if m.snippets == nil || len(in) != 1 || in[0].Kind != parser.Text {
		return false
	}
	raw := strings.TrimSpace(m.rawInline(in[0]))
	links := vaultindex.Wikilinks(raw)
	if len(links) != 1 || !links[0].Embed || links[0].Start != 0 || links[0].End != len(raw) {
		return false
	}
	title, ok := strings.CutPrefix(links[0].Target, snippetReferencePrefix)
	if !ok || title == "" {
		return false
	}
	item, found := snippets.FindByTitle(m.snippets.items, title)
	if !found {
		fmt.Fprintf(&m.out, `<div class="snippet-missing"%s>📎 %s (not found)</div>`, anchor, html.EscapeString(links[0].Target))
		return true
	}
	fmt.Fprintf(&m.out, `<div class="callout callout-note snippet-embed"%s><div class="callout-title">✂️ %s</div><div class="callout-content">`,
		anchor, html.EscapeString(item.Title))
	m.writeSnippetContent(item)
	m.out.WriteString(`</div></div>`)
	return true
}

func (m *previewMarkup) writeSnippetHoverTemplates() {
	if m.snippets == nil {
		return
	}
	for i, item := range m.snippets.hoverItems {
		fmt.Fprintf(&m.out, `<template data-snippet-template="%d"><div class="snippet-hover-title">✂️ %s</div>`,
			i+1, html.EscapeString(item.Title))
		m.writeSnippetContent(item)
		m.out.WriteString(`</template>`)
	}
}

func (m *previewMarkup) writeSnippetContent(item snippets.Snippet) {
	language := strings.TrimSpace(item.Language)
	child := previewMarkup{codeTheme: m.snippets.codeTheme, embedded: true}
	if language == "" || strings.EqualFold(language, "markdown") {
		child.doc = parser.New(previewExtensions)
		body, _ := child.render(item.Content, "", false)
		m.out.WriteString(body)
	} else if child.writeHighlightedFence(language, "", item.Content) {
		m.out.WriteString(child.out.String())
	} else {
		fmt.Fprintf(&m.out, `<pre><code>%s</code></pre>`, html.EscapeString(item.Content))
	}
}

const previewSnippetCSS = `
  html[data-preview-theme] .snippet-link { text-decoration: none; border-bottom: 1px dashed currentColor; padding-bottom: 1px; }
  html[data-preview-theme] .snippet-link:hover { border-bottom-style: solid; }
  .snippet-embed, .snippet-missing { margin-block: var(--doc-paragraph-gap, 16px); border-radius: 6px; overflow: hidden; }
  .snippet-embed { border-inline-start: 4px solid var(--doc-link); background: var(--doc-code-background); }
  .snippet-embed > .callout-title { padding: 8px 14px; font-weight: 600; font-size: .95em; }
  .snippet-embed > .callout-content { padding: 4px 14px 12px; }
  html[data-preview-theme] .snippet-embed pre { white-space: pre; background: transparent; }
  .snippet-missing { padding: 10px 14px; color: var(--doc-quote-text); background: var(--doc-code-background); }
  .snippet-hover-card {
    position: fixed; z-index: 10000; width: max-content; max-width: min(420px, calc(100vw - 20px));
    max-height: min(400px, calc(100vh - 20px)); overflow: auto; overflow-wrap: anywhere;
    background: var(--doc-background, Canvas); color: var(--doc-text, CanvasText);
    border: 1px solid var(--doc-table-border, GrayText); border-radius: 8px; padding: 12px 16px;
    box-shadow: 0 8px 24px #0004; font-size: 12px; line-height: 1.5;
  }
  .snippet-hover-title { font-weight: 600; color: var(--doc-quote-text); }
  html[data-preview-theme] #cw-snippet-hover pre { padding: 8px 10px; border-radius: 6px; margin: 4px 0; white-space: pre-wrap; }
  html[data-preview-theme] .snippet-hover-card p { margin: 3px 0; }
  html[data-preview-theme] .snippet-hover-card pre code { font-size: 11px; }
`

//go:embed preview_snippets.js
var previewSnippetScript string
