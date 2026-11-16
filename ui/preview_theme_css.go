package ui

import (
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
)

const documentThemeElements = `
  html[data-preview-theme] body {
    background: var(--doc-background); color: var(--doc-text);
    font-family: var(--doc-body-font); line-height: var(--preview-line-height, var(--doc-line-height));
    font-feature-settings: var(--doc-font-features);
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
    font-feature-settings: var(--doc-font-features);
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
	features := "normal"
	if theme.editorFont != nil {
		features = theme.editorFont.cssFeatures
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
    --doc-font-features: %s;
    --doc-font-size: %dpt; --doc-line-height: %g;
    --doc-paragraph-gap: %dpx; --doc-heading-gap: %dpx; --doc-heading-after: %dpx;
  }
`, theme.ID, scheme, theme.BackgroundColor, theme.BodyTextColor, theme.HeadingColor,
		theme.LinkColor, theme.LinkHoverColor, theme.CodeBackgroundColor, theme.CodeTextColor,
		theme.BlockquoteBorderColor, theme.BlockquoteTextColor, theme.TableBorderColor, theme.TableHeaderBackground,
		theme.BodyFontFamily, theme.HeadingFontFamily, theme.CodeFontFamily, features, theme.BaseFontSize, theme.LineHeight,
		theme.ParagraphSpacing, theme.HeadingSpacing, theme.HeadingSpacing/2)
}

type documentEditorFont struct {
	family, nativeFamily, features, cssFamily, cssFeatures string
}

var documentEditorFonts sync.Map

func resolveDocumentEditorFont(preference string) *documentEditorFont {
	family := resolvedSourceFontFamily(preference)
	if existing, ok := documentEditorFonts.Load(family); ok {
		return existing.(*documentEditorFont)
	}
	native := sourceEditorFontFamily(family)
	font := &documentEditorFont{family: family, nativeFamily: native, features: sourceFontFeatures(native), cssFamily: "monospace"}
	if family == "" {
		font.nativeFamily = "monospace"
	} else {
		font.cssFamily = cssFontName(family) + ", monospace"
	}
	font.cssFeatures = cssFontFeatures(font.features)
	resolved, _ := documentEditorFonts.LoadOrStore(family, font)
	return resolved.(*documentEditorFont)
}

var cssFontNameEscape = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "<", `\3c `, "\r", `\d `, "\n", `\a `, "\f", `\c `, "\x00", "�")

func cssFontName(family string) string {
	return `"` + cssFontNameEscape.Replace(family) + `"`
}

func cssFontFeatures(features string) string {
	var rules []string
	for _, feature := range strings.Split(features, ",") {
		tag, value, assigned := strings.Cut(strings.TrimSpace(feature), "=")
		if len(tag) != 4 {
			continue
		}
		rule := strconv.Quote(tag)
		if assigned {
			number, err := strconv.Atoi(value)
			if err != nil {
				continue
			}
			rule += " " + strconv.Itoa(number)
		}
		rules = append(rules, rule)
	}
	if len(rules) == 0 {
		return "normal"
	}
	return strings.Join(rules, ", ")
}

var spaceGroteskPreviewFont = sync.OnceValue(func() string {
	return previewFontFace("SpaceGrotesk-Variable", "Space Grotesk", "normal", "300 700")
})

var editorPreviewFonts = map[string]func() string{
	sourceFontDefaultFamily: sync.OnceValue(func() string {
		return previewFontFace("MapleMono-Regular", sourceFontDefaultFamily, "normal", "400") +
			previewFontFace("MapleMono-Bold", sourceFontDefaultFamily, "normal", "700") +
			previewFontFace("MapleMono-Italic", sourceFontDefaultFamily, "italic", "400") +
			previewFontFace("MapleMono-BoldItalic", sourceFontDefaultFamily, "italic", "700")
	}),
	"JetBrains Mono": sync.OnceValue(func() string {
		return previewFontFace("JetBrainsMono-Regular", "JetBrains Mono", "normal", "400")
	}),
	"Fira Code": sync.OnceValue(func() string {
		return previewFontFace("FiraCode-Regular", "Fira Code", "normal", "400")
	}),
	"Cascadia Code": sync.OnceValue(func() string {
		return previewFontFace("CascadiaCode-Regular", "Cascadia Code", "normal", "400")
	}),
	"Space Grotesk": spaceGroteskPreviewFont,
	"Nunito Sans":   nunitoPreviewFonts,
	"Noto Sans": sync.OnceValue(func() string {
		return previewFontFace("NotoSans-Variable", "Noto Sans", "normal", "100 900") +
			previewFontFace("NotoSans-Italic-Variable", "Noto Sans", "italic", "100 900")
	}),
}

func (theme documentPreviewTheme) fontFaces() string {
	if theme.editorFont == nil {
		return previewThemeFonts(theme.ID)
	}
	if fontFaces := editorPreviewFonts[theme.editorFont.family]; fontFaces != nil {
		return fontFaces()
	}
	return ""
}

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
	return fmt.Sprintf(`@font-face { font-family: %s; font-style: %s; font-weight: %s; src: url(data:font/ttf;base64,%s) format("truetype"); }`,
		cssFontName(family), style, weights, base64.StdEncoding.EncodeToString(data))
}
