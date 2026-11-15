
  const diagramSelector = "pre.mermaid";

  function diagramSource(node) {
    return node.dataset.mermaidSource ?? node.textContent.trim();
  }

  function diagramKey(node) {
    return JSON.stringify(["mermaid", node.dataset.mermaidBackground, diagramSource(node)]);
  }

  function colorChannels(value) {
    return value.match(/[\d.]+/g)?.map(Number) ?? [0, 0, 0, 0];
  }

  function mixColor(left, right, amount) {
    return left.slice(0, 3).map((channel, i) => Math.round(channel * (1 - amount) + right[i] * amount));
  }

  function hexColor(channels) {
    return "#" + channels.slice(0, 3).map(channel => channel.toString(16).padStart(2, "0")).join("");
  }

  function darkColor([red, green, blue]) {
    return (0.2126 * red + 0.7152 * green + 0.0722 * blue) / 255 < 0.5;
  }

  function diagramTheme(node) {
    const background = node.dataset.mermaidBackground.toLowerCase();
    const transparent = background === "none" || background === "transparent";
    const custom = !transparent && /^(?:#[0-9a-f]{3}(?:[0-9a-f]{3}(?:[0-9a-f]{2})?)?|[a-z]+)$/.test(background)
      && CSS.supports("color", background) && !["inherit", "initial", "unset", "revert", "currentcolor"].includes(background);
    if (custom) node.style.backgroundColor = background;
    const body = getComputedStyle(document.body);
    const canvas = colorChannels(body.getPropertyValue("--pdf-chart-paper").trim() || body.backgroundColor);
    const surface = colorChannels(getComputedStyle(node).backgroundColor);
    const base = transparent ? canvas : mixColor(canvas, surface, surface[3] ?? 1);
    const darkMode = darkColor(base);
    const preferred = colorChannels(body.getPropertyValue("--pdf-chart-ink").trim() || body.color);
    const text = custom || darkColor(preferred) === darkMode
      ? (darkMode ? [232, 232, 234] : [28, 28, 30])
      : preferred;
    const linkProbe = document.createElement("span");
    linkProbe.style.color = "var(--mermaid-link, LinkText)";
    node.append(linkProbe);
    const link = colorChannels(getComputedStyle(linkProbe).color);
    linkProbe.remove();
    if (transparent) node.dataset.mermaidTransparent = "";
    node.style.setProperty("--mermaid-border", hexColor(mixColor(base, text, 0.1)));
    const ink = hexColor(text);
    if (custom) {
      node.dataset.mermaidInk = "";
      node.style.setProperty("--mermaid-ink", ink);
    }
    const line = hexColor(mixColor(base, text, 0.6));
    return {
      background: hexColor(base), darkMode, fontFamily: "-apple-system, BlinkMacSystemFont, sans-serif",
      textColor: ink, primaryTextColor: ink, secondaryTextColor: ink, tertiaryTextColor: ink, noteTextColor: ink,
      primaryColor: hexColor(mixColor(base, text, 0.14)), primaryBorderColor: hexColor(mixColor(base, text, 0.45)),
      secondaryColor: hexColor(mixColor(base, link, 0.18)), secondaryBorderColor: hexColor(mixColor(base, link, 0.45)),
      tertiaryColor: hexColor(mixColor(base, text, 0.07)), tertiaryBorderColor: hexColor(mixColor(base, text, 0.38)),
      noteBkgColor: hexColor(mixColor(base, link, 0.2)), lineColor: line, defaultLinkColor: line,
    };
  }

  let diagramSequence = 0;
  let pendingDiagramPreview;
  let renderingDiagrams = false;

  async function renderDiagrams(preview) {
    if (!preview) return;
    pendingDiagramPreview = preview;
    const engine = window.mermaid;
    if (renderingDiagrams || typeof engine?.render !== "function") return;
    renderingDiagrams = true;
    try {
      while (pendingDiagramPreview) {
        const diagrams = pendingDiagramPreview.querySelectorAll(diagramSelector);
        pendingDiagramPreview = undefined;
        for (const node of diagrams) {
          if (!node.isConnected || node.dataset.mermaidState) continue;
          const source = diagramSource(node);
          node.dataset.mermaidSource = source;
          try {
            engine.initialize({
              startOnLoad: false, securityLevel: "strict", suppressErrorRendering: true, theme: "base",
              // Preserve the measured 11.12.3 geometry with the 12.3.0 engine.
              layout: "dagre", themeVariables: diagramTheme(node), sequence: { useMaxWidth: true },
              flowchart: { useMaxWidth: true, theme: "base", look: "classic", wrappingWidth: 200, minNodeWidth: 0 },
            });
            const { svg } = await engine.render("cw-diagram-" + ++diagramSequence, source);
            if (!node.isConnected) continue;
            node.innerHTML = svg;
            node.dataset.mermaidState = "rendered";
          } catch {
            if (!node.isConnected) continue;
            node.dataset.mermaidState = "error";
            const status = document.createElement("span");
            status.className = "mermaid-error";
            status.setAttribute("role", "status");
            status.textContent = "Invalid Mermaid diagram";
            node.append(status);
          }
        }
      }
    } finally {
      renderingDiagrams = false;
    }
  }

  window.addEventListener("cyberwriter:mermaid-ready", () =>
    renderDiagrams(document.querySelector("main, article.markdown-body")));
