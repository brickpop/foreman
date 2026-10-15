import { describe, expect, test } from "vitest";
import { openPty, type PtyHandlers, type SocketLike } from "./pty";

class FakeSocket implements SocketLike {
  binaryType = "blob";
  readyState = 0;
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: unknown }) => void) | null = null;
  onclose: ((e: { code: number }) => void) | null = null;
  sent: (string | Uint8Array)[] = [];
  closedByClient = false;

  constructor(readonly url: string) {}
  open() {
    this.readyState = 1;
    this.onopen?.();
  }
  binary(s: string) {
    this.onmessage?.({ data: new TextEncoder().encode(s).buffer });
  }
  text(s: string) {
    this.onmessage?.({ data: s });
  }
  serverClose(code: number) {
    this.readyState = 3;
    this.onclose?.({ code });
  }
  send(data: string | Uint8Array) {
    this.sent.push(data);
  }
  close() {
    this.closedByClient = true;
    this.readyState = 3;
  }
}

function setup() {
  const sockets: FakeSocket[] = [];
  const scheduled: { fn: () => void; ms: number }[] = [];
  const log: string[] = [];
  const handlers: PtyHandlers = {
    output: (d) => log.push(`out:${new TextDecoder().decode(d)}`),
    reset: () => log.push("reset"),
    exit: (c) => log.push(`exit:${c}`),
  };
  const conn = openPty(
    "ws://x/ws/pty/mara%40ledger",
    handlers,
    (url) => {
      const s = new FakeSocket(url);
      sockets.push(s);
      return s;
    },
    (fn, ms) => scheduled.push({ fn, ms }),
  );
  const last = () => sockets[sockets.length - 1];
  const runScheduled = () => {
    for (const s of scheduled.splice(0)) s.fn();
  };
  return { conn, sockets, scheduled, log, last, runScheduled };
}

const decode = (d: string | Uint8Array) =>
  typeof d === "string" ? d : new TextDecoder().decode(d);

describe("openPty", () => {
  test("receives raw bytes as binary, resetting once before the replay", () => {
    const { last, log } = setup();
    expect(last().binaryType).toBe("arraybuffer");
    last().open();
    last().binary("replayed ");
    last().binary("\x1b[32mlive\x1b[0m");
    expect(log).toEqual(["reset", "out:replayed ", "out:\x1b[32mlive\x1b[0m"]);
  });

  test("sends input as binary and the size as a JSON control frame", () => {
    const { conn, last } = setup();
    conn.resize(100, 30); // before the socket opens: remembered, sent on open
    last().open();
    conn.send("ls\r");
    expect(last().sent.map(decode)).toEqual(['{"type":"resize","cols":100,"rows":30}', "ls\r"]);
    expect(last().sent[1]).toBeInstanceOf(Uint8Array);
  });

  test("drops input while not connected rather than throwing", () => {
    const { conn, last } = setup();
    conn.send("lost");
    expect(last().sent).toEqual([]);
  });

  test("reports the exit and doesn't reconnect after it", () => {
    const { last, log, scheduled } = setup();
    last().open();
    last().text('{"type":"exit","code":4}');
    last().serverClose(1000);
    expect(log).toEqual(["exit:4"]);
    expect(scheduled).toEqual([]);
  });

  test("ignores unknown or malformed control frames", () => {
    const { last, log } = setup();
    last().open();
    last().text("not json");
    last().text('{"type":"future-thing"}');
    last().text('{"type":"exit"}'); // no code: not an exit
    expect(log).toEqual([]);
  });

  test("reconnects at once after falling behind, replays from a reset, and resends the size", () => {
    const { conn, sockets, last, log, scheduled, runScheduled } = setup();
    conn.resize(80, 24);
    last().open();
    last().binary("a");
    last().serverClose(1013);
    expect(scheduled.map((s) => s.ms)).toEqual([0]);
    runScheduled();
    expect(sockets).toHaveLength(2);
    last().open();
    last().binary("a and more");
    expect(log).toEqual(["reset", "out:a", "reset", "out:a and more"]);
    expect(last().sent.map(decode)).toEqual(['{"type":"resize","cols":80,"rows":24}']);
  });

  test("retries other disconnects with a growing, capped backoff that resets on success", () => {
    const { last, scheduled, runScheduled } = setup();
    const delays: number[] = [];
    for (let i = 0; i < 7; i++) {
      last().serverClose(1006);
      delays.push(scheduled[0].ms);
      runScheduled();
    }
    expect(delays).toEqual([250, 500, 1000, 2000, 4000, 5000, 5000]);
    last().open();
    last().serverClose(1006);
    expect(scheduled[0].ms).toBe(250);
  });

  test("close detaches for good", () => {
    const { conn, last, scheduled } = setup();
    last().open();
    conn.close();
    expect(last().closedByClient).toBe(true);
    last().serverClose(1000);
    expect(scheduled).toEqual([]);
  });
});
