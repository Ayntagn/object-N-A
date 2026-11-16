package ui

import (
	"context"
	"fmt"
	"io"
)

func writeAIReferencePDFPage(ctx context.Context, output io.Writer, pdf []byte) error {
	if _, err := fmt.Fprintf(output, `<!doctype html><meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="%s">`, pdfContentPolicy(aiReferencePDFJS)); err != nil {
		return err
	}
	if err := writePortablePDFBlobs(ctx, output, pdf); err != nil {
		return err
	}
	// Register the extraction function before the host receives the load event.
	_, err := io.WriteString(output, `<script>`+aiReferencePDFJS+`</script>`)
	return err
}

const aiReferencePDFJS = `
function pdfBytes(id) {
  const element = document.getElementById(id);
  const encoded = element.textContent;
  element.remove();
  if (typeof Uint8Array.fromBase64 === "function") return Uint8Array.fromBase64(encoded);
  const decoded = atob(encoded);
  const bytes = new Uint8Array(decoded.length);
  for (let i = 0; i < decoded.length; i++) bytes[i] = decoded.charCodeAt(i);
  return bytes;
}

function pdfModule(id) {
  return URL.createObjectURL(new Blob([pdfBytes(id)], { type: "text/javascript" }));
}

window.extractAIReferencePDFPages = async function() {
  const apiURL = pdfModule("pdf-js");
  const workerURL = pdfModule("pdf-worker");
  let task;
  try {
    // Use the same file-origin worker transport as the PDF Grid.
    await import(workerURL);
    const pdfjs = await import(apiURL);
    task = pdfjs.getDocument({ data: pdfBytes("pdf-bytes"), useWasm: false, useWorkerFetch: false,
      disableWorkerRendering: true, enableHWA: false, isEvalSupported: false });
    const doc = await task.promise;
    const pages = [];
    for (let index = 1; index <= doc.numPages; index++) {
      const page = await doc.getPage(index);
      try {
        const { items } = await page.getTextContent({ disableNormalization: true });
        const textItems = items.filter(item => typeof item.str === "string");
        if (textItems.length) {
          pages.push(textItems.map(item => item.str + (item.hasEOL ? "\n" : "")).join(""));
        }
      } finally {
        page.cleanup();
      }
    }
    return pages;
  } finally {
    await task?.destroy();
    URL.revokeObjectURL(apiURL);
    URL.revokeObjectURL(workerURL);
  }
};
`
