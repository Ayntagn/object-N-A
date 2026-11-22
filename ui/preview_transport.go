package ui

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	previewResyncScheme      = "cyberwriter-preview-resync://frame/"
	previewSourceLine        = ` data-source-line="`
	minPreviewTransportBytes = 64 << 10
	maxPreviewTransportBytes = 2 << 20
)

func previewTransportResyncURL(document uint64) string {
	return previewResyncScheme + strconv.FormatUint(document, 10)
}

var previewCopyAddress = regexp.MustCompile(`<a class="preview-code-copy" href="cyberwriter-copy-code://block/[0-9]+/[0-9]+"`)

// Each block represents exactly one top-level node. Rendering code supplies
// nil for documents whose content could cross the block boundaries.
type previewTransportBlock struct {
	html, key          string
	line, renderedLine int
	copies             int
}

func makePreviewTransportBlock(html string) previewTransportBlock {
	block := previewTransportBlock{html: html}
	block.key = rewritePreviewSourceLines(html, func(line int) int {
		if block.line == 0 {
			block.line = line
		}
		return line - block.line
	})
	block.renderedLine = block.line
	if strings.Contains(block.key, previewCodeCopyScheme) {
		block.key = previewCopyAddress.ReplaceAllStringFunc(block.key, func(string) string {
			block.copies++
			return `<a class="preview-code-copy" href="cyberwriter-copy-code://block/"`
		})
	}
	return block
}

// withLine relocates an unchanged block. Only a full or inserted block
// needs its serialized HTML rewritten; retained nodes get an offset in JS.
func (block previewTransportBlock) withLine(line int) previewTransportBlock {
	if block.renderedLine != 0 {
		block.line = line
	}
	return block
}

func (block previewTransportBlock) markup() string {
	if block.renderedLine == block.line {
		return block.html
	}
	delta := block.line - block.renderedLine
	return rewritePreviewSourceLines(block.html, func(line int) int { return line + delta })
}

func rewritePreviewSourceLines(markup string, change func(int) int) string {
	var out strings.Builder
	start, written := 0, 0
	for {
		from := strings.Index(markup[start:], previewSourceLine)
		if from < 0 {
			break
		}
		from += start + len(previewSourceLine)
		end := strings.IndexByte(markup[from:], '"')
		if end < 0 {
			break
		}
		end += from
		start = end + 1
		line, err := strconv.Atoi(markup[from:end])
		if err != nil || line <= 0 {
			continue
		}
		out.WriteString(markup[written:from])
		out.WriteString(strconv.Itoa(change(line)))
		written = end
	}
	if written == 0 {
		return markup
	}
	out.WriteString(markup[written:])
	return out.String()
}

type previewLineShift struct {
	Start int `json:"start"`
	Count int `json:"count"`
	Delta int `json:"delta"`
}

type previewBodyPatch struct {
	BaseRevision uint64             `json:"baseRevision"`
	Before       int                `json:"before"`
	Start        int                `json:"start"`
	Delete       int                `json:"delete"`
	Insert       int                `json:"insert"`
	HTML         string             `json:"html,omitempty"`
	Shifts       []previewLineShift `json:"shifts,omitempty"`
	Copies       int                `json:"copies,omitempty"`
}

type previewTransport struct {
	document, revision uint64
	blocks             []previewTransportBlock
}

func previewTransportEligible(blocks []previewTransportBlock) bool {
	size := 0
	for _, block := range blocks {
		if block.html == "" {
			return false
		}
		size += len(block.html)
		if size > maxPreviewTransportBytes {
			return false
		}
	}
	return size >= minPreviewTransportBytes
}

func (s *previewTransport) commit(document, revision uint64, blocks []previewTransportBlock) {
	*s = previewTransport{}
	if previewTransportEligible(blocks) {
		// The renderer may reuse its slice capacity on the next frame.
		s.document, s.revision = document, revision
		s.blocks = append([]previewTransportBlock(nil), blocks...)
	}
}

func (s previewTransport) patch(document, revision uint64, blocks []previewTransportBlock) *previewBodyPatch {
	if s.revision == 0 || revision <= s.revision || document != s.document || !previewTransportEligible(blocks) {
		return nil
	}
	first, oldEnd, newEnd := 0, len(s.blocks), len(blocks)
	for first < oldEnd && first < newEnd && s.blocks[first].key == blocks[first].key {
		first++
	}
	for first < oldEnd && first < newEnd && s.blocks[oldEnd-1].key == blocks[newEnd-1].key {
		oldEnd--
		newEnd--
	}
	if first == 0 && oldEnd == len(s.blocks) {
		return nil // every block would be removed anyway
	}
	patch := &previewBodyPatch{BaseRevision: s.revision, Before: len(s.blocks), Start: first,
		Delete: oldEnd - first, Insert: newEnd - first}
	for index, block := range blocks {
		patch.Copies += block.copies
		switch {
		case index < first:
			patch.addLineShift(index, block.line-s.blocks[index].line)
		case index >= newEnd:
			oldIndex := index - newEnd + oldEnd
			patch.addLineShift(oldIndex, block.line-s.blocks[oldIndex].line)
		}
	}
	var html strings.Builder
	for _, block := range blocks[first:newEnd] {
		html.WriteString(block.markup())
	}
	patch.HTML = html.String()
	return patch
}

func (p *previewBodyPatch) addLineShift(index, delta int) {
	if delta == 0 {
		return
	}
	if count := len(p.Shifts); count > 0 {
		previous := &p.Shifts[count-1]
		if previous.Start+previous.Count == index && previous.Delta == delta {
			previous.Count++
			return
		}
	}
	p.Shifts = append(p.Shifts, previewLineShift{Start: index, Count: 1, Delta: delta})
}
