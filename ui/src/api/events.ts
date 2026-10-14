// Client side of the event stream (wiki/product/architecture/api.md#events):
// server-sent events, each one JSON Event. The browser reconnects by itself;
// after a reconnect the UI refetches what it shows, since events sent while
// it was away are not replayed.
import type { Event } from "./types/events";

export interface EventHandlers {
  /** One event, as the server published it. */
  event(e: Event): void;
  /** The stream came back after a drop: refetch what is on screen. */
  reconnected(): void;
}

/** The subset of EventSource the client uses, so tests can provide their own. */
export interface EventSourceLike {
  onopen: (() => void) | null;
  onmessage: ((e: { data: string }) => void) | null;
  close(): void;
}

/** Follows the stream at url until the returned function is called. */
export function subscribeEvents(
  url: string,
  handlers: EventHandlers,
  open: (url: string) => EventSourceLike = (u) => new EventSource(u) as unknown as EventSourceLike,
): () => void {
  const source = open(url);
  let opened = false;
  source.onopen = () => {
    if (opened) handlers.reconnected();
    opened = true;
  };
  source.onmessage = (m) => {
    let event: Event;
    try {
      event = JSON.parse(m.data) as Event;
    } catch {
      return; // not ours to crash on; the server only sends JSON
    }
    handlers.event(event);
  };
  return () => source.close();
}
