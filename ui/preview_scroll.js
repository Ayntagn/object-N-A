  let lastSourceFraction = 0;
  let pauseFraction = null;
  let manualUntil = 0;
  let pointerDown = false;
  let sourceLineAnchors = null;

  function invalidatePreviewScrollAnchors() {
    sourceLineAnchors = null;
  }

  function resetPreviewScroll() {
    lastSourceFraction = 0;
    pauseFraction = null;
    manualUntil = 0;
    pointerDown = false;
  }

  function setPreviewScrollPadding(enabled) {
    document.documentElement.toggleAttribute("data-cw-scroll-past-end", enabled);
  }

  function markScrollIntent() {
    manualUntil = performance.now() + 50;
  }

  window.addEventListener("wheel", markScrollIntent, { passive: true });
  window.addEventListener("mousedown", () => { pointerDown = true; });
  window.addEventListener("mouseup", () => { pointerDown = false; });
  window.addEventListener("blur", () => { pointerDown = false; });
  window.addEventListener("scroll", () => {
    if (pointerDown || performance.now() < manualUntil) pauseFraction = lastSourceFraction;
  }, { passive: true });

  function findPreviewSourceLine(targetLine) {
    let nearest = null;
    let nearestLine = 0;
    for (const block of document.querySelectorAll("[data-source-line]")) {
      const line = Number.parseInt(block.dataset.sourceLine, 10);
      if (line > nearestLine && line <= targetLine) {
        nearest = block;
        nearestLine = line;
      }
    }
    return nearest;
  }

  function findCachedPreviewSourceLine(targetLine) {
    if (sourceLineAnchors === null) {
      const byLine = new Map();
      for (const block of document.querySelectorAll("[data-source-line]")) {
        const line = Number.parseInt(block.dataset.sourceLine, 10);
        if (line > 0 && !byLine.has(line)) byLine.set(line, block);
      }
      sourceLineAnchors = Array.from(byLine).sort((a, b) => a[0] - b[0]);
    }
    let low = 0;
    let high = sourceLineAnchors.length;
    while (low < high) {
      const mid = (low + high) >>> 1;
      if (sourceLineAnchors[mid][0] <= targetLine) low = mid + 1;
      else high = mid;
    }
    return sourceLineAnchors[low - 1]?.[1];
  }

  function scrollPreview(target, useCachedSourceLines = false) {
    if (target.line > 0) {
      const nearest = useCachedSourceLines === true
        ? findCachedPreviewSourceLine(target.line)
        : findPreviewSourceLine(target.line);
      nearest?.scrollIntoView({ block: "start", behavior: "instant" });
      return;
    }
    const fraction = target.percentage;
    lastSourceFraction = fraction;
    if (pauseFraction !== null && Math.abs(fraction - pauseFraction) <= 0.03) return;
    pauseFraction = null;

    const root = document.documentElement;
    const padding = root.hasAttribute("data-cw-scroll-past-end") ? window.innerHeight * 0.7 : 0;
    const extent = Math.max(0, root.scrollHeight - root.clientHeight - padding);
    window.scrollTo({ left: 0, top: extent * (fraction >= 0.98 ? 1 : fraction), behavior: "instant" });
  }
