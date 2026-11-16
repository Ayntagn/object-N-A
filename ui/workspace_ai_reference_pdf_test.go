//go:build darwin

// spec: AI reference PDFs retain each selectable page and skip blank pages.
package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestReadAIReferencePDFText(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "reference.pdf")
	if err := os.WriteFile(filename, aiReferencePDFFixture(), 0o600); err != nil {
		t.Fatal(err)
	}
	text, err := aiReferencePDFText(context.Background(), filename)
	if want := "First page\nSecond line\n\n---\n\nThird page"; err != nil || text != want {
		t.Fatalf("PDF page text = %q, %v, want %q", text, err, want)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := aiReferencePDFText(ctx, filename); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled PDF extraction returned %v", err)
	}
}

func aiReferencePDFFixture() []byte {
	stream := func(text string) string {
		return fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(text), text)
	}
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 3 /Kids [3 0 R 4 0 R 5 0 R] /MediaBox [0 0 612 792] /Resources << /Font << /F1 6 0 R >> >> >>",
		"<< /Type /Page /Parent 2 0 R /Contents 7 0 R >>",
		"<< /Type /Page /Parent 2 0 R /Contents 9 0 R >>",
		"<< /Type /Page /Parent 2 0 R /Contents 8 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		stream("BT /F1 14 Tf 50 700 Td (First page) Tj 0 -18 Td (Second line) Tj ET"),
		stream("BT /F1 14 Tf 50 700 Td (Third page) Tj ET"),
		stream(""),
	}
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for index, object := range objects {
		offsets[index] = pdf.Len()
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xref := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return pdf.Bytes()
}
