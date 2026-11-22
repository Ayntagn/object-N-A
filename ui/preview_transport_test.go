package ui

import (
	"strings"
	"testing"
)

func TestPreviewTransportKeepsShiftedBlockAndCodeCopy(t *testing.T) {
	intro := makePreviewTransportBlock(`<p data-source-line="1">` + strings.Repeat("先前不变段落 ", 5500) + `</p>`)
	fence := makePreviewTransportBlock(`<div><a class="preview-code-copy" href="cyberwriter-copy-code://block/1/1">Copy</a><pre data-source-line="4">x</pre></div>`)
	quote := makePreviewTransportBlock(`<blockquote data-source-line="10"><p data-source-line="11">same</p></blockquote>`)
	before := []previewTransportBlock{intro, fence, quote}
	var transport previewTransport
	transport.commit(7, 1, before)
	// The renderer can recycle its slice. Its committed state still describes
	// the last frame emitted to this document.
	before[2] = makePreviewTransportBlock(`<h1 data-source-line="10">other</h1>`)
	newBlock := makePreviewTransportBlock(`<p data-source-line="3">new</p>`)
	after := []previewTransportBlock{intro, newBlock,
		makePreviewTransportBlock(`<div><a class="preview-code-copy" href="cyberwriter-copy-code://block/2/1">Copy</a><pre data-source-line="6">x</pre></div>`),
		quote.withLine(12)}
	update := transport.patch(7, 2, after)
	if update == nil || update.BaseRevision != 1 || update.Start != 1 || update.Delete != 0 || update.Insert != 1 ||
		update.HTML != newBlock.markup() || update.Copies != 1 {
		t.Fatalf("insert patch: %+v", update)
	}
	if len(update.Shifts) != 1 || update.Shifts[0] != (previewLineShift{Start: 1, Count: 2, Delta: 2}) {
		t.Fatalf("shift: %+v", update.Shifts)
	}
	if got := after[3].markup(); got != `<blockquote data-source-line="12"><p data-source-line="13">same</p></blockquote>` {
		t.Fatalf("retargeted full fallback: %s", got)
	}
	if update := transport.patch(8, 2, after); update != nil {
		t.Fatalf("document switch used a patch: %+v", update)
	}
	if update := transport.patch(7, 2, nil); update != nil {
		t.Fatalf("renderer without block boundaries used a patch: %+v", update)
	}
	transport.commit(7, 2, after)
	if update := transport.patch(7, 3, after); update == nil || update.BaseRevision != 2 || update.Delete != 0 || update.Insert != 0 || update.HTML != "" {
		t.Fatalf("metadata-only patch: %+v", update)
	}
}
