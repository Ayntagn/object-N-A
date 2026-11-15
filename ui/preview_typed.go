package ui

import (
	"fmt"
	"html"
	"path"
	"strings"

	mygoui "github.com/egoist/mygo/ui"
)

const sourceFilePreviewMessage = "Source file - no rendered preview"

func sourceFileBadge(filename, fallback string) string {
	if extension := strings.TrimPrefix(path.Ext(filename), "."); extension != "" {
		return strings.ToUpper(extension)
	}
	return fallback
}

func sourceFilePreview(source, badge string, dark bool) string {
	class := "source-file-preview"
	if dark {
		class += " source-file-dark"
	}
	return fmt.Sprintf(`<section class="%s"><header><span class="source-file-badge">%s</span><span class="source-file-note">%s</span></header><pre>%s</pre></section>`,
		class, html.EscapeString(badge), sourceFilePreviewMessage, html.EscapeString(source))
}

func drawSourceFilePreview(c *mygoui.Context, label, text, badge string, zoom int) {
	mygoui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Label(label).Padding(20).Children(func() {
		mygoui.Row(c).Gap(8).AlignItems(mygoui.Center).Children(func() {
			mygoui.Text(c, badge).FontSize(12).FontWeight(500)
			mygoui.Text(c, sourceFilePreviewMessage).FontSize(13).TextColor(c.Theme().TextMuted)
		})
		mygoui.Text(c, text).Selectable().Font("monospace").FontSize(13*float32(zoom)/100).
			LineHeight(1.5).FillWidth().Padding(16).Margin(16, 0, 0, 0).Background(c.Theme().Surface).Radius(6)
	})
}

const sourceFilePreviewCSS = `
main:has(> .source-file-preview) { max-width: none; padding: 20px; }
.source-file-preview {
  font: 16px/normal -apple-system, sans-serif;
  color: #333333;
  --source-file-fill: rgba(0,0,0,.03);
  --source-file-border: rgba(0,0,0,.1);
}
.source-file-preview.source-file-dark {
  color: #e0e0e0;
  --source-file-fill: rgba(255,255,255,.05);
  --source-file-border: rgba(255,255,255,.1);
}
.source-file-preview header { display: flex; align-items: center; gap: 8px; margin-bottom: 16px;
  padding-bottom: 12px; border-bottom: 1px solid var(--source-file-border); }
.source-file-badge { padding: 4px 10px; border-radius: 4px; font-size: 12px; font-weight: 500; }
.source-file-note { font-size: 13px; opacity: .7; }
.source-file-badge, html[data-preview-theme] .source-file-preview pre, .source-file-preview pre {
  background: var(--source-file-fill); border: 1px solid var(--source-file-border);
}
.source-file-preview pre { margin: 0; padding: 16px; border-radius: 6px; overflow-x: auto;
  font: 13px/1.5 'SF Mono', Menlo, Monaco, monospace; }
`
