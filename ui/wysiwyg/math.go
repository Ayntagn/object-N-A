package wysiwyg

import (
	"crypto/rand"
	"regexp"
	"strings"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
	"golang.org/x/image/colornames"
)

const (
	formulaLabel = "Formula"
	formulaURL   = "cyberwriter-formula:"
	mathCaretID  = "cyberwriter-math-caret"
	latex        = "LaTeX"
)

// FormulaColorTag marks the source suffix for presentation only.
const FormulaColorTag = "cyberwriter-formula-color"

var formulaColorSuffix = regexp.MustCompile(`^:([#A-Za-z0-9]+)`)
var formulaColorHex = regexp.MustCompile(`^#[0-9a-f]+$`)

// FormulaColor resolves the CSS color immediately following an inline atom.
// Length is zero when the punctuation is ordinary body text.
func FormulaColor(tail string, current mygoui.Color) (mygoui.Color, int) {
	match := formulaColorSuffix.FindStringSubmatch(tail)
	if len(match) == 0 {
		return mygoui.Color{}, 0
	}
	name := strings.ToLower(match[1])
	switch name {
	case "currentcolor":
		return current, len(match[0])
	case "transparent":
		return mygoui.Transparent, len(match[0])
	case "rebeccapurple":
		return mygoui.Hex("#663399"), len(match[0])
	}
	if color, ok := colornames.Map[name]; ok {
		return mygoui.RGB(color.R, color.G, color.B), len(match[0])
	}
	switch len(name) {
	case 4, 5, 7, 9:
		if formulaColorHex.MatchString(name) {
			return mygoui.Hex(name), len(match[0])
		}
	}
	return mygoui.Color{}, 0
}

func tagFormulaColors(text richtext.Text) richtext.Text {
	styled := text
	for offset, run := range text.Runs(0, text.Len()) {
		if run.Attributes.Attachment == nil {
			continue
		}
		if f, ok := run.Attributes.Attachment.Data.(*Formula); ok && !f.Block {
			start := offset + 1
			tail := text.Slice(start, min(text.Len(), start+32)).String()
			if _, length := FormulaColor(tail, mygoui.Color{}); length > 0 {
				styled = styled.Format(start, start+length, func(a *richtext.Attributes) { a.Tag = FormulaColorTag })
			}
		}
	}
	return styled
}

// Formula is a version of an inline formula or a fenced LaTeX editor. The
// same block can retain its editor when its language changes.
type Formula struct {
	ID, Source, Language string
	Block                bool
	markdown             string
	object               *richtext.Attachment
	descent              float32
}

func newFormula(id, source, language, markdown string, block bool) *Formula {
	if id == "" {
		id = rand.Text()
	}
	f := &Formula{ID: id, Source: source, Language: language, Block: block, markdown: markdown}
	height := float32(17)
	if block {
		height = 95
	}
	f.object = &richtext.Attachment{Name: formulaLabel, URL: formulaURL + id + ":" + rand.Text(),
		Width: 18, Height: height, Data: f, Draw: func(*mygoui.Painter, mygoui.Rect) {}}
	return f
}

func (f *Formula) IsLaTeX() bool    { return strings.EqualFold(f.Language, latex) }
func (f *Formula) Tag() string      { return f.object.URL }
func (f *Formula) Height() float32  { return f.object.Height }
func (f *Formula) Descent() float32 { return f.descent }

// SetVisual updates layout and painting without changing the undoable source.
func (f *Formula) SetVisual(width, ascent, descent float32, draw func(*mygoui.Painter, mygoui.Rect)) bool {
	changed := f.object.Width != width || f.object.Height != ascent || f.descent != descent
	f.object.Width, f.object.Height, f.descent = width, ascent, descent
	if draw != nil {
		f.object.Draw = draw
	}
	return changed
}

func (f *Formula) attributes(a richtext.Attributes) richtext.Attributes {
	a.Attachment, a.Tag = f.object, f.Tag()
	return a
}

func FormulaAt(text richtext.Text, offset int) (*Formula, bool) {
	if offset < 0 || offset >= text.Len() {
		return nil, false
	}
	object := text.AttributesAt(offset).Attachment
	if object == nil {
		return nil, false
	}
	f, ok := object.Data.(*Formula)
	if ok {
		_, end := text.ParagraphRange(offset)
		return f, offset < end
	}
	return nil, false
}

type mathRegion struct {
	start, end       int
	source, language string
	block            bool
}

// mathRegions follows the Markdown parser's code and inline boundaries.
// Crepe treats a same-line $$...$$ as an inline formula.
func mathRegions(doc *parser.Document) []mathRegion {
	var regions []mathRegion
	raw := doc.Source()
	for i := 1; i < doc.Len(); i++ {
		b := doc.Block(i)
		if b.Parent == 0 {
			switch b.Kind {
			case parser.MathBlock:
				if !strings.HasPrefix(raw[b.Start:], "$$") {
					break
				}
				firstEnd := mathLineEnd(raw, b.Start)
				first := raw[b.Start:firstEnd]
				if tex, tail, closed := strings.Cut(first[2:], "$$"); closed && tex != "" && strings.TrimSpace(tail) == "" {
					regions = append(regions, mathRegion{b.Start, b.Start + len(tex) + 4, tex, latex, false})
					continue
				}
				lines := doc.Lines(i)
				if doc.LineCount(i) > 0 {
					_, at := doc.Line(i, 0)
					if at < firstEnd {
						// A display math fence can have an info string on its opening line.
						_, lines, _ = strings.Cut(lines, "\n")
					}
				}
				end := mathLineEnd(raw, b.End)
				// The parser may report the last TeX segment as End. Include
				// the closing line when it follows the segment on its own line.
				if !b.Open && end < len(raw) && !strings.HasSuffix(strings.TrimSpace(raw[mathLineStart(raw, b.End):end]), "$$") {
					next := mathLineEnd(raw, end+1)
					line := strings.TrimSpace(raw[end+1 : next])
					if len(line) >= 2 && strings.Trim(line, "$") == "" {
						end = next
					}
				}
				regions = append(regions, mathRegion{mathLineStart(raw, b.Start), end, strings.TrimSuffix(lines, "\n"), latex, true})
			case parser.CodeBlock:
				language := b.Info()
				if b.Fenced && !b.Open && (strings.EqualFold(language, latex) ||
					language != "" && language != "bash" && editor.LanguageNamed(language) != nil) {
					regions = append(regions, mathRegion{mathLineStart(raw, b.Start), trimMathLineEnd(raw, b.End),
						strings.TrimSuffix(doc.Lines(i), "\n"), language, true})
				}
			}
		}
		if b.Kind != parser.Paragraph && b.Kind != parser.Heading {
			continue
		}
		for _, n := range doc.Inlines(i) {
			if n.Kind != parser.Math || n.Pending || n.Start < b.Start || n.End > b.End {
				continue
			}
			value := raw[n.Start:n.End]
			if strings.HasPrefix(value, "$") || strings.HasPrefix(value, `\(`) {
				regions = append(regions, mathRegion{n.Start, n.End, n.Text, latex, false})
			}
		}
	}
	return regions
}

func mathLineStart(s string, start int) int { return strings.LastIndexByte(s[:start], '\n') + 1 }

func mathLineEnd(s string, start int) int {
	if offset := strings.IndexByte(s[start:], '\n'); offset >= 0 {
		return start + offset
	}
	return len(s)
}

func trimMathLineEnd(s string, end int) int {
	if end > 0 && s[end-1] == '\n' {
		return end - 1
	}
	return end
}

func projectMath(doc *parser.Document, source string) (string, map[string]*Formula) {
	regions := mathRegions(doc)
	if len(regions) == 0 {
		return source, nil
	}
	var out strings.Builder
	formulas := make(map[string]*Formula, len(regions))
	pos := 0
	for _, region := range regions {
		if region.start < pos || region.end <= region.start {
			continue
		}
		f := newFormula("", region.source, region.language, source[region.start:region.end], region.block)
		formulas[f.Tag()] = f
		out.WriteString(source[pos:region.start])
		if region.block {
			out.WriteString("\n\n")
		}
		out.WriteString(attachmentMarkdown(f.object))
		if region.block {
			out.WriteString("\n\n")
		}
		pos = region.end
	}
	out.WriteString(source[pos:])
	return out.String(), formulas
}

func mathMarkdown(source, language string, block bool) string {
	if !block {
		fence := "$"
		if strings.Contains(source, "$") {
			fence = "$$"
		}
		return fence + source + fence
	}
	if strings.EqualFold(language, latex) {
		if source == "" {
			return "$$\n$$"
		}
		return "$$\n" + source + "\n$$"
	}
	return fencedDiagramMarkdown(source, language)
}

// ChangeFormula keeps the block identity and its editor while replacing
// the source stored in one rich-text undo step.
func (s *Session) ChangeFormula(offset int, previous *Formula, source, language string) (*Formula, bool) {
	if s.Document == nil || !s.Current(s.draft) {
		return nil, false
	}
	text := s.Document.Text()
	f, ok := FormulaAt(text, offset)
	if !ok || f != previous || f.Source == source && f.Language == language {
		return f, false
	}
	next := newFormula(f.ID, source, language, mathMarkdown(source, language, f.Block), f.Block)
	next.SetVisual(f.object.Width, f.Height(), f.Descent(), nil)
	a, z := s.Document.Selection()
	s.Document.Edit(func(edit *richtext.Edit) {
		edit.ReplaceString(offset, offset+1, string(richtext.ObjectReplacement), next.attributes(text.AttributesAt(offset)))
		edit.Select(a, z)
	})
	return next, true
}

// ReplaceInlineFormula collapses a typed pair into one undoable atom.
func (s *Session) ReplaceInlineFormula(start, end int, source string) {
	s.replaceInlineFormula(start, end, source, "", "")
}

func (s *Session) replaceInlineFormula(start, end int, source, suffix, markdown string) {
	if markdown == "" {
		markdown = mathMarkdown(source, latex, false)
	}
	f := newFormula("", source, latex, markdown, false)
	attrs := s.Document.TypingAttributes()
	attrs.Attachment, attrs.Tag, attrs.Link, attrs.Code = nil, "", "", false
	s.Document.Edit(func(edit *richtext.Edit) {
		edit.ReplaceString(start, end, string(richtext.ObjectReplacement), f.attributes(attrs))
		if suffix != "" {
			colorAttrs := attrs
			colorAttrs.Tag = FormulaColorTag
			edit.ReplaceString(start+1, start+1, suffix, colorAttrs)
		}
		caret := start + 1 + utf8.RuneCountInString(suffix)
		edit.Select(caret, caret)
	})
}

// InsertInlineMathTemplate mirrors the bridge's display-flagged inline
// sample on an empty paragraph. Other selections use ordinary inline TeX.
func (s *Session) InsertInlineMathTemplate(source string, suffix string) {
	a, z := s.Document.Selection()
	var markdown string
	start, end := s.Document.Text().ParagraphRange(z)
	if a == z && start == end && suffix == "" {
		markdown = "$$" + source + "$$"
	}
	s.replaceInlineFormula(min(a, z), max(a, z), source, suffix, markdown)
}

func (s *Session) InsertDisplayFormula(source string) (*Formula, bool) {
	f := newFormula("", source, latex, mathMarkdown(source, latex, true), true)
	return f, s.insertStandalone(f.object, mathCaretID, false)
}

// ExitFormula inserts the ordinary caret paragraph that Cmd+Return creates.
func (s *Session) ExitFormula(offset int, remove bool) {
	s.Document.Edit(func(edit *richtext.Edit) {
		if remove {
			edit.ReplaceString(offset, offset+1, "", richtext.Attributes{})
			edit.FormatParagraphs(offset, offset, func(p *richtext.ParagraphStyle) {
				*p = richtext.ParagraphStyle{ID: tableExitID}
			})
			edit.Select(offset, offset)
			return
		}
		end := offset + 1
		if end == edit.Text().Len() || end+1 < edit.Text().Len() {
			edit.ReplaceString(end, end, "\n", richtext.Attributes{})
		}
		edit.FormatParagraphs(offset+2, offset+2, func(p *richtext.ParagraphStyle) {
			*p = richtext.ParagraphStyle{ID: tableExitID}
		})
		edit.Select(offset+2, offset+2)
	})
}

// SplitBeforeInlineFormula handles Return on the selected atom while
// preserving the formula and the following text in the new paragraph.
func (s *Session) SplitBeforeInlineFormula(offset int) {
	s.Document.Edit(func(edit *richtext.Edit) {
		edit.ReplaceString(offset, offset, "\n", richtext.Attributes{})
		edit.Select(offset+1, offset+2)
	})
}

// A formula-only edit can leave other parts of the original source byte
// identical, including frontmatter, fence variants and emphasis markers.
func (s *Session) patchFormulaSource(markdown string, text richtext.Text) (string, bool) {
	const prefix = "![" + formulaLabel + "](" + formulaURL
	if !strings.Contains(markdown, prefix) || !strings.Contains(s.lastMarkdown, prefix) {
		return "", false
	}
	change := changedRange(s.lastMarkdown, markdown)
	for _, run := range text.Runs(0, text.Len()) {
		if run.Attributes.Attachment == nil {
			continue
		}
		f, ok := run.Attributes.Attachment.Data.(*Formula)
		if !ok {
			continue
		}
		marker := attachmentMarkdown(f.object)
		start := strings.Index(markdown, marker)
		if start < 0 || change.Start < start || change.Start+len(change.Text) > start+len(marker) {
			continue
		}
		end := start + len(marker) - len(change.Text) + change.End - change.Start
		if end <= start || end > len(s.lastMarkdown) || !strings.HasPrefix(s.lastMarkdown[start:end], prefix+f.ID+":") {
			return "", false
		}
		head, _, body := workspace.SplitFrontmatter(s.source)
		doc := parser.New(Extensions)
		doc.Set(body)
		regions := mathRegions(doc)
		index := strings.Count(markdown[:start], prefix)
		if len(regions) != strings.Count(s.lastMarkdown, prefix) || index >= len(regions) {
			return "", false
		}
		r := regions[index]
		return head + body[:r.start] + f.markdown + body[r.end:], true
	}
	return "", false
}
