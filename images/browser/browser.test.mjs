// The browser and its commands: what a page can reach, Chromium's sandbox, the
// fonts, screenshot and screenshot-diff. install.sh runs it on every new
// install, which becomes current only if it passes:
//   . <folder>/env.sh && cd <folder> && node --test browser.test.mjs
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import fs from "node:fs";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import { after, before, test } from "node:test";
import { promisify } from "node:util";
import { launch, open, visit } from "./browser.mjs";

const run = promisify(execFile);

// The pages under test, served on this machine.
const pages = {
  "/tall":
    '<!doctype html><main style="height: 3000px; background: linear-gradient(red, blue)">tall</main>',
  "/": '<!doctype html><meta name="color-scheme" content="light dark"><title>t</title><p>local</p>',
  // The generic names, then the fonts they should be.
  "/fonts": [
    "system-ui",
    "sans-serif",
    "serif",
    "monospace",
    "'Liberation Sans'",
    "'Liberation Serif'",
    "'Liberation Mono'",
  ]
    .map((f) => `<span style="font: 20px ${f}">Hamburgefonstiv quick</span><br>`)
    .join(""),
  "/live": '<!doctype html><p>live</p><script src="/live.js"></script>',
  "/live.js": 'fetch("/events").then((r) => r.body.getReader().read());',
  "/sw.js": `self.onmessage = (e) => fetch(e.data, { mode: "no-cors" }).then(() => "reached", () => "failed").then((r) => e.source.postMessage(r));`,
};
const server = http.createServer((req, res) => {
  if (req.url === "/away") return res.writeHead(302, { location: "http://example.com/" }).end();
  // An event stream that never ends, as an app's live updates are.
  if (req.url === "/events")
    return res.writeHead(200, { "content-type": "text/event-stream" }).write("data: 1\n\n");
  if (!(req.url in pages)) return res.writeHead(404).end();
  res
    .writeHead(200, { "content-type": req.url.endsWith(".js") ? "text/javascript" : "text/html" })
    .end(pages[req.url]);
});
let base, browser;

before(async () => {
  await new Promise((ok) => server.listen(0, "127.0.0.1", ok));
  base = `http://127.0.0.1:${server.address().port}`;
  browser = await launch();
});
after(async () => {
  server.closeAllConnections();
  server.close();
  await browser?.close();
});

async function local() {
  const shot = await open(browser);
  await visit(shot.page, `${base}/`);
  return shot;
}

test("the renderers run in Chromium's sandbox", async () => {
  const shot = await local();
  const ours = fs.readlinkSync("/proc/self/ns/user");
  const renderers = fs.readdirSync("/proc").filter((pid) => {
    try {
      return /--type=renderer/.test(fs.readFileSync(`/proc/${pid}/cmdline`, "utf8"));
    } catch {
      return false;
    }
  });
  assert.ok(renderers.length > 0, "no renderer found");
  for (const pid of renderers) {
    assert.doesNotMatch(fs.readFileSync(`/proc/${pid}/cmdline`, "utf8"), /--no-sandbox/);
    // Unreadable means another user namespace too.
    let ns;
    try {
      ns = fs.readlinkSync(`/proc/${pid}/ns/user`);
    } catch {
      continue;
    }
    assert.notEqual(ns, ours, `renderer ${pid} shares our user namespace`);
  }
  await shot.context.close();
});

test("fetch reaches this machine only", async () => {
  const shot = await local();
  const result = await shot.page.evaluate(async (base) => {
    const get = (u) =>
      fetch(u, { mode: "no-cors" }).then(
        () => "reached",
        () => "failed",
      );
    return {
      here: await get(`${base}/`),
      ip: await get("http://1.1.1.1/"),
      name: await get("https://example.com/"),
    };
  }, base);
  assert.deepEqual(result, { here: "reached", ip: "failed", name: "failed" });
  assert.equal(shot.refused.size, 2);
  await shot.context.close();
});

test("a WebSocket to another host never opens", async () => {
  const shot = await local();
  const result = await shot.page.evaluate(
    () =>
      new Promise((done) => {
        const ws = new WebSocket("ws://1.1.1.1/");
        ws.onopen = () => done("open");
        ws.onerror = ws.onclose = () => done("closed");
        setTimeout(() => done("timeout"), 10_000);
      }),
  );
  assert.notEqual(result, "open");
  assert.equal(shot.refused.size, 1);
  await shot.context.close();
});

test("a redirect off this machine is refused", async () => {
  const shot = await open(browser);
  await assert.rejects(visit(shot.page, `${base}/away`));
  await shot.context.close();
});

test("a service worker reaches this machine only", async () => {
  const shot = await local();
  const result = await shot.page.evaluate(async () => {
    if (!navigator.serviceWorker) return "no service workers";
    await navigator.serviceWorker.register("/sw.js");
    const sw = await Promise.race([
      navigator.serviceWorker.ready,
      new Promise((done) => setTimeout(done, 5_000)),
    ]);
    if (!sw?.active) return "never active";
    return new Promise((done) => {
      navigator.serviceWorker.onmessage = (e) => done(e.data);
      sw.active.postMessage("http://1.1.1.1/");
      setTimeout(() => done("no answer"), 10_000);
    });
  });
  assert.notEqual(result, "reached");
  await shot.context.close();
});

test("WebRTC finds no address through a public STUN server", async () => {
  const shot = await local();
  const candidates = await shot.page.evaluate(
    () =>
      new Promise((done) => {
        const found = [];
        const pc = new RTCPeerConnection({ iceServers: [{ urls: "stun:74.125.250.129:19302" }] });
        pc.createDataChannel("x");
        pc.onicecandidate = (e) => (e.candidate ? found.push(e.candidate.type) : done(found));
        pc.createOffer().then((o) => pc.setLocalDescription(o));
        setTimeout(() => done(found), 10_000);
      }),
  );
  assert.ok(!candidates.includes("srflx"), `candidates: ${candidates}`);
  await shot.context.close();
});

test("a page that holds an event stream open still loads", async () => {
  const shot = await open(browser);
  await visit(shot.page, `${base}/live`);
  assert.equal(await shot.page.textContent("p"), "live");
  await shot.context.close();
});

test("screenshot prints only the paths of its files", async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "shots-"));
  const { stdout } = await run("node", ["screenshot.mjs", `${base}/`, dir]);
  const names = ["light-desktop", "light-phone", "dark-desktop", "dark-phone"];
  assert.deepEqual(
    stdout.trim().split("\n"),
    names.map((n) => path.join(dir, `${n}.png`)),
  );
  fs.rmSync(dir, { recursive: true });
});

test("screenshot refuses a page elsewhere", async () => {
  for (const url of [
    "https://example.com/",
    "http://1.1.1.1/",
    "file:///etc/passwd",
    "ws://localhost/",
  ]) {
    await assert.rejects(run("node", ["screenshot.mjs", url, os.tmpdir()]), { code: 2 }, url);
  }
});

// The request filter refuses first in every test above. Without it, Chromium's
// own flags still keep a page here: each layer is enough on its own.
test("without the request filter, Chromium's flags alone keep a page here", async () => {
  const context = await browser.newContext({ serviceWorkers: "block" });
  const page = await context.newPage();
  await page.goto(`${base}/`);
  const result = await page.evaluate(async () => {
    const get = (u) =>
      fetch(u, { mode: "no-cors" }).then(
        () => "reached",
        () => "failed",
      );
    const ws = (u) =>
      new Promise((done) => {
        const s = new WebSocket(u);
        s.onopen = () => done("open");
        s.onerror = s.onclose = () => done("closed");
        setTimeout(() => done("timeout"), 10_000);
      });
    return [
      await get("http://1.1.1.1/"),
      await get("https://example.com/"),
      await ws("ws://1.1.1.1/"),
      await ws("wss://example.com/"),
    ];
  });
  assert.deepEqual(result.slice(0, 2), ["failed", "failed"]);
  assert.ok(!result.slice(2).includes("open"), `WebSockets: ${result.slice(2)}`);
  await context.close();
});

test("generic font names are Liberation's, as a page asks", async () => {
  const shot = await open(browser);
  await visit(shot.page, `${base}/fonts`);
  const [ui, sans, serif, mono, lsans, lserif, lmono] = await shot.page.evaluate(() =>
    [...document.querySelectorAll("span")].map((s) => s.getBoundingClientRect().width),
  );
  assert.deepEqual([ui, sans, serif, mono], [lsans, lsans, lserif, lmono]);
  assert.notEqual(lsans, lmono);
  await shot.context.close();
});

test("screenshot --selector shoots a tall element whole", async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "shots-"));
  await run("node", [
    "screenshot.mjs",
    `${base}/tall`,
    dir,
    "--width",
    "500",
    "--selector",
    "main",
  ]);
  const png = fs.readFileSync(path.join(dir, "light-500.png"));
  // A PNG's height is the big-endian word at byte 20.
  assert.equal(png.readUInt32BE(20), 3000);
  fs.rmSync(dir, { recursive: true });
});

test("screenshot takes --width and --selector", async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "shots-"));
  const { stdout } = await run("node", [
    "screenshot.mjs",
    `${base}/`,
    dir,
    "--width",
    "780",
    "--selector",
    "p",
  ]);
  assert.deepEqual(
    stdout.trim().split("\n"),
    ["light-780", "dark-780"].map((n) => path.join(dir, `${n}.png`)),
  );
  fs.rmSync(dir, { recursive: true });
});

test("screenshot-diff: same, different, other sizes, not a PNG", async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "diff-"));
  const a = path.join(dir, "a");
  const b = path.join(dir, "b");
  await run("node", ["screenshot.mjs", `${base}/`, a]);
  await run("node", ["screenshot.mjs", `${base}/`, b]);
  const diff = (...files) =>
    run("node", ["screenshot-diff.mjs", ...files]).then(
      (r) => ({ code: 0, out: r.stdout.trim() }),
      (e) => ({ code: e.code, out: e.stdout.trim() }),
    );

  assert.deepEqual(
    await diff(path.join(a, "light-desktop.png"), path.join(b, "light-desktop.png")),
    { code: 0, out: "same" },
  );
  const out = path.join(dir, "diff.png");
  const themes = await diff(
    path.join(a, "light-desktop.png"),
    path.join(a, "dark-desktop.png"),
    out,
  );
  assert.equal(themes.code, 1);
  assert.match(themes.out, /^\d+ pixels differ$/);
  assert.ok(fs.statSync(out).size > 0);
  const sizes = await diff(path.join(a, "light-desktop.png"), path.join(a, "light-phone.png"));
  assert.equal(sizes.code, 1);
  assert.match(sizes.out, /^sizes differ/);
  fs.writeFileSync(path.join(dir, "not.png"), "not a png");
  assert.equal((await diff(path.join(dir, "not.png"), path.join(a, "light-desktop.png"))).code, 2);
  fs.rmSync(dir, { recursive: true });
});
