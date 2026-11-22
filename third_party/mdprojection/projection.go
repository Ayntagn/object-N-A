// Package mdprojection tracks mapped Markdown source ranges through local edits.
package mdprojection

import (
	"slices"
	"sort"
)

// Version identifies the visual and source snapshots that a Map describes.
type Version struct {
	Visual, Source uint64
}

// Span maps a nonempty visual rune interval to a nonempty source byte
// interval. Both ends are exclusive. Spans are certified by the caller.
type Span struct {
	VisualStart, VisualEnd int
	SourceStart, SourceEnd int
}

// Change replaces visual runes [Start, OldEnd) with [Start, NewEnd).
type Change struct {
	Start, OldEnd, NewEnd int
}

// Map tracks ordered source ranges across edits in those ranges. It is
// intended for use by a single editing session.
type Map struct {
	version Version
	spans   []Span
	deltas  []offsets
}

type offsets struct {
	visual, source int
}

// New returns nil if spans are empty or out of order. Visual spans must
// leave a gap at boundaries, so an insertion can belong to only one span.
func New(version Version, spans []Span) *Map {
	if len(spans) == 0 {
		return nil
	}
	for i, span := range spans {
		if span.VisualStart < 0 || span.VisualEnd <= span.VisualStart ||
			span.SourceStart < 0 || span.SourceEnd <= span.SourceStart {
			return nil
		}
		if i > 0 && (spans[i-1].VisualEnd >= span.VisualStart || spans[i-1].SourceEnd > span.SourceStart) {
			return nil
		}
	}
	return &Map{version: version, spans: slices.Clone(spans), deltas: make([]offsets, len(spans)+1)}
}

// Patch is a single proposal against one Map revision. Its coordinates
// must be checked against both snapshots before the caller writes source.
type Patch struct {
	owner     *Map
	version   Version
	span      Span
	index     int
	visualEnd int
}

// Span returns the visual and source ranges before the edit.
func (p Patch) Span() Span { return p.span }

// VisualEnd returns the end of the visual span after the edit.
func (p Patch) VisualEnd() int { return p.visualEnd }

// Patch proposes a range for an edit entirely inside one mapped span.
// An edit that deletes the span or crosses a gap is not a local patch.
func (m *Map) Patch(version Version, change Change) (Patch, bool) {
	if m == nil || m.version != version || change.Start < 0 || change.OldEnd < change.Start || change.NewEnd < change.Start {
		return Patch{}, false
	}
	i := sort.Search(len(m.spans), func(i int) bool {
		return m.spans[i].VisualStart+m.deltaBefore(i).visual > change.Start
	}) - 1
	if i < 0 {
		return Patch{}, false
	}
	span := m.spanAt(i)
	if change.OldEnd > span.VisualEnd || change.NewEnd > maxInt-(span.VisualEnd-change.OldEnd) {
		return Patch{}, false
	}
	newEnd := change.NewEnd + (span.VisualEnd - change.OldEnd)
	if newEnd <= span.VisualStart {
		return Patch{}, false
	}
	return Patch{owner: m, version: version, span: span, index: i, visualEnd: newEnd}, true
}

// Advance installs the new lengths and versions after a patch has been
// published by the caller. sourceBytes is the entire updated source span.
func (m *Map) Advance(patch Patch, next Version, sourceBytes int) bool {
	if m == nil || patch.owner != m || patch.version != m.version || next.Visual <= m.version.Visual ||
		next.Source < m.version.Source || sourceBytes <= 0 {
		return false
	}
	delta := offsets{
		visual: patch.visualEnd - patch.span.VisualEnd,
		source: sourceBytes - (patch.span.SourceEnd - patch.span.SourceStart),
	}
	last := m.spanAt(len(m.spans) - 1)
	if delta.visual > maxInt-last.VisualEnd || delta.source > maxInt-last.SourceEnd ||
		(next.Source == m.version.Source && delta.source != 0) {
		return false
	}
	// A Fenwick tree accumulates length deltas. This edit shifts every
	// subsequent span without rewriting any of its original coordinates.
	for i := patch.index + 1; i < len(m.deltas); i += i & -i {
		m.deltas[i].visual += delta.visual
		m.deltas[i].source += delta.source
	}
	m.version = next
	return true
}

func (m *Map) spanAt(i int) Span {
	span := m.spans[i]
	start, end := m.deltaBefore(i), m.deltaBefore(i+1)
	span.VisualStart += start.visual
	span.VisualEnd += end.visual
	span.SourceStart += start.source
	span.SourceEnd += end.source
	return span
}

func (m *Map) deltaBefore(i int) offsets {
	var delta offsets
	for i > 0 {
		delta.visual += m.deltas[i].visual
		delta.source += m.deltas[i].source
		i -= i & -i
	}
	return delta
}

const maxInt = int(^uint(0) >> 1)
