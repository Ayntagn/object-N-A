// spec: EV-UI-164, EV-UI-165
package ui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestPreviewLiveAPIRendersSavedResponse(t *testing.T) {
	source := "---\ntitle: 测试\n---\n\n# Synthetic weather 😀\n\nPrice $5. Example `$x$`.\n\n" +
		"```api[title: Local Weather, out: preview]\nGET https://example.invalid/weather\nX-Fixture: synthetic\n```\n\n" +
		"```api-result\nHTTP 200 OK · 1 ms · 27 B · 2026-11-14 12:21\n" +
		`[{"reading":"<b>clear</b>"}]` + "\n```\n\nAfter the request.\n"
	offset := strings.Index(source, "```api[")
	markup := previewMarkup{doc: parser.New(webPreviewExtensions), liveAPIFences: true,
		apiActions: &previewAPIActions{version: 4, document: 2}}
	preview, _ := markup.render(source, "Synthetic weather", true)
	button := `<button type="button" class="api-run-btn" data-offset="` + strconv.Itoa(offset) + `" data-version="4" data-document="2">Run</button>`
	for _, want := range []string{button, `<summary>Local Weather</summary>`, `<summary>Raw response</summary>`, `&lt;b&gt;clear&lt;/b&gt;`, `Example <code>$x$</code>.`} {
		if !strings.Contains(preview, want) {
			t.Errorf("Preview omitted %q from %s", want, preview)
		}
	}
	if strings.Contains(preview, "X-Fixture") || strings.Contains(preview, "<b>clear</b>") {
		t.Errorf("Preview displayed a request header or active response markup in %s", preview)
	}
	location, ok := previewAPINavigation("cyberwriter-api://run?offset=" + strconv.Itoa(offset) + "&version=4&document=2")
	if !ok || location.offset != offset || location.version != 4 || location.document != 2 {
		t.Fatalf("Preview button route = %+v, %v", location, ok)
	}

	doc := exportDocument{source: source, name: "Synthetic weather", kind: documentMarkdown, theme: "modern"}
	_, pdfBody := renderPDFHTML(doc)
	html, err := renderStandaloneHTML(doc, htmlResponsive, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{"PDF": pdfBody, "HTML": html} {
		for _, want := range []string{"Local Weather", "<table", "&lt;b&gt;clear&lt;/b&gt;", "fetched 2026-11-14 12:21", "After the request."} {
			if !strings.Contains(output, want) {
				t.Errorf("%s omitted %q", name, want)
			}
		}
		for _, unwanted := range []string{"example.invalid", "X-Fixture", "<b>clear</b>", "<button", "<details"} {
			if strings.Contains(output, unwanted) {
				t.Errorf("%s emitted %q", name, unwanted)
			}
		}
	}
}
