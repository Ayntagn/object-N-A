const api = window.PDFLib;
const paper = api.rgb(...plan.Paper);
const doc = plan.SystemPrint ? window.cyberwriterPrintedPDF : await api.PDFDocument.create();
if (!plan.SystemPrint) doc.setTitle(document.title);
const font = plan.Pages.some(page => page.number) ? await doc.embedFont(api.StandardFonts.Helvetica) : null;

function drawNumber(page, text, height) {
  if (!text) return;
  const size = 10;
  const textWidth = font.widthOfTextAtSize(text, size);
  const position = plan.NumberPosition;
  let x = plan.Left + (plan.ContentWidth - textWidth) / 2;
  if (position === 1 || position === 4) x = plan.Left + plan.ContentWidth - textWidth;
  if (position === 2 || position === 5) x = plan.Left;
  const y = position >= 3 ? height - plan.Top / 2 - 6 : plan.Bottom / 2 - 6;
  page.drawText(text, {x, y, font, size, color: api.rgb(103 / 255, 103 / 255, 103 / 255)});
}

if (plan.SystemPrint) {
  const selected = new Map(plan.Pages.map(slice => [slice.start, slice.number]));
  const pages = doc.getPages();
  for (let i = pages.length - 1; i >= 0; i--) {
    if (!selected.has(i)) {
      doc.removePage(i);
      continue;
    }
    const page = pages[i];
    const {x, y, width, height} = page.getMediaBox();
    const stream = doc.context.contentStream([
      api.pushGraphicsState(), api.setFillingColor(paper),
      api.rectangle(x, y, width, height), api.fill(), api.popGraphicsState(),
    ]);
    const reference = doc.context.register(stream);
    page.node.addContentStream(reference);
    const contents = page.node.normalizedEntries().Contents;
    contents.remove(contents.size() - 1);
    contents.insert(0, reference);
    drawNumber(page, selected.get(i), height);
  }
} else {
  const sources = [];
  for (const {top, height: captureHeight, input} of window.cyberwriterPDFSources) {
    if (plan.Unicode) restorePDFUnicode(api, input, plan.Unicode);
    const page = input.getPage(0);
    const {width, height} = page.getMediaBox();
    if (width < plan.ContentWidth - 1 || height < captureHeight - 1)
      throw new Error("PDF capture was clipped by the system WebView");
    const form = page.node.Contents() ? await doc.embedPage(page) : null;
    sources.push({top, height: captureHeight, form, mediaHeight: height});
  }
  const pages = plan.Pages.map(slice => ({...slice, page: doc.addPage([plan.Width, plan.Height])}));
  for (const slice of pages) {
    const {page, start, end} = slice;
    const contentTop = plan.Height - plan.Top;
    page.drawRectangle({x: 0, y: 0, width: plan.Width, height: plan.Height, color: paper});
    for (const source of sources) {
      const top = Math.max(start, source.top);
      const bottom = Math.min(end, source.top + source.height);
      if (top >= bottom || !source.form) continue;
      page.pushOperators(
        api.pushGraphicsState(),
        api.rectangle(plan.Left, contentTop + start - bottom, plan.ContentWidth, bottom - top),
        api.clip(), api.endPath(),
      );
      page.drawPage(source.form, {x: plan.Left, y: contentTop + start - source.top - source.mediaHeight});
      page.pushOperators(api.popGraphicsState());
    }
    for (const link of plan.Links || []) {
      const [x0, y0, x1, y1] = link.rect;
      const left = Math.max(x0, 0);
      const right = Math.min(x1, plan.ContentWidth);
      const top = Math.max(y0, start);
      const bottom = Math.min(y1, end);
      if (left >= right || top >= bottom) continue;
      let action;
      if (link.url) {
        const encoded = Array.from(new TextEncoder().encode(link.url), byte => byte.toString(16).padStart(2, "0")).join("");
        action = {A: {Type: "Action", S: "URI", URI: api.PDFHexString.of(encoded)}};
      } else {
        const [x, y] = link.destination;
        const target = pages.find(candidate => y >= candidate.start && y < candidate.end);
        if (!target) continue;
        action = {Dest: [target.page.ref, api.PDFName.of("XYZ"), plan.Left + x, contentTop + target.start - y, 0]};
      }
      const rect = [plan.Left + left, contentTop + start - bottom, plan.Left + right, contentTop + start - top];
      page.node.addAnnot(doc.context.register(doc.context.obj({
        Type: "Annot", Subtype: "Link", Border: [0, 0, 0], Rect: rect, ...action,
      })));
    }
    drawNumber(page, slice.number, plan.Height);
  }
}
return JSON.stringify({Data: await doc.saveAsBase64({updateFieldAppearances: false}), Pages: doc.getPageCount()});
