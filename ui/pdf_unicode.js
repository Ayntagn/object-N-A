// WebKit emits a ToUnicode CMap for each subset font. Its generated maps
// use explicit single-code destinations for the CJK fallback fonts.
function pdfUnicodeMaps(api, document) {
  const byStream = new Map();
  const name = api.PDFName.of;
  for (const [, font] of document.context.enumerateIndirectObjects()) {
    if (!(font instanceof api.PDFDict)) continue;
    const reference = font.get(name("ToUnicode"));
    if (!(reference instanceof api.PDFRef)) continue;
    const base = font.lookupMaybe(name("BaseFont"), api.PDFName);
    if (!base) continue;
    const family = base.decodeText().replace(/^[A-Z]{6}\+/, "");
    const key = reference.toString();
    if (!byStream.has(key)) byStream.set(key, {reference, families: new Set()});
    byStream.get(key).families.add(family);
  }
  const maps = [];
  for (const {reference, families} of byStream.values()) {
    if (families.size !== 1) continue;
    const stream = document.context.lookupMaybe(reference, api.PDFRawStream);
    if (!stream) continue;
    const text = new TextDecoder().decode(api.decodePDFRawStream(stream).decode());
    maps.push({reference, family: [...families][0], text});
  }
  return maps;
}

function pdfUnicodeDestinations(text, visit) {
  return text.replace(/beginbf(char|range)\b([\s\S]*?)endbf\1\b/g, (block, kind, entries) => {
    const rules = kind === "char"
      ? /(<[0-9a-f]+>\s*)<([0-9a-f]{4})>/gi
      : /(<([0-9a-f]+)>\s*<([0-9a-f]+)>\s*)<([0-9a-f]{4})>/gi;
    const updated = entries.replace(rules, (match, prefix, first, last, destination) => {
      if (kind === "range" && first.toLowerCase() !== last.toLowerCase()) return match;
      const code = parseInt(kind === "range" ? destination : first, 16);
      if (code < 0x2e80 || code > 0x2fdf) return match;
      const replacement = visit(code);
      return replacement ? prefix + "<" + replacement.toString(16).padStart(4, "0") + ">" : match;
    });
    return "beginbf" + kind + updated + "endbf" + kind;
  });
}

function pdfUnicodeRadicals(api, documents) {
  const families = new Map();
  for (const {input} of documents) {
    for (const {family, text} of pdfUnicodeMaps(api, input)) {
      pdfUnicodeDestinations(text, code => {
        if (!families.has(family)) families.set(family, new Set());
        families.get(family).add(code);
      });
    }
  }
  return Object.fromEntries(Array.from(families, ([family, codes]) => [family, [...codes]]));
}

function restorePDFUnicode(api, document, replacements) {
  for (const {reference, family, text} of pdfUnicodeMaps(api, document)) {
    if (!Object.hasOwn(replacements, family)) continue;
    const updated = pdfUnicodeDestinations(text, code => replacements[family][code]);
    if (updated !== text) document.context.assign(reference, document.context.flateStream(updated));
  }
}
