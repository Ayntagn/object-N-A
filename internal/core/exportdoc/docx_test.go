// spec: EV-UI-135 / EV-UI-136 / EV-EDITOR-004
package exportdoc

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"image"
	"image/png"
	"io"
	"strings"
	"testing"
)

func TestDOCXPreservesDocumentStructure(t *testing.T) {
	source := "---\ntitle: Hidden metadata\nline-height: 1.25\n---\n\n" +
		"# 中文 & notes\n\n**Bold** and *italic* [link](https://example.com), [relative](notes/one.md), " +
		"<em>HTML text</em>, `inline`, and $E = mc^2$.\n\n" +
		"![Caption](figure.png \"Ignored title\")\n\n" +
		"> Quote\n\n3. First\n4. Second\n   1. Child\n\n- [x] Done\n\n" +
		"[pagebreak]\n\n| A | B |\n|---|---:|\n| α | β |\n\n" +
		"```vega-lite\nfirst\n\nlast\n```\n"
	layout := DefaultLayout()
	layout.PaperWidth, layout.PaperHeight = 595, 842
	layout.Landscape = true
	layout.LineSpacing = 1.75
	layout.FontScale = 150
	payload, err := DOCX(source, layout)
	if err != nil {
		t.Fatal(err)
	}
	parts := readWordParts(t, payload)
	for part, expected := range map[string][]string{
		"[Content_Types].xml": {`PartName="/word/document.xml"`, `PartName="/docProps/core.xml"`},
		"_rels/.rels":         {`Target="word/document.xml"`, `Target="docProps/core.xml"`},
		"word/document.xml": {
			`w:val="Heading1"`, "中文 &amp; notes", "<w:b/>", "<w:i/>", "link (https://example.com)",
			"relative (notes/one.md)", "HTML text", "$E = mc^2$", "[IMAGE: Caption]", " (Source: figure.png)",
			`<w:left w:val="single" w:sz="24" w:space="4" w:color="CCCCCC"/>`,
			`<w:ilvl w:val="0"/><w:numId w:val="2"/>`, "<w:t>☑ </w:t>", `<w:br w:type="page"/>`,
			`<w:tblStyle w:val="TableGrid"/>`, "<w:tblGrid>", `w:fill="F0F0F0"`, "β",
			`w:w="16840" w:h="11900" w:orient="landscape"`, `w:header="720" w:footer="720"`, `<w:cols w:space="720"/>`,
		},
		"word/styles.xml":    {`w:ascii="Arial"`, `<w:sz w:val="24"/>`, `w:after="320" w:line="300" w:lineRule="auto"`},
		"word/numbering.xml": {`w:val="hybridMultilevel"`, `w:val="lowerLetter"`, `w:val="lowerRoman"`},
		"docProps/core.xml":  {"<dc:title>Document</dc:title>"},
	} {
		for _, fragment := range expected {
			if !strings.Contains(parts[part], fragment) {
				t.Errorf("%s is missing %q", part, fragment)
			}
		}
	}
	for _, text := range []string{"Hidden metadata", "Ignored title", "&lt;em&gt;", "<w:hyperlink", "<w:drawing", `<w:ilvl w:val="1"/>`} {
		if strings.Contains(parts["word/document.xml"], text) {
			t.Errorf("unexpected DOCX content %q", text)
		}
	}
	if document := parts["word/document.xml"]; strings.Count(document, `<w:pStyle w:val="Code"/>`) != 4 ||
		strings.Count(document, `<w:br`) != 1 || strings.Contains(parts["word/numbering.xml"], "startOverride") {
		t.Error("code lines, page break or flat list numbering differ from the native structure")
	}
}

func TestDOCXEmbedsMermaidAndPreservesMissingDiagramCode(t *testing.T) {
	var diagram bytes.Buffer
	if err := png.Encode(&diagram, image.NewGray(image.Rect(0, 0, 1600, 262))); err != nil {
		t.Fatal(err)
	}
	const flowchart = "flowchart LR\nA --> B\n"
	const fallback = "graph LR\nB --> A\n"
	source := "```mermaid\n" + flowchart + "```\n\n```mermaid\n" + fallback + "```\n"
	layout := DefaultLayout()
	layout.LineSpacing = 1.75
	payload, err := DOCXWithDiagrams(source, layout, 14, map[string][]byte{flowchart: diagram.Bytes()})
	if err != nil {
		t.Fatal(err)
	}
	parts := readWordParts(t, payload)
	if parts["word/media/mermaid_0.png"] != diagram.String() {
		t.Fatal("diagram PNG is missing or changed in the package")
	}
	for part, expected := range map[string][]string{
		"[Content_Types].xml":          {`Extension="png" ContentType="image/png"`},
		"word/_rels/document.xml.rels": {`Id="rIdMermaid0"`, `Target="media/mermaid_0.png"`},
		"word/document.xml": {`<w:jc w:val="center"/>`, `<wp:extent cx="5486400" cy="898398"/>`,
			`name="Mermaid Diagram 1"`, `r:embed="rIdMermaid0"`, `<w:pStyle w:val="Code"/>`, "B --&gt; A"},
		"word/styles.xml": {`<w:sz w:val="28"/>`, `w:after="320" w:line="420"`},
	} {
		for _, fragment := range expected {
			if !strings.Contains(parts[part], fragment) {
				t.Errorf("%s is missing %q", part, fragment)
			}
		}
	}
	if document := parts["word/document.xml"]; strings.Count(document, "<w:drawing>") != 1 || strings.Contains(document, "flowchart LR") {
		t.Error("expected the prepared diagram as an image and the missing diagram as code")
	}
}

func readWordParts(t *testing.T, payload []byte) map[string]string {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	parts := make(map[string]string)
	for _, part := range archive.File {
		r, err := part.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(part.Name, ".xml") || strings.HasSuffix(part.Name, ".rels") {
			var element struct{ XMLName xml.Name }
			if err := xml.Unmarshal(data, &element); err != nil {
				t.Fatalf("invalid %s: %v", part.Name, err)
			}
		}
		parts[part.Name] = string(data)
	}
	return parts
}
