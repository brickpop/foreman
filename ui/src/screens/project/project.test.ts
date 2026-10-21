import { describe, expect, it } from "vitest";
import type { Board, Card, MergeRequest, Project, Request } from "../../api/client";
import { forgeOf, linkLine, mainOf } from "../../app/model";
import { cellLook, columnLabel, laneLabel, lanesShown, moveOf, revealBy } from "./Board";
import { sourceLabel } from "./Pipeline";

const card = (path: string, column: string, lane: string) => ({ path, column, lane }) as Card;
const board = (cards: Card[], lanes: string[] = []) =>
  ({ columns: ["todo", "done"], lanes, cards }) as unknown as Board;

describe("Board", () => {
  it("labels columns", () => {
    expect(["todo", "in-progress", "review", "done", "blocked_on_ceo"].map(columnLabel)).toEqual([
      "To do",
      "In progress",
      "Review",
      "Done",
      "Blocked on ceo",
    ]);
  });

  it("labels the cells for no value", () => {
    expect(columnLabel("")).toBe("No status");
    expect(laneLabel("", "priority")).toBe("No priority");
    expect(laneLabel("high", "priority")).toBe("High priority");
  });

  it("writes what a drop changes, and nothing to a cell for no value", () => {
    const flat = board([card("a", "todo", "")]);
    expect(moveOf(flat, flat.cards[0], "done", "")).toEqual({ column: "done", lane: "" });
    expect(moveOf(flat, flat.cards[0], "todo", "")).toBeUndefined(); // where it is
    expect(moveOf(flat, flat.cards[0], "", "")).toBeUndefined(); // removing the status isn't a move

    const lanes = board([card("a", "todo", "high"), card("b", "", "")], ["high", "low", ""]);
    const [a, b] = lanes.cards;
    expect(moveOf(lanes, a, "todo", "low")).toEqual({ column: "todo", lane: "low" });
    expect(moveOf(lanes, a, "done", "high")).toEqual({ column: "done", lane: "" }); // lane kept
    expect(moveOf(lanes, a, "done", "")).toBeUndefined(); // removing the lane isn't either
    expect(moveOf(lanes, b, "todo", "")).toEqual({ column: "todo", lane: "" }); // gets a status
    expect(moveOf(lanes, b, "todo", "high")).toEqual({ column: "todo", lane: "high" });
  });

  it("lists in a column only the lanes with its cards, in lane order, and every lane while dragging", () => {
    const lanes = ["high", "medium", "low", ""];
    const cards = [card("a", "todo", "low"), card("b", "todo", ""), card("c", "todo", "high")];
    expect(lanesShown(lanes, cards, false)).toEqual(["high", "low", ""]);
    expect(lanesShown(lanes, [], false)).toEqual([]);
    expect(lanesShown(lanes, [], true)).toEqual(lanes);
  });

  it("keeps a drop cell's size while a card is over it: only its color changes", () => {
    const size = (look: string) => look.split(" ").filter((c) => !c.startsWith("border"));
    expect(size(cellLook(true, true))).toEqual(size(cellLook(true, false)));
    expect(cellLook(true, false)).toContain("min-h-12");
    expect(cellLook(true, true)).toContain("border-acc");
    expect(cellLook(false, false)).toBe("border-transparent"); // no drag, or a cell that takes none
  });

  it("scrolls an opened card out from under the sheet, 16px clear of it", () => {
    // A 1600px window: the 460px sheet leaves 1124px, so a card may end at 1124.
    expect(revealBy(1000, 1600)).toBe(0); // already beside it
    expect(revealBy(1124, 1600)).toBe(0); // exactly at the edge
    expect(revealBy(1400, 1600)).toBe(276); // under it
    expect(revealBy(1400.2, 1600)).toBe(277); // never short of a pixel
  });
});

describe("Changes", () => {
  it("names each request's source", () => {
    const r = (kind: string, source = "agent") => ({ kind, source }) as Request;
    expect(
      [r("approval"), r("question"), r("finding"), r("review"), r("blocked", "foreman")].map(
        sourceLabel,
      ),
    ).toEqual([
      "escalate(approval)",
      "escalate(question)",
      "escalate(finding)",
      "request_merge()",
      "raised by Foreman",
    ]);
  });

  it("writes a merge request's link line", () => {
    const mr = {
      repo: "web",
      branch: "lead/atlas/integration",
      checksStatus: "pass",
    } as MergeRequest;
    expect(linkLine(mr, "forgejo")).toBe("web · lead/atlas/integration → main · checks pass");
    expect(linkLine({ ...mr, forgePrNumber: 128 }, "forgejo")).toBe(
      "PR #128 on Forgejo · web · lead/atlas/integration → main · checks pass",
    );
    expect(linkLine({ ...mr, conflicting: true }, "none")).toBe(
      "web · lead/atlas/integration → main · conflicts with main",
    );
    expect(linkLine({ ...mr, checksStatus: "" }, "none")).toBe(
      "web · lead/atlas/integration → main",
    );
    expect(linkLine({ ...mr, checksStatus: "running" }, "none", "develop")).toBe(
      "web · lead/atlas/integration → develop · checks running",
    );
    expect(linkLine({ ...mr, conflicting: true }, "none", "develop")).toBe(
      "web · lead/atlas/integration → develop · conflicts with develop",
    );
  });
});

describe("forgeOf", () => {
  const project = (kind: string) =>
    ({ config: { forge: { kind, url: "", projectRepo: "" } } }) as unknown as Project;
  it("is the project's forge, or none without a project", () => {
    expect(forgeOf(project("github"))).toBe("github");
    expect(forgeOf(project("none"))).toBe("none");
    expect(forgeOf(undefined)).toBe("none");
  });
});

describe("mainOf", () => {
  const project = {
    repos: [
      { name: "api", path: "/h/api", main: "main", problem: "" },
      { name: "poc", path: "/h/poc", main: "master", problem: "" },
    ],
  } as unknown as Project;
  it("is the repo's own main branch", () => {
    expect(mainOf(project, "poc")).toBe("master");
    expect(mainOf(project, "api")).toBe("main");
  });
  it("is main for a repo it doesn't have, or before the project loads", () => {
    expect(mainOf(project, "gone")).toBe("main");
    expect(mainOf(undefined, "poc")).toBe("main");
  });
});
