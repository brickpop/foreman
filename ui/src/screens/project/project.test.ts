import { describe, expect, it } from "vitest";
import type { Board, Card, MergeRequest, Request } from "../../api/client";
import { filesChanged, linkLine } from "../../app/model";
import { nameClash, repoDetail, repoHint } from "../../components/AddProject";
import { cell, columnLabel, layout } from "./Board";
import { sourceLabel } from "./Changes";

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

  it("shows every card: unknown statuses and lanes go to Other", () => {
    const b = board(
      [card("a", "todo", "high"), card("b", "wontfix", "high"), card("c", "done", "urgent")],
      ["high", "low"],
    );
    expect(layout(b)).toEqual({
      columns: ["todo", "done", "Other"],
      lanes: ["high", "low", "Other"],
    });
    expect(cell(b, b.cards[1])).toEqual({ column: "Other", lane: "high" });
    expect(cell(b, b.cards[2])).toEqual({ column: "done", lane: "Other" });
  });

  it("adds no Other when everything fits, and no lanes without a lane field", () => {
    const b = board([card("a", "todo", "")]);
    expect(layout(b)).toEqual({ columns: ["todo", "done"], lanes: [] });
    expect(cell(b, b.cards[0])).toEqual({ column: "todo", lane: "" });
  });
});

describe("Changes", () => {
  it("names each request's source", () => {
    const r = (kind: string, source = "agent") => ({ kind, source }) as Request;
    expect(
      [
        r("approval"),
        r("question"),
        r("finding"),
        r("review"),
        r("blocked", "foreman"),
        r("review", "cos"),
      ].map(sourceLabel),
    ).toEqual([
      "escalate(approval)",
      "escalate(question)",
      "escalate(finding)",
      "request_merge()",
      "raised by Foreman",
      "Chief of Staff",
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
    expect(linkLine({ ...mr, checksStatus: "" }, "none")).toContain("checks running");
    // A Chief of Staff change has no checks: it says how many files it changes.
    const cos = {
      branch: "foreman/config-1",
      authorInstance: "cos",
      checksStatus: "",
    } as MergeRequest;
    expect(linkLine(cos, "none", 1)).toBe("foreman/config-1 → main · 1 file changed");
    expect(linkLine(cos, "none", 2)).toBe("foreman/config-1 → main · 2 files changed");
  });
});

describe("Add a project", () => {
  it("describes a folder's repos", () => {
    expect(repoDetail([])).toBe("No git repos yet");
    expect(repoDetail(["mobile-app"])).toBe("1 git repo: mobile-app");
    expect(repoDetail(["billing-api", "billing-web"])).toBe(
      "2 git repos: billing-api, billing-web",
    );
  });
  it("refuses a name a workspace folder has, in any case", () => {
    const projects = ["Ledger"];
    const candidates = ["billing", "DevProject"];
    expect(nameClash("ledger", projects, candidates)).toBe("Ledger is already a project.");
    expect(nameClash("  devproject ", projects, candidates)).toBe(
      "DevProject/ already exists: pick it below to add it.",
    );
    expect(nameClash("billing-2", projects, candidates)).toBe("");
    expect(nameClash("   ", projects, candidates)).toBe("");
  });

  it("names the repo it creates, with placeholders until typed", () => {
    expect(repoHint("ledger", "api")).toMatch(/^Creates ledger\/api\/ and initializes/);
    expect(repoHint("", "")).toMatch(/^Creates <project>\/<repo>\/ /);
    expect(repoHint("ledger", "")).toMatch(/Leave it blank to add one later\.$/);
  });
});

describe("filesChanged", () => {
  it("counts the files of a unified diff", () => {
    expect(filesChanged("--- a/x\n+++ b/x\n+one\n--- a/y\n+++ b/y\n-two")).toBe(2);
    expect(filesChanged("")).toBe(0);
  });
});
