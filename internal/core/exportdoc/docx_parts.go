package exportdoc

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const wordTableBorders = `<w:tblBorders>
<w:top w:val="single" w:sz="4" w:color="auto"/>
<w:left w:val="single" w:sz="4" w:color="auto"/>
<w:bottom w:val="single" w:sz="4" w:color="auto"/>
<w:right w:val="single" w:sz="4" w:color="auto"/>
<w:insideH w:val="single" w:sz="4" w:color="auto"/>
<w:insideV w:val="single" w:sz="4" w:color="auto"/>
</w:tblBorders>`

func wordLineSpacing(properties string, fallback float64) int {
	for entry := range strings.SplitSeq(properties, "\n") {
		key, value, _ := strings.Cut(entry, ":")
		if strings.TrimRight(key, " \t") != "line-height" {
			continue
		}
		value, _, _ = strings.Cut(value, "#")
		if height, err := strconv.ParseFloat(strings.Trim(strings.TrimSpace(value), `"'`), 64); err == nil && validWordLineHeight(height) {
			return max(240, int(height*240))
		}
	}
	if !validWordLineHeight(fallback) {
		fallback = 1.6
	}
	return max(240, int(fallback*240))
}

func validWordLineHeight(height float64) bool {
	return height > 0 && height*240 <= math.MaxInt32
}

// EV-UI-135/136: Word's default type and spacing come from the document theme.
// Its paragraph and code styles are independent of PDF appearance overrides.
func wordStyles(baseFontSize, lineSpacing int) string {
	if baseFontSize <= 0 {
		baseFontSize = 12
	}
	size := baseFontSize * 2
	var out strings.Builder
	fmt.Fprintf(&out, `<w:styles xmlns:w="%s">
<w:docDefaults><w:rPrDefault><w:rPr>
<w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="%d"/><w:szCs w:val="%d"/>
</w:rPr></w:rPrDefault><w:pPrDefault><w:pPr>
<w:spacing w:after="320" w:line="%d" w:lineRule="auto"/>
</w:pPr></w:pPrDefault></w:docDefaults>
<w:style w:type="paragraph" w:styleId="Normal" w:default="1"><w:name w:val="Normal"/><w:qFormat/></w:style>`,
		wordNS, size, size, lineSpacing)
	for index, heading := range [...]struct{ size, before, after int }{
		{48, 480, 120}, {36, 360, 80}, {28, 280, 80}, {24, 240, 60}, {22, 200, 40}, {20, 200, 40},
	} {
		var color string
		if index == 5 {
			color = `<w:color w:val="666666"/>`
		}
		fmt.Fprintf(&out, `
<w:style w:type="paragraph" w:styleId="Heading%d"><w:name w:val="Heading %d"/>
<w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/>
<w:pPr><w:keepNext/><w:keepLines/><w:spacing w:before="%d" w:after="%d"/><w:outlineLvl w:val="%d"/></w:pPr>
<w:rPr><w:b/>%s<w:sz w:val="%d"/><w:szCs w:val="%d"/></w:rPr></w:style>`,
			index+1, index+1, heading.before, heading.after, index, color, heading.size, heading.size)
	}
	out.WriteString(`
<w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:basedOn w:val="Normal"/><w:qFormat/>
<w:pPr><w:shd w:val="clear" w:fill="F5F5F5"/><w:spacing w:after="0" w:line="240" w:lineRule="auto"/></w:pPr>
<w:rPr>` + wordCodeFont + `<w:sz w:val="20"/><w:szCs w:val="20"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/><w:basedOn w:val="Normal"/><w:qFormat/>
<w:pPr><w:ind w:left="720"/></w:pPr><w:rPr><w:i/><w:color w:val="666666"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:qFormat/>
<w:pPr><w:ind w:left="720"/></w:pPr></w:style>
<w:style w:type="table" w:styleId="TableNormal" w:default="1"><w:name w:val="Normal Table"/>
<w:tblPr><w:tblInd w:w="0" w:type="dxa"/><w:tblCellMar>
<w:top w:w="0" w:type="dxa"/><w:left w:w="108" w:type="dxa"/><w:bottom w:w="0" w:type="dxa"/><w:right w:w="108" w:type="dxa"/>
</w:tblCellMar></w:tblPr></w:style>
<w:style w:type="table" w:styleId="TableGrid"><w:name w:val="Table Grid"/><w:basedOn w:val="TableNormal"/>
<w:tblPr>` + wordTableBorders + `</w:tblPr></w:style></w:styles>`)
	return out.String()
}

func wordNumbering() string {
	var out strings.Builder
	fmt.Fprintf(&out, `<w:numbering xmlns:w="%s">`, wordNS)
	for index, levels := range [...][3]struct{ format, label string }{
		{{"bullet", "•"}, {"bullet", "○"}, {"bullet", "■"}},
		{{"decimal", "%1."}, {"lowerLetter", "%2."}, {"lowerRoman", "%3."}},
	} {
		fmt.Fprintf(&out, `<w:abstractNum w:abstractNumId="%d"><w:nsid w:val="%08X"/><w:multiLevelType w:val="hybridMultilevel"/>`, index, index+1)
		for level, choice := range levels {
			fmt.Fprintf(&out, `<w:lvl w:ilvl="%d"><w:start w:val="1"/><w:numFmt w:val="%s"/><w:lvlText w:val="%s"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr></w:lvl>`,
				level, choice.format, choice.label, (level+1)*720)
		}
		out.WriteString(`</w:abstractNum>`)
	}
	out.WriteString(`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num><w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num></w:numbering>`)
	return out.String()
}

const rootRelationships = `<Relationships xmlns="` + packageNS + `">
<Relationship Id="rId1" Type="` + officeRelNS + `/officeDocument" Target="word/document.xml"/>
<Relationship Id="rId2" Type="` + packageNS + `/metadata/core-properties" Target="docProps/core.xml"/>
<Relationship Id="rId3" Type="` + officeRelNS + `/extended-properties" Target="docProps/app.xml"/>
</Relationships>`

func wordContentTypes(hasDiagrams bool) string {
	var media string
	if hasDiagrams {
		media = `<Default Extension="png" ContentType="image/png"/>`
	}
	return `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>` + media + `
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>
<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
</Types>`
}

func wordCoreProperties() string {
	now := time.Now().UTC().Format(time.RFC3339)
	return `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
<dc:title>Document</dc:title><dc:creator>cyberWriter</dc:creator><cp:lastModifiedBy>cyberWriter</cp:lastModifiedBy>
<dcterms:created xsi:type="dcterms:W3CDTF">` + now + `</dcterms:created><dcterms:modified xsi:type="dcterms:W3CDTF">` + now + `</dcterms:modified>
</cp:coreProperties>`
}

const wordAppProperties = `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
<Application>cyberWriter</Application><AppVersion>1.0</AppVersion>
</Properties>`
