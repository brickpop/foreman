import type { TeamAction, TeamMessage } from "../api/client";

// The Team traffic panel's model (wiki/product/features/sessions.md#team-traffic):
// who calls whom, and the exchanges as threads, newest first.

/** One exchange: a message, and the replies to it when it is an ask. */
export type Thread = { message: TeamMessage; replies: TeamMessage[] };

/** The messages as threads, newest activity first: each ask with its replies
 * under it; everything else (a hand-off, a contribution, a reply whose ask
 * isn't listed) on its own. */
export function threads(messages: TeamMessage[]): Thread[] {
  const asks = new Map<string, Thread>();
  const all: Thread[] = [];
  for (const m of messages) {
    if (m.kind === "reply" && m.inReplyTo && asks.has(m.inReplyTo)) {
      asks.get(m.inReplyTo)?.replies.push(m);
      continue;
    }
    const t = { message: m, replies: [] };
    if (m.kind === "ask") asks.set(m.ref, t);
    all.push(t);
  }
  const latest = (t: Thread) => Math.max(...[t.message, ...t.replies].map((m) => m.id));
  return all.sort((a, b) => latest(b) - latest(a));
}

/** A pair of agents and their traffic, both ways. */
export type Link = {
  a: string;
  b: string;
  /** Messages from a to b, and from b to a. */
  ab: number;
  ba: number;
  /** A message between them still waits to be delivered. */
  waiting: boolean;
};

/** Who calls whom: one link per pair that exchanged messages, keyed by the
 * pair's instances in sorted order. */
export function links(messages: TeamMessage[]): Link[] {
  const pairs = new Map<string, Link>();
  for (const m of messages) {
    const [a, b] = [m.fromInstance, m.toInstance].sort();
    const key = `${a} ${b}`;
    const l = pairs.get(key) ?? { a, b, ab: 0, ba: 0, waiting: false };
    if (m.fromInstance === a) l.ab++;
    else l.ba++;
    l.waiting ||= m.state === "queued";
    pairs.set(key, l);
  }
  return [...pairs.values()];
}

/** What the list shows: everything, one agent's traffic, or one pair's. */
export type Filter = { agent: string } | { a: string; b: string } | null;

export function matches(t: Thread, f: Filter): boolean {
  if (!f) return true;
  return [t.message, ...t.replies].some((m) =>
    "agent" in f
      ? m.fromInstance === f.agent || m.toInstance === f.agent
      : [m.fromInstance, m.toInstance].sort().join(" ") === [f.a, f.b].sort().join(" "),
  );
}

/** One line of the panel: a thread, or an action (a git tool an agent called). */
export type Entry = { thread: Thread } | { action: TeamAction };

/** Threads and actions together, newest activity first. */
export function entries(messages: TeamMessage[], actions: TeamAction[]): Entry[] {
  const at = (e: Entry) =>
    "thread" in e
      ? Math.max(...[e.thread.message, ...e.thread.replies].map((m) => Date.parse(m.createdAt)))
      : Date.parse(e.action.createdAt);
  return [
    ...threads(messages).map((thread) => ({ thread })),
    ...actions.map((action) => ({ action })),
  ].sort((a, b) => at(b) - at(a));
}

/** Whether an entry shows under a filter: an action is one agent's, so a
 * pair filter leaves it out. */
export function shows(e: Entry, f: Filter): boolean {
  if ("thread" in e) return matches(e.thread, f);
  return !f || ("agent" in f && f.agent === e.action.instance);
}
