function sizePDFThumbnail(image, width, height) {
  const pixelsWide = Math.max(1, width), pixelsHigh = Math.max(1, height);
  image.width = pixelsWide;
  image.height = pixelsHigh;
  image.style.setProperty("--thumbnail-width", String(pixelsWide));
  image.style.setProperty("--thumbnail-height", String(pixelsHigh));
}

function resetPDFThumbnail(image) {
  image.classList.add("thumbnail-placeholder");
  image.removeAttribute("src");
}

function loadPDFThumbnailImage(image, source, signal) {
  return new Promise((resolve, reject) => {
    const loaded = () => settle();
    const failed = () => settle(new Error("Could not load thumbnail"));
    const canceled = () => settle(signal.reason ?? new DOMException("Thumbnail canceled", "AbortError"));
    function settle(error) {
      image.removeEventListener("load", loaded);
      image.removeEventListener("error", failed);
      signal.removeEventListener("abort", canceled);
      if (error) {
        resetPDFThumbnail(image);
        reject(error);
      } else {
        image.classList.remove("thumbnail-placeholder");
        resolve();
      }
    }
    if (signal.aborted) {
      canceled();
      return;
    }
    image.addEventListener("load", loaded);
    image.addEventListener("error", failed);
    signal.addEventListener("abort", canceled, { once: true });
    image.src = source;
  });
}

function pdfThumbnailCost(image) {
  return Number(image.getAttribute("width")) * Number(image.getAttribute("height")) * 4;
}

function pdfThumbnailWindow(cards) {
  const viewport = window.visualViewport;
  const top = viewport?.offsetTop ?? 0;
  const height = viewport?.height || window.innerHeight;
  const bottom = top + height;
  const margin = Math.min(400, height / 2);
  // Cards are in grid row order. Find the first candidate without measuring
  // the hundreds of cards above the viewport on every scroll event.
  let start = 0, end = cards.length;
  while (start < end) {
    const mid = (start + end) >>> 1;
    if (cards[mid].element.getBoundingClientRect().bottom < top - margin) start = mid + 1;
    else end = mid;
  }
  const windowCards = [];
  for (let at = start; at < cards.length; at++) {
    const card = cards[at];
    const rect = card.element.getBoundingClientRect();
    if (rect.top > bottom + margin) break;
    const distance = Math.max(0, top - rect.bottom, rect.top - bottom);
    windowCards.push({ card, distance });
  }
  // Visible pages come first, followed by the nearest offscreen pages.
  windowCards.sort((a, b) => a.distance - b.distance || a.card.number - b.card.number);
  let cost = 0;
  return windowCards.filter(item => {
    const next = cost + pdfThumbnailCost(item.card.image);
    if (item.distance > 0 && next > thumbnailBudget) return false;
    cost = next;
    return true;
  });
}

function watchPDFThumbnailWindow(cards, render) {
  const resident = new Map();
  const failed = new Map();
  let visible = new Set();
  let bytes = 0, timer = 0, active = null, closed = false;

  function release(card, raster) {
    bytes -= raster.bytes;
    resident.delete(card);
    raster.release();
  }
  function schedule() {
    if (!closed && !timer) timer = setTimeout(reconcile, 0);
  }
  function reconcile() {
    timer = 0;
    if (closed) return;
    const windowCards = pdfThumbnailWindow(cards);
    const wanted = new Set(), nowVisible = new Set();
    for (const { card, distance } of windowCards) {
      wanted.add(card);
      if (distance === 0) nowVisible.add(card);
      const raster = resident.get(card);
      if (raster) {
        resident.delete(card);
        resident.set(card, raster);
      }
    }
    // A failed page becomes eligible on re-entry, even if it never left the preload window.
    for (const card of visible) {
      const failure = failed.get(card);
      if (failure && !nowVisible.has(card)) failure.retryOnEntry = true;
    }
    visible = nowVisible;
    for (const [card, raster] of resident) {
      if (bytes <= thumbnailBudget) break;
      if (!wanted.has(card)) release(card, raster);
    }
    const next = windowCards.find(({ card, distance }) => {
      if (resident.has(card)) return false;
      const failure = failed.get(card);
      return !failure || (distance === 0 && failure.retryOnEntry && !failure.exhausted);
    });
    if (active) {
      const current = windowCards.find(item => item.card === active.card);
      if (!current || (next?.distance === 0 && current.distance > 0)) active.controller.abort();
      return;
    }
    if (next) void request(next.card);
  }
  async function request(card) {
    const controller = new AbortController();
    const failure = failed.get(card);
    active = { card, controller };
    try {
      const dispose = await render(card, controller.signal);
      if (closed || controller.signal.aborted) {
        dispose();
      } else {
        if (failure) {
          failure.label.remove();
          failed.delete(card);
        }
        const weight = pdfThumbnailCost(card.image);
        resident.set(card, { bytes: weight, release: dispose });
        bytes += weight;
      }
    } catch (error) {
      if (!closed && !controller.signal.aborted) {
        if (failure) {
          failure.exhausted = true;
        } else {
          const label = document.createElement("span");
          label.className = "thumbnail-error";
          label.setAttribute("role", "status");
          label.textContent = "Thumbnail unavailable";
          card.element.append(label);
          failed.set(card, { label, retryOnEntry: !visible.has(card), exhausted: false });
        }
        console.warn(`Could not create thumbnail for page ${card.number}`, error);
      }
    } finally {
      active = null;
      schedule();
    }
  }
  window.addEventListener("scroll", schedule, { passive: true });
  window.addEventListener("resize", schedule, { passive: true });
  window.visualViewport?.addEventListener("scroll", schedule, { passive: true });
  window.visualViewport?.addEventListener("resize", schedule, { passive: true });
  window.addEventListener("pagehide", () => {
    closed = true;
    clearTimeout(timer);
    active?.controller.abort();
    for (const [card, raster] of resident) release(card, raster);
  }, { once: true });
  globalThis.refreshPDFThumbnailWindow = schedule;
  schedule();
}
