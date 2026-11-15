package ui

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"cyberwriter/internal/core/exportdoc"
	"cyberwriter/internal/core/mermaid"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func renderExportDOCX(parent *mygo.Window, doc exportDocument) ([]byte, error) {
	layout := wordExportLayout(doc.layout, doc.paper)
	baseFontSize := 0
	if theme, ok := findPreviewTheme(doc.theme); ok {
		baseFontSize = theme.BaseFontSize
	}
	diagrams := renderWordDiagrams(parent, doc.source)
	return exportdoc.DOCXWithDiagrams(doc.source, layout, baseFontSize, diagrams)
}

const wordMermaidViewport = 800

const wordMermaidPage = `<!doctype html><meta charset="utf-8"><title>Word diagram</title>
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src '` + mermaidScriptHash + `'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<style>
html { background: white; }
body { box-sizing: border-box; width: 100%; margin: 0; padding: 16px; background: white; }
#diagram { text-align: center; }
svg { max-width: 100%; height: auto; }
</style><pre id="diagram"></pre>`

const wordMermaidRender = `
const host = document.getElementById("diagram");
window.mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: "default",
  flowchart: { useMaxWidth: true }, sequence: { useMaxWidth: true } });
try {
  const diagram = await window.mermaid.render("word-mermaid", source);
  host.innerHTML = diagram.svg;
} catch (_) {
  const errorDiagram = document.querySelector('body > div:not(#diagram) svg');
  if (!errorDiagram) return 0;
  host.replaceChildren(errorDiagram);
  document.querySelectorAll('body > div:not(#diagram)').forEach(node => node.remove());
}
await document.fonts.ready;
return Math.ceil(document.body.scrollHeight);
`

func renderWordDiagrams(parent *mygo.Window, source string) map[string][]byte {
	if parent.IsDestroyed() || !mygo.WebViewAvailable() {
		return nil
	}
	_, _, body := workspace.SplitFrontmatter(source)
	markdown := parser.New(parser.GFM)
	markdown.Set(body)
	var diagrams []string
	seen := make(map[string]bool)
	for index := 1; index < markdown.Len(); index++ {
		block := markdown.Block(index)
		if block.Kind != parser.CodeBlock || !block.Fenced || block.Open {
			continue
		}
		if _, ok := mermaid.Background(block.Info()); !ok {
			continue
		}
		text := markdown.Lines(index)
		if !seen[text] {
			seen[text] = true
			diagrams = append(diagrams, text)
		}
	}
	if len(diagrams) == 0 {
		return nil
	}

	window := mygo.NewWindow(mygo.WindowOptions{
		Title: "Word diagram", Parent: parent, Hidden: true, UseContentSize: true,
		Width: wordMermaidViewport, Height: 600,
		Page: mygo.PageOptions{Untrusted: true, DevTools: mygo.DevToolsDisabled},
	})
	defer window.Destroy()
	page := window.Page()
	loaded := make(chan bool, 1)
	ready := func(ok bool) {
		select {
		case loaded <- ok:
		default:
		}
	}
	page.OnDidFinishLoad(func() { ready(true) })
	page.OnDidFailLoad(func(*mygo.LoadError) { ready(false) })
	page.OnRenderProcessGone(func(string) { ready(false) })
	window.OnClosed(func() { ready(false) })
	page.LoadHTML(wordMermaidPage, "")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	select {
	case ok := <-loaded:
		if !ok {
			return nil
		}
	case <-ctx.Done():
		return nil
	}
	loader, err := mermaidLoader()
	if err != nil {
		return nil
	}
	if _, err := page.EvalContext(ctx, loader); err != nil {
		return nil
	}

	pngs := make(map[string][]byte, len(diagrams))
	for _, text := range diagrams {
		literal, _ := json.Marshal(text)
		value, err := page.EvalContext(ctx, "const source = "+string(literal)+";\n"+wordMermaidRender)
		height, ok := value.(float64)
		if err != nil || !ok || !(height > 0 && height <= 8192) {
			continue
		}
		window.SetContentSize(wordMermaidViewport, int(math.Ceil(height)))
		// Wait for the new viewport to lay out before taking the system WebView snapshot.
		if _, err := page.EvalContext(ctx, `return document.getElementById("diagram").offsetHeight;`); err != nil {
			continue
		}
		if png, err := window.CapturePage(); err == nil && len(png) != 0 {
			pngs[text] = png
		}
	}
	return pngs
}
