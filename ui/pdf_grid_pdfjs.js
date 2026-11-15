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

async function appendPage(pdf, pageNumber, pages) {
  const page = await pdf.getPage(pageNumber);
  const canvas = document.createElement("canvas");
  try {
    const bounds = page.getViewport({ scale: 1 });
    const viewport = page.getViewport({ scale: maxSide / Math.max(bounds.width, bounds.height) });
    canvas.width = Math.max(1, Math.floor(viewport.width));
    canvas.height = Math.max(1, Math.floor(viewport.height));
    // These are static thumbnails of printed pages. Print intent keeps
    // rendering when the hosting window is hidden or minimized.
    await page.render({ canvas, viewport, intent: "print", background: "#fff" }).promise;
    const thumbnail = await new Promise(resolve => canvas.toBlob(resolve, "image/jpeg", 1));
    if (!thumbnail) return;
    const number = pages.childElementCount + 1;
    const image = new Image();
    image.alt = `Page ${number}`;
    image.width = canvas.width;
    image.height = canvas.height;
    image.src = URL.createObjectURL(thumbnail);
    const label = document.createElement("span");
    label.className = "page-number";
    label.textContent = String(number);
    const card = document.createElement("div");
    card.className = "page-card";
    card.append(image, label);
    pages.append(card);
  } finally {
    canvas.width = canvas.height = 0;
    page.cleanup();
  }
}

const pages = document.getElementById("pages");
const status = document.getElementById("status");
const apiURL = moduleURL("pdf-js");
const workerURL = moduleURL("pdf-worker");
let task;
try {
  // A file-origin grid uses the bundled worker module in PDF.js's local
  // loopback transport. This also works where file-origin module workers
  // cannot attach to the document.
  await import(workerURL);
  const pdfjs = await import(apiURL);
  task = pdfjs.getDocument({ data: takeBytes("pdf-bytes"), useWasm: false, useWorkerFetch: false,
    disableWorkerRendering: true, enableHWA: false, isEvalSupported: false });
  const pdf = await task.promise;
  for (let i = 1; i <= Math.min(pdf.numPages, pageLimit); i++) {
    try {
      await appendPage(pdf, i, pages);
    } catch (error) {
      console.warn(`Could not create thumbnail for page ${i}`, error);
    }
  }
  if (pages.childElementCount) status.remove();
  else status.textContent = "Could not create PDF thumbnails.";
} catch (error) {
  console.error(error);
  status.textContent = "Could not create PDF thumbnails.";
} finally {
  await task?.destroy();
  URL.revokeObjectURL(apiURL);
  URL.revokeObjectURL(workerURL);
}
