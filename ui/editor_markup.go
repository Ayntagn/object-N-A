package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"
)

type markdownFormat string

const (
	boldFormat          markdownFormat = "**"
	italicFormat        markdownFormat = "*"
	strikethroughFormat markdownFormat = "~~"
	inlineCodeFormat    markdownFormat = "`"
	linkFormat          markdownFormat = "link"
)

var escapeLinkLabel = strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`)

func formatMarkdown(state *editor.State, format markdownFormat) bool {
	if state == nil || state.Buffer == nil || state.ReadOnly {
		return false
	}
	selections, primary := state.Selections(), state.Selection()
	snapshot := state.Buffer.Snapshot()
	edits := make([]editor.Edit, 0, 2*len(selections))
	after := make([]editor.Selection, 0, len(selections))
	var primaryAfter editor.Selection
	left, shift := 0, 0
	for i, sel := range selections {
		right := snapshot.Len()
		if i+1 < len(selections) {
			right = selections[i+1].Start()
		}
		changes, start, end := formattingEdits(snapshot, format, sel, left, right)
		updated := editor.Selection{Anchor: start + shift, Head: end + shift}
		if sel.Anchor > sel.Head {
			updated.Anchor, updated.Head = updated.Head, updated.Anchor
		}
		if sel == primary {
			primaryAfter = updated
		} else {
			after = append(after, updated)
		}
		left = sel.End()
		for _, edit := range changes {
			left = max(left, edit.End)
			shift += len(edit.Text) - (edit.End - edit.Start)
		}
		edits = append(edits, changes...)
	}
	if len(edits) == 0 {
		return false
	}
	// Editing the boundaries separately keeps the selected bytes in place
	// when MyGo moves the selection through undo and redo.
	state.Buffer.Edit(edits...)
	state.SetSelections(append(after, primaryAfter)...)
	return true
}

func formattingEdits(snapshot editor.Snapshot, format markdownFormat, sel editor.Selection, left, right int) ([]editor.Edit, int, int) {
	start, end := sel.Start(), sel.End()
	if !textBoundary(snapshot, start) || !textBoundary(snapshot, end) {
		return nil, start, end
	}
	selected := snapshot.Slice(start, end)
	if format == linkFormat {
		label := escapeLinkLabel.Replace(selected)
		caret := start + 1
		if !sel.Empty() {
			caret = start + len(label) + 3
		}
		return []editor.Edit{{Start: start, End: end, Text: "[" + label + "]()"}}, caret, caret
	}
	marker := string(format)
	if sel.Empty() {
		caret := start + len(marker)
		return []editor.Edit{{Start: start, End: end, Text: marker + marker}}, caret, caret
	}
	if format != inlineCodeFormat {
		// CommonMark emphasis markers must touch the text they enclose.
		trimmed := strings.TrimSpace(selected)
		if trimmed == "" {
			return nil, start, end
		}
		start += strings.Index(selected, trimmed)
		end = start + len(trimmed)
		selected = trimmed
	}
	n := len(marker)
	if start-n >= left && end+n <= right && !strings.Contains(selected, marker) &&
		exactMarker(snapshot, start-n, marker) && exactMarker(snapshot, end, marker) {
		edits := []editor.Edit{
			{Start: start - n, End: start},
			{Start: end, End: end + n},
		}
		return edits, start - n, end - n
	}
	if end-start > 2*n && exactMarker(snapshot, start, marker) && exactMarker(snapshot, end-n, marker) {
		inner := selected[n : len(selected)-n]
		if !strings.Contains(inner, marker) && (format == inlineCodeFormat || strings.TrimSpace(inner) == inner) {
			edits := []editor.Edit{
				{Start: start, End: start + n},
				{Start: end - n, End: end},
			}
			return edits, start, end - 2*n
		}
	}
	prefix, suffix := marker, marker
	if format == inlineCodeFormat {
		prefix, suffix = codeSpanMarkers(selected)
	}
	edits := []editor.Edit{
		{Start: start, End: start, Text: prefix},
		{Start: end, End: end, Text: suffix},
	}
	return edits, start + len(prefix), end + len(prefix)
}

func textBoundary(snapshot editor.Snapshot, offset int) bool {
	if offset == snapshot.Len() {
		return true
	}
	return offset >= 0 && offset < snapshot.Len() && utf8.RuneStart(snapshot.Slice(offset, offset+1)[0])
}

func exactMarker(snapshot editor.Snapshot, start int, marker string) bool {
	end := start + len(marker)
	if snapshot.Slice(start, end) != marker {
		return false
	}
	if start > 0 {
		previous := snapshot.Slice(start-1, start)
		if previous == marker[:1] || previous == `\` {
			return false
		}
	}
	return end == snapshot.Len() || snapshot.Slice(end, end+1) != marker[:1]
}

func codeSpanMarkers(text string) (prefix, suffix string) {
	longest, run := 0, 0
	for i := range len(text) {
		if text[i] == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	marker := strings.Repeat("`", longest+1)
	touchesBacktick := strings.HasPrefix(text, "`") || strings.HasSuffix(text, "`")
	preserveSpaces := strings.HasPrefix(text, " ") && strings.HasSuffix(text, " ") && strings.Trim(text, " ") != ""
	if touchesBacktick || preserveSpaces {
		return marker + " ", " " + marker
	}
	return marker, marker
}
