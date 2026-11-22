  // Bound the retained markup copy and keep the direct path for small pages.
  const minPreviewPatchLength = 64 * 1024;
  const maxPreviewPatchLength = 2 * 1024 * 1024;
  const codeCopyPrefix = "cyberwriter-copy-code://block/";
  const codeCopySelector = 'a.preview-code-copy[href^="' + codeCopyPrefix + '"]';
  const codeCopyRevision = /(<a class="preview-code-copy" href="cyberwriter-copy-code:\/\/block\/)\d+(\/\d+")/g;
  let previewBlockKeys = null;

  function previewBlockKey(node) {
    const markup = node.outerHTML ?? ("\0" + node.nodeType + ":" + node.nodeValue);
    return markup.includes(codeCopyPrefix) ? markup.replace(codeCopyRevision, "$1$2") : markup;
  }

  function refreshPreviewCopyLinks(previous, incoming, key) {
    if (!key.includes(codeCopyPrefix)) return true;
    const oldLinks = previous.querySelectorAll(codeCopySelector);
    const nextLinks = incoming.querySelectorAll(codeCopySelector);
    if (oldLinks.length !== nextLinks.length) return false;
    for (let i = 0; i < nextLinks.length; i++) {
      // Copy requests must use the revision registered by the current frame.
      oldLinks[i].setAttribute("href", nextLinks[i].getAttribute("href"));
    }
    return true;
  }

  // All indices refer to the acknowledged body, before this splice.
  function applyPreviewBodyPatch(preview, patch, revision) {
    const {before, start, insert} = patch;
    const end = start + patch.delete;
    if (preview.childNodes.length !== before || !Number.isSafeInteger(start) || start < 0 ||
        !Number.isSafeInteger(end) || end < start || end > before ||
        !Number.isSafeInteger(insert) || insert < 0) return false;

    const template = document.createElement("template");
    if (insert > 0) template.innerHTML = patch.html;
    if (template.content.childNodes.length !== insert) return false;

    const changes = [];
    const queueLine = (anchor, delta) => {
      const line = Number(anchor.getAttribute("data-source-line")) + delta;
      if (!Number.isSafeInteger(line) || line <= 0) return false;
      changes.push([anchor, String(line)]);
      return true;
    };
    for (const shift of patch.shifts || []) {
      const limit = shift.start + shift.count;
      if (!Number.isSafeInteger(shift.start) || shift.start < 0 || !Number.isSafeInteger(limit) ||
          limit <= shift.start || limit > before || (end > start && shift.start < end && limit > start) ||
          !Number.isSafeInteger(shift.delta)) return false;
      for (let index = shift.start; index < limit; index++) {
        const node = preview.childNodes[index];
        if (node.nodeType !== Node.ELEMENT_NODE) return false;
        if (node.hasAttribute("data-source-line") && !queueLine(node, shift.delta)) return false;
        for (const anchor of node.querySelectorAll("[data-source-line]")) {
          if (!queueLine(anchor, shift.delta)) return false;
        }
      }
    }

    const removed = [];
    const charts = [];
    const after = preview.childNodes[end] || null;
    for (let index = start; index < end; index++) {
      const node = preview.childNodes[index];
      removed.push(node);
      if (node.matches?.(chartSelector)) charts.push(node);
      if (node.querySelectorAll) {
        for (const chart of node.querySelectorAll(chartSelector)) charts.push(chart);
      }
    }
    previewBlockKeys = null;
    reusePreviewCharts(charts, template.content);
    for (const node of removed) if (node.parentNode === preview) node.remove();
    if (insert > 0) preview.insertBefore(template.content, after);
    for (const [anchor, line] of changes) {
      anchor.setAttribute("data-source-line", line);
    }
    if (patch.copies > 0) {
      const links = preview.querySelectorAll(codeCopySelector);
      if (links.length !== patch.copies) return false;
      for (let i = 0; i < links.length; i++) {
        links[i].setAttribute("href", codeCopyPrefix + revision + "/" + (i + 1));
      }
    }
    return true;
  }

  function replacePreviewBody(preview, html, reuse) {
    const track = html.length >= minPreviewPatchLength && html.length <= maxPreviewPatchLength;
    const previous = previewBlockKeys;
    previewBlockKeys = null;

    if (!track || !reuse || previous?.length !== preview.childNodes.length) {
      const charts = preview.querySelectorAll(chartSelector);
      if (!track && (!reuse || charts.length === 0)) {
        for (const chart of charts) releaseVegaChart(chart);
        preview.innerHTML = html;
        return;
      }
      const frame = document.createElement("template");
      frame.innerHTML = html;
      if (track) previewBlockKeys = Array.from(frame.content.childNodes, previewBlockKey);
      if (reuse) reusePreviewCharts(charts, frame.content);
      else for (const chart of charts) releaseVegaChart(chart);
      preview.replaceChildren(frame.content);
      return;
    }

    const frame = document.createElement("template");
    frame.innerHTML = html;
    const incoming = Array.from(frame.content.childNodes);
    const next = incoming.map(previewBlockKey);
    const current = Array.from(preview.childNodes);
    let first = 0;
    let oldEnd = previous.length;
    let newEnd = next.length;
    const keep = (oldIndex, newIndex) => {
      if (previous[oldIndex] !== next[newIndex] ||
          !refreshPreviewCopyLinks(current[oldIndex], incoming[newIndex], next[newIndex])) return false;
      incoming[newIndex].remove();
      return true;
    };
    while (first < oldEnd && first < newEnd && keep(first, first)) {
      first++;
    }
    while (first < oldEnd && first < newEnd && keep(oldEnd - 1, newEnd - 1)) {
      oldEnd--;
      newEnd--;
    }

    const removed = current.slice(first, oldEnd);
    const charts = [];
    for (const node of removed) {
      if (node.matches?.(chartSelector)) charts.push(node);
      if (node.querySelectorAll) {
        for (const chart of node.querySelectorAll(chartSelector)) charts.push(chart);
      }
    }
    reusePreviewCharts(charts, frame.content);
    for (const node of removed) {
      if (node.parentNode === preview) node.remove();
    }
    if (first < newEnd) preview.insertBefore(frame.content, current[oldEnd] ?? null);
    previewBlockKeys = next;
  }
