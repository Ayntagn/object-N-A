const pages = document.getElementById("pages");
const status = document.getElementById("status");
const failure = document.getElementById("failure");
const activePages = new Set();
const zoomStep = 2 ** 0.25;
let pdfTask;
let pdf;
let pdfjs;
let observer;
let sizeObserver;
let resizeTimer;
let fitSpreadWidth;
let manualScale = null;
let closed = false;

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

async function importBundledModule(id) {
  const url = URL.createObjectURL(new Blob([takeBytes(id)], { type: "text/javascript" }));
  try {
    return await import(url);
  } finally {
    URL.revokeObjectURL(url);
  }
}

function clearPage(element) {
  const drawing = element.drawing;
  if (!drawing) return;
  element.drawing = null;
  drawing.task?.cancel();
  drawing.text?.cancel();
  for (const canvas of element.querySelectorAll("canvas")) canvas.width = canvas.height = 0;
  element.replaceChildren();
}

function renderPage(element) {
  const width = element.getBoundingClientRect().width;
  if (!width || element.drawing?.width === width) return;
  clearPage(element);
  const drawing = { width };
  element.drawing = drawing;
  drawPage(element, drawing);
}

async function drawPage(element, drawing) {
  try {
    const page = await pdf.getPage(Number(element.dataset.page));
    if (element.drawing !== drawing || closed) return;
    const base = page.getViewport({ scale: 1 });
    if (!(base.width > 0 && base.height > 0)) throw new Error("Invalid PDF page size");
    element.style.setProperty("--actual-width", base.width);
    drawing.width = element.getBoundingClientRect().width;
    const viewport = page.getViewport({ scale: drawing.width / base.width });
    element.style.setProperty("--page-aspect", viewport.width / viewport.height);

    const canvas = document.createElement("canvas");
    canvas.setAttribute("aria-hidden", "true");
    const deviceScale = window.devicePixelRatio || 1;
    // Match the platform's pixel density within a bounded canvas allocation.
    const pixelScale = Math.min(deviceScale, 4096 / viewport.width, 4096 / viewport.height);
    canvas.width = Math.max(1, Math.floor(viewport.width * pixelScale));
    canvas.height = Math.max(1, Math.floor(viewport.height * pixelScale));
    element.append(canvas);
    drawing.task = page.render({
      canvas, viewport,
      transform: [canvas.width / viewport.width, 0, 0, canvas.height / viewport.height, 0, 0],
      background: "#fff",
    });
    await drawing.task.promise;
    drawing.task = null;
    if (element.drawing !== drawing || closed) return;

    const text = document.createElement("div");
    text.className = "text-layer";
    text.style.setProperty("--total-scale-factor", viewport.scale * viewport.userUnit);
    element.append(text);
    drawing.text = new pdfjs.TextLayer({ textContentSource: page.streamTextContent(), container: text, viewport });
    try {
      await drawing.text.render();
    } catch {
      text.remove();
    } finally {
      drawing.text = null;
    }
    if (element.drawing === drawing) page.cleanup();
  } catch {
    if (element.drawing !== drawing || closed) return;
    element.replaceChildren();
    const message = document.createElement("p");
    message.textContent = "Could not display this page.";
    message.style.cssText = "text-align:center;color:#333";
    element.append(message);
  }
}

function resizePages() {
  clearTimeout(resizeTimer);
  resizeTimer = setTimeout(() => {
    for (const element of activePages) renderPage(element);
  }, 80);
}

window.scrollAttachmentPDFPage = () => {
  const number = window.attachmentPDFPage;
  if (closed || !Number.isSafeInteger(number) || number <= 0) return;
  pages.children[number - 1]?.scrollIntoView({ block: "start", inline: "nearest" });
};

window.adjustAttachmentPDFZoom = action => {
  if (closed || !fitSpreadWidth) return;
  const scale = manualScale ?? pages.clientWidth / fitSpreadWidth;
  let nextScale;
  switch (action) {
    case "in":
      nextScale = Math.max(scale, Math.min(scale * zoomStep, 8));
      break;
    case "out":
      nextScale = Math.min(scale, Math.max(scale / zoomStep, 0.125));
      break;
    case "actual":
      nextScale = 1;
      break;
    case "fit":
      nextScale = null;
      break;
    default:
      return;
  }
  if (nextScale === manualScale) return;

  const bounds = pages.getBoundingClientRect();
  const centerX = bounds.left + pages.clientWidth / 2;
  const centerY = bounds.top + pages.clientHeight / 2;
  let anchor;
  let original;
  let nearest = Infinity;
  for (const element of activePages) {
    const rect = element.getBoundingClientRect();
    const distance = Math.max(rect.top - centerY, centerY - rect.bottom, 0);
    if (distance < nearest) {
      nearest = distance;
      anchor = element;
      original = rect;
    }
  }
  manualScale = nextScale;
  pages.classList.toggle("zoomed", nextScale !== null);
  if (nextScale === null) {
    pages.style.removeProperty("--manual-scale");
  } else {
    pages.style.setProperty("--manual-scale", `${nextScale}px`);
  }
  if (anchor && original.width > 0 && original.height > 0) {
    const current = anchor.getBoundingClientRect();
    pages.scrollLeft += current.left + (centerX - original.left) * current.width / original.width - centerX;
    pages.scrollTop += current.top + (centerY - original.top) * current.height / original.height - centerY;
  }
  resizePages();
};

function stop() {
  if (closed) return;
  closed = true;
  clearTimeout(resizeTimer);
  observer?.disconnect();
  sizeObserver?.disconnect();
  for (const element of activePages) clearPage(element);
  activePages.clear();
  pdfTask?.destroy().catch(() => {});
}

window.addEventListener("pagehide", stop, { once: true });

try {
  // The worker module registers PDF.js's local loopback transport.
  await importBundledModule("pdf-worker");
  pdfjs = await importBundledModule("pdf-js");
  if (!closed) {
    pdfTask = pdfjs.getDocument({
      data: takeBytes("pdf-bytes"), useWasm: false, useWorkerFetch: false,
      disableWorkerRendering: true, enableHWA: false,
    });
    pdf = await pdfTask.promise;
  }
  if (!closed) {
    if (!pdf.numPages) throw new Error("PDF has no pages");
    const first = (await pdf.getPage(1)).getViewport({ scale: 1 });
    if (!(first.width > 0 && first.height > 0)) throw new Error("Invalid PDF page size");

    // Start with the first page's dimensions. Visible pages with a different
    // orientation or size update their own aspect ratio after being read.
    fitSpreadWidth = first.width + 8;
    pages.style.setProperty("--page-width", first.width);
    pages.style.setProperty("--spread-width", fitSpreadWidth);
    pages.style.setProperty("--page-aspect", first.width / first.height);

    const fragment = document.createDocumentFragment();
    for (let number = 1; number <= pdf.numPages; number++) {
      const element = document.createElement("section");
      element.className = "pdf-page";
      element.dataset.page = number;
      element.setAttribute("aria-label", "Page " + number);
      fragment.append(element);
    }
    pages.append(fragment);
    window.scrollAttachmentPDFPage();
    pages.removeAttribute("aria-busy");
    status.remove();
    pages.focus({ preventScroll: true });

    observer = new IntersectionObserver(entries => {
      for (const entry of entries) {
        const element = entry.target;
        if (entry.isIntersecting) {
          activePages.add(element);
          renderPage(element);
        } else {
          activePages.delete(element);
          clearPage(element);
        }
      }
    }, { root: pages, rootMargin: "40% 0px" });
    for (const element of pages.children) observer.observe(element);
    sizeObserver = new ResizeObserver(resizePages);
    sizeObserver.observe(pages);
  }
} catch {
  if (!closed) {
    stop();
    pages.hidden = true;
    status.remove();
    failure.hidden = false;
  }
}
