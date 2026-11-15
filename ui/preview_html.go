package ui

import (
	"crypto/rand"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"cyberwriter/internal/core/exportdoc"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"golang.org/x/net/html"
)

// Inline scripts belong to this page's document. The isolated page has no Go bridge or network sources.
const htmlPreviewPolicy = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; media-src data: blob:; base-uri 'none'; form-action 'none'"

type workspaceHTMLPreview struct {
	view          *previewViewport
	tab           *workspaceTab
	path          string
	text          string
	blackout      bool
	zoom          int
	ready         bool
	paddingSent   bool
	scrollPastEnd bool
}

func (p *workspaceHTMLPreview) draw(c *mygoui.Context, app *workspaceWindow, text string, zoom int) {
	if p.view == nil || p.view.IsDestroyed() {
		if !p.init(app.win) {
			mygoui.Column(c).Grow(1).MinHeight(0).FillWidth().Padding(20).Label("HTML Preview").Children(func() {
				mygoui.Text(c, "HTML preview requires the system WebView.").Role(mygoui.RoleStatus)
			})
			return
		}
	}
	if p.tab != app.workspaceTab || p.path != app.document.Path || p.text != text || p.blackout != app.blackout {
		p.tab, p.path, p.text, p.blackout = app.workspaceTab, app.document.Path, text, app.blackout
		p.ready, p.paddingSent = false, false
		markup, err := renderHTMLPreview(text, app.blackout, app.previewOrigin())
		if err != nil {
			p.view.Page().LoadHTML(`<!doctype html><p>Could not load the HTML preview.</p>`, "")
			log.Printf("Render HTML Preview: %v", err)
		} else {
			p.view.Page().LoadHTML(markup, "")
		}
	}
	if p.ready && p.zoom != zoom {
		p.zoom = zoom
		p.view.setZoom(zoom)
	}
	if p.ready && (!p.paddingSent || p.scrollPastEnd != app.preferences.ScrollPastEnd) {
		p.scrollPastEnd = app.preferences.ScrollPastEnd
		if err := previewPaddingChanged.EmitPage(p.view.Page(), p.scrollPastEnd); err != nil {
			log.Printf("Set HTML preview scroll padding: %v", err)
		} else {
			p.paddingSent = true
		}
	}
	mygoui.WebView(c, p.view).Grow(1).MinHeight(0).MinWidth(0).Label("HTML Preview")
}

func (p *workspaceHTMLPreview) init(win *mygo.Window) bool {
	if win == nil || !mygo.WebViewAvailable() {
		return false
	}
	view, err := newPreviewViewport(win, mygo.WebViewOptions{
		Page: mygo.PageOptions{Untrusted: true, DevTools: mygo.DevToolsDisabled, PreloadScript: htmlPreviewScrollScript()},
	})
	if err != nil {
		log.Printf("HTML Preview WebView: %v", err)
		return false
	}
	p.view, p.tab, p.zoom = view, nil, 0
	page := view.Page()
	page.OnDOMReady(func() {
		if p.view == view {
			p.ready, p.paddingSent = true, false
			p.zoom = 0
			win.Invalidate()
		}
	})
	page.OnDidFinishLoad(func() {
		if p.view == view {
			p.zoom = 0
			win.Invalidate()
		}
	})
	page.OnWillNavigate(func(e *mygo.NavigateEvent) {
		target, err := url.Parse(e.URL)
		if err == nil && e.UserInitiated && target.Scheme == "about" && target.Opaque == "blank" && target.Fragment != "" {
			return
		}
		e.PreventDefault()
		if e.UserInitiated {
			openPreviewExternalLink(e.URL)
		}
	})
	page.OnRenderProcessGone(func(string) {
		if p.view == view {
			p.tab = nil
			p.ready = false
			win.Invalidate()
		}
	})
	return true
}

func (p *workspaceHTMLPreview) close() {
	if view := p.view; view != nil {
		*p = workspaceHTMLPreview{}
		view.Destroy()
	}
}

func openPreviewExternalLink(raw string) {
	target := previewExternalURL(raw)
	if target == "" {
		return
	}
	go func() {
		if err := mygo.Shell.OpenExternal(target); err != nil {
			log.Printf("Open preview link: %v", err)
		}
	}()
}

func (app *workspaceWindow) openPreviewInBrowser() {
	if app.draft == nil || app.modalOpen {
		return
	}
	document := app.exportDocument(newExportPageForm(app.preferences.Export).layout)
	blackout, win := app.blackout, app.win
	go func() {
		markup, err := browserPreviewMarkup(document, blackout)
		var filename string
		if err == nil {
			filename, err = writeHTMLBrowserFile(markup)
		}
		if err == nil {
			err = mygo.Shell.OpenPath(filename)
		}
		if err != nil {
			if filename != "" {
				os.RemoveAll(filepath.Dir(filename))
			}
			win.Update(func() { app.problem = fmt.Sprintf("Could not open preview in a browser. %v", err) })
		}
	}()
}

func writeHTMLBrowserFile(source string) (string, error) {
	dir, err := os.MkdirTemp("", "cyberwriter-html-")
	if err != nil {
		return "", err
	}
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	filename := filepath.Join(dir, fmt.Sprintf("cyberwriter_preview_%X.html", suffix))
	if err := exportdoc.WriteFile(filename, []byte(source)); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return filename, nil
}

func renderHTMLPreview(source string, blackout bool, origin previewResourceOrigin) (string, error) {
	header := strings.ToLower(strings.TrimSpace(source))
	if !strings.HasPrefix(header, "<!doctype") && !strings.Contains(header, "<html") {
		canvas, text := "Canvas", "CanvasText"
		if blackout {
			canvas, text = "#000", blackoutTextCSS
		}
		source = fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  line-height: 1.6; padding: 20px; max-width: 900px; margin: 0 auto; background: %s; color: %s; }
img { max-width: 100%%; height: auto; }
table { border-collapse: collapse; width: 100%%; }
th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
</style></head><body>%s</body></html>`, canvas, text, source)
	}
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return "", err
	}
	resources := previewMarkup{resources: origin.resources(), relative: origin.note}
	for node := range doc.Descendants() {
		if node.Type != html.ElementNode || node.Namespace != "" {
			continue
		}
		switch node.Data {
		case "head":
			meta := &html.Node{Type: html.ElementNode, Data: "meta", Attr: []html.Attribute{
				{Key: "http-equiv", Val: "Content-Security-Policy"},
				{Key: "content", Val: htmlPreviewPolicy},
			}}
			node.InsertBefore(meta, node.FirstChild)
			node.InsertBefore(&html.Node{Type: html.ElementNode, Data: "meta", Attr: []html.Attribute{
				{Key: "http-equiv", Val: "x-dns-prefetch-control"},
				{Key: "content", Val: "off"},
			}}, meta.NextSibling)
			style := &html.Node{Type: html.ElementNode, Data: "style"}
			style.AppendChild(&html.Node{Type: html.TextNode, Data: previewScrollCSS})
			node.AppendChild(style)
		case "img":
			for i := range node.Attr {
				attr := &node.Attr[i]
				if attr.Key == "src" {
					if image := resources.imageSource(attr.Val, false); image != "" {
						attr.Val = image
					}
				}
			}
		}
	}
	var output strings.Builder
	if err := html.Render(&output, doc); err != nil {
		return "", err
	}
	return output.String(), nil
}
