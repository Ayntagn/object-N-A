// spec: FEAT-NOTE-DROP / EV-MYGO-282, EV-MYGO-283.
package ui

import (
	"testing"

	"github.com/egoist/mygo/transfer"
)

func TestNoteDropRecognizesBrowserImageOverWrapperLink(t *testing.T) {
	const wrapper = "https://example.test/article"
	const image = "https://example.test/media/render?id=42"
	wrappedImage := transfer.NewItem(
		transfer.Bytes(transfer.URIList, []byte(wrapper)),
		transfer.Bytes(transfer.HTML, []byte(`<a href="`+wrapper+`"><img src="`+image+`" alt="Diagram"></a>`)),
	)
	tab := &workspaceTab{}
	app := &workspaceWindow{workspaceTab: tab}
	if markdown, _ := app.noteDropItemsMarkdown(tab, classifyNoteDrop(transfer.New(wrappedImage)), nil); markdown != "" || app.problem == "" {
		t.Fatalf("dropping a web image into an unsaved note inserted %q, problem %q", markdown, app.problem)
	}
	const directImage = "https://example.test/media/cover.WEBP?width=400"
	const plainImage = "https://example.test/media/photo.jpg"
	payload := transfer.New(wrappedImage, transfer.NewItem(
		transfer.Bytes(transfer.URIList, []byte(directImage)),
		transfer.Bytes(transfer.PNG, []byte("not a PNG")),
	), transfer.NewItem(transfer.Bytes(transfer.Text, []byte(plainImage))), transfer.NewItem(
		transfer.Bytes(transfer.HTML, []byte(`<a href="`+wrapper+`">Article</a>`)),
	))
	items := classifyNoteDrop(payload)
	if len(items) != 4 || !items[0].remoteImage || items[0].address != image || items[0].label != "Diagram" ||
		!items[1].remoteImage || items[1].address != directImage || !items[2].remoteImage || items[2].address != plainImage ||
		items[3].remoteImage || items[3].address != wrapper || items[3].label != "Article" {
		t.Fatalf("unexpected web drop plan: %+v", items)
	}
}
