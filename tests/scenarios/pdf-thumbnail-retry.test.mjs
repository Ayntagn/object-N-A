// spec: EV-MYGO-326
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../../ui/pdf_grid_viewport.js", import.meta.url), "utf8");

test("PDF thumbnail failure stays on the page and retries once on re-entry", async () => {
  const window = Object.assign(new EventTarget(), { innerHeight: 100 });
  let offset = 0;
  const cards = [10, 10, 10, 600].map((top, index) => ({
    number: index + 1,
    image: { getAttribute: () => "100" },
    element: {
      getBoundingClientRect: () => ({ top: top - offset, bottom: top - offset + 80 }),
      append(notice) {
        this.notice = notice;
        notice.remove = () => { delete this.notice; };
      }
    }
  }));
  const started = [];
  async function render(card) {
    const firstAttempt = !started.includes(card.number);
    started.push(card.number);
    if (card.number === 2 || (card.number === 1 && firstAttempt)) throw new Error("Raster rejected");
    return () => {};
  }

  const timers = new Map();
  let nextTimer = 0;
  const context = vm.createContext({
    window, cards, render, AbortController, thumbnailBudget: 1 << 20,
    console: { warn() {} },
    document: {
      getElementById: () => ({}),
      createElement: () => ({ setAttribute(name, value) { this[name] = value; } })
    },
    setTimeout(callback) { timers.set(++nextTimer, callback); return nextTimer; },
    clearTimeout(timer) { timers.delete(timer); }
  });
  vm.runInContext(source + "\nwatchPDFThumbnailWindow(cards, render);", context);

  async function move(to, event = "scroll") {
    offset = to;
    window.dispatchEvent(new Event(event));
    for (let limit = 10; limit > 0 && timers.size; limit--) {
      const [id, callback] = timers.entries().next().value;
      timers.delete(id);
      callback();
      await new Promise(setImmediate);
    }
    assert.equal(timers.size, 0, "rendering must settle without a retry loop");
  }

  await move(0);
  assert.deepEqual(started, [1, 2, 3]);
  assert.match(cards[0].element.notice?.textContent ?? "", /thumbnail/i);
  assert.equal(cards[1].element.notice?.role, "status");
  assert.equal(cards[2].element.notice, undefined);
  await move(0, "resize");
  await move(110); // Failed pages are offscreen but still within the preload margin.
  assert.deepEqual(started, [1, 2, 3]);
  await move(590);
  await move(0);
  assert.deepEqual(started, [1, 2, 3, 4, 1, 2]);
  assert.equal(cards[0].element.notice, undefined);
  assert.match(cards[1].element.notice?.textContent ?? "", /thumbnail/i);
  await move(110);
  await move(0);
  assert.deepEqual(started, [1, 2, 3, 4, 1, 2]);
});
