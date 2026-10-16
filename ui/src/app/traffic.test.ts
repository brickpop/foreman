import { describe, expect, it } from "vitest";
import type { TeamMessage } from "../api/client";
import { links, matches, threads } from "./traffic";

let next = 0;
function msg(
  from: string,
  to: string,
  kind: string,
  ref: string,
  extra: Partial<TeamMessage> = {},
) {
  next++;
  return {
    id: next,
    deploymentId: "atlas@ledger",
    ref,
    fromInstance: `${from}@ledger`,
    toInstance: `${to}@ledger`,
    kind,
    inReplyTo: "",
    body: "",
    artifactRef: "",
    state: "delivered",
    reason: "",
    createdAt: "2026-10-16T18:00:00Z",
    ...extra,
  } as TeamMessage;
}

describe("threads", () => {
  it("nests each reply under its ask, newest activity first", () => {
    const ask1 = msg("tic", "tac", "ask", "msg-1");
    const ask2 = msg("tic", "toe", "ask", "msg-2");
    const handoff = msg("tac", "tic", "handoff", "msg-3");
    const reply1 = msg("tac", "tic", "reply", "msg-4", { inReplyTo: "msg-1" });
    const orphan = msg("toe", "tic", "reply", "msg-5", { inReplyTo: "msg-99" });
    const t = threads([ask1, ask2, handoff, reply1, orphan]);
    expect(t.map((x) => x.message.ref)).toEqual(["msg-5", "msg-1", "msg-3", "msg-2"]);
    expect(t[1].replies.map((m) => m.ref)).toEqual(["msg-4"]);
    expect(t[0].replies).toEqual([]);
  });
});

describe("links", () => {
  it("counts each pair both ways, and flags one with a message waiting", () => {
    const l = links([
      msg("tic", "tac", "ask", "msg-1"),
      msg("tac", "tic", "reply", "msg-2"),
      msg("tic", "tac", "ask", "msg-3", { state: "queued" }),
      msg("tic", "toe", "ask", "msg-4"),
    ]);
    expect(l).toEqual([
      { a: "tac@ledger", b: "tic@ledger", ab: 1, ba: 2, waiting: true },
      { a: "tic@ledger", b: "toe@ledger", ab: 1, ba: 0, waiting: false },
    ]);
  });
});

describe("matches", () => {
  const t = threads([
    msg("tic", "tac", "ask", "msg-1"),
    msg("tac", "tic", "reply", "msg-2", { inReplyTo: "msg-1" }),
  ])[0];
  it("filters by agent or by pair, in either direction", () => {
    expect(matches(t, null)).toBe(true);
    expect(matches(t, { agent: "tac@ledger" })).toBe(true);
    expect(matches(t, { agent: "toe@ledger" })).toBe(false);
    expect(matches(t, { a: "tac@ledger", b: "tic@ledger" })).toBe(true);
    expect(matches(t, { a: "tic@ledger", b: "toe@ledger" })).toBe(false);
  });
});
