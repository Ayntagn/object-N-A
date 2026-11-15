package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo"
)

const pdfMaximumViewportHeight = 100000

type pdfGeometry struct {
	Height   float64
	Manual   []float64
	Ordinary []float64
	Links    []pdfLink
}

type pdfLink struct {
	URL         string    `json:"url,omitempty"`
	Destination []float64 `json:"destination,omitempty"`
	Rect        []float64 `json:"rect"`
}

// EV-UI-132: markers occupy zero height in the capture document. Collect cut
// coordinates before the final resize, using screen media.
const pdfMeasureGeometry = `
document.head.appendChild(Object.assign(document.createElement("style"), {
  id: "pdf-capture-markers",
  textContent: ".page-break, div[style*='page-break-after'] {"
    + "height:0!important;max-height:0!important;margin:0!important;padding:0!important;"
    + "border:0!important;overflow:hidden!important;visibility:hidden!important;break-after:auto!important;}"
}));
await document.fonts.ready;
let bottom = 0;
for (const block of document.querySelectorAll("body > *, article.markdown-body > *")) {
  const bounds = block.getBoundingClientRect();
  if (bounds.height > 0) bottom = Math.max(bottom, bounds.bottom + scrollY);
}
const height = Math.max(1, Math.ceil(bottom || document.body.getBoundingClientRect().height) + 1);
const manual = new Set();
for (const block of document.querySelectorAll(".page-break, div[style*='page-break-after']")) {
  manual.add(Math.round(block.getBoundingClientRect().top + scrollY));
}
const ordinary = new Set();
for (const block of document.querySelectorAll("h1,h2,h3,h4,h5,h6,p,pre,blockquote,table,ul,ol,hr,.markdown-body > *")) {
  const {top, bottom} = block.getBoundingClientRect();
  if (top + scrollY > 0) ordinary.add(Math.round(top + scrollY));
  ordinary.add(Math.round(bottom + scrollY));
}
return JSON.stringify({height, manual: [...manual], ordinary: [...ordinary]});
`

const pdfMeasureLinks = `
await new Promise(resolve => setTimeout(resolve, 100));
await document.fonts.ready;
const links = [];
for (const anchor of document.querySelectorAll("article a[href]")) {
  const url = anchor.getAttribute("href");
  let target;
  if (url.startsWith("#")) {
    let id;
    try { id = decodeURIComponent(url.slice(1)); } catch (_) { continue; }
    const element = document.getElementById(id);
    if (!element) continue;
    const bounds = element.getBoundingClientRect();
    target = [bounds.left + scrollX, bounds.top + scrollY];
  }
  for (const box of anchor.getClientRects()) {
    if (box.width > 0 && box.height > 0)
      links.push({url, destination: target, rect: [box.left + scrollX, box.top + scrollY, box.right + scrollX, box.bottom + scrollY]});
  }
}
return JSON.stringify(links);
`

func measurePDFGeometry(window *mygo.Window, width, paperContentHeight int) (pdfGeometry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	page := window.Page()
	var geometry pdfGeometry
	if err := decodePDFScript(ctx, page, pdfMeasureGeometry, &geometry); err != nil {
		return geometry, fmt.Errorf("measure PDF content: %w", err)
	}
	if geometry.Height <= 0 || math.IsNaN(geometry.Height) || math.IsInf(geometry.Height, 0) {
		return geometry, errors.New("the WebView reported an invalid PDF height")
	}
	window.SetContentSize(width, max(paperContentHeight, int(min(geometry.Height, pdfMaximumViewportHeight))))
	if err := decodePDFScript(ctx, page, pdfMeasureLinks, &geometry.Links); err != nil {
		return geometry, fmt.Errorf("measure PDF links: %w", err)
	}
	links := geometry.Links[:0]
	for _, link := range geometry.Links {
		if len(link.Destination) == 2 {
			link.URL = ""
		} else if link.URL = previewExternalURL(link.URL); link.URL == "" {
			continue
		}
		links = append(links, link)
	}
	geometry.Links = links
	return geometry, nil
}

func decodePDFScript(ctx context.Context, page *mygo.Page, script string, result any) error {
	value, err := page.EvalContext(ctx, script)
	if err != nil {
		return err
	}
	encoded, ok := value.(string)
	if !ok {
		return fmt.Errorf("unexpected WebView result %T", value)
	}
	return json.Unmarshal([]byte(encoded), result)
}
