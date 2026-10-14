import { describe, expect, it } from "vitest";
import { type EventSourceLike, subscribeEvents } from "./events";
import type { Event } from "./types/events";

function fakeSource() {
  const source: EventSourceLike & { closed: boolean } = {
    onopen: null,
    onmessage: null,
    closed: false,
    close() {
      source.closed = true;
    },
  };
  return source;
}

function follow() {
  const source = fakeSource();
  const events: Event[] = [];
  let reconnects = 0;
  const stop = subscribeEvents(
    "/api/events",
    { event: (e) => events.push(e), reconnected: () => reconnects++ },
    () => source,
  );
  return { source, events, reconnects: () => reconnects, stop };
}

describe("subscribeEvents", () => {
  it("hands over each event as the server sent it", () => {
    const f = follow();
    f.source.onopen?.();
    f.source.onmessage?.({
      data: '{"type":"config.changed","data":{"object":"team","id":"atlas"}}',
    });
    expect(f.events).toEqual([{ type: "config.changed", data: { object: "team", id: "atlas" } }]);
  });

  it("asks for a refetch on every reopen, not on the first open", () => {
    const f = follow();
    f.source.onopen?.();
    expect(f.reconnects()).toBe(0);
    f.source.onopen?.();
    f.source.onopen?.();
    expect(f.reconnects()).toBe(2);
  });

  it("ignores what isn't JSON instead of throwing", () => {
    const f = follow();
    expect(() => f.source.onmessage?.({ data: "not json" })).not.toThrow();
    expect(f.events).toEqual([]);
  });

  it("closes the stream when stopped", () => {
    const f = follow();
    f.stop();
    expect(f.source.closed).toBe(true);
  });
});
