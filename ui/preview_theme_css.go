package ui

import (
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"sync"
)

const documentThemeElements = `
  html[data-preview-theme] body {
    background: var(--doc-background); color: var(--doc-text);
    font-family: var(--doc-body-font); line-height: var(--preview-line-height, var(--doc-line-height));
  }
  html[data-preview-theme] :is(h1, h2, h3, h4, h5, h6) {
    color: var(--doc-heading); font-family: var(--doc-heading-font);
    margin-block: var(--doc-heading-gap) var(--doc-heading-after);
  }
  html[data-preview-theme] h6 { color: var(--doc-quote-text); }
  html[data-preview-theme] p { margin: 0 0 var(--doc-paragraph-gap); }
  html[data-preview-theme] a { color: var(--doc-link); text-decoration: underline 1px; text-underline-offset: 3px; }
  html[data-preview-theme] a:hover { color: var(--doc-link-hover); }
  html[data-preview-theme] code {
    font-family: var(--doc-code-font); line-height: var(--preview-line-height, var(--doc-line-height));
    background: var(--doc-code-background); color: var(--doc-code-text);
  }
  html[data-preview-theme] pre { background: var(--doc-code-background); }
  html[data-preview-theme] pre:not(.source-file-preview pre) { margin-block: var(--doc-paragraph-gap); }
  html[data-preview-theme] pre code { background: none; font-size: .85em; }
  html[data-preview-theme] blockquote {
    margin: var(--doc-paragraph-gap) 0; padding: 8px 16px;
    border-inline-start: 4px solid var(--doc-quote-border);
    background: var(--doc-code-background); color: var(--doc-quote-text);
  }
  html[data-preview-theme] blockquote p:last-child { margin-bottom: 0; }
  html[data-preview-theme] .table-scroll { margin-block: var(--doc-paragraph-gap); }
  html[data-preview-theme] table { width: 100%; }
  html[data-preview-theme] :is(td, th) { border-color: var(--doc-table-border); }
  html[data-preview-theme] th { background: var(--doc-table-header); font-weight: 600; }
`

const documentPreviewLayout = `
  html[data-preview-theme] *, html[data-preview-theme] *::before, html[data-preview-theme] *::after { box-sizing: border-box; }
  html[data-preview-theme] body { font-size: var(--doc-font-size); }
  html[data-preview-theme] main:not(:has(> :is(.source-file-preview, .delimited-preview))) { padding: 24px; max-width: 800px; }
`

var documentPreviewPalettes = func() string {
	var styles strings.Builder
	for _, theme := range previewThemes {
		styles.WriteString(theme.cssVariables())
	}
	return styles.String()
}()

func (theme documentPreviewTheme) cssVariables() string {
	scheme := "light"
	if theme.dark() {
		scheme = "dark"
	}
	return fmt.Sprintf(`
  html[data-preview-theme=%q] {
    color-scheme: %s;
    --doc-background: %s; --doc-text: %s; --doc-heading: %s;
    --doc-link: %s; --doc-link-hover: %s; --mermaid-link: var(--doc-link);
    --doc-code-background: %s; --doc-code-text: %s;
    --doc-quote-border: %s; --doc-quote-text: %s;
    --doc-table-border: %s; --doc-table-header: %s;
    --doc-body-font: %s; --doc-heading-font: %s; --doc-code-font: %s;
    --doc-font-size: %dpt; --doc-line-height: %g;
    --doc-paragraph-gap: %dpx; --doc-heading-gap: %dpx; --doc-heading-after: %dpx;
  }
`, theme.ID, scheme, theme.BackgroundColor, theme.BodyTextColor, theme.HeadingColor,
		theme.LinkColor, theme.LinkHoverColor, theme.CodeBackgroundColor, theme.CodeTextColor,
		theme.BlockquoteBorderColor, theme.BlockquoteTextColor, theme.TableBorderColor, theme.TableHeaderBackground,
		theme.BodyFontFamily, theme.HeadingFontFamily, theme.CodeFontFamily, theme.BaseFontSize, theme.LineHeight,
		theme.ParagraphSpacing, theme.HeadingSpacing, theme.HeadingSpacing/2)
}

var spaceGroteskPreviewFont = sync.OnceValue(func() string {
	return previewFontFace("SpaceGrotesk-Variable", "Space Grotesk", "normal", "300 700")
})

var nunitoPreviewFonts = sync.OnceValue(func() string {
	return previewFontFace("NunitoSans-Variable", "Nunito Sans", "normal", "200 1000") +
		previewFontFace("NunitoSans-Italic-Variable", "Nunito Sans", "italic", "200 1000")
})

func previewThemeFonts(id string) string {
	switch id {
	case "synthetic-editorial", "synthetic-editorial-black":
		return spaceGroteskPreviewFont()
	case "modern":
		return nunitoPreviewFonts()
	default:
		return ""
	}
}

func previewFontFace(file, family, style, weights string) string {
	data, err := bundledSourceFont(file)
	if err != nil {
		log.Printf("Load preview font %s: %v", family, err)
		return ""
	}
	return fmt.Sprintf(`@font-face { font-family: %q; font-style: %s; font-weight: %s; src: url(data:font/ttf;base64,%s) format("truetype"); }`,
		family, style, weights, base64.StdEncoding.EncodeToString(data))
}
