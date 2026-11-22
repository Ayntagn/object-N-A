package ui

import (
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

//go:embed webassets/pdfjs/pdf.min.mjs.gz
var pdfGridAPI []byte

//go:embed webassets/pdfjs/pdf.worker.min.mjs.gz
var pdfGridWorker []byte

//go:embed pdf_grid_pdfjs.js
var pdfGridModule string

func renderPortablePDFGrid(ctx context.Context, w io.Writer, pdf []byte, style pdfThumbnailStyle) error {
	script := fmt.Sprintf("const pageLimit = %d, maxSide = %d, thumbnailBudget = %d;\n", pdfGridMaxPages, style.maxSide(), pdfGridCacheBytes) +
		pdfGridViewport + "\n" + pdfGridModule
	if err := writePDFGridHeader(w, script, style, ""); err != nil {
		return err
	}
	if _, err := io.WriteString(w, `<main id="pages" aria-label="PDF pages"></main><p id="status" role="status">Rendering pages…</p>`); err != nil {
		return err
	}
	if err := writePortablePDFBlobs(ctx, w, pdf); err != nil {
		return err
	}
	_, err := io.Copy(w, strings.NewReader(`<script type="module">`+script+`</script></body></html>`))
	return err
}

func writePortablePDFBlobs(ctx context.Context, w io.Writer, pdf []byte) error {
	if err := writePDFGridBlob(w, "pdf-bytes", bytes.NewReader(pdf)); err != nil {
		return err
	}
	for _, module := range []struct {
		name string
		gzip []byte
	}{{"pdf-js", pdfGridAPI}, {"pdf-worker", pdfGridWorker}} {
		if err := ctx.Err(); err != nil {
			return err
		}
		reader, err := gzip.NewReader(bytes.NewReader(module.gzip))
		if err != nil {
			return err
		}
		err = errors.Join(writePDFGridBlob(w, module.name, reader), reader.Close())
		if err != nil {
			return err
		}
	}
	return nil
}

func writePDFGridBlob(w io.Writer, id string, data io.Reader) error {
	if _, err := fmt.Fprintf(w, `<script type="application/octet-stream" id="%s">`, id); err != nil {
		return err
	}
	encoded := base64.NewEncoder(base64.StdEncoding, w)
	_, copyErr := io.Copy(encoded, data)
	if err := errors.Join(copyErr, encoded.Close()); err != nil {
		return err
	}
	_, err := io.WriteString(w, `</script>`)
	return err
}
