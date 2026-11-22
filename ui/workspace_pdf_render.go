package ui

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

var lightPDFCodeTheme = func() *editor.Theme {
	palette := *previewCodeTheme(false)
	// The preview's muted comment ink is too faint on a light printed block.
	palette.Tokens[editor.TokenComment].Color = mygoui.Hex("#606168")
	return &palette
}()

const pdfDocumentCSS = `
img { max-width: 100%; height: auto; }
html[data-preview-theme] a { text-decoration: none; }
html[data-preview-theme] pre { padding: 16px; border-radius: 8px; }
html[data-preview-theme] pre:not(.mermaid, .vega-error) { font-family: var(--doc-code-font); }
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

// Quartz's PDF capture can duplicate glyphs in contextual substitutions when
// deriving the searchable text. Standard ligatures use a separate feature.
const pdfQuartzTextCSS = `
html[data-preview-theme] :is(body, code) { font-variant-ligatures: no-contextual; }
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
	baseTheme, _ := findPreviewTheme(doc.theme)
	theme := doc.pdfTheme(baseTheme)
	codeTheme := lightPDFCodeTheme
	if theme.CodeBackgroundColor != "" && !lightPDFColor(theme.CodeBackgroundColor) {
		codeTheme = previewCodeTheme(true)
	}
	if doc.appearance.CodeBackground != "" && lightPDFColor(theme.CodeBackgroundColor) == lightPDFColor(theme.CodeTextColor) {
		ink := codeTheme.Text
		theme.CodeTextColor = fmt.Sprintf("#%02x%02x%02x", ink.R, ink.G, ink.B)
	}
	figures := figureRender{numbered: doc.numberFigures, pdf: true}
	if counter != nil {
		figures.counter = *counter
	}
	markup := previewMarkup{doc: parser.New(webPreviewExtensions), resources: doc.origin.resources(), relative: doc.origin.note,
		charts: true, figures: &figures, codeTheme: codeTheme, liveAPIFences: doc.kind == documentMarkdown || doc.kind == documentUndetermined,
		calloutStyles: doc.calloutStyles()}
	body, title := markup.render(doc.source, doc.name, false)
	if counter != nil {
		*counter = figures.counter
	}
	policy := strings.Replace(previewContentPolicy, "script-src ", "script-src '"+pdfPaperScriptHash+"' ", 1)
	policy = strings.Replace(policy, noteHTMLResourcePolicy, "; img-src data:; media-src 'none'; frame-src 'none'", 1)
	policy = localAssetConnectPolicy(policy)
	css := previewMermaidCSS + previewVegaCSS + previewAPICSS + previewCalloutCSS
	if runtime.GOOS == "darwin" {
		css += pdfQuartzTextCSS
	}
	if baseTheme.ID != "" && doc.appearance != (pdfAppearance{}) {
		css += fmt.Sprintf("html { --pdf-chart-ink: %s; --pdf-chart-paper: %s; --pdf-chart-surface: %s; }",
			pdfChartColor(baseTheme.BodyTextColor), pdfChartColor(baseTheme.BackgroundColor), pdfChartColor(baseTheme.CodeBackgroundColor))
	}
	document = exportPDFPage(body, title, doc, theme, policy, css)
	return document, body
}

func (doc exportDocument) pdfTheme(theme documentPreviewTheme) documentPreviewTheme {
	theme = doc.appearance.applyTo(theme)
	if pdfFontLabel(doc.layout.FontFamily) != pdfThemeDefault {
		family := doc.layout.FontFamily.CSSStack()
		theme.BodyFontFamily, theme.HeadingFontFamily, theme.CodeFontFamily = family, family, family
	} else if doc.hasEditorFont {
		theme = theme.withEditorFont(doc.editorFont)
		if runtime.GOOS == "darwin" && strings.Contains(theme.editorFont.features, mapleMonoOpenTypeFeatures) {
			// Maple explicitly enables calt, which overrides the CSS variant.
			font := *theme.editorFont
			font.features = strings.Replace(font.features, "calt,", "calt=0,", 1)
			font.cssFeatures = cssFontFeatures(font.features)
			theme.editorFont = &font
		}
	}
	return theme
}

func (doc exportDocument) pdfFontFaces(theme documentPreviewTheme) string {
	if pdfFontLabel(doc.layout.FontFamily) == pdfThemeDefault {
		return theme.fontFaces()
	}
	if font := pdfOverrideFonts[doc.layout.FontFamily]; font != nil {
		return font()
	}
	return ""
}

func pdfChartColor(value string) string {
	color := mygoui.Hex(value)
	return fmt.Sprintf("rgb(%d, %d, %d)", color.R, color.G, color.B)
}

// EV-UI-052 waits for typesetting and each chart's completed render before pagination.
func preparePDFPage(page *mygo.Page, body string, doc exportDocument) error {
	baseTheme, _ := findPreviewTheme(doc.theme)
	waitForFonts := doc.customCSS != "" || doc.pdfFontFaces(doc.pdfTheme(baseTheme)) != ""
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
