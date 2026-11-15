package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const pdfDocumentCSS = `
img { max-width: 100%; height: auto; }
html[data-preview-theme] a { text-decoration: none; }
html[data-preview-theme] pre { padding: 16px; border-radius: 8px; }
html[data-preview-theme] code { padding: 2px 6px; border-radius: 4px; font-size: .9em; }
html[data-preview-theme] pre code { padding: 0; font-size: .85em; }
html[data-preview-theme] :is(td, th) { padding: 8px 12px; }
html[data-preview-theme] .table-scroll { margin-block: 0; }
html[data-preview-theme] table { margin-block: var(--doc-paragraph-gap); }
html[data-preview-theme] pre.mermaid[data-mermaid-transparent] { padding: 16px 0; }
pre.mermaid:not([data-mermaid-transparent]) { background: var(--pdf-chart-surface, var(--doc-code-background)); }
pre.mermaid > svg { display: inline; margin: 0; }
.vega-stage > svg { max-width: 100%; height: auto; }
pre.mermaid[data-mermaid-state="rendered"] { white-space: normal; }
pre.vega-error, .mermaid-error { color: #9c2230; }
article.markdown-body { margin: 0; padding: 0; }
h1 { font-size: 2em; }
h2 { font-size: 1.5em; }
h3 { font-size: 1.25em; }
h4 { font-size: 1.1em; }
h5 { font-size: 1em; }
h6 { font-size: 0.9em; }
@media print {
  pre.mermaid, [data-vega-preview], .math-display { break-inside: avoid; }
}
`

// Keep screen media decisions when the system print backend changes to print media.
const pdfStylesheetMedia = `
const stylesheet = document.getElementById("custom-stylesheet").sheet;
function useScreenMedia(rules) {
  for (let i = rules.cssRules.length - 1; i >= 0; i--) {
    const rule = rules.cssRules[i];
    if (rule.type === CSSRule.PAGE_RULE) {
      rules.deleteRule(i);
      continue;
    }
    if (rule instanceof CSSMediaRule) {
      rule.media.mediaText = matchMedia(rule.media.mediaText).matches ? "all" : "not all";
    }
    if (rule.cssRules) useScreenMedia(rule);
  }
}
useScreenMedia(stylesheet);
document.querySelector("article").getBoundingClientRect();
await document.fonts.ready;
`

func renderPDFHTML(doc exportDocument) (document, body string) {
	return renderPDFHTMLWithFigures(doc, nil)
}

func renderPDFHTMLWithFigures(doc exportDocument, counter *int) (document, body string) {
	figures := figureRender{numbered: doc.numberFigures, pdf: true}
	if counter != nil {
		figures.counter = *counter
	}
	markup := previewMarkup{doc: parser.New(webPreviewExtensions), resources: doc.origin.resources(), relative: doc.origin.note,
		charts: true, figures: &figures, liveAPIFences: doc.kind == documentMarkdown || doc.kind == documentUndetermined}
	body, title := markup.render(doc.source, doc.name, false)
	if counter != nil {
		*counter = figures.counter
	}
	policy := strings.Replace(previewContentPolicy, "script-src ", "script-src '"+pdfPaperScriptHash+"' ", 1)
	css := previewMermaidCSS + previewVegaCSS + previewAPICSS
	if theme, ok := findPreviewTheme(doc.theme); ok && doc.appearance != (pdfAppearance{}) {
		css += fmt.Sprintf("html { --pdf-chart-ink: %s; --pdf-chart-paper: %s; --pdf-chart-surface: %s; }",
			pdfChartColor(theme.BodyTextColor), pdfChartColor(theme.BackgroundColor), pdfChartColor(theme.CodeBackgroundColor))
	}
	document = exportHTMLPage(body, title, doc, policy, css, exportPDF)
	return document, body
}

func pdfChartColor(value string) string {
	color := mygoui.Hex(value)
	return fmt.Sprintf("rgb(%d, %d, %d)", color.R, color.G, color.B)
}

// EV-UI-052 waits for typesetting and each chart's completed render before pagination.
func preparePDFPage(page *mygo.Page, body string, doc exportDocument) error {
	waitForFonts := doc.customCSS != "" || previewThemeFonts(doc.theme) != "" || pdfOverrideFonts[doc.layout.FontFamily] != nil
	math := strings.Contains(body, `class="math-inline"`) || strings.Contains(body, `class="math-display"`)
	mermaid := strings.Contains(body, mermaidBlockStart)
	vega := strings.Contains(body, vegaBlockMarker)
	if !math && !mermaid && !vega && !waitForFonts {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var script strings.Builder
	script.WriteString(`const preview = document.querySelector("article.markdown-body");` + "\n")
	if math {
		script.WriteString(previewMathScript())
		script.WriteString("renderMath(preview);\n")
	}
	if mermaid {
		if err := loadPDFAsset(ctx, page, "Mermaid", mermaidLoader); err != nil {
			return err
		}
		script.WriteString(previewMermaidJS)
		script.WriteString("await renderDiagrams(preview);\n")
	}
	if vega {
		if err := loadPDFAsset(ctx, page, "Vega", vegaLoader); err != nil {
			return err
		}
		script.WriteString(previewVegaJS)
		script.WriteString("await renderVegaCharts(preview);\n")
	}
	// Measure after the renderers update the DOM so the font promise covers the new glyphs.
	script.WriteString("preview.getBoundingClientRect();\nawait document.fonts.ready;\nreturn true;")
	value, err := page.EvalContext(ctx, script.String())
	if err != nil {
		return fmt.Errorf("Could not render PDF content. %w", err)
	}
	if value != true {
		return errors.New("PDF content did not finish rendering")
	}
	return nil
}

func loadPDFAsset(ctx context.Context, page *mygo.Page, name string, loader func() (string, error)) error {
	script, err := loader()
	if err == nil {
		_, err = page.EvalContext(ctx, script)
	}
	if err != nil {
		return fmt.Errorf("Could not load %s for PDF export. %w", name, err)
	}
	return nil
}
