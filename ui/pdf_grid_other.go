//go:build !darwin

package ui

import (
	"context"
	"io"
)

func renderPlatformPDFGrid(ctx context.Context, w io.Writer, pdf []byte, style pdfThumbnailStyle) error {
	return renderPortablePDFGrid(ctx, w, pdf, style)
}

func nativeMenuSymbol(string) []byte { return nil }
