import { describe, expect, it } from "vitest";
import type { Instance, Project, Request } from "../api/client";
import {
  badgeText,
  cycle,
  cycleStep,
  elsewhere,
  groupMark,
  isPopoverShortcut,
  markLabel,
  markOf,
  needsYou,
  type RailProject,
  railModel,
} from "./rail";

const project = (id: string, name = "") => ({ id, name }) as Project;
const instance = (deploymentId: string, status: string) => ({ deploymentId, status }) as Instance;
const request = (projectId: string, status = "open", kind = "question") =>
  ({ projectId, status, kind }) as Request;
const row = (id: string, inbox = 0, busy?: string): RailProject => ({ id, name: id, inbox, busy });

describe("railModel", () => {
  it("counts each project's Inbox: its open requests, reviews included", () => {
    const m = railModel(
      [project("ledger"), project("relay"), project("harbor")],
      [],
      [
        request("ledger"),
        request("ledger", "open", "review"),
        request("relay", "resolved", "review"),
        request("elsewhere"),
      ],
    );
    expect(m.projects).toEqual([
      { id: "ledger", name: "Ledger", inbox: 2 },
      { id: "relay", name: "Relay", inbox: 0 },
      { id: "harbor", name: "Harbor", inbox: 0 },
    ]);
  });

  it("puts the hidden projects in their own group, still marked when they need the CEO", () => {
    const m = railModel(
      [
        project("ledger"),
        { ...project("relay"), hidden: true },
        { ...project("harbor"), hidden: true },
      ],
      [],
      [request("harbor")],
    );
    expect(m.projects).toEqual([{ id: "ledger", name: "Ledger", inbox: 0 }]);
    expect(m.hidden).toEqual([
      { id: "relay", name: "Relay", inbox: 0 },
      { id: "harbor", name: "Harbor", inbox: 1 },
    ]);
  });

  it("shows a project by the name the CEO gave it, else by its id made readable", () => {
    const m = railModel([project("ledger", "Ledger · EUR"), project("big-ledger")], [], []);
    expect(m.projects.map((p) => p.name)).toEqual(["Ledger · EUR", "Big ledger"]);
  });

  it("marks a project's Session while its team is busy, not at rest", () => {
    const m = railModel(
      [project("ledger"), project("relay"), project("harbor"), project("big-ledger")],
      [
        instance("atlas@ledger", "idle"),
        instance("atlas@ledger", "running"),
        instance("orion@relay", "idle"),
        instance("orion@relay", "asleep"),
        instance("vesper@big-ledger", "awaiting"),
      ],
      [],
    );
    expect(m.projects.map((p) => p.busy)).toEqual(["running", undefined, undefined, "awaiting"]);
  });
});

describe("an item's one mark", () => {
  it("is the needs-you badge when something waits, busy or not", () => {
    expect(markOf(row("a", 2, "running"))).toEqual({ count: 2 });
  });

  it("else the busy dot, else nothing", () => {
    expect(markOf(row("a", 0, "queued"))).toEqual({ status: "queued" });
    expect(markOf(row("a"))).toBeUndefined();
  });

  it("folds the hidden projects into one: the badge total, else the first busy dot", () => {
    expect(groupMark([row("a", 0, "running"), row("b", 2), row("c", 1)])).toEqual({ count: 3 });
    expect(groupMark([row("a"), row("b", 0, "queued"), row("c", 0, "running")])).toEqual({
      status: "queued",
    });
    expect(groupMark([row("a")])).toBeUndefined();
    expect(groupMark([])).toBeUndefined();
  });

  it("caps the badge at 9+", () => {
    expect(badgeText(9)).toBe("9");
    expect(badgeText(10)).toBe("9+");
  });

  it("is heard in the item's name", () => {
    expect(markLabel({ count: 1 })).toBe(". 1 request needs you");
    expect(markLabel({ count: 3 }, " in other projects")).toBe(
      ". 3 requests need you in other projects",
    );
    expect(markLabel({ status: "running" })).toBe(". running");
    expect(markLabel(undefined)).toBe("");
    expect(needsYou(1)).toBe("1 request needs you"); // a badge's tooltip
  });
});

describe("the Foreman tile's badge", () => {
  const model = { projects: [row("a", 2), row("b", 1)], hidden: [row("c", 4)] };

  it("counts every project but the open one, hidden ones included", () => {
    expect(elsewhere(model, "a")).toBe(5);
    expect(elsewhere(model, "c")).toBe(3);
  });

  it("counts them all on the Dashboard", () => {
    expect(elsewhere(model, undefined)).toBe(7);
  });
});

describe("cycling projects", () => {
  const model = { projects: [row("a"), row("b"), row("c")], hidden: [row("h")] };

  it("moves to the next or previous visible project, round the ends", () => {
    expect(cycle(model, "a", 1)).toBe("b");
    expect(cycle(model, "c", 1)).toBe("a");
    expect(cycle(model, "a", -1)).toBe("c");
  });

  it("goes to the first visible one from a hidden project, or from none", () => {
    expect(cycle(model, "h", 1)).toBe("a");
    expect(cycle(model, "h", -1)).toBe("a");
    expect(cycle(model, undefined, 1)).toBe("a");
  });

  it("has nowhere to go with no visible project", () => {
    expect(cycle({ projects: [], hidden: [row("h")] }, "h", 1)).toBeUndefined();
  });
});

describe("the project keys", () => {
  const none = { code: "KeyP", ctrlKey: false, altKey: false, shiftKey: false, metaKey: false };
  const ctrlAlt = { ...none, ctrlKey: true, altKey: true };

  it("opens the popover on Ctrl+Alt+P only, by key position", () => {
    expect(isPopoverShortcut(ctrlAlt)).toBe(true);
    expect(isPopoverShortcut({ ...ctrlAlt, altKey: false })).toBe(false);
    expect(isPopoverShortcut({ ...ctrlAlt, shiftKey: true })).toBe(false);
    expect(isPopoverShortcut({ ...ctrlAlt, metaKey: true })).toBe(false);
    expect(isPopoverShortcut({ ...ctrlAlt, code: "KeyK" })).toBe(false);
  });

  it("cycles on Ctrl+Alt+↓ / ↑ and leaves other keys alone", () => {
    expect(cycleStep({ ...ctrlAlt, code: "ArrowDown" })).toBe(1);
    expect(cycleStep({ ...ctrlAlt, code: "ArrowUp" })).toBe(-1);
    expect(cycleStep({ ...ctrlAlt, code: "ArrowLeft" })).toBe(0);
    expect(cycleStep({ ...none, ctrlKey: true, code: "ArrowDown" })).toBe(0);
  });
});
