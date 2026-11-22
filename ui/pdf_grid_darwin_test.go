// spec: FEAT-001 / EV-UI-061 / EV-MYGO-031 / EV-MYGO-326
package ui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
	"object-na.local/pdfthumb"
)

func TestPDFGridServesPagesForItsOwnWebView(t *testing.T) {
	pdf := pdfGridFixture(501)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	style := pdfThumbnailStyle{layout: pdfThumbnailsSheetColumn}
	file, source, err := writePDFGrid(ctx, pdf, style, pdfthumb.New(pdfGridCacheBytes, 1))
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(file))
	html, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if source.count != 500 || bytes.Count(html, []byte(`class="page-card"`)) != 500 ||
		!bytes.Contains(html, []byte(`alt="Page 500"`)) || bytes.Contains(html, []byte("data:image/jpeg;base64,")) {
		t.Fatal("expected 500 numbered slots and no eagerly embedded JPEGs")
	}
	owner, unrelated := new(mygo.Page), new(mygo.Page)
	publishPDFGrid(owner, source)
	defer unpublishPDFGrid(owner)
	request := func(caller *mygo.Page, page string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, pdfGridImageOrigin+source.route+page, nil)
		servePDFGrid(response, req, caller)
		return response
	}
	if status := request(unrelated, "1").Code; status != http.StatusNotFound {
		t.Fatalf("unrelated page: status %d", status)
	}
	if status := request(owner, "501").Code; status != http.StatusNotFound {
		t.Fatalf("outside the cap: status %d", status)
	}
	render := source.render
	source.render = func(context.Context, int) ([]byte, error) { return nil, io.ErrUnexpectedEOF }
	failure := request(owner, "1")
	source.render = render
	if failure.Code != http.StatusNotFound || failure.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("transient failure: status=%d, cache policy=%q", failure.Code, failure.Header().Get("Cache-Control"))
	}
	response := request(owner, "1")
	size, err := jpeg.DecodeConfig(response.Body)
	if response.Code != http.StatusOK || err != nil || size.Width != 278 || size.Height != 360 {
		t.Fatalf("first page: status=%d JPEG=%+v err=%v", response.Code, size, err)
	}
	cancel()
	if status := request(owner, "1").Code; status != http.StatusNotFound {
		t.Fatalf("closed grid: status %d", status)
	}
}

// Compare renderer work that must finish before the old grid could be loaded
// with the new shell and nine genuine PDFKit thumbnail requests.
func BenchmarkPDFGridInitialWindow(b *testing.B) {
	for _, pages := range []int{100, 500} {
		pdf := pdfGridFixture(pages)
		b.Run(fmt.Sprintf("%d/eager", pages), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := writeEagerPDFGridForBenchmark(pdf); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("%d/first9", pages), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				ctx, cancel := context.WithCancel(b.Context())
				cache := pdfthumb.New(pdfGridCacheBytes, 1)
				file, source, err := writePDFGrid(ctx, pdf, pdfThumbnailStyle{}, cache)
				if err != nil {
					cancel()
					b.Fatal(err)
				}
				for page := 1; page <= 9; page++ {
					key := pdfthumb.Key{Document: source.digest, Page: page, Side: source.side}
					_, err = cache.Thumbnail(ctx, key, func(ctx context.Context) ([]byte, error) {
						return source.render(ctx, page)
					})
					if err != nil {
						break
					}
				}
				cancel()
				_ = os.RemoveAll(filepath.Dir(file))
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func writeEagerPDFGridForBenchmark(pdf []byte) error {
	if err := openPDFKit(); err != nil {
		return err
	}
	defer nativeAutoreleasePool()()
	data := objc.ID(objc.GetClass("NSData")).Send(objc.RegisterName("dataWithBytes:length:"), &pdf[0], uint(len(pdf)))
	runtime.KeepAlive(pdf)
	doc := objc.ID(objc.GetClass("PDFDocument")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithData:"), data)
	defer doc.Send(objc.RegisterName("release"))
	quality := objc.ID(objc.GetClass("NSNumber")).Send(objc.RegisterName("numberWithInt:"), int32(2))
	options := objc.ID(objc.GetClass("NSDictionary")).Send(objc.RegisterName("dictionaryWithObject:forKey:"), quality, nativeNSString("NSImageCompressionFactor"))
	file, err := os.CreateTemp("", "pdf-grid-eager-test-*.html")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	writer := bufio.NewWriter(file)
	for page := range min(objc.Send[uint](doc, objc.RegisterName("pageCount")), pdfGridMaxPages) {
		jpeg := pdfKitPageThumbnail(doc, page, options, pdfGridThumbnailSide)
		if _, err := fmt.Fprintf(writer, `<img alt="Page %d" src="data:image/jpeg;base64,%s">`, page+1, base64.StdEncoding.EncodeToString(jpeg)); err != nil {
			return err
		}
	}
	return writer.Flush()
}

// A generated Letter document exercises the actual raster path without using
// archived fixtures or an additional PDF-writing dependency.
func pdfGridFixture(pages int) []byte {
	var pdf strings.Builder
	io.WriteString(&pdf, "%PDF-1.4\n")
	offsets := make([]int, pages+5)
	object := func(n int, data string) {
		offsets[n] = pdf.Len()
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", n, data)
	}
	object(1, "<< /Type /Catalog /Pages 2 0 R >>")
	var kids strings.Builder
	for i := range pages {
		fmt.Fprintf(&kids, "%d 0 R ", i+5)
	}
	object(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids.String(), pages))
	var paint strings.Builder
	io.WriteString(&paint, "0.8 0.86 0.92 rg 50 640 512 90 re f\n")
	io.WriteString(&paint, "0.12 0.21 0.38 rg BT /F1 36 Tf 68 670 Td (Thumbnail fixture) Tj ET\n")
	for row := range 26 {
		fmt.Fprintf(&paint, "0.50 g BT /F1 12 Tf 62 %d Td (PDF grid vector page for viewport timing.) Tj ET\n", 600-row*20)
	}
	object(3, fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", paint.Len(), paint.String()))
	object(4, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	for i := range pages {
		object(i+5, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 3 0 R /Resources << /Font << /F1 4 0 R >> >> >>")
	}
	xref := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Root 1 0 R /Size %d >>\nstartxref\n%s\n%%%%EOF\n", len(offsets), strconv.Itoa(xref))
	return []byte(pdf.String())
}
