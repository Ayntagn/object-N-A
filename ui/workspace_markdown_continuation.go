package ui

import (
	"slices"
	"strconv"
	"strings"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func continueSourceMarkdownLine(code *editor.State, event mygoui.InputEvent) bool {
	if event.Kind != mygoui.InputKeyDown || event.Key != mygoui.KeyEnter || event.Mods != 0 || code == nil || code.ReadOnly {
		return false
	}
	selections := code.Selections()
	if len(selections) != 1 || selections[0].Anchor != selections[0].Head {
		return false
	}
	snapshot := code.Buffer.Snapshot()
	edit, ok := sourceMarkdownContinuationEdit(snapshot, selections[0].Head)
	if !ok || code.Buffer.Version() != snapshot.Version() {
		return false
	}
	code.Buffer.Edit(edit)
	caret := edit.Start + len(edit.Text)
	code.Select(caret, caret)
	return true
}

func sourceMarkdownContinuationEdit(snapshot editor.Snapshot, caret int) (editor.Edit, bool) {
	line := snapshot.LineOf(caret)
	start, end := snapshot.LineStart(line), snapshot.LineEnd(line)
	current := snapshot.Slice(start, end)
	if start == 0 && strings.HasPrefix(current, "\ufeff") {
		current = strings.TrimPrefix(current, "\ufeff")
		start += len("\ufeff")
	}
	markers := sourceMarkdownLineMarkers(current)
	if len(markers) == 0 || caret < start+markers[0].content {
		return editor.Edit{}, false
	}

	// A candidate only needs the block context up to its line. The editor's
	// normal Enter path does not parse prose or allocate the document text.
	prefix, _, body := workspace.SplitFrontmatter(snapshot.Slice(0, end))
	if start < len(prefix) {
		return editor.Edit{}, false
	}
	if prefix == "" || prefix == "\ufeff" {
		head, _, _ := strings.Cut(body, "\n")
		switch strings.TrimRight(head, " \t") {
		case "---", "+++":
			// An opening delimiter without its closing line can still be metadata.
			return editor.Edit{}, false
		}
	}

	lineStart := start - len(prefix)
	blocks := sourceMarkdownLineContainers(body, current, markers[len(markers)-1])
	var before strings.Builder
	position, matched := 0, -1
	parentQuote, quotePrefix, continued := "", "", ""
	for depth, block := range blocks {
		marker := markers[0]
		if block.Kind == parser.Item && (marker.quote || block.Start != lineStart+marker.start) ||
			block.Kind == parser.Quote && (!marker.quote || block.Start > lineStart+marker.start) {
			continue
		}
		before.WriteString(current[position:marker.start])
		parentQuote, matched = quotePrefix, depth
		if marker.quote {
			continued = before.String() + "> "
			quotePrefix = continued
			before.WriteString(current[marker.start:marker.content])
		} else {
			continued = before.String() + sourceMarkdownNextItem(block)
			before.WriteString(strings.Repeat(" ", marker.width))
			before.WriteString(current[marker.start+marker.width : marker.content])
		}
		position = marker.content
		markers = markers[1:]
		if len(markers) == 0 {
			break
		}
	}
	if matched < 0 || caret < start+position {
		return editor.Edit{}, false
	}
	if strings.Trim(current[position:], " \t") != "" {
		return editor.Edit{Start: caret, End: caret, Text: "\n" + continued}, true
	}

	// A blank line in the parent breaks CommonMark's lazy continuation.
	exit := parentQuote + "\n" + parentQuote
	if matched > 0 && blocks[matched-1].Kind == parser.Item {
		exit = sourceMarkdownParentItem(body, blocks[:matched])
	}
	return editor.Edit{Start: start, End: end, Text: exit}, true
}

type sourceMarkdownLineMarker struct {
	start, width, content int
	quote                 bool
}

func sourceMarkdownLineMarkers(line string) []sourceMarkdownLineMarker {
	var markers []sourceMarkdownLineMarker
	for pos := 0; pos < len(line); {
		lead := strings.TrimLeft(line[pos:], " \t")
		if lead == "" {
			break
		}
		start := len(line) - len(lead)
		width := 1
		quote := lead[0] == '>'
		if !quote && !strings.ContainsRune("-+*", rune(lead[0])) {
			n := len(lead) - len(strings.TrimLeft(lead, "0123456789"))
			if n == 0 || n > 9 || n >= len(lead) || lead[n] != '.' && lead[n] != ')' {
				break
			}
			width = n + 1
		}
		pos = start + width
		if pos < len(line) {
			if line[pos] == ' ' || line[pos] == '\t' {
				pos++
			} else if !quote {
				break
			}
		}
		markers = append(markers, sourceMarkdownLineMarker{start: start, width: width, content: pos, quote: quote})
	}
	return markers
}

func sourceMarkdownLineContainers(body, line string, lastMarker sourceMarkdownLineMarker) []*parser.Block {
	doc := parser.New(webPreviewExtensions)
	at := len(strings.TrimRight(body, " \t")) - 1
	if strings.Trim(line[lastMarker.content:], " \t") == "" {
		// An empty list item may cover only the first byte of its marker.
		at = len(body) - len(line) + lastMarker.start
		if lastMarker.quote {
			// A quote's trailing empty lines can be absent from its range.
			body += " x"
			at = len(body) - 1
		}
	}
	doc.Set(body)
	last := 0
	for i := 1; i < doc.Len(); {
		b := doc.Block(i)
		if b.Start <= at && at <= b.End {
			last = i
			i++
		} else {
			i = b.Next
		}
	}
	var containers []*parser.Block
	for i := last; i > 0; {
		b := doc.Block(i)
		switch b.Kind {
		case parser.CodeBlock, parser.MathBlock, parser.HTMLBlock:
			return nil
		case parser.Item, parser.Quote:
			containers = append(containers, b)
		}
		i = b.Parent
	}
	slices.Reverse(containers)
	return containers
}

func sourceMarkdownParentItem(body string, items []*parser.Block) string {
	parent := items[len(items)-1]
	from := strings.LastIndexByte(body[:parent.Start], '\n') + 1
	var next strings.Builder
	for _, item := range items[:len(items)-1] {
		if item.Kind == parser.Item && item.Start >= from {
			width := strings.IndexByte(body[item.Start:], item.Marker) + 1
			next.WriteString(body[from:item.Start])
			next.WriteString(strings.Repeat(" ", width))
			from = item.Start + width
		}
	}
	next.WriteString(body[from:parent.Start])
	next.WriteString(sourceMarkdownNextItem(parent))
	return next.String()
}

func sourceMarkdownNextItem(item *parser.Block) string {
	marker := string(item.Marker) + " "
	if item.Ordered {
		return strconv.Itoa(item.Number+1) + marker
	}
	return marker
}
