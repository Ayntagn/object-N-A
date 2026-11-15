package ui

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"strings"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

//go:embed presentation.css
var presentationStyles string

const presentationActionURL = "cyberwriter-presentation://controls/"

type presentationFrameKey struct {
	index, layout, zoom int
	revealed            bool
}

type presentationWeb struct {
	view             *mygo.WebView
	ready            bool
	shown            bool
	frame            presentationFrameKey
	revision         uint64
	mermaidRequested bool
	vegaRequested    bool
}

func (w *presentationWeb) init(app *workspaceWindow, p *workspacePresentation) {
	if !mygo.WebViewAvailable() {
		return
	}
	view, err := app.win.NewWebView(mygo.WebViewOptions{
		BackgroundColor: "#000000",
		Page:            mygo.PageOptions{Untrusted: true, PreloadScript: presentationScript()},
	})
	if err != nil {
		log.Printf("Presentation WebView: %v", err)
		return
	}
	w.view = view
	page := view.Page()
	page.OnDOMReady(func() {
		if app.presentation != p {
			return
		}
		w.ready, w.shown = true, false
		w.mermaidRequested, w.vegaRequested = false, false
		app.win.Invalidate()
	})
	page.OnRenderProcessGone(func(string) {
		if app.presentation == p {
			w.ready = false
			page.LoadHTML(presentationPage, "")
		}
	})
	page.OnWillNavigate(func(e *mygo.NavigateEvent) {
		e.PreventDefault()
		if action, ok := strings.CutPrefix(e.URL, presentationActionURL); ok {
			// The preload emits these fixed controls. The Markdown renderer
			// filters out links using the internal action scheme.
			for _, key := range presentationKeys {
				if string(key.action) == action {
					app.win.Update(func() { p.act(app, key.action) })
					break
				}
			}
			return
		}
		if !e.UserInitiated {
			return
		}
		if relative, anchor, _, ok := previewNoteNavigation(e.URL); ok {
			app.win.Update(func() {
				if app.presentation == p {
					app.closePresentation()
					app.openNoteAt(relative, true, anchor)
				}
			})
			return
		}
		openPreviewExternalLink(e.URL)
	})
	page.LoadHTML(presentationPage, "")
}

var presentationPage = strings.Replace(
	strings.Replace(previewHTMLShell("Presentation Mode", "",
		strings.Replace(previewContentPolicy, "connect-src https: http: data:", "connect-src data:", 1),
		"", 0, "", ""), "<html lang", "<html class=\"cw-presentation\" lang", 1),
	"</head>", "<style>"+presentationStyles+"</style></head>", 1,
)

func presentationScript() string {
	keys := make(map[string]presentationAction, len(presentationKeys))
	for _, key := range presentationKeys {
		keys[key.browser] = key.action
	}
	bindings, _ := json.Marshal(keys)
	return previewScript() + `
;(() => {
  const keys = ` + string(bindings) + `;
  function command(action) {
    window.location.href = "` + presentationActionURL + `" + action;
  }
  window.addEventListener("keydown", event => {
    if (event.altKey || event.ctrlKey || event.metaKey || event.defaultPrevented) return;
    if (!Object.prototype.hasOwnProperty.call(keys, event.key)) return;
    event.preventDefault();
    command(keys[event.key]);
  });
  document.addEventListener("click", event => {
    if (!document.querySelector("article.flash-card")) return;
    if (event.target.closest?.("a, button, input") || String(window.getSelection())) return;
    command("primary");
  });
  const cue = () => document.documentElement.toggleAttribute("data-cw-more",
    window.scrollY + window.innerHeight + 2 < document.documentElement.scrollHeight);
  window.addEventListener("scroll", cue, {passive: true});
  document.addEventListener("DOMContentLoaded", () => {
    new ResizeObserver(cue).observe(document.querySelector("main"));
    cue();
  });
})();`
}

func (w *presentationWeb) draw(c *mygoui.Context, p *workspacePresentation) {
	if w.view == nil || w.view.IsDestroyed() {
		p.drawWithoutWebView(c)
		return
	}
	page := w.view.Page()
	frame := presentationFrameKey{p.index(), p.layout, p.zoom, p.revealed}
	if w.ready && (!w.shown || w.frame != frame) {
		w.revision++
		body := p.renderBody()
		update := previewFrame{
			Revision: w.revision, Document: uint64(frame.index + 1),
			Label: p.label(), Title: p.label(), HTML: body,
		}
		if err := previewChanged.EmitPage(page, update); err != nil {
			log.Printf("Update presentation: %v", err)
		} else {
			w.frame, w.shown = frame, true
			if !w.mermaidRequested && strings.Contains(body, mermaidBlockStart) {
				w.mermaidRequested = true
				go loadPreviewAsset(page, "Mermaid", mermaidLoader)
			}
			if !w.vegaRequested && strings.Contains(body, vegaBlockMarker) {
				w.vegaRequested = true
				go loadPreviewAsset(page, "Vega", vegaLoader)
			}
		}
	}
	mygoui.WebView(c, w.view).Grow(1).MinHeight(0).MinWidth(0).Label(p.label())
}

func (p *workspacePresentation) label() string {
	if p.flash {
		return "Flash Cards"
	}
	return "Play as Slides"
}

func (p *workspacePresentation) renderBody() string {
	var out strings.Builder
	class := "slide"
	if p.flash {
		class = "flash-card"
	}
	index := p.index()
	if !p.flash && index == 0 {
		class += " cover"
	}
	fmt.Fprintf(&out, `<article class="%s %s" style="--deck-scale:%.2f">`,
		class, presentationLayouts[p.layout].css, float64(p.zoom)/100)
	doc := parser.New(webPreviewExtensions)
	resources := p.origin.resources()
	markup := func(text string) string {
		body, _ := renderPreviewResources(doc, text, "", resources, p.origin.note, true)
		return body
	}
	switch {
	case p.count() == 0:
		out.WriteString("<p>No flash cards</p>")
	case p.flash:
		card := p.cards[index]
		out.WriteString(`<section class="question">` + markup(card.Question) + "</section>")
		if p.revealed && card.Answer != "" {
			out.WriteString(`<hr class="card-divider"><section class="answer">` + markup(card.Answer) + "</section>")
		}
	case index == 0:
		out.WriteString("<h1>" + html.EscapeString(p.slides.Title) + "</h1>")
		if p.slides.Subtitle != "" {
			out.WriteString("<p>" + html.EscapeString(p.slides.Subtitle) + "</p>")
		}
	default:
		out.WriteString(markup(p.slides.Pages[index-1]))
	}
	out.WriteString("</article>")
	return out.String()
}

func (p *workspacePresentation) drawWithoutWebView(c *mygoui.Context) {
	width, _ := c.Size()
	if p.layout == 0 {
		width = min(width, 1100)
	} else if p.layout == 2 {
		width = min(width, 700)
	}
	mygoui.Scroll(c).Grow(1).MinHeight(0).Width(width).AlignSelf(mygoui.Center).Padding(24, 40).
		Label(p.label()).Children(func() {
		index := p.index()
		theme, _ := findPreviewTheme("synthetic-editorial-black")
		options := &markdown.Options{Extensions: webPreviewExtensions, Theme: theme.markdownTheme(p.zoom, 1.45)}
		switch {
		case p.count() == 0:
			mygoui.Text(c, "No flash cards").TextColor(presentationText)
		case p.flash:
			markdown.View(c, p.cards[index].Question, options)
			if p.revealed {
				markdown.View(c, p.cards[index].Answer, options)
			}
		case index == 0:
			mygoui.Text(c, p.slides.Title).Role(mygoui.RoleHeading).FontSize(42).Bold().TextColor(presentationText)
			mygoui.Text(c, p.slides.Subtitle).TextColor(presentationMuted)
		default:
			markdown.View(c, p.slides.Pages[index-1], options)
		}
	})
}
