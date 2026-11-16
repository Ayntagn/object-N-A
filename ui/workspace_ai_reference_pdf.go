package ui

import (
	"context"
	"path/filepath"
	"strings"
)

// aiReferencePDFText extracts selectable text from a PDF selected in the
// attachment picker. Call it from the file-reading worker.
func aiReferencePDFText(ctx context.Context, filename string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	pdf, err := readWorkspacePDF(filename, filepath.Base(filename), nil)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(pdf) == 0 {
		return "", errUnreadableWorkspacePDF
	}
	pages, err := readAIReferencePDFPages(ctx, pdf)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	text := strings.Join(pages, "\n\n---\n\n")
	if strings.TrimSpace(text) == "" {
		return "", nil
	}
	return text, nil
}
