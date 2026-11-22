// spec: EV-MYGO-301
package notehtml

import (
	"strings"
	"testing"
)

func TestFragmentKeepsNoteHTML(t *testing.T) {
	const source = `<div class="reference math-inline" data-test="example" data-source-line="999"><span style="color: red" title="letter">A</span>
<table border="1" cellpadding="4"><thead><tr><th colspan="2" scope="col">Example</th></tr></thead><tbody><tr><td rowspan="2">Answer</td><td>**literal**</td></tr></tbody></table>
<figure><img src="assets/answer.png" width="160" alt="Answer"><figcaption>Answer</figcaption></figure>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><rect width="100" height="100" fill="red"/></svg>
<math xmlns="http://www.w3.org/1998/Math/MathML"><msup><mi>x</mi><mn>2</mn></msup></math></div>`
	policy := Policy{Image: func(source string) string {
		if source != "assets/answer.png" {
			return ""
		}
		return "data:image/png;base64,AA=="
	}}
	markup, ok := policy.Fragment(source)
	if !ok {
		t.Fatal("note HTML was not recognized")
	}
	for _, want := range []string{
		`class="reference" data-test="example"`, `title="letter"`, `color:`, `border="1" cellpadding="4"`,
		`colspan="2" scope="col"`, `rowspan="2"`, `<td>**literal**</td>`,
		`src="data:image/png;base64,AA==" width="160" alt="Answer"`,
		`viewBox="0 0 100 100"`, `<rect width="100" height="100" fill="red"`, `<msup><mi>x</mi><mn>2</mn></msup>`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("missing %q in %s", want, markup)
		}
	}
	if strings.Contains(markup, "math-inline") || strings.Contains(markup, "data-source-line") {
		t.Errorf("app-reserved markers survived in %s", markup)
	}
}

func TestFragmentFiltersActiveMarkupAndKeepsSandboxedEmbeds(t *testing.T) {
	const source = `<script>script-marker</script><style>style-marker</style><!--comment-marker-->
<div id="custom-stylesheet" onclick="handler-marker"><widget>Safe text</widget><a href="file:///fixture">Local</a></div>
<iframe src="https://example.invalid/embed" title="Embed" srcdoc="document-marker" sandbox="allow-same-origin allow-scripts">frame-fallback-marker</iframe>
<audio src="https://example.invalid/sound.wav" controls preload="none"></audio>
<video controls title="Example clip"><source src="https://example.invalid/clip.mp4" type="video/mp4"></video>
<form action="https://example.invalid/form"><input type="text" value="Example"><button type="submit">Done</button></form>
<details><summary>Answer</summary><p>Open in print</p></details>`
	markup, ok := (Policy{}).Fragment(source)
	if !ok {
		t.Fatal("note HTML was not recognized")
	}
	for _, want := range []string{`Safe text<a>Local</a>`, `src="https://example.invalid/embed"`, `sandbox="allow-scripts"`, `referrerpolicy="no-referrer"`, `<audio src="https://example.invalid/sound.wav" controls="" preload="none"`, `<button type="button">Done</button>`} {
		if !strings.Contains(markup, want) {
			t.Errorf("missing %q in %s", want, markup)
		}
	}
	for _, bad := range []string{"-marker", "<script", "<style", "custom-stylesheet", "onclick", "file:", "srcdoc", "allow-same-origin", "action=", "type=\"submit\"", "<widget"} {
		if strings.Contains(markup, bad) {
			t.Errorf("unexpected %q in %s", bad, markup)
		}
	}
	printed, _ := (Policy{Print: true}).Fragment(source)
	if strings.Contains(printed, "<iframe") || strings.Contains(printed, "<audio") || strings.Contains(printed, "<video") ||
		!strings.Contains(printed, `<a href="https://example.invalid/embed">Embed</a>`) ||
		!strings.Contains(printed, `<a href="https://example.invalid/sound.wav">https://example.invalid/sound.wav</a>`) ||
		!strings.Contains(printed, `<a href="https://example.invalid/clip.mp4">Example clip</a>`) ||
		!strings.Contains(printed, `<details open=""><summary>Answer</summary>`) {
		t.Fatalf("print preview did not expose notes and links: %s", printed)
	}
}

func TestInlineTagRecognizesFormattingAndAtomicMedia(t *testing.T) {
	policy := Policy{}
	open := policy.InlineTag(`<B CLASS="note api-run-btn" onclick="handler-marker">`)
	close := policy.InlineTag(`</b>`)
	frame := policy.InlineTag(`<iframe src="https://example.invalid/embed">`)
	if open.Name != "b" || open.Close || open.Void || open.Markup != `<b class="note">` ||
		close.Name != "b" || !close.Close ||
		frame.Name != "iframe" || !frame.Void || !strings.HasSuffix(frame.Markup, "</iframe>") {
		t.Fatalf("unexpected inline tags: %+v %+v %+v", open, close, frame)
	}
	if script := policy.InlineTag("<script>"); script.Name != "script" || !script.DropContent || script.Markup != "" {
		t.Fatalf("script was not suppressed: %+v", script)
	}
	printed := (Policy{Print: true}).InlineTag(`<audio src="https://example.invalid/sound.wav" title="Sound">`)
	if !printed.Void || printed.Markup != `<a href="https://example.invalid/sound.wav">Sound</a>` {
		t.Fatalf("inline media did not become an atomic print link: %+v", printed)
	}
}
