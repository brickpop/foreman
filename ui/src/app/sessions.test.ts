import { describe, expect, it } from "vitest";
import type { Instance, Request, Team } from "../api/client";
import {
  applyMention,
  clickTab,
  closePane,
  countdown,
  deploymentStatus,
  deploymentTabs,
  inOrder,
  isLineShortcut,
  linePlaceholder,
  lineShortcut,
  mentionAt,
  mentionMatches,
  mostRecentAgent,
  moved,
  panesFrom,
  rosterOrder,
  searchFor,
  stoppedLine,
  toggleSplit,
  withDoing,
} from "./sessions";

const inst = (id: string, deploymentId: string) =>
  ({ id, deploymentId, agentId: id.split("@")[0] }) as Instance;
const atlas = {
  id: "atlas",
  config: {
    name: "Atlas",
    lead: "mara",
    agents: { ilya: {}, mara: {}, tove: {} },
    order: ["tove", "mara", "ilya"],
  },
} as unknown as Team;

const all = [
  inst("tove@ledger", "atlas@ledger"),
  inst("mara@ledger", "atlas@ledger"),
  inst("ilya@ledger", "atlas@ledger"),
  inst("lumen@relay", "orion@relay"),
];

describe("Sessions", () => {
  it("has a tab per deployment, with its needs-you dot", () => {
    const open = { status: "open", deploymentId: "orion@relay" } as Request;
    expect(deploymentTabs(all, [atlas], [open])).toEqual([
      {
        id: "atlas@ledger",
        team: "Atlas",
        project: "ledger",
        label: "Atlas · ledger",
        needsYou: false,
        status: "asleep",
      },
      {
        id: "orion@relay",
        team: "orion",
        project: "relay",
        label: "orion · relay",
        needsYou: true,
        status: "asleep",
      },
    ]);
  });

  it("shows a team's liveliest agent on its tab", () => {
    const with_ = (...statuses: string[]) => statuses.map((status) => ({ status }) as Instance);
    expect(deploymentStatus(with_("asleep", "running", "idle"))).toBe("running");
    expect(deploymentStatus(with_("running", "awaiting"))).toBe("awaiting"); // waiting on the CEO first
    expect(deploymentStatus(with_("idle", "done", "asleep"))).toBe("idle");
    expect(deploymentStatus(with_("asleep", "stopped"))).toBe("asleep");
    expect(deploymentStatus(with_("weird"))).toBe("asleep");
    expect(deploymentStatus([])).toBe("asleep");
  });

  it("keeps the tabs in the order they were dragged to", () => {
    const tabs = [{ id: "a" }, { id: "b" }, { id: "c" }, { id: "new" }];
    expect(inOrder(tabs, ["c", "a", "b", "gone"]).map((t) => t.id)).toEqual(["c", "a", "b", "new"]);
    expect(inOrder(tabs, []).map((t) => t.id)).toEqual(["a", "b", "c", "new"]);
    expect(moved(["a", "b", "c"], "a", "c")).toEqual(["b", "c", "a"]); // rightwards: after the target
    expect(moved(["a", "b", "c"], "c", "a")).toEqual(["c", "a", "b"]); // leftwards: before it
    expect(moved(["a", "b", "c"], "b", "b")).toEqual(["a", "b", "c"]);
    expect(moved(["a", "b", "c"], "x", "a")).toEqual(["a", "b", "c"]);
  });

  it("finds the agent that did something last", () => {
    const at = (id: string, deploymentId: string, doing?: string) =>
      ({ ...inst(id, deploymentId), doing: doing && { line: "Started", at: doing } }) as Instance;
    const list = [
      at("tove@ledger", "atlas@ledger", "2026-10-17T10:00:00Z"),
      at("mara@ledger", "atlas@ledger", "2026-10-17T11:30:00Z"),
      at("ilya@ledger", "atlas@ledger"),
      at("lumen@relay", "orion@relay", "2026-10-17T12:00:00Z"),
    ];
    expect(mostRecentAgent(list, "atlas@ledger")).toBe("mara");
    expect(mostRecentAgent([at("ilya@ledger", "atlas@ledger")], "atlas@ledger")).toBeUndefined();
    expect(mostRecentAgent(list, "nope@x")).toBeUndefined();
  });

  it("replaces one agent's activity line, leaving the rest", () => {
    const doing = { line: "Bash: go test ./...", at: "2026-10-18T10:00:00Z" };
    const list = [inst("tove@ledger", "atlas@ledger"), inst("mara@ledger", "atlas@ledger")];
    const got = withDoing(list, { id: "mara@ledger", doing });
    expect(got?.map((i) => i.doing)).toEqual([undefined, doing]);
    expect(list[1].doing).toBeUndefined(); // the cached list itself is never changed
    expect(withDoing(undefined, { id: "mara@ledger", doing })).toBeUndefined(); // nothing fetched yet
    expect(withDoing(list, { id: "nobody@x", doing })).toEqual(list);
  });

  it("puts the lead first, then the team file's order", () => {
    expect(rosterOrder("atlas@ledger", all, [atlas]).map((i) => i.agentId)).toEqual([
      "mara",
      "tove",
      "ilya",
    ]);
  });

  it("opens on the lead, splits with the first other member, and goes back", () => {
    const roster = rosterOrder("atlas@ledger", all, [atlas]);
    let p = panesFrom({}, roster);
    expect(p).toEqual({ open: ["mara@ledger"], active: 0 });
    p = toggleSplit(p, roster);
    expect(p).toEqual({ open: ["mara@ledger", "tove@ledger"], active: 1 });
    // A tab already in a pane activates it; another replaces the active pane.
    expect(clickTab(p, "mara@ledger")).toEqual({ open: ["mara@ledger", "tove@ledger"], active: 0 });
    expect(clickTab(p, "ilya@ledger")).toEqual({ open: ["mara@ledger", "ilya@ledger"], active: 1 });
    // Single keeps the active pane's agent; closing a pane leaves pane 1 active.
    expect(toggleSplit(p, roster)).toEqual({ open: ["tove@ledger"], active: 0 });
    expect(closePane(p, 0)).toEqual({ open: ["tove@ledger"], active: 0 });
    // A team of one can't split.
    expect(toggleSplit({ open: ["x"], active: 0 }, [inst("x", "d")])).toEqual({
      open: ["x"],
      active: 0,
    });
  });

  it("words the line input for each case", () => {
    expect(linePlaceholder("Mara", "awaiting", true)).toBe("Reply to Mara…");
    expect(linePlaceholder("Mara", "running", true)).toBe(
      "Tell Mara what you need. They’ll delegate.",
    );
    expect(linePlaceholder("Tove", "idle", false)).toBe("Direct instruction to Tove…");
  });
});

describe("countdown", () => {
  it("shows m:ss, rounded up", () => {
    expect(countdown(60_000)).toBe("1:00");
    expect(countdown(41_200)).toBe("0:42");
    expect(countdown(1)).toBe("0:01");
    expect(countdown(0)).toBe("0:00");
    expect(countdown(-5)).toBe("0:00");
  });
});

describe("the line input's shortcut", () => {
  const none = { code: "KeyL", ctrlKey: false, shiftKey: true, altKey: false, metaKey: false };
  const ctrl = { ...none, ctrlKey: true };
  const cmd = { ...none, metaKey: true };

  it("is Ctrl+Shift+L on a PC and Cmd+Shift+L on a Mac, by key position (any layout)", () => {
    expect(isLineShortcut(ctrl, false)).toBe(true);
    expect(isLineShortcut(cmd, true)).toBe(true);
    expect(lineShortcut(false)).toBe("Ctrl+Shift+L");
    expect(lineShortcut(true)).toBe("⌘⇧L");
  });

  it("leaves the agent its own keys", () => {
    // The other platform's combination, and Ctrl+Shift+L on a Mac (a terminal key there).
    expect(isLineShortcut(cmd, false)).toBe(false);
    expect(isLineShortcut(ctrl, true)).toBe(false);
    for (const mac of [false, true]) {
      const mod = mac ? cmd : ctrl;
      expect(isLineShortcut({ ...mod, shiftKey: false }, mac)).toBe(false); // Ctrl+L / Cmd+L
      expect(isLineShortcut({ ...mod, altKey: true }, mac)).toBe(false);
      expect(isLineShortcut({ ...mod, code: "KeyK" }, mac)).toBe(false);
      expect(isLineShortcut({ ...mod, ctrlKey: true, metaKey: true }, mac)).toBe(false);
    }
  });
});

describe("mentions in the line input", () => {
  it("finds the mention being typed before the caret", () => {
    expect(mentionAt("ask @Ta", 7)).toEqual({ start: 4, query: "Ta" });
    expect(mentionAt("@", 1)).toEqual({ start: 0, query: "" });
    expect(mentionAt("ask @Tac to", 11)).toBeNull(); // past it
    expect(mentionAt("mail me@host", 12)).toBeNull(); // not after a space
    expect(mentionAt("ask @Tö", 7)).toEqual({ start: 4, query: "Tö" });
  });
  it("offers the names that start with it, any case", () => {
    expect(mentionMatches(["Tic", "Tac", "Toe"], "t")).toEqual(["Tic", "Tac", "Toe"]);
    expect(mentionMatches(["Tic", "Tac", "Toe"], "TA")).toEqual(["Tac"]);
    expect(mentionMatches(["Tic"], "x")).toEqual([]);
  });
  it("puts the plain name in, then a space, keeping what follows", () => {
    expect(applyMention("ask @Ta now", 7, 4, "Tac")).toEqual({ text: "ask Tac  now", caret: 8 });
    expect(applyMention("@", 1, 0, "Toe")).toEqual({ text: "Toe ", caret: 4 });
  });
});

describe("the Sessions URL", () => {
  const inst = (agent: string) => ({ id: `${agent}@ledger`, agentId: agent }) as Instance;
  const roster = [inst("tic"), inst("tac"), inst("toe")];
  it("opens the lead when it names no agent, or one that isn't there", () => {
    expect(panesFrom({}, roster)).toEqual({ open: ["tic@ledger"], active: 0 });
    expect(panesFrom({ agent: "gone" }, roster)).toEqual({ open: ["tic@ledger"], active: 0 });
    expect(panesFrom({}, [])).toEqual({ open: [], active: 0 });
  });
  it("reads the active agent, by id or full instance id", () => {
    expect(panesFrom({ agent: "tac" }, roster)).toEqual({ open: ["tac@ledger"], active: 0 });
    expect(panesFrom({ agent: "toe@ledger" }, roster)).toEqual({ open: ["toe@ledger"], active: 0 });
  });
  it("reads a split, left and right, with its active side", () => {
    expect(panesFrom({ agent: "toe", panes: "tic,toe" }, roster)).toEqual({
      open: ["tic@ledger", "toe@ledger"],
      active: 1,
    });
    // An agent outside the split (a notification's link) takes the left pane.
    expect(panesFrom({ agent: "tac", panes: "tic,toe" }, roster)).toEqual({
      open: ["tac@ledger", "toe@ledger"],
      active: 0,
    });
    // A split that can't be made is one pane.
    expect(panesFrom({ agent: "tic", panes: "tic,gone" }, roster)).toEqual({
      open: ["tic@ledger"],
      active: 0,
    });
    expect(panesFrom({ agent: "tic", panes: "tic,tic" }, roster)).toEqual({
      open: ["tic@ledger"],
      active: 0,
    });
  });
  it("writes what it reads back", () => {
    for (const p of [
      { open: ["tic@ledger"], active: 0 },
      { open: ["tic@ledger", "toe@ledger"], active: 1 },
    ]) {
      for (const activity of [false, true]) {
        const s = searchFor("atlas@ledger", p, activity);
        expect(panesFrom(s, roster)).toEqual(p);
        expect(s.view).toBe(activity ? "activity" : undefined);
      }
    }
    expect(
      searchFor("atlas@ledger", { open: ["tic@ledger", "toe@ledger"], active: 1 }, false),
    ).toEqual({
      deployment: "atlas@ledger",
      agent: "toe",
      panes: "tic,toe",
      view: undefined,
    });
  });
});

describe("stoppedLine", () => {
  it("says why Foreman stopped the agent", () => {
    expect(stoppedLine("Leady", `couldn't start: poc has no branch "main"`)).toBe(
      `Leady stopped: couldn't start: poc has no branch "main". Messages for them wait until you start them again; they pick up their conversation.`,
    );
  });
  it("says only that it stopped when there's no reason (from before reasons were kept)", () => {
    expect(stoppedLine("Leady", "")).toBe(
      "Leady stopped. Messages for them wait until you start them again; they pick up their conversation.",
    );
  });
});
