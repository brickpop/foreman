// The browser the commands use, and what it may reach: this machine only.
// Four layers, each enough on its own for a page load:
//   1. every request, and every WebSocket, to another host is refused here;
//   2. Chromium's host resolver finds no name but localhost;
//   3. Chromium sends every other address through a proxy on a closed port
//      (it never proxies the loopback);
//   4. WebRTC sends UDP only through that proxy, which is to say never.
// Chromium's own sandbox stays on: launch fails rather than run without it.
import { chromium } from "playwright";

// limit is the time, in ms, that a navigation, the wait for the network to go
// quiet, and a whole shot each get.
export const limit = 30_000;

// local says whether a URL is http or https (or ws and wss, for WebSockets)
// on this machine.
export function local(url) {
  try {
    const u = new URL(url);
    return (
      ["http:", "https:", "ws:", "wss:"].includes(u.protocol) &&
      ["localhost", "127.0.0.1", "[::1]"].includes(u.hostname)
    );
  } catch {
    return false;
  }
}

export const flags = [
  "--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE localhost, EXCLUDE 127.0.0.1, EXCLUDE [::1]",
  "--proxy-server=http://127.0.0.1:9",
  "--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
];

export function launch() {
  return chromium.launch({ args: flags, chromiumSandbox: true, timeout: limit });
}

// open makes a page whose requests stay on this machine. refused holds the
// addresses it refused, each once.
export async function open(browser, options) {
  const context = await browser.newContext({ ...options, serviceWorkers: "block" });
  const page = { context, refused: new Set() };
  await context.route("**/*", (route) => {
    if (local(route.request().url())) return route.continue();
    page.refused.add(route.request().url());
    return route.abort();
  });
  await context.routeWebSocket(/.*/, (ws) => {
    if (local(ws.url())) return ws.connectToServer();
    page.refused.add(ws.url());
    return ws.close();
  });
  page.page = await context.newPage();
  page.page.setDefaultTimeout(limit);
  return page;
}

// visit loads url and waits until the network is quiet. It throws if the page
// failed, or ended up off this machine.
export async function visit(page, url) {
  const quiet = watchRequests(page);
  try {
    const response = await page.goto(url, { timeout: limit });
    if (!response?.ok()) throw new Error(`the page answered ${response?.status() ?? "nothing"}`);
    await quiet.settled();
  } finally {
    quiet.stop();
  }
  if (!local(page.url())) throw new Error("the page left this machine");
}

// quietMs is how long no request may be in flight before a page counts as loaded.
const quietMs = 500;

// watchRequests tracks a page's requests in flight, but for event streams:
// an app's live updates hold one open for as long as the page is, so
// Playwright's "networkidle" never comes for it.
function watchRequests(page) {
  const open = new Set();
  let last = Date.now();
  const start = (request) => {
    open.add(request);
    last = Date.now();
  };
  const end = (request) => {
    if (open.delete(request)) last = Date.now();
  };
  const response = (res) => {
    if ((res.headers()["content-type"] ?? "").startsWith("text/event-stream")) end(res.request());
  };
  const events = { request: start, requestfinished: end, requestfailed: end, response };
  for (const [name, handler] of Object.entries(events)) page.on(name, handler);
  return {
    async settled() {
      const deadline = Date.now() + limit;
      while (open.size > 0 || Date.now() - last < quietMs) {
        if (Date.now() > deadline)
          throw new Error(`the network was still busy after ${limit / 1000} s`);
        await new Promise((ok) => setTimeout(ok, 50));
      }
    },
    stop() {
      for (const [name, handler] of Object.entries(events)) page.off(name, handler);
    },
  };
}
