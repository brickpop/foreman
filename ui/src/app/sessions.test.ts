import { describe, expect, it } from "vitest";
import type { Instance, Request, Team } from "../api/client";
import {
  clickTab,
  closePane,
  deploymentTabs,
  initialPanes,
  linePlaceholder,
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
