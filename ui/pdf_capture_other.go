//go:build !darwin

package ui

import (
	"context"
	"time"

	"github.com/egoist/mygo"
)

const pdfInitialViewportHeight = 0

func capturePDFContent(window *mygo.Window, width, height float64) ([]pdfSource, error) {
	page := window.Page()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Printing maps 96 CSS pixels to 72 points. Match the measured screen width
	// and leave room for the print backend's paper-height rounding.
	if _, err := page.EvalContext(ctx, `
document.head.appendChild(Object.assign(document.createElement("style"), {
  id: "pdf-long-print", media: "print",
  textContent: "html { zoom: 1.333333333333 !important; }"
}));`); err != nil {
		return nil, err
	}
	data, err := printPDFPage(page, mygo.PDFOptions{
		PageSize:   mygo.PageSize{Width: width / 72, Height: (height + 12) / 72},
		Background: true,
		Margins:    &mygo.Margins{},
	})
	if err != nil {
		return nil, err
	}
	return []pdfSource{{Height: height, Data: data}}, nil
}
