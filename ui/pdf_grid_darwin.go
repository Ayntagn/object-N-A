package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var openPDFKit = sync.OnceValue(func() error {
	_, err := purego.Dlopen("/System/Library/Frameworks/PDFKit.framework/PDFKit", purego.RTLD_LAZY|purego.RTLD_LOCAL)
	return err
})

func renderPlatformPDFGrid(ctx context.Context, w io.Writer, pdf []byte, style pdfThumbnailStyle, route string) (*pdfGridSource, error) {
	grid, err := newPDFKitGrid(ctx, pdf, style.maxSide())
	if err != nil {
		return nil, err
	}
	script := fmt.Sprintf("const thumbnailBudget = %d, thumbnailRoot = %q;\n", pdfGridCacheBytes, pdfGridImageOrigin+route) + pdfGridViewport + `
const cards = Array.from(document.querySelectorAll(".page-card"), (element, index) =>
  ({ element, image: element.querySelector("img"), number: index + 1 }));
watchPDFThumbnailWindow(cards, async (card, signal) => {
  await loadPDFThumbnailImage(card.image, thumbnailRoot + card.number, signal);
  sizePDFThumbnail(card.image, card.image.naturalWidth, card.image.naturalHeight);
  return () => resetPDFThumbnail(card.image);
});`
	if err := writePDFGridHeader(w, script, style, pdfGridImageOrigin); err != nil {
		return nil, err
	}
	if _, err := io.WriteString(w, `<main id="pages" aria-label="PDF pages">`); err != nil {
		return nil, err
	}
	for i, size := range grid.sizes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n := i + 1
		if _, err := fmt.Fprintf(w, `<div class="page-card"><img class="thumbnail-placeholder" alt="Page %d" width="%d" height="%d" style="--thumbnail-width:%d;--thumbnail-height:%d"><span class="page-number">%d</span></div>`,
			n, size.width, size.height, size.width, size.height, n); err != nil {
			return nil, err
		}
	}
	if _, err := io.WriteString(w, `</main><script type="module">`+script+`</script></body></html>`); err != nil {
		return nil, err
	}
	return &pdfGridSource{count: len(grid.sizes), render: grid.render}, nil
}

type pdfKitGridSize struct{ width, height int }

type pdfKitGridRequest struct {
	ctx    context.Context
	page   int
	result chan []byte
}

type pdfKitGrid struct {
	ctx   context.Context
	sizes []pdfKitGridSize
	jobs  chan pdfKitGridRequest
}

func newPDFKitGrid(ctx context.Context, pdf []byte, side int) (*pdfKitGrid, error) {
	if len(pdf) == 0 {
		return nil, errors.New("PDF preview is empty")
	}
	if err := openPDFKit(); err != nil {
		return nil, err
	}
	grid := &pdfKitGrid{ctx: ctx, jobs: make(chan pdfKitGridRequest)}
	ready := make(chan error, 1)
	go grid.run(pdf, side, ready)
	select {
	case err := <-ready:
		if err != nil {
			return nil, err
		}
		return grid, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (grid *pdfKitGrid) run(pdf []byte, side int, ready chan<- error) {
	if err := grid.ctx.Err(); err != nil {
		ready <- err
		return
	}
	// The pool keeps the PDFKit document and every page access on one
	// native thread. Each actual raster has a short-lived nested pool.
	defer nativeAutoreleasePool()()
	data := objc.ID(objc.GetClass("NSData")).Send(objc.RegisterName("dataWithBytes:length:"), &pdf[0], uint(len(pdf)))
	runtime.KeepAlive(pdf)
	doc := objc.ID(objc.GetClass("PDFDocument")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithData:"), data)
	if doc == 0 {
		ready <- errors.New("PDFKit could not open the PDF preview")
		return
	}
	defer doc.Send(objc.RegisterName("release"))
	count := min(objc.Send[uint](doc, objc.RegisterName("pageCount")), pdfGridMaxPages)
	if count == 0 {
		ready <- errors.New("PDF preview has no pages")
		return
	}
	grid.sizes = make([]pdfKitGridSize, count)
	for index := range count {
		if err := grid.ctx.Err(); err != nil {
			ready <- err
			return
		}
		page := doc.Send(objc.RegisterName("pageAtIndex:"), index)
		var size previewNativeSize
		if page != 0 {
			size = objc.Send[previewNativeRect](page, objc.RegisterName("boundsForBox:"), pdfKitMediaBox).Size
		}
		width, height := size.Width, size.Height
		if width <= 0 || height <= 0 || math.IsNaN(width+height) || math.IsInf(width+height, 0) {
			width, height = 612, 792
		}
		longest := max(width, height)
		grid.sizes[index] = pdfKitGridSize{
			width:  max(1, int(width/longest*float64(side))),
			height: max(1, int(height/longest*float64(side))),
		}
	}
	quality := objc.ID(objc.GetClass("NSNumber")).Send(objc.RegisterName("numberWithInt:"), int32(2))
	options := objc.ID(objc.GetClass("NSDictionary")).Send(objc.RegisterName("dictionaryWithObject:forKey:"), quality, nativeNSString("NSImageCompressionFactor"))
	ready <- nil
	for {
		select {
		case <-grid.ctx.Done():
			return
		case request := <-grid.jobs:
			var jpeg []byte
			if request.ctx.Err() == nil && grid.ctx.Err() == nil {
				jpeg = pdfKitPageThumbnail(doc, uint(request.page-1), options, float64(side))
			}
			request.result <- jpeg
		}
	}
}

func (grid *pdfKitGrid) render(ctx context.Context, page int) ([]byte, error) {
	if page < 1 || page > len(grid.sizes) {
		return nil, errors.New("PDF page outside thumbnail range")
	}
	result := make(chan []byte, 1)
	select {
	case grid.jobs <- pdfKitGridRequest{ctx: ctx, page: page, result: result}:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-grid.ctx.Done():
		return nil, grid.ctx.Err()
	}
	select {
	case jpeg := <-result:
		if len(jpeg) == 0 {
			return nil, fmt.Errorf("PDFKit could not render page %d", page)
		}
		return jpeg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-grid.ctx.Done():
		return nil, grid.ctx.Err()
	}
}

const pdfKitMediaBox int64 = 0

func pdfKitPageThumbnail(doc objc.ID, index uint, options objc.ID, maxSide float64) []byte {
	defer nativeAutoreleasePool()()
	page := doc.Send(objc.RegisterName("pageAtIndex:"), index)
	if page == 0 {
		return nil
	}
	bounds := objc.Send[previewNativeRect](page, objc.RegisterName("boundsForBox:"), pdfKitMediaBox)
	size := bounds.Size
	if size.Width <= 0 || size.Height <= 0 || math.IsNaN(size.Width+size.Height) || math.IsInf(size.Width+size.Height, 0) {
		return nil
	}
	fit := maxSide / max(size.Width, size.Height)
	size = previewNativeSize{Width: size.Width * fit, Height: size.Height * fit}
	image := page.Send(objc.RegisterName("thumbnailOfSize:forBox:"), size, pdfKitMediaBox)
	return nativeBitmapBytes(image, nativeBitmapJPEG, options)
}
