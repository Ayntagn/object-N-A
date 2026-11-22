function takeBytes(id) {
  const element = document.getElementById(id);
  const encoded = element.textContent;
  element.remove();
  if (typeof Uint8Array.fromBase64 === "function") return Uint8Array.fromBase64(encoded);
  const decoded = atob(encoded);
  const bytes = new Uint8Array(decoded.length);
  for (let i = 0; i < decoded.length; i++) bytes[i] = decoded.charCodeAt(i);
  return bytes;
}

function moduleURL(id) {
  return URL.createObjectURL(new Blob([takeBytes(id)], { type: "text/javascript" }));
}

function pdfPageViewport(page) {
  const bounds = page.getViewport({ scale: 1 });
  return page.getViewport({ scale: maxSide / Math.max(bounds.width, bounds.height) });
}

function makePDFThumbnailCards(pages, count, first) {
  const dimensions = pdfPageViewport(first);
  const fragment = document.createDocumentFragment(), cards = [];
  for (let number = 1; number <= count; number++) {
    const image = new Image();
    image.alt = `Page ${number}`;
    image.className = "thumbnail-placeholder";
    sizePDFThumbnail(image, Math.floor(dimensions.width), Math.floor(dimensions.height));
    const label = document.createElement("span");
    label.className = "page-number";
    label.textContent = String(number);
    const element = document.createElement("div");
    element.className = "page-card";
    element.append(image, label);
    fragment.append(element);
    cards.push({ element, image, number });
  }
  pages.append(fragment);
  return cards;
}

async function renderPDFThumbnail(pdf, card, signal) {
  const page = await pdf.getPage(card.number);
  const canvas = document.createElement("canvas");
  let render, objectURL;
  const cancel = () => render?.cancel();
  try {
    if (signal.aborted) throw signal.reason;
    const viewport = pdfPageViewport(page);
    canvas.width = Math.max(1, Math.floor(viewport.width));
    canvas.height = Math.max(1, Math.floor(viewport.height));
    sizePDFThumbnail(card.image, canvas.width, canvas.height);
    // These are static thumbnails of printed pages. Print intent keeps
    // rendering when the hosting window is hidden or minimized.
    render = page.render({ canvas, viewport, intent: "print", background: "#fff" });
    signal.addEventListener("abort", cancel, { once: true });
    await render.promise;
    if (signal.aborted) throw signal.reason;
    const thumbnail = await new Promise(resolve => canvas.toBlob(resolve, "image/jpeg", 1));
    if (!thumbnail) throw new Error("Could not encode thumbnail");
    if (signal.aborted) throw signal.reason;
    objectURL = URL.createObjectURL(thumbnail);
    await loadPDFThumbnailImage(card.image, objectURL, signal);
    return () => {
      resetPDFThumbnail(card.image);
      URL.revokeObjectURL(objectURL);
    };
  } catch (error) {
    if (objectURL) URL.revokeObjectURL(objectURL);
    throw error;
  } finally {
    signal.removeEventListener("abort", cancel);
    canvas.width = canvas.height = 0;
    page.cleanup();
  }
}

const pages = document.getElementById("pages");
const status = document.getElementById("status");
const apiURL = moduleURL("pdf-js");
const workerURL = moduleURL("pdf-worker");
let task, closed = false;
function closePDF() {
  if (closed) return;
  closed = true;
  void task?.destroy().catch(() => {});
  URL.revokeObjectURL(apiURL);
  URL.revokeObjectURL(workerURL);
}
window.addEventListener("pagehide", closePDF, { once: true });
try {
  // A file-origin grid uses the bundled worker module in PDF.js's local
  // loopback transport. This also works where file-origin module workers
  // cannot attach to the document.
  await import(workerURL);
  const pdfjs = await import(apiURL);
  if (closed) throw new Error("PDF grid closed");
  task = pdfjs.getDocument({ data: takeBytes("pdf-bytes"), useWasm: false, useWorkerFetch: false,
    disableWorkerRendering: true, enableHWA: false, isEvalSupported: false });
  const pdf = await task.promise;
  const count = Math.min(pdf.numPages, pageLimit);
  if (!count) throw new Error("PDF preview has no pages");
  const first = await pdf.getPage(1);
  if (closed) throw new Error("PDF grid closed");
  const cards = makePDFThumbnailCards(pages, count, first);
  status.hidden = true;
  watchPDFThumbnailWindow(cards, (card, signal) => renderPDFThumbnail(pdf, card, signal));
} catch (error) {
  if (!closed) {
    console.error(error);
    status.textContent = "Could not create PDF thumbnails.";
    status.hidden = false;
    closePDF();
  }
}
