package ui

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"

	"cyberwriter/internal/core/exportdoc"
	"github.com/egoist/mygo/ui/markdown/parser"
)

//go:embed webassets/katex/katex.min.js
var katexJS string

//go:embed webassets/katex/katex.min.css
var katexCSS string

//go:embed webassets/katex/fonts/*.woff2
var katexFonts embed.FS

const previewMathReadyEvent = "cyberwriter:math-ready"

func containsPreviewMath(body string) bool {
	return strings.Contains(body, `class="math-inline"`) || strings.Contains(body, `class="math-display"`)
}

func previewMathLoader() (string, error) {
	return `
if (typeof window.renderWorkspacePreviewMath !== "function") {
  (() => {` + previewMathScript() + `
    window.renderWorkspacePreviewMath = renderMath;
  })();
}
window.dispatchEvent(new Event("` + previewMathReadyEvent + `"));
return true;`, nil
}

// A stray dollar can close inside the next code span. Keep the parser's
// original code boundaries when that happens and escape the false opener in
// the preview's copy of the source.
func setPreviewDocument(doc *parser.Document, source string) {
	if doc.Extensions()&parser.ExtMath != 0 && strings.Contains(source, `\`+exportdoc.PageBreakMarker) {
		var normalized strings.Builder
		start := 0
		for _, marker := range exportdoc.PageBreaks(source) {
			if offset := strings.IndexByte(source[marker.Start:marker.End], '\\'); offset >= 0 {
				at := marker.Start + offset
				normalized.WriteString(source[start:at])
				// An extra leading space keeps preview line offsets stable.
				normalized.WriteByte(' ')
				start = at + 1
			}
		}
		if start != 0 {
			normalized.WriteString(source[start:])
			source = normalized.String()
		}
	}
	doc.Set(source)
	if doc.Extensions()&parser.ExtDollarMath == 0 || !strings.ContainsRune(source, '`') {
		return
	}
	var candidates []parser.Inline
	for block := 1; block < doc.Len(); block++ {
		for _, formula := range doc.Inlines(block) {
			if formula.Kind == parser.Math && !formula.Display && strings.ContainsRune(formula.Text, '`') &&
				formula.Start >= 0 && formula.Start < len(source) && source[formula.Start] == '$' {
				candidates = append(candidates, formula)
			}
		}
	}
	if len(candidates) == 0 {
		return
	}

	literal := parser.New(doc.Extensions() &^ parser.ExtDollarMath)
	literal.Set(source)
	var openers []int
	for block := 1; block < literal.Len(); block++ {
		for _, code := range literal.Inlines(block) {
			if code.Kind != parser.Code {
				continue
			}
			for _, formula := range candidates {
				if formula.Start < code.Start && code.Start < formula.End && formula.End < code.End {
					openers = append(openers, formula.Start)
				}
			}
		}
	}
	if len(openers) == 0 {
		return
	}
	slices.Sort(openers)
	var text strings.Builder
	start := 0
	for _, opener := range slices.Compact(openers) {
		text.WriteString(source[start:opener])
		text.WriteByte('\\')
		start = opener
	}
	text.WriteString(source[start:])
	doc.Set(text.String())
}

// The WebView loads the fonts without a file origin or a network request.
var previewMathScript = sync.OnceValue(func() string {
	files, _ := fs.Glob(katexFonts, "webassets/katex/fonts/*.woff2")
	replacements := make([]string, 0, 6*len(files))
	for _, file := range files {
		font, _ := katexFonts.ReadFile(file)
		name := strings.TrimSuffix(path.Base(file), ".woff2")
		replacements = append(replacements,
			"url(fonts/"+name+".woff2)", "url(data:font/woff2;base64,"+base64.StdEncoding.EncodeToString(font)+")",
			`,url(fonts/`+name+`.woff) format("woff")`, "",
			`,url(fonts/`+name+`.ttf) format("truetype")`, "",
		)
	}
	css, _ := json.Marshal(strings.NewReplacer(replacements...).Replace(katexCSS))
	return `
  let mathReady = false;
  const renderedMath = new WeakSet();
  function renderMath(preview) {
    const formulas = preview.querySelectorAll(".math-inline, .math-display");
    if (formulas.length === 0) return;
    if (!mathReady) {
` + katexJS + `
      const style = document.createElement("style");
      style.textContent = ` + string(css) + `;
      document.head.insertBefore(style, document.getElementById("custom-stylesheet"));
      mathReady = true;
    }
    for (const formula of formulas) {
      if (renderedMath.has(formula)) continue;
      window.katex.render(formula.textContent, formula, {
        displayMode: formula.classList.contains("math-display"),
        throwOnError: false,
        trust: false,
      });
      renderedMath.add(formula);
    }
  }
`
})
