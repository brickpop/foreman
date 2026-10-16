import { describe, expect, it } from "vitest";
import type { Instance, Request, Team } from "../api/client";
import {
  clickTab,
  closePane,
  countdown,
  deploymentTabs,
  initialPanes,
  isLineShortcut,
  linePlaceholder,
  lineShortcut,
  rosterOrder,
  toggleSplit,
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
      { id: "atlas@ledger", label: "Atlas · ledger", needsYou: false },
      { id: "orion@relay", label: "orion · relay", needsYou: true },
    ]);
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
    let p = initialPanes(roster);
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
