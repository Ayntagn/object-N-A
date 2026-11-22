package wysiwyg

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/richtext"
	"object-na.local/mdprojection"
)

// incrementalProjection keeps the paired rope snapshots for local writes.
// The full serialization and table offsets are refreshed on demand.
type incrementalProjection struct {
	ranges   *mdprojection.Map
	source   editor.Snapshot
	visual   richtext.Text
	needsMap bool
	dirty    bool
}

func (s *Session) resetIncremental(source editor.Snapshot, visual richtext.Text, canonical bool) {
	// A full write gets fresh offsets when the next edit can use them.
	*s.incremental = incrementalProjection{
		source:   source,
		visual:   visual,
		needsMap: canonical && source.Len() == len(s.source),
	}
}

// mapParagraphs certifies the source of each ordinary paragraph. It reads
// the canonical source from left to right, including the serialized gaps.
// Matching the entire partition distinguishes repeated paragraphs and
// makes any different writer grouping use the full serializer.
func (s *Session) mapParagraphs(visual richtext.Text) *mdprojection.Map {
	body, ok := strings.CutPrefix(s.source, s.prefix)
	if !ok {
		return nil
	}
	body, ok = strings.CutSuffix(body, s.suffix)
	if !ok {
		return nil
	}
	var spans []mdprojection.Span
	var group []richtext.Paragraph
	left := body
	consumed := false
	take := func(piece string) (int, bool) {
		if consumed {
			var ok bool
			left, ok = strings.CutPrefix(left, "\n\n")
			if !ok {
				return 0, false
			}
		}
		consumed = true
		start := len(s.prefix) + len(body) - len(left)
		remaining, match := strings.CutPrefix(left, piece)
		left = remaining
		return start, match
	}
	flush := func() bool {
		if len(group) == 0 {
			return true
		}
		text := splitAttachmentParagraphs(richtext.TextOf(group...))
		_, ok := take(s.restoredBody(visualMarkdown(text), text))
		group = group[:0]
		return ok
	}
	add := func(p richtext.Paragraph, start, end int, continues bool) bool {
		literal, ok := ordinaryLiteral(p)
		if !ok || continues {
			group = append(group, p)
			return true
		}
		if !flush() {
			return false
		}
		at, ok := take(literal)
		if !ok {
			return false
		}
		spans = append(spans, mdprojection.Span{
			VisualStart: start, VisualEnd: end,
			SourceStart: at, SourceEnd: at + len(literal),
		})
		return true
	}

	var previous richtext.Paragraph
	previousStart := -1
	for at, p := range visual.Paragraphs() {
		if previousStart >= 0 && !add(previous, previousStart, at-1, p.Style.SoftBreak) {
			return nil
		}
		previous, previousStart = p, at
	}
	if previousStart < 0 || !add(previous, previousStart, visual.Len(), false) || !flush() || left != "" {
		return nil
	}
	return mdprojection.New(mdprojection.Version{Visual: s.visualVersion, Source: s.draftVersion}, spans)
}

func (s *Session) rebuildMapForEdit(visual richtext.Text, change richtext.EditRange) bool {
	state := s.incremental
	if state.visual.ParagraphStyleAt(change.Start) != (richtext.ParagraphStyle{}) {
		return false
	}
	start, end := state.visual.ParagraphRange(change.Start)
	newStart, newEnd := visual.ParagraphRange(change.Start)
	if change.OldEnd > end || newStart != start || newEnd-change.NewEnd != end-change.OldEnd {
		return false
	}
	if _, ok := ordinaryParagraphLiteral(state.visual, start, end); !ok {
		return false
	}
	if _, ok := ordinaryParagraphLiteral(visual, newStart, newEnd); !ok {
		return false
	}
	state.needsMap = false
	state.ranges = s.mapParagraphs(state.visual)
	return state.ranges != nil
}

func (s *Session) syncParagraph(draft *editor.Buffer, visual richtext.Text, version uint64, change richtext.EditRange) (changed, applied bool) {
	state := s.incremental
	if visual.ParagraphCount() != state.visual.ParagraphCount() ||
		state.needsMap && !s.rebuildMapForEdit(visual, change) {
		return false, false
	}
	patch, ok := state.ranges.Patch(
		mdprojection.Version{Visual: s.visualVersion, Source: s.draftVersion},
		mdprojection.Change{Start: change.Start, OldEnd: change.OldEnd, NewEnd: change.NewEnd},
	)
	if !ok {
		return false, false
	}
	span := patch.Span()
	start, end := visual.ParagraphRange(span.VisualStart)
	if start != span.VisualStart || end != patch.VisualEnd() {
		return false, false
	}
	literal, ok := ordinaryParagraphLiteral(visual, start, end)
	if !ok {
		return false, false
	}
	before := state.source.Slice(span.SourceStart, span.SourceEnd)
	changed = before != literal
	var edits []editor.Edit
	if changed {
		edit := changedRange(before, literal)
		edit.Start += span.SourceStart
		edit.End += span.SourceStart
		edits = []editor.Edit{edit}
	}
	snapshot, ok := draft.EditAtVersion(s.draftVersion, edits...)
	if !ok {
		// A newer source edit invalidated this session. The full
		// serializer has the same stale source base.
		return false, true
	}
	if !state.ranges.Advance(patch, mdprojection.Version{Visual: version, Source: snapshot.Version()}, len(literal)) {
		state.ranges = nil
	}
	s.visualVersion, s.draftVersion = version, snapshot.Version()
	state.visual, state.source = visual, snapshot
	state.dirty = state.dirty || changed
	return changed, true
}

func (s *Session) materializeProjection() {
	state := s.incremental
	if state == nil || !state.dirty {
		return
	}
	s.source = state.source.String()
	text := splitAttachmentParagraphs(state.visual)
	s.setProjectionMetadata(visualMarkdown(text), text)
	if len(s.tables) != 0 || len(s.visualTables) != 0 {
		s.readDraftTables()
	}
	state.dirty = false
}

func ordinaryParagraphLiteral(visual richtext.Text, start, end int) (string, bool) {
	if end < visual.Len() && visual.ParagraphStyleAt(end+1).SoftBreak {
		return "", false
	}
	var literal string
	var ok bool
	for _, paragraph := range visual.Slice(start, end).Paragraphs() {
		literal, ok = ordinaryLiteral(paragraph)
	}
	return literal, ok
}

// ordinaryLiteral is a standalone, unstyled paragraph that the Markdown
// writer emits literally. Block starts, escapes and decoration take the
// same full path as structural edits.
func ordinaryLiteral(p richtext.Paragraph) (string, bool) {
	if p.Style != (richtext.ParagraphStyle{}) || len(p.Runs) != 1 ||
		p.Runs[0].Attributes != (richtext.Attributes{}) {
		return "", false
	}
	literal := p.Runs[0].Text
	if literal == "" || !utf8.ValidString(literal) ||
		strings.ContainsAny(literal, "\\*_`[]<~&$\r\n\x00\ufffc") ||
		strings.Contains(literal, "==") || strings.ContainsRune("#>-+=", rune(literal[0])) {
		return "", false
	}
	first, _ := utf8.DecodeRuneInString(literal)
	if unicode.IsSpace(first) || strings.HasSuffix(strings.TrimRightFunc(literal, unicode.IsSpace), "!") {
		return "", false
	}
	var i int
	for i < len(literal) && literal[i] >= '0' && literal[i] <= '9' {
		i++
	}
	if i > 0 && i < len(literal) && (literal[i] == '.' || literal[i] == ')') {
		return "", false
	}
	return literal, true
}
