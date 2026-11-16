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
