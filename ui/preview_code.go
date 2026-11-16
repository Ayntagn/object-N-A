package ui

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	previewCodeCopyScheme  = "cyberwriter-copy-code:"
	previewCodeCopiedEvent = "cyberwriter:code-copied"
)

var previewCodeCopied = mygo.NewEvent[string](previewCodeCopiedEvent)

type previewCodeCopies struct {
	revision uint64
	blocks   []string
}

func (c *previewCodeCopies) register(code string) string {
	c.blocks = append(c.blocks, code)
	return fmt.Sprintf("%s//block/%d/%d", previewCodeCopyScheme, c.revision, len(c.blocks))
}

func (c previewCodeCopies) codeForURL(raw string) (string, bool) {
	prefix := fmt.Sprintf("%s//block/%d/", previewCodeCopyScheme, c.revision)
	suffix, ok := strings.CutPrefix(raw, prefix)
	if !ok {
		return "", false
	}
	index, err := strconv.Atoi(suffix)
	if err != nil || index < 1 || index > len(c.blocks) || strconv.Itoa(index) != suffix {
		return "", false
	}
	return c.blocks[index-1], true
}

func (p *workspacePreview) handleCodeCopyNavigation(app *workspaceWindow, view *previewViewport, target string) bool {
	if !strings.HasPrefix(target, previewCodeCopyScheme) {
		return false
	}
	app.win.Update(func() {
		if p.view != view || view.IsDestroyed() || !p.ready || app.previewDocumentType() != documentMarkdown ||
			p.tab != app.workspaceTab || p.draft != app.draft || p.path != app.document.Path {
			return
		}
		if code, ok := p.codeCopies.codeForURL(target); ok {
			if err := mygo.Clipboard.Write(transfer.TextData(code)); err == nil {
				_ = previewCodeCopied.EmitPage(view.Page(), target)
			}
		}
	})
	return true
}

var darkPreviewCodeTheme = func() *editor.Theme {
	palette := *sourceThemeForID("night-owl").palette
	// Night Owl uses the same cyan for CSS selectors and properties.
	palette.Tokens[editor.TokenProperty].Color = mygoui.Hex("#a9c7ff")
	// Night Owl assigns the same grey to both change kinds.
	changes := sourceThemeForID("github-dark").palette
	palette.Tokens[editor.TokenInserted].Color = changes.TokenColor(editor.TokenInserted)
	palette.Tokens[editor.TokenDeleted].Color = changes.TokenColor(editor.TokenDeleted)
	return &palette
}()

var lightPreviewCodeTheme = func() *editor.Theme {
	palette := *sourceThemeForID("atom-one-light").palette
	// Separate markup attributes from quoted values and property names from numbers.
	palette.Tokens[editor.TokenAttribute].Color = mygoui.Hex("#7d3d5a")
	palette.Tokens[editor.TokenProperty].Color = mygoui.Hex("#235a96")
	return &palette
}()

func previewCodeTheme(dark bool) *editor.Theme {
	if dark {
		return darkPreviewCodeTheme
	}
	return lightPreviewCodeTheme
}

func (m *previewMarkup) writeFencedCode(language, anchor, source string) {
	interactive := m.codeCopies != nil && language != ""
	if interactive {
		tag := html.EscapeString(language)
		fmt.Fprintf(&m.out, `<div class="preview-code-block"><div class="preview-code-header"><a class="preview-code-copy" href="%s" aria-label="Copy %s code" title="Copy code"><span aria-live="polite">%s</span></a></div>`,
			m.codeCopies.register(source), tag, tag)
	}
	if m.codeTheme == nil || !m.writeHighlightedFence(language, anchor, source) {
		fmt.Fprintf(&m.out, "<pre%s><code>%s</code></pre>", anchor, html.EscapeString(source))
	}
	if interactive {
		m.out.WriteString("</div>")
	}
}

const previewCodeCSS = `
  .preview-code-block {
    background: var(--doc-code-background, rgba(127,127,127,.14));
    border-radius: 8px;
    margin: 1em 0;
  }
  .preview-code-header { display: flex; justify-content: flex-end; padding: 6px 9px 0; }
  .preview-code-block > pre { background: transparent; margin: 0; padding-top: 4px; }
  .preview-code-copy {
    box-sizing: border-box;
    max-width: 100%;
    min-height: 22px;
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
    padding: 2px 7px;
    border-radius: 5px;
    font: 0.8rem/18px ui-monospace, monospace;
    color: var(--doc-text, CanvasText);
    background: color-mix(in srgb, currentColor 9%, transparent);
    text-decoration: none;
    user-select: none;
  }
  .preview-code-copy:hover { background: color-mix(in srgb, currentColor 16%, transparent); }
  .preview-code-copy:focus-visible { outline: 2px solid currentColor; outline-offset: 2px; }
`

const previewCodeScript = `
  const copyLabels = new WeakMap();
  window.mygo.on("` + previewCodeCopiedEvent + `", target => {
    const link = Array.from(document.querySelectorAll(".preview-code-copy"))
      .find(node => node.getAttribute("href") === target);
    const label = link?.querySelector("span");
    if (!label) return;
    const prior = copyLabels.get(label);
    const language = prior?.language ?? label.textContent;
    if (prior) clearTimeout(prior.timer);
    label.textContent = "Copied";
    const timer = setTimeout(() => {
      label.textContent = language;
      copyLabels.delete(label);
    }, 1400);
    copyLabels.set(label, {language, timer});
  });
`
