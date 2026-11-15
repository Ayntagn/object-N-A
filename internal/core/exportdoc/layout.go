package exportdoc

import (
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// PageBreakMarker is the standalone Markdown paragraph observed in EV-EDITOR-004.
const PageBreakMarker = "[pagebreak]"

// PageBreaks locates standalone markers before a math-aware parser can
// mistake an escaped page break for an opening LaTeX delimiter.
func PageBreaks(source string) []struct{ Start, End int } {
	if !strings.Contains(source, PageBreakMarker) {
		return nil
	}
	doc := parser.New(parser.GFM)
	doc.Set(source)
	var result []struct{ Start, End int }
	for i := range doc.Children(0) {
		if doc.Block(i).Kind != parser.Paragraph || doc.LineCount(i) != 1 {
			continue
		}
		line, offset := doc.Line(i, 0)
		if strings.TrimPrefix(strings.TrimSpace(line), `\`) == PageBreakMarker {
			result = append(result, struct{ Start, End int }{offset, offset + len(line)})
		}
	}
	return result
}

// Layout describes paper and margins in points, and typography.
type Layout struct {
	PaperWidth, PaperHeight  float64
	Landscape                bool
	Top, Right, Bottom, Left float64
	FontScale                float64
	FontFamily               FontFamily
	LineSpacing              float64 // Zero keeps the renderer's default.
}

// FontFamily selects a known PDF preview font and its system fallback.
type FontFamily string

func (f FontFamily) CSSStack() string {
	switch f {
	case "System Default", "Sans Serif":
		return `-apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif`
	case "Space Grotesk":
		return `"Space Grotesk", -apple-system, sans-serif`
	case "Serif":
		return `Georgia, "Times New Roman", Times, serif`
	case "Georgia":
		return `Georgia, "Times New Roman", serif`
	case "Palatino":
		return `Palatino, "Palatino Linotype", Georgia, serif`
	case "Times New Roman":
		return `"Times New Roman", Times, serif`
	case "Helvetica":
		return `Helvetica, Arial, sans-serif`
	case "Arial":
		return `Arial, Helvetica, sans-serif`
	case "Verdana":
		return `Verdana, Geneva, sans-serif`
	case "JetBrains Mono", "Fira Code", "Cascadia Code":
		return `"` + string(f) + `", "SF Mono", Menlo, monospace`
	default:
		return "system-ui, sans-serif"
	}
}

func (l Layout) LineHeight() float64 {
	if l.LineSpacing > 0 {
		return l.LineSpacing
	}
	return 1.5
}

// DefaultLayout matches the initial page setup observed in EV-UI-024.
func DefaultLayout() Layout {
	return Layout{
		PaperWidth: 612, PaperHeight: 792,
		Top: 54, Right: 54, Bottom: 54, Left: 54,
		FontScale: 85,
	}
}

func (l Layout) Dimensions() (width, height float64) {
	if l.Landscape {
		return l.PaperHeight, l.PaperWidth
	}
	return l.PaperWidth, l.PaperHeight
}
