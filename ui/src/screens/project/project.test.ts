import { describe, expect, it } from "vitest";
import type { Board, Card, MergeRequest, Project, Request } from "../../api/client";
import { forgeOf, linkLine, mainOf } from "../../app/model";
import { columnLabel, laneLabel, moveOf } from "./Board";
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
    const mr = { branch: "lead/atlas/integration", checksStatus: "pass" } as MergeRequest;
    expect(linkLine(mr, "forgejo")).toBe("lead/atlas/integration → main · checks pass");
    expect(linkLine({ ...mr, forgePrNumber: 128 }, "forgejo")).toBe(
      "PR #128 on Forgejo · lead/atlas/integration → main · checks pass",
    );
    expect(linkLine({ ...mr, conflicting: true }, "none")).toBe(
      "lead/atlas/integration → main · conflicts with main",
    );
    expect(linkLine({ ...mr, checksStatus: "" }, "none")).toBe("lead/atlas/integration → main");
    expect(linkLine({ ...mr, checksStatus: "running" }, "none", "develop")).toBe(
      "lead/atlas/integration → develop · checks running",
    );
    expect(linkLine({ ...mr, conflicting: true }, "none", "develop")).toBe(
      "lead/atlas/integration → develop · conflicts with develop",
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
