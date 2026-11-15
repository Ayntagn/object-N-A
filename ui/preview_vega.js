
  const vegaSelector = "[data-vega-preview]";
  const vegaViews = new WeakMap();
  let vegaFormatsReady = false;

  function vegaKey(node) {
    return JSON.stringify([node.className, node.firstElementChild.textContent]);
  }

  function releaseVegaChart(node) {
    vegaViews.get(node)?.finalize();
    vegaViews.delete(node);
  }

  function vegaConfig() {
    const body = getComputedStyle(document.body);
    const ink = body.getPropertyValue("--pdf-chart-ink").trim() || body.color;
    return {
      background: "transparent", view: { stroke: "transparent" },
      title: { color: ink, subtitleColor: ink },
      axis: { labelColor: ink, titleColor: ink, gridColor: "rgba(128,128,128,.25)",
        domainColor: "rgba(128,128,128,.5)", tickColor: "rgba(128,128,128,.5)" },
      legend: { labelColor: ink, titleColor: ink },
    };
  }

  function vegaDataLoader(options) {
    const loader = window.vega.loader({ http: { credentials: "omit", referrerPolicy: "no-referrer" } });
    const load = loader.load.bind(loader);
    loader.load = async (source, options) => {
      const raw = source.trim();
      const url = new URL(raw.startsWith("//") ? "https:" + raw : raw);
      if (!["http:", "https:", "data:"].includes(url.protocol) || url.username || url.password) {
        throw new Error("Unavailable chart data source");
      }
      return load(url.href, options);
    };
    if (options.blockLinks) {
      const sanitize = loader.sanitize.bind(loader);
      loader.sanitize = async (source, request) => {
        if (request?.context === "href") throw new Error("Chart links are disabled here");
        return sanitize(source, request);
      };
    }
    return loader;
  }

  // D3's object converter uses dynamic code generation. Read rows and give
  // Vega the same columns and string cells for its own type inference.
  function delimitedData(delimiter) {
    const read = (text, format) => {
      const parser = window.d3.dsvFormat(delimiter ?? format.delimiter);
      const rows = parser.parseRows(String(text).replace(/^\uFEFF/, ""));
      const columns = format?.header ?? rows.shift() ?? [];
      const values = rows.map(row => Object.fromEntries(columns.map((name, index) => [name, row[index] ?? ""])));
      values.columns = columns;
      return values;
    };
    read.responseType = "text";
    return read;
  }

  function vegaError(node, message, options, invalidJSON = false) {
    node.dataset.vegaDone = "error";
    const status = document.createElement("pre");
    status.className = "vega-error";
    status.setAttribute("role", "status");
    status.textContent = options.errorText?.(message) ?? "Vega: " + (invalidJSON ? "invalid JSON - " : "") + message;
    node.append(status);
  }

  async function renderVegaChart(node, embed, options) {
    node.dataset.vegaDone = "pending";
    let spec;
    try {
      spec = JSON.parse(node.firstElementChild.textContent);
    } catch (error) {
      vegaError(node, error.message, options, true);
      return;
    }
    if (!spec || typeof spec !== "object" || Array.isArray(spec)) {
      vegaError(node, "expected a chart specification object", options);
      return;
    }
    let mode = node.classList.contains("vega") ? "vega" : "vega-lite";
    if (typeof spec.$schema === "string") {
      if (spec.$schema.includes("vega-lite")) mode = "vega-lite";
      else if (spec.$schema.includes("/vega/")) mode = "vega";
    }
    if (mode === "vega-lite" && spec.width === undefined &&
      !spec.facet && !spec.repeat && !spec.hconcat && !spec.vconcat && !spec.concat) {
      spec.width = "container";
    }
    // Embed options belong to the preview host, while the note supplies the spec.
    if (spec.usermeta && typeof spec.usermeta === "object") delete spec.usermeta.embedOptions;
    const stage = document.createElement("div");
    stage.className = "vega-stage";
    node.append(stage);
    try {
      const result = await embed(stage, spec, {
        actions: false, mode, renderer: "svg", config: vegaConfig(), ast: true, loader: vegaDataLoader(options),
      });
      if (!node.isConnected) {
        result.finalize();
        return;
      }
      vegaViews.set(node, result);
      node.dataset.vegaDone = "rendered";
    } catch (error) {
      stage.remove();
      if (node.isConnected) vegaError(node, error?.message || String(error), options);
    }
  }

  async function renderVegaCharts(preview, options = {}) {
    const embed = window.vegaEmbed;
    const vega = window.vega;
    if (!preview || typeof embed !== "function" || typeof vega?.parse !== "function" ||
      typeof window.d3?.dsvFormat !== "function") return;
    if (!vegaFormatsReady) {
      vega.formats("csv", delimitedData(","));
      vega.formats("tsv", delimitedData("\t"));
      vega.formats("dsv", delimitedData());
      vegaFormatsReady = true;
    }
    const pending = preview.querySelectorAll(vegaSelector + ":not([data-vega-done])");
    await Promise.all(Array.from(pending, node => renderVegaChart(node, embed, options)));
  }

  window.addEventListener("cyberwriter:vega-ready", () =>
    renderVegaCharts(document.querySelector("main, article.markdown-body")));
