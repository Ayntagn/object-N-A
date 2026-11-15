package exportdoc

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"image/png"
	"io"
	"strings"

	"cyberwriter/internal/core/mermaid"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const (
	wordNS      = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	officeRelNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	packageNS   = "http://schemas.openxmlformats.org/package/2006/relationships"
	drawingNS   = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
)

// DOCX renders the working Markdown into an Office Open XML document.
func DOCX(source string, layout Layout) ([]byte, error) {
	return DOCXWithDiagrams(source, layout, 0, nil)
}

// DOCXWithDiagrams embeds prepared Mermaid PNGs keyed by their Markdown code
// block contents, including the trailing newline returned by parser.Lines.
// A missing diagram is rendered as code. A zero baseFontSize selects 12 pt.
func DOCXWithDiagrams(source string, layout Layout, baseFontSize int, diagrams map[string][]byte) ([]byte, error) {
	_, properties, body := workspace.SplitFrontmatter(source)
	lineSpacing := wordLineSpacing(properties, layout.LineSpacing)
	doc := wordDocument{source: parser.New(parser.GFM), layout: layout, diagrams: diagrams}
	doc.source.Set(body)
	fmt.Fprintf(&doc.body, `<w:document xmlns:w="%s" xmlns:r="%s" xmlns:wp="%s"><w:body>`, wordNS, officeRelNS, drawingNS)
	for index := range doc.source.Children(0) {
		doc.block(index, "")
	}
	doc.section()
	doc.body.WriteString(`</w:body></w:document>`)

	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	parts := []struct{ name, data string }{
		{"[Content_Types].xml", wordContentTypes(len(doc.media) > 0)},
		{"_rels/.rels", rootRelationships},
		{"word/document.xml", doc.body.String()},
		{"word/styles.xml", wordStyles(baseFontSize, lineSpacing)},
		{"word/numbering.xml", wordNumbering()},
		{"word/_rels/document.xml.rels", doc.relationships()},
		{"docProps/core.xml", wordCoreProperties()},
		{"docProps/app.xml", wordAppProperties},
	}
	for index, data := range doc.media {
		w, err := archive.Create("word/" + wordMediaPath(index))
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	for _, part := range parts {
		w, err := archive.Create(part.name)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(w, xml.Header+part.data); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

type wordDocument struct {
	source   *parser.Document
	layout   Layout
	body     strings.Builder
	diagrams map[string][]byte
	media    [][]byte
}

type paragraphOptions struct {
	style, align string
	numbering    int
	indent       int
	rule         bool
}

func (d *wordDocument) block(index int, style string) {
	block := d.source.Block(index)
	options := paragraphOptions{style: style}
	switch block.Kind {
	case parser.Paragraph, parser.Heading:
		if block.Kind == parser.Paragraph && strings.TrimPrefix(strings.TrimSpace(d.source.Lines(index)), `\`) == PageBreakMarker {
			d.body.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
			return
		}
		if block.Kind == parser.Heading {
			options.style = fmt.Sprintf("Heading%d", block.Level)
		}
		d.paragraph(options)
		d.inlineBlock(index, 0)
		d.body.WriteString(`</w:p>`)
	case parser.ThematicBreak:
		options.rule = true
		d.paragraph(options)
		d.body.WriteString(`</w:p>`)
	case parser.CodeBlock:
		lines := d.source.Lines(index)
		if block.Fenced && !block.Open {
			if _, ok := mermaid.Background(block.Info()); ok && d.diagram(d.diagrams[lines]) {
				return
			}
		}
		for line := range strings.SplitSeq(lines, "\n") {
			d.paragraph(paragraphOptions{style: "Code"})
			d.textRun(line, wordCodeFont)
			d.body.WriteString(`</w:p>`)
		}
	case parser.Quote:
		for child := range d.source.Children(index) {
			d.block(child, "Quote")
		}
	case parser.List:
		d.list(index)
	case parser.Table:
		d.table(index)
	}
}

func (d *wordDocument) paragraph(p paragraphOptions) {
	d.body.WriteString(`<w:p>`)
	if p == (paragraphOptions{}) {
		return
	}
	d.body.WriteString(`<w:pPr>`)
	if p.style != "" {
		fmt.Fprintf(&d.body, `<w:pStyle w:val="%s"/>`, p.style)
	}
	if p.numbering != 0 {
		fmt.Fprintf(&d.body, `<w:numPr><w:ilvl w:val="0"/><w:numId w:val="%d"/></w:numPr>`, p.numbering)
	}
	if p.rule {
		d.body.WriteString(`<w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="auto"/></w:pBdr>`)
	} else if p.style == "Quote" {
		d.body.WriteString(`<w:pBdr><w:left w:val="single" w:sz="24" w:space="4" w:color="CCCCCC"/></w:pBdr>`)
		p.indent = 720
	}
	if p.indent != 0 {
		fmt.Fprintf(&d.body, `<w:ind w:left="%d"/>`, p.indent)
	}
	if p.align != "" {
		fmt.Fprintf(&d.body, `<w:jc w:val="%s"/>`, p.align)
	}
	d.body.WriteString(`</w:pPr>`)
}

func (d *wordDocument) list(index int) {
	id := 1
	if d.source.Block(index).Ordered {
		id = 2
	}
	for item := range d.source.Children(index) {
		options := paragraphOptions{style: "ListParagraph", numbering: id}
		var prefix string
		switch d.source.Block(item).Task {
		case parser.Checked:
			prefix = "☑ "
		case parser.Unchecked:
			prefix = "☐ "
		}
		d.paragraph(options)
		if prefix != "" {
			d.body.WriteString(`</w:p>`)
			d.paragraph(paragraphOptions{style: "ListParagraph", indent: 720})
			d.body.WriteString(`<w:r><w:t>` + prefix + `</w:t></w:r>`)
		}
		first := true
		for child := range d.source.Children(item) {
			if first {
				first = false
				if d.source.Block(child).Kind == parser.Paragraph {
					d.inlineBlock(child, 0)
					d.body.WriteString(`</w:p>`)
					continue
				}
				d.body.WriteString(`</w:p>`)
			}
			d.block(child, "")
		}
		if first {
			d.body.WriteString(`</w:p>`)
		}
	}
}

func (d *wordDocument) table(index int) {
	align := d.source.Block(index).Align
	width, _ := d.layout.Dimensions()
	columnWidth := twips(max(0, width-d.layout.Left-d.layout.Right)) / max(1, len(align))
	d.body.WriteString(`<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/><w:tblW w:w="5000" w:type="pct"/>` + wordTableBorders + `</w:tblPr><w:tblGrid>`)
	for range align {
		fmt.Fprintf(&d.body, `<w:gridCol w:w="%d"/>`, columnWidth)
	}
	d.body.WriteString(`</w:tblGrid>`)
	for row := range d.source.Children(index) {
		d.body.WriteString(`<w:tr>`)
		header := d.source.Block(row).Header
		column := 0
		for cell := range d.source.Children(row) {
			alignment := "left"
			if column < len(align) {
				switch align[column] {
				case parser.AlignCenter:
					alignment = "center"
				case parser.AlignRight:
					alignment = "right"
				}
			}
			d.body.WriteString(`<w:tc>`)
			if header {
				d.body.WriteString(`<w:tcPr><w:shd w:val="clear" w:fill="F0F0F0"/></w:tcPr>`)
			}
			d.paragraph(paragraphOptions{align: alignment})
			var marks wordMarks
			if header {
				marks = wordBold
			}
			d.inlineBlock(cell, marks)
			d.body.WriteString(`</w:p></w:tc>`)
			column++
		}
		d.body.WriteString(`</w:tr>`)
	}
	d.body.WriteString(`</w:tbl>`)
}

type wordMarks uint8

const (
	wordBold wordMarks = 1 << iota
	wordItalic
	wordStrike
	wordCode
)

const wordCodeFont = `<w:rFonts w:ascii="Courier New" w:hAnsi="Courier New" w:cs="Courier New"/>`

func (d *wordDocument) inlineBlock(index int, marks wordMarks) {
	in := d.source.Inlines(index)
	d.inlines(in, 0, len(in), marks)
}

func (d *wordDocument) inlines(in []parser.Inline, start, end int, marks wordMarks) {
	for i := start; i < end; i = in[i].Next {
		n := in[i]
		switch n.Kind {
		case parser.Text:
			d.run(n.Text, marks)
		case parser.SoftBreak:
			d.run(" ", marks)
		case parser.LineBreak:
			d.run("\n", marks)
		case parser.Code:
			d.run(n.Text, marks|wordCode)
		case parser.Strong:
			d.inlines(in, i+1, n.Next, marks|wordBold)
		case parser.Emphasis:
			d.inlines(in, i+1, n.Next, marks|wordItalic)
		case parser.Strikethrough:
			d.inlines(in, i+1, n.Next, marks|wordStrike)
		case parser.Link:
			d.run(parser.PlainText(in[i+1:n.Next])+" ("+n.Dest+")", 0)
		case parser.Image:
			d.image(n.Dest, parser.PlainText(in[i+1:n.Next]))
		}
	}
}

func (d *wordDocument) run(text string, marks wordMarks) {
	if text == "" {
		return
	}
	d.body.WriteString(`<w:r>`)
	if marks != 0 {
		d.body.WriteString(`<w:rPr>`)
		if marks&wordCode != 0 {
			d.body.WriteString(wordCodeFont)
		}
		for _, mark := range []struct {
			flag wordMarks
			tag  string
		}{
			{wordBold, `<w:b/>`}, {wordItalic, `<w:i/>`}, {wordStrike, `<w:strike/>`},
		} {
			if marks&mark.flag != 0 {
				d.body.WriteString(mark.tag)
			}
		}
		if marks&wordCode != 0 {
			d.body.WriteString(`<w:shd w:val="clear" w:fill="F5F5F5"/>`)
		}
		d.body.WriteString(`</w:rPr>`)
	}
	for len(text) > 0 {
		end := strings.IndexAny(text, "\t\r\n")
		if end < 0 {
			end = len(text)
		}
		if end > 0 {
			d.runText(text[:end])
		}
		text = text[end:]
		if len(text) > 0 {
			if text[0] == '\t' {
				d.body.WriteString(`<w:tab/>`)
			} else {
				d.body.WriteString(`<w:br/>`)
				if strings.HasPrefix(text, "\r\n") {
					text = text[1:]
				}
			}
			text = text[1:]
		}
	}
	d.body.WriteString(`</w:r>`)
}

func (d *wordDocument) image(destination, alt string) {
	if alt == "" {
		alt = destination
	}
	d.textRun("[IMAGE: "+alt+"]", `<w:color w:val="996600"/><w:shd w:val="clear" w:fill="FFF8E1"/>`)
	d.textRun(" (Source: "+destination+")", `<w:color w:val="999999"/><w:sz w:val="16"/>`)
}

func (d *wordDocument) textRun(text, properties string) {
	d.body.WriteString(`<w:r><w:rPr>` + properties + `</w:rPr>`)
	d.runText(text)
	d.body.WriteString(`</w:r>`)
}

func (d *wordDocument) runText(text string) {
	d.body.WriteString(`<w:t xml:space="preserve">`)
	xml.EscapeText(&d.body, []byte(text))
	d.body.WriteString(`</w:t>`)
}

func (d *wordDocument) diagram(data []byte) bool {
	size, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || size.Width <= 0 || size.Height <= 0 {
		return false
	}
	const emuPerPixel, maxWidth = 9525, 5486400
	width, height := int64(size.Width)*emuPerPixel, int64(size.Height)*emuPerPixel
	if width > maxWidth {
		height = max(1, int64(size.Height)*maxWidth/int64(size.Width))
		width = maxWidth
	}
	mediaIndex := len(d.media)
	d.media = append(d.media, data)
	// The DrawingML extents are in EMUs. A CSS pixel is 9525 EMUs.
	fmt.Fprintf(&d.body, `<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:drawing>
<wp:inline distT="0" distB="0" distL="0" distR="0">
<wp:extent cx="%d" cy="%d"/><wp:docPr id="%d" name="Mermaid Diagram %d"/>
<a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
<pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
<pic:nvPicPr><pic:cNvPr id="%d" name="%s"/><pic:cNvPicPr/></pic:nvPicPr>
<pic:blipFill><a:blip r:embed="rIdMermaid%d"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>
<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>
</pic:pic></a:graphicData></a:graphic>
</wp:inline></w:drawing></w:r></w:p>`, width, height, mediaIndex+1, mediaIndex+1,
		mediaIndex+1, wordMediaName(mediaIndex), mediaIndex, width, height)
	return true
}

func wordMediaName(index int) string {
	return fmt.Sprintf("mermaid_%d.png", index)
}

func wordMediaPath(index int) string {
	return "media/" + wordMediaName(index)
}

func (d *wordDocument) section() {
	width, height := d.layout.Dimensions()
	orientation := "portrait"
	if d.layout.Landscape {
		orientation = "landscape"
	}
	fmt.Fprintf(&d.body, `<w:sectPr><w:pgSz w:w="%d" w:h="%d" w:orient="%s"/>`, twips(width), twips(height), orientation)
	fmt.Fprintf(&d.body, `<w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="720" w:footer="720" w:gutter="0"/><w:cols w:space="720"/></w:sectPr>`,
		twips(d.layout.Top), twips(d.layout.Right), twips(d.layout.Bottom), twips(d.layout.Left))
}

func twips(points float64) int { return int(points * 20) }

func (d *wordDocument) relationships() string {
	var out strings.Builder
	fmt.Fprintf(&out, `<Relationships xmlns="%s"><Relationship Id="rId1" Type="%s/styles" Target="styles.xml"/><Relationship Id="rId2" Type="%s/numbering" Target="numbering.xml"/>`, packageNS, officeRelNS, officeRelNS)
	for index := range d.media {
		fmt.Fprintf(&out, `<Relationship Id="rIdMermaid%d" Type="%s/image" Target="%s"/>`, index, officeRelNS, wordMediaPath(index))
	}
	out.WriteString(`</Relationships>`)
	return out.String()
}
