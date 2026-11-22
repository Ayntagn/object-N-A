package ui

import (
	"fmt"
	"html"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type standaloneHTMLMode uint8

const (
	htmlInApp standaloneHTMLMode = iota
	htmlResponsive
	htmlPageSetup
)

// EV-UI-137/138: the sheet uses the PDF template's layout and leaves Mermaid
// as source. Share and browser output use the responsive document layout.
func renderExportHTML(doc exportDocument, figureCounter *int) (string, error) {
	return renderStandaloneHTML(doc, htmlPageSetup, figureCounter)
}

func renderStandaloneHTML(doc exportDocument, mode standaloneHTMLMode, figureCounter *int) (string, error) {
	theme, ok := findPreviewTheme(doc.theme)
	if !ok {
		theme, _ = findPreviewTheme("modern")
	}
	if mode == htmlPageSetup {
		theme = doc.appearance.applyTo(theme)
		scale := doc.layout.FontScale
		if scale <= 0 {
			scale = 85
		}
		theme.BaseFontSize = int(math.Round(float64(theme.BaseFontSize) * scale / 100))
		if pdfFontLabel(doc.layout.FontFamily) != pdfThemeDefault {
			theme.BodyFontFamily = doc.layout.FontFamily.CSSStack()
			theme.HeadingFontFamily = theme.BodyFontFamily
		}
	}
	name, body := doc.name, ""
	if mode == htmlResponsive && (doc.kind == documentCode || doc.kind == documentPlainText) {
		body = sourceFilePreview(doc.source, sourceFileBadge(doc.origin.note, documentTypeLabels[doc.kind]), theme.dark())
	} else {
		figures := figureRender{numbered: doc.numberFigures}
		if figureCounter != nil {
			figures.counter = *figureCounter
		}
		markup := previewMarkup{
			doc: parser.New(webPreviewExtensions), resources: doc.origin.resources(), relative: doc.origin.note,
			charts: true, figures: &figures, standalone: mode, codeTheme: previewCodeTheme(theme.dark()),
			liveAPIFences: doc.kind == documentMarkdown || doc.kind == documentUndetermined,
			calloutStyles: doc.calloutStyles(),
		}
		body, name = markup.render(doc.source, name, true)
		body = markup.fillWikiFigures(body)
		if figureCounter != nil {
			*figureCounter = figures.counter
		}
	}
	bootstrap, policy, err := sharedHTMLScripts(body, mode == htmlResponsive)
	if err != nil {
		return "", err
	}
	policy, stylesheet := standaloneCustomStylesheet(policy, doc.customCSS)
	attrs := ` data-preview-theme="` + html.EscapeString(theme.ID) + `"`
	if spacing := previewLineSpacing(doc.source, doc.layout.LineSpacing); spacing > 0 {
		attrs += ` style="--preview-line-height:` + strconv.FormatFloat(spacing, 'f', -1, 64) + `"`
	}
	viewport, layoutCSS := "width=device-width", responsiveExportCSS
	fonts := previewThemeFonts(theme.ID)
	if mode == htmlPageSetup {
		layout := doc.layout
		if paper, found := findExportPaper(doc.paper); found {
			// The two export formats use the same whole-point paper presets.
			layout.PaperWidth, layout.PaperHeight = paper.word.Width, paper.word.Height
		}
		width, _ := layout.Dimensions()
		viewport = fmt.Sprintf("width=%d", int(max(1, width-layout.Left-layout.Right)))
		layoutCSS = pageSetupExportCSS
		if font := pdfOverrideFonts[doc.layout.FontFamily]; font != nil &&
			(doc.layout.FontFamily != "Space Grotesk" || (theme.ID != "synthetic-editorial" && theme.ID != "synthetic-editorial-black")) {
			fonts += font()
		}
	}
	script := ""
	if bootstrap != "" {
		script = "<script>" + bootstrap + "</script>"
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en" dir="ltr"%s><head><meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="%s">
<meta http-equiv="x-dns-prefetch-control" content="off">
<meta name="viewport" content="%s, initial-scale=1.0">
<title>%s</title><style>@layer builtin {%s}</style><style>%s</style>%s</head>
<body><article class="markdown-body">%s</article>%s</body></html>`,
		attrs, html.EscapeString(policy), viewport, html.EscapeString(name),
		theme.cssVariables()+standaloneExportCSS+documentThemeElements+layoutCSS, fonts, stylesheet, body, script), nil
}

var standaloneExportCSS = `
* { box-sizing: border-box; }
body {
  margin: 0; color: var(--doc-text); background: var(--doc-background);
  font-family: var(--doc-body-font); font-size: var(--doc-font-size);
  line-height: var(--preview-line-height, var(--doc-line-height));
}
h1, h2, h3, h4, h5, h6 { line-height: 1.3; }
h1 { font-size: 2em; }
h2 { font-size: 1.5em; }
h3 { font-size: 1.25em; }
h4 { font-size: 1.1em; }
h5 { font-size: 1em; }
h6 { font-size: .9em; }
pre { padding: 16px; border-radius: 8px; overflow-x: auto; }
code { font-size: .9em; padding: 2px 6px; border-radius: 4px; }
pre code { padding: 0; }
img { max-width: 100%; height: auto; }
table { border-collapse: collapse; }
th, td { padding: 8px 12px; text-align: left; border: 1px solid var(--doc-table-border); }
th.center, td.center { text-align: center; }
th.right, td.right { text-align: right; }
.table-scroll { overflow-x: auto; }
html[data-preview-theme] code, html[data-preview-theme] pre {
  line-height: var(--preview-line-height, var(--doc-line-height));
}
.raw-markup { white-space: pre-wrap; }
.embed-note { border-inline-start: 3px solid var(--doc-link); padding: 8px 12px; margin: 8px 0; opacity: .85; }
.embed-note > header { font-size: .75em; opacity: .5; margin-bottom: 4px; }
figure.md-figure { break-inside: avoid; }
` + previewMermaidCSS + previewVegaCSS + previewAPICSS + previewCalloutCSS + sourceFilePreviewCSS

const responsiveExportCSS = `
body { max-width: 800px; margin-inline: auto; padding: 24px; }
html[data-preview-theme] a.wikilink {
  text-decoration: none; border-bottom: 1px dashed var(--doc-link); padding-bottom: 1px;
}
html[data-preview-theme] a.wikilink:hover { border-bottom-style: solid; }
.tag {
  color: var(--doc-quote-text); background: var(--doc-code-background);
  padding: 1px 6px; border-radius: 10px; font-size: .85em; white-space: nowrap;
}
figure.md-figure { margin: 1.2em 0; padding: 0; text-align: center; }
figure.md-figure img { border-radius: 6px; }
figure.md-figure > figcaption { margin-top: .5em; font-size: .88em; line-height: 1.4; opacity: .75; }
figure.md-figure .figure-label { font-weight: 600; opacity: .9; }
hr.page-break { border: 0; border-top: 2px dashed #999; margin: 2em 0; position: relative; }
hr.page-break::after {
  content: "PAGE BREAK"; position: absolute; top: -10px; left: 50%; transform: translateX(-50%);
  background: var(--doc-background); color: #999; font: 10px system-ui, sans-serif; padding: 0 8px;
}
`

const pageSetupExportCSS = `
body { padding: 0; orphans: 3; widows: 3; }
article.markdown-body { margin: 0; padding: 0; }
html[data-preview-theme] a { text-decoration: none; }
html[data-preview-theme] .table-scroll { display: contents; }
html[data-preview-theme] table { margin-block: var(--doc-paragraph-gap); }
h1, h2, h3, h4, h5, h6 { break-after: avoid; break-inside: avoid; }
p, pre, img, .math-display { break-inside: avoid; }
pre, pre code { white-space: pre-wrap; overflow-wrap: break-word; }
pre.mermaid { text-align: center; border: 1px solid color-mix(in srgb, var(--doc-code-background), var(--doc-text) 10%); }
hr.page-break { height: 0; margin: 0; padding: 0; border: 0; visibility: hidden; break-after: page; }
`

func standaloneRelativeURL(raw string) string {
	target, err := url.Parse(raw)
	if err != nil || target.Scheme != "" || target.Host != "" || target.User != nil || target.Opaque != "" ||
		strings.HasPrefix(raw, "/") || strings.ContainsRune(raw, '\\') || strings.TrimSpace(raw) != raw {
		return ""
	}
	return raw
}

func (m *previewMarkup) markdownImageSource(destination string) string {
	if src := m.imageSource(destination, false); src != "" {
		return src
	}
	if m.standalone != htmlInApp {
		return html.EscapeString(standaloneRelativeURL(destination))
	}
	return ""
}

func (m *previewMarkup) writeHighlightedFence(language, anchor, source string) bool {
	language = strings.ToLower(language)
	lexer := nativeFenceTokenizer(language)
	source = strings.TrimSuffix(source, "\n")
	var additional []editor.Token
	if lexer == nil {
		fallback := additionalFenceLexer(language)
		if fallback == nil {
			return false
		}
		additional = tokenizeAdditionalFence(fallback, source)
		if len(additional) == 0 {
			return false
		}
	}
	attr := html.EscapeString(language)
	fmt.Fprintf(&m.out, `<pre%s><code class="hljs language-%s" data-language="%s">`, anchor, attr, attr)
	if lexer == nil {
		start := 0
		for _, run := range additional {
			color := m.codeTheme.TokenColor(run.Kind)
			fmt.Fprintf(&m.out, `<span style="color:#%02x%02x%02x">%s</span>`, color.R, color.G, color.B,
				html.EscapeString(source[start:run.End]))
			start = run.End
		}
	} else {
		var tokens []editor.Token
		var state uint64
		for i, line := range strings.Split(source, "\n") {
			if i > 0 {
				m.out.WriteByte('\n')
			}
			tokens, state = lexer.Tokenize(line, state, tokens[:0])
			start := 0
			for _, token := range tokens {
				color := m.codeTheme.Tokens[token.Kind].Color
				if token.Kind == editor.TokenText || token.Kind == editor.TokenVariable {
					color = m.codeTheme.Text
				}
				fmt.Fprintf(&m.out, `<span style="color:#%02x%02x%02x">%s</span>`, color.R, color.G, color.B,
					html.EscapeString(line[start:token.End]))
				start = token.End
			}
			m.out.WriteString(html.EscapeString(line[start:]))
		}
	}
	m.out.WriteString("</code></pre>")
	return true
}
