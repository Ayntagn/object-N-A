package ui

import (
	"fmt"
	"html"
	"path"
	"strings"

	"cyberwriter/internal/core/workspace"
)

func renderDelimitedPreview(source, filename string, dark bool) (string, bool) {
	rows := workspace.ParseDelimitedPreview(source, filename)
	if len(rows) == 0 {
		return "", false
	}
	class := "delimited-preview"
	if dark {
		class += " delimited-preview-dark"
	}
	var markup strings.Builder
	fmt.Fprintf(&markup, `<section class="%s"><div class="delimited-preview-info">%s - %d rows, %d columns</div>`,
		class, html.EscapeString(path.Base(filename)), len(rows)-1, len(rows[0]))
	markup.WriteString(`<div class="delimited-preview-scroll"><table>`)
	for i, row := range rows {
		tag := "td"
		if i == 0 {
			tag = "th"
		}
		markup.WriteString(`<tr>`)
		for _, value := range row {
			fmt.Fprintf(&markup, `<%s>%s</%s>`, tag, html.EscapeString(value), tag)
		}
		markup.WriteString(`</tr>`)
	}
	markup.WriteString(`</table></div></section>`)
	return markup.String(), true
}

// EV-UI-179 records the delimited table template and theme palettes.
const delimitedPreviewCSS = `
main:has(> .delimited-preview) { max-width: none; padding: 20px; }
.delimited-preview {
  font: 16px/normal -apple-system, sans-serif;
  color: #333333;
  overflow-wrap: normal;
  --delimited-border: rgba(0,0,0,.1);
  --delimited-header: rgba(0,0,0,.06);
  --delimited-stripe: rgba(0,0,0,.02);
}
.delimited-preview.delimited-preview-dark {
  color: #e0e0e0;
  --delimited-border: rgba(255,255,255,.1);
  --delimited-header: rgba(255,255,255,.08);
  --delimited-stripe: rgba(255,255,255,.03);
}
.delimited-preview-info { font-size: 12px; opacity: .5; margin-bottom: 12px; }
.delimited-preview-scroll { overflow: auto; max-height: calc(100vh - 60px); }
.delimited-preview table { width: 100%; border-collapse: collapse; font-size: 13px; }
.delimited-preview .delimited-preview-scroll :is(th, td) {
  padding: 6px 12px;
  border: 1px solid var(--delimited-border);
  text-align: left;
  white-space: nowrap;
}
.delimited-preview .delimited-preview-scroll th {
  background: var(--delimited-header);
  font-weight: 600;
  position: sticky;
  top: 0;
  z-index: 1;
}
.delimited-preview tr:nth-child(2n + 3) { background: var(--delimited-stripe); }
`
