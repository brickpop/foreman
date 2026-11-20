// screenshot <url> [dir] [--width <px>]... [--selector <css>]: PNGs of a page
// on this machine, light and dark, into dir (./screenshots by default). At
// desktop (1440) and phone (390) width, named light-desktop.png and so on; or
// at each --width given, named light-780.png. The whole page, or with
// --selector only the first element it matches. It prints the paths, and how
// many addresses elsewhere it refused: never anything from the page, which is
// untrusted.
import fs from "node:fs";
import path from "node:path";
import { launch, limit, local, open, visit } from "./browser.mjs";

const usage =
  "usage: screenshot <url> [dir] [--width <px>]... [--selector <css>], the url being http or https on localhost, 127.0.0.1 or [::1]";
const args = process.argv.slice(2);
const widths = [];
let selector;
const rest = [];
for (let i = 0; i < args.length; i++) {
  if (args[i] === "--width") widths.push(Number(args[++i]));
  else if (args[i] === "--selector") selector = args[++i] ?? "";
  else rest.push(args[i]);
}
const [url, dir = "screenshots"] = rest;
const bad = !url || !local(url) || !/^https?:/.test(url) || rest.length > 2 || selector === "";
if (bad || widths.some((w) => !(w >= 200 && w <= 4000))) {
  console.error(usage);
  process.exit(2);
}
fs.mkdirSync(dir, { recursive: true });

const sizes = widths.length
  ? widths.map((w) => [String(w), { width: w, height: 900 }])
  : [
      ["desktop", { width: 1440, height: 900 }],
      ["phone", { width: 390, height: 844 }],
    ];

function fail(why) {
  console.error(`screenshot: ${why}`);
  process.exit(1);
}

const refused = new Set();
try {
  const browser = await launch();
  for (const theme of ["light", "dark"]) {
    for (const [name, viewport] of sizes) {
      // Each shot gets the limit; exiting kills the browser too, since
      // Playwright closes it with its parent.
      const timer = setTimeout(() => fail(`gave up after ${limit / 1000} s`), limit);
      const shot = await open(browser, { viewport, colorScheme: theme, deviceScaleFactor: 1 });
      await visit(shot.page, url);
      await shot.page.evaluate(() => document.fonts.ready);
      const file = path.join(dir, `${theme}-${name}.png`);
      const options = { path: file, animations: "disabled", caret: "hide" };
      if (selector) {
        // An element is shot only as far as its scroller shows it: first make
        // the window as tall as the element reaches. Only the width lays it out.
        const element = shot.page.locator(selector).first();
        const bottom = await element.evaluate((el) => Math.ceil(el.getBoundingClientRect().bottom));
        if (bottom > viewport.height)
          await shot.page.setViewportSize({ width: viewport.width, height: bottom + 100 });
        await element.screenshot(options);
      } else await shot.page.screenshot({ ...options, fullPage: true });
      console.log(file);
      for (const u of shot.refused) refused.add(u);
      await shot.context.close();
      clearTimeout(timer);
    }
  }
  await browser.close();
} catch (e) {
  fail(e.message.split("\n")[0]);
}
// The same address refused on every shot counts once.
if (refused.size) console.error(`screenshot: refused ${refused.size} addresses on other hosts`);
process.exit(0);
