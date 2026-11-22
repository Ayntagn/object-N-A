package ui

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"time"

	"cyberwriter/internal/core/exportdoc"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

const pdfPaperScriptHash = "sha256-D5pcrQeUHwgmWGyU4InYm5GMRuXBfPLVo8b2ZuO8aU8="

//go:embed webassets/pdf-lib/pdf-lib.min.js.gz
var pdfPaperBundle []byte

//go:embed pdf_output.js
var pdfOutputScript string

//go:embed pdf_unicode.js
var pdfUnicodeScript string

var pdfPaperLoader = previewAssetLoader(`typeof window.PDFLib?.PDFDocument?.load === "function"`, "cyberwriter:pdf-paper-ready", pdfPaperBundle)

type pdfSource struct {
	Top    float64 `json:"top"`
	Height float64 `json:"height"`
	Data   []byte  `json:"data,omitempty"`
	URL    string  `json:"url,omitempty"`
}

type pdfOutput struct {
	Data  []byte
	Pages int
}

type pdfPageSlice struct {
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Number string  `json:"number,omitempty"`
}

type pdfComposition struct {
	Width, Height  float64
	ContentWidth   float64
	Left, Top      float64
	Bottom         float64
	NumberPosition exportdoc.PDFNumberPosition
	Paper          [3]float64
	Pages          []pdfPageSlice
	Links          []pdfLink
	SystemPrint    bool
	Unicode        map[string]map[uint16]uint16
	Upload         *pdfResultUpload `json:",omitempty"`
}

type pdfResultUpload struct {
	URL        string `json:"url"`
	MinBytes   int    `json:"minBytes"`
	MaxBytes   int    `json:"maxBytes"`
	ChunkBytes int    `json:"chunkBytes"`
}

const maxPDFResultUpload = 64 << 20

func composePDF(window *mygo.Window, doc exportDocument, width float64, geometry pdfGeometry, sources []pdfSource) (pdfOutput, error) {
	paperWidth, paperHeight := doc.layout.Dimensions()
	params := pdfComposition{
		Width: paperWidth, Height: paperHeight, ContentWidth: width,
		Left: doc.layout.Left, Top: doc.layout.Top, Bottom: doc.layout.Bottom,
		NumberPosition: doc.pagination.NumberPosition,
		Paper:          pdfPaperColor(doc),
		Links:          geometry.Links,
	}
	cuts := []float64{0, geometry.Height}
	if doc.pagination.Continuous {
		params.Width, params.Height = width, geometry.Height
		params.Left, params.Top, params.Bottom = 0, 0, 0
	} else {
		cuts = exportdoc.PDFPageCuts(geometry.Height, paperHeight-doc.layout.Top-doc.layout.Bottom, geometry.Manual, geometry.Ordinary)
	}
	if len(cuts) < 2 {
		return pdfOutput{}, errors.New("the PDF margins leave no space for content")
	}
	for _, index := range selectedPDFPages(doc.pagination.Range, len(cuts)-1) {
		params.Pages = append(params.Pages, pdfPageSlice{
			Start: cuts[index], End: cuts[index+1], Number: doc.pagination.NumberLabel(index),
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	page := window.Page()
	if err := loadPDFAsset(ctx, page, "pdf-lib", pdfPaperLoader); err != nil {
		return pdfOutput{}, err
	}
	replacements, err := loadPDFSources(ctx, page, sources)
	if err != nil {
		return pdfOutput{}, err
	}
	params.Unicode = replacements
	return buildPDFOutput(ctx, page, params)
}

func loadPDFSources(ctx context.Context, page *mygo.Page, sources []pdfSource) (map[string]map[uint16]uint16, error) {
	if pageAssetAvailable(page) {
		scoped, grants := grantPDFSources(page, sources)
		if len(grants) > 0 {
			replacements, err := readPDFSources(ctx, page, scoped)
			for _, grant := range grants {
				pageLocalAssets.Revoke(page, grant)
			}
			if err == nil || ctx.Err() != nil {
				return replacements, err
			}
		}
	}
	return readPDFSources(ctx, page, sources)
}

func grantPDFSources(page *mygo.Page, sources []pdfSource) ([]pdfSource, []string) {
	var scoped []pdfSource
	var grants []string
	for i, source := range sources {
		if len(source.Data) < localAssetThreshold {
			continue
		}
		token, ok := pageLocalAssets.AddBytes(page, "application/pdf", source.Data)
		if !ok {
			continue
		}
		if scoped == nil {
			scoped = slices.Clone(sources)
		}
		scoped[i].Data = nil
		scoped[i].URL = localAssetOrigin + "/" + token
		grants = append(grants, token)
	}
	if scoped == nil {
		scoped = sources
	}
	return scoped, grants
}

func readPDFSources(ctx context.Context, page *mygo.Page, sources []pdfSource) (map[string]map[uint16]uint16, error) {
	encoded, err := json.Marshal(sources)
	if err != nil {
		return nil, err
	}
	script := "const chunks = " + string(encoded) + ";\n" + `
window.cyberwriterPDFSources = [];
for (const chunk of chunks) {
  let data = chunk.data;
  if (chunk.url) {
    const response = await fetch(chunk.url, {credentials: "omit"});
    if (!response.ok) throw new Error("PDF capture transfer failed");
    data = new Uint8Array(await response.arrayBuffer());
  }
  const input = await PDFLib.PDFDocument.load(data, {updateMetadata: false});
  if (input.getPageCount() !== 1) throw new Error("expected one PDF page per capture");
  window.cyberwriterPDFSources.push({top: chunk.top, height: chunk.height, input});
}
`
	var radicals map[string][]uint16
	if runtime.GOOS == "darwin" {
		script += pdfUnicodeScript + "\nreturn JSON.stringify(pdfUnicodeRadicals(PDFLib, window.cyberwriterPDFSources));"
		err = decodePDFScript(ctx, page, script, &radicals)
	} else {
		_, err = page.EvalContext(ctx, script)
	}
	if err != nil {
		return nil, fmt.Errorf("read PDF capture: %w", err)
	}
	return pdfUnicodeReplacements(radicals), nil
}

func finishSystemPrint(page *mygo.Page, doc exportDocument, data []byte) (pdfOutput, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := loadPDFAsset(ctx, page, "pdf-lib", pdfPaperLoader); err != nil {
		return pdfOutput{}, err
	}
	var count int
	source := base64.StdEncoding.EncodeToString(data)
	if err := decodePDFScript(ctx, page, fmt.Sprintf(`
window.cyberwriterPrintedPDF = await PDFLib.PDFDocument.load(%q, {updateMetadata: false});
return JSON.stringify(window.cyberwriterPrintedPDF.getPageCount());`, source), &count); err != nil {
		return pdfOutput{}, err
	}
	if count < 1 {
		return pdfOutput{}, errors.New("the system WebView returned an empty PDF")
	}
	width, height := doc.layout.Dimensions()
	params := pdfComposition{
		Width: width, Height: height, ContentWidth: width - doc.layout.Left - doc.layout.Right,
		Left: doc.layout.Left, Top: doc.layout.Top, Bottom: doc.layout.Bottom,
		NumberPosition: doc.pagination.NumberPosition, Paper: pdfPaperColor(doc), SystemPrint: true,
	}
	for _, index := range selectedPDFPages(doc.pagination.Range, count) {
		params.Pages = append(params.Pages, pdfPageSlice{Start: float64(index), Number: doc.pagination.NumberLabel(index)})
	}
	return buildPDFOutput(ctx, page, params)
}

func selectedPDFPages(query string, count int) []int {
	if selection := exportdoc.PDFPageRange(query, count); len(selection) != 0 {
		return selection
	}
	selection := make([]int, count)
	for index := range selection {
		selection[index] = index
	}
	return selection
}

func pdfPaperColor(doc exportDocument) [3]float64 {
	if theme, ok := findPreviewTheme(doc.theme); ok {
		theme = doc.appearance.applyTo(theme)
		color := mygoui.Hex(theme.BackgroundColor)
		return [3]float64{float64(color.R) / 255, float64(color.G) / 255, float64(color.B) / 255}
	}
	return [3]float64{1, 1, 1}
}

func buildPDFOutput(ctx context.Context, page *mygo.Page, params pdfComposition) (pdfOutput, error) {
	var token string
	if pageAssetAvailable(page) {
		if grant, ok := pageLocalAssets.ReserveUpload(page, maxPDFResultUpload); ok {
			token = grant
			defer pageLocalAssets.Revoke(page, token)
			params.Upload = &pdfResultUpload{
				URL: localAssetOrigin + "/" + token, MinBytes: localAssetThreshold,
				MaxBytes: maxPDFResultUpload, ChunkBytes: localAssetUploadChunkBytes,
			}
		}
	}
	args, err := json.Marshal(params)
	if err != nil {
		return pdfOutput{}, err
	}
	var result struct {
		pdfOutput
		Uploaded bool
	}
	script := "const plan = " + string(args) + ";\n"
	if len(params.Unicode) > 0 {
		script += pdfUnicodeScript + "\n"
	}
	if err := decodePDFScript(ctx, page, script+pdfOutputScript, &result); err != nil {
		return pdfOutput{}, fmt.Errorf("assemble PDF pages: %w", err)
	}
	if result.Uploaded {
		var ok bool
		result.Data, ok = pageLocalAssets.TakeUpload(page, token)
		if !ok {
			return pdfOutput{}, errors.New("the PDF compositor did not deliver its result")
		}
	}
	if result.Pages < 1 || !bytes.HasPrefix(result.Data, []byte("%PDF-")) {
		return pdfOutput{}, errors.New("the PDF compositor returned an invalid document")
	}
	return result.pdfOutput, nil
}
