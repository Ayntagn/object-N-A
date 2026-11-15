package ui

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
)

func shareExportPayload(win *mygo.Window, document exportDocument, format exportFormat, blackout bool, pdf []byte) ([]byte, error) {
	if format == exportHTML {
		markup, err := renderSharedHTML(document, blackout)
		return []byte(markup), err
	}
	return exportPayload(win, document, format, pdf)
}

func browserPreviewMarkup(document exportDocument, blackout bool) (string, error) {
	if document.kind == documentHTML {
		return document.source, nil
	}
	return renderSharedHTML(document, blackout)
}

// EV-UI-061: direct HTML export and browser preview share the complete renderer.
func renderSharedHTML(document exportDocument, blackout bool) (string, error) {
	if document.kind == documentHTML {
		return renderHTMLPreview(document.source, blackout, document.origin)
	}
	return renderStandaloneHTML(document, htmlResponsive, nil)
}

func previewLineSpacing(source string, defaultSpacing float64) float64 {
	_, properties, _ := workspace.SplitFrontmatter(source)
	for line := range strings.SplitSeq(properties, "\n") {
		key, value, _ := strings.Cut(line, ":")
		if strings.TrimRight(key, " \t") != "line-height" {
			continue
		}
		value, _, _ = strings.Cut(value, "#")
		if height, err := strconv.ParseFloat(strings.Trim(strings.TrimSpace(value), `"'`), 64); err == nil && height >= 0.8 && height <= 4 {
			return height
		}
	}
	return defaultSpacing
}

func sharedHTMLScripts(body string, includeMermaid bool) (bootstrap, policy string, err error) {
	policy = strings.Replace(previewContentPolicy, "connect-src https: http: data:", "connect-src data:", 1)
	var renderers strings.Builder
	if strings.Contains(body, `class="math-inline"`) || strings.Contains(body, `class="math-display"`) {
		renderers.WriteString(previewMathScript())
		renderers.WriteString(`renderMath(document.querySelector("article.markdown-body"));`)
	}
	for _, asset := range []struct {
		marker, script string
		load           func() (string, error)
	}{{mermaidBlockStart, previewMermaidJS, mermaidLoader}, {vegaBlockMarker, previewVegaJS, vegaLoader}} {
		if strings.Contains(body, asset.marker) && (asset.marker != mermaidBlockStart || includeMermaid) {
			loader, err := asset.load()
			if err != nil {
				return "", "", err
			}
			renderers.WriteString(asset.script)
			fmt.Fprintf(&renderers, "\n(() => {\n%s\n})();\n", loader)
		}
	}
	if renderers.Len() == 0 {
		return "", policy, nil
	}
	script := "(() => {\n" + renderers.String() + "\n})();"
	encoded, _ := json.Marshal(script)
	// JSON quoting keeps the upstream JavaScript out of the HTML parser.
	bootstrap = `const script = document.createElement("script"); script.textContent = ` + string(encoded) + `; document.head.append(script); script.remove();`
	for _, text := range []string{script, bootstrap} {
		digest := sha256.Sum256([]byte(text))
		policy = strings.Replace(policy, "script-src ", "script-src 'sha256-"+base64.StdEncoding.EncodeToString(digest[:])+"' ", 1)
	}
	return bootstrap, policy, nil
}

func previewHTMLShell(title, body, policy, theme string, spacing float64, bootstrap, customCSS string) string {
	var attrs, script string
	if theme != "" {
		attrs = ` data-preview-theme="` + html.EscapeString(theme) + `"`
	}
	if spacing > 0 {
		attrs += ` style="--preview-line-height:` + strconv.FormatFloat(spacing, 'f', -1, 64) + `"`
	}
	if bootstrap != "" {
		script = "<script>" + bootstrap + "</script>"
	}
	policy, stylesheet := standaloneCustomStylesheet(policy, customCSS)
	if stylesheet == "" {
		stylesheet = `<style id="custom-stylesheet"></style>`
	}
	return fmt.Sprintf(`<!doctype html><html lang="en"%s><head><meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="%s">
<meta http-equiv="x-dns-prefetch-control" content="off">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title><style>@layer builtin {%s}</style><style id="preview-theme-fonts">%s</style>%s</head><body>
<main class="markdown-body" aria-label="Markdown Preview">%s</main>%s</body></html>`,
		attrs, html.EscapeString(policy), html.EscapeString(title), previewStyles, previewThemeFonts(theme), stylesheet, body, script)
}
