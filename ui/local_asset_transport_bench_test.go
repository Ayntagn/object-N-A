package ui

// spec: EV-MYGO-314 / EV-UI-052 / EV-MYGO-023

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func BenchmarkLocalAssetBaseline(b *testing.B) {
	b.Run("VegaRepeatedCSV", func(b *testing.B) {
		vault, source := localAssetChartFixture(b)
		render := func(input string, parsed *parser.Document, cache *previewRenderCache) livePreviewRender {
			origin := previewResourceOrigin{vault: vault, note: "Note.md"}
			markup := previewMarkup{doc: parsed, relative: origin.note, resources: origin.resources(), charts: true}
			return markup.renderLive(input, "Note", cache, false)
		}
		benchmarkLocalAssetChart(b, source, render)
	})
	b.Run("PDFCaptureJSON", func(b *testing.B) {
		data := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte{0, 128, 227, 0, 112}, (2<<20)/5)...)
		sources := []pdfSource{{Height: 385, Data: data}}
		var encoded []byte
		for b.Loop() {
			var err error
			encoded, err = json.Marshal(sources)
			if err != nil {
				b.Fatal(err)
			}
		}
		b.ReportMetric(float64(len(encoded)), "eval_bytes/op")
	})
	b.Run("PDFResultJSON", func(b *testing.B) {
		data := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte{0, 128, 227, 0, 112}, (2<<20)/5)...)
		result := pdfOutput{Data: data, Pages: 2}
		encoded, err := json.Marshal(result)
		if err != nil {
			b.Fatal(err)
		}
		for b.Loop() {
			var received pdfOutput
			if err := json.Unmarshal(encoded, &received); err != nil {
				b.Fatal(err)
			}
			if len(received.Data) != len(data) {
				b.Fatal("decoded PDF length differs")
			}
		}
		b.ReportMetric(float64(len(encoded)), "eval_bytes/op")
	})
}

func BenchmarkLocalAssetScoped(b *testing.B) {
	b.Run("VegaRepeatedCSV", func(b *testing.B) {
		vault, source := localAssetChartFixture(b)
		page := &mygo.Page{}
		b.Cleanup(func() { pageLocalAssets.RevokeOwner(page) })
		var previous map[previewChartAssetKey]previewChartAsset
		render := func(input string, parsed *parser.Document, cache *previewRenderCache) livePreviewRender {
			origin := previewResourceOrigin{vault: vault, note: "Note.md"}
			resources := origin.resources()
			assets := previewChartAssetFrame{page: page, previous: previous}
			resources.chartAssets = &assets
			markup := previewMarkup{doc: parsed, relative: origin.note, resources: resources, charts: true}
			result := markup.renderLive(input, "Note", cache, false)
			previous = assets.publish()
			return result
		}
		benchmarkLocalAssetChart(b, source, render)
	})
	b.Run("PDFCaptureJSON", func(b *testing.B) {
		page := &mygo.Page{}
		data := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte{0, 128, 227, 0, 112}, (2<<20)/5)...)
		sources := []pdfSource{{Height: 385, Data: data}}
		var encoded []byte
		for b.Loop() {
			scoped, tokens := grantPDFSources(page, sources)
			if len(tokens) != 1 {
				b.Fatal("expected one resource grant")
			}
			var err error
			encoded, err = json.Marshal(scoped)
			pageLocalAssets.Revoke(page, tokens[0])
			if err != nil {
				b.Fatal(err)
			}
		}
		b.ReportMetric(float64(len(encoded)), "eval_bytes/op")
	})
	b.Run("PDFResultJSON", func(b *testing.B) {
		page := &mygo.Page{}
		data := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte{0, 128, 227, 0, 112}, (2<<20)/5)...)
		var frames [][]byte
		for offset := 0; offset < len(data); offset += localAssetUploadChunkBytes {
			end := min(offset+localAssetUploadChunkBytes, len(data))
			frame := binary.BigEndian.AppendUint32(make([]byte, 0, end-offset+8), uint32(len(data)))
			frame = binary.BigEndian.AppendUint32(frame, uint32(offset))
			frames = append(frames, append(frame, data[offset:end]...))
		}
		const encoded = `{"Uploaded":true,"Pages":2}`
		for b.Loop() {
			token, ok := pageLocalAssets.ReserveUpload(page, maxPDFResultUpload)
			if !ok {
				b.Fatal("reserve upload")
			}
			for _, frame := range frames {
				if err := pageLocalAssets.ReceiveUpload(page, token, bytes.NewReader(frame)); err != nil {
					b.Fatal(err)
				}
			}
			var result struct {
				pdfOutput
				Uploaded bool
			}
			if err := json.Unmarshal([]byte(encoded), &result); err != nil || !result.Uploaded {
				b.Fatalf("decode upload result: %v", err)
			}
			received, ok := pageLocalAssets.TakeUpload(page, token)
			if !ok || len(received) != len(data) {
				b.Fatal("uploaded PDF length differs")
			}
		}
		b.ReportMetric(float64(len(encoded)), "eval_bytes/op")
	})
}

func localAssetChartFixture(b testing.TB) (*workspace.Vault, string) {
	b.Helper()
	dir := b.TempDir()
	const row = "small,123.45\nlarge,67.89\n"
	csv := "group,amount\n" + strings.Repeat(row, (2<<20)/len(row))
	if err := os.WriteFile(filepath.Join(dir, "figures.csv"), []byte(csv), 0o600); err != nil {
		b.Fatal(err)
	}
	vault, err := workspace.Open(dir)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = vault.Close() })
	const chart = "```vega-lite\n" + `{"data":{"url":"figures.csv"},"mark":"bar","encoding":{"x":{"field":"group"},"y":{"aggregate":"sum","field":"amount","type":"quantitative"}}}` + "\n```\n"
	return vault, "Before edit\n\n" + chart + "\n" + strings.Repeat("A paragraph fills the preview without changing the Vega data.\n\n", 1200) + "\n" + chart
}

func benchmarkLocalAssetChart(b *testing.B, source string, render func(string, *parser.Document, *previewRenderCache) livePreviewRender) {
	b.Helper()
	edited := strings.Replace(source, "Before edit", "After edit", 1)
	var cache previewRenderCache
	var transport previewTransport
	parsed := parser.New(webPreviewExtensions)
	revision := uint64(1)
	previous := render(source, parsed, &cache)
	fullBytes := len(previous.fullHTML())
	transport.commit(1, revision, previous.blocks)
	var frame previewFrame
	for b.Loop() {
		revision++
		text := edited
		if revision%2 == 1 {
			text = source
		}
		current := render(text, parsed, &cache)
		frame = previewFrame{Document: 1, Revision: revision, Patch: transport.patch(1, revision, current.blocks)}
		if frame.Patch == nil {
			frame.HTML = current.fullHTML()
		}
		transport.commit(1, revision, current.blocks)
	}
	editBytes := len(frame.HTML)
	if frame.Patch != nil {
		editBytes = len(frame.Patch.HTML)
	}
	b.ReportMetric(float64(fullBytes), "initial_html_bytes")
	b.ReportMetric(float64(editBytes), "edit_html_bytes/op")
}
