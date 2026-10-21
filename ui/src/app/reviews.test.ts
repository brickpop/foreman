import { describe, expect, it } from "vitest";
import type { MergeRequest, Review, ReviewComment } from "../api/client";
import {
  commentAt,
  diffKey,
  drafts,
  hasSince,
  lineKey,
  mergeHeads,
  mergeLabel,
  openRepos,
  placeComments,
  reviewGroups,
} from "./reviews";

const mr = (repo: string, more: Partial<MergeRequest> = {}) =>
  ({ repo, state: "open", reviewedHead: "", conflicting: false, ...more }) as MergeRequest;
const review = (mrs: MergeRequest[]) =>
  ({
    title: "Export invoices",
    entry: "backlog/x.md",
    status: "needs-review",
    mergeRequests: mrs,
  }) as Review;
const mains = (repo: string) => (repo === "web" ? "trunk" : "main");

const comment = (more: Partial<ReviewComment>) =>
  ({
    repo: "api",
    path: "a.go",
    side: "new",
    line: 3,
    now: 3,
    outdated: false,
    state: "sent",
    ...more,
  }) as ReviewComment;

describe("Reviews", () => {
  it("lists the repos still open, each with its main", () => {
    const v = review([mr("api"), mr("web"), mr("cli", { state: "merged" })]);
    expect(openRepos(v, mains).map((r) => [r.repo, r.main])).toEqual([
      ["api", "main"],
      ["web", "trunk"],
    ]);
  });

  it("labels the merge by the mains ticked", () => {
    expect(mergeLabel([{ main: "main" }, { main: "main" }])).toBe("Merge to main");
    expect(mergeLabel([{ main: "main" }, { main: "trunk" }])).toBe("Merge");
  });

  it("has a since-your-last-look only once changes were requested on an open repo", () => {
    expect(hasSince(review([mr("api")]))).toBe(false);
    expect(hasSince(review([mr("api", { reviewedHead: "abc", state: "merged" })]))).toBe(false);
    expect(hasSince(review([mr("api"), mr("web", { reviewedHead: "abc" })]))).toBe(true);
  });

  it("places comments on their line, moved; general and outdated apart", () => {
    const moved = comment({ id: "c1", line: 3, now: 5 });
    const removed = comment({ id: "c2", side: "old", line: 7, now: 7 });
    const changed = comment({ id: "c3", outdated: true });
    const general = comment({ id: "c4", repo: "", path: "", side: "", line: 0, now: 0 });
    const twice = comment({ id: "c5", line: 5, now: 5 });
    const merged = comment({ id: "c6", repo: "web", now: 5 });
    const all = [moved, removed, changed, general, twice, merged];
    // The lines the diff shows: api's a.go, line 5 kept (it was 3 on main) and 7 removed.
    const shown = new Set(
      [
        diffKey("api", "a.go", { kind: "ctx", text: " x", old: 3, new: 5 }),
        diffKey("api", "a.go", { kind: "del", text: "-y", old: 7 }),
        diffKey("api", "a.go", { kind: "hunk", text: "@@ -1 +1 @@" }),
      ].filter((k) => k !== undefined),
    );
    expect(shown.size).toBe(2);
    const main = placeComments(all, false, shown);
    expect(main.at.get(lineKey("api", "a.go", "new", 5))?.map((c) => c.id)).toEqual(["c1", "c5"]);
    expect(main.at.get(lineKey("api", "a.go", "old", 7))?.map((c) => c.id)).toEqual(["c2"]);
    expect(main.at.has(lineKey("api", "a.go", "new", 3))).toBe(false);
    expect(main.general.map((c) => c.id)).toEqual(["c4"]);
    expect(main.outdated.map((c) => c.id)).toEqual(["c3"]);
    // web merged: its diff isn't shown, so its comment is listed apart, not lost.
    expect(main.elsewhere.map((c) => c.id)).toEqual(["c6"]);
    // Since the last look, main's side isn't shown: a removed line's comment can't sit there.
    expect(placeComments(all, true, shown).outdated.map((c) => c.id)).toEqual(["c2", "c3"]);
    expect(drafts([comment({ state: "draft" }), moved, comment({ state: "draft" })])).toBe(2);
  });

  it("starts a comment on the side its line is on", () => {
    const add = { kind: "add" as const, text: "+x := 1", new: 12 };
    expect(commentAt("api", "a.go", add, "h", false)).toEqual({
      repo: "api",
      path: "a.go",
      side: "new",
      line: 12,
      commit: "h",
      snippet: "x := 1",
      body: "",
    });
    const del = { kind: "del" as const, text: "-y := 2", old: 9 };
    expect(commentAt("api", "a.go", del, "h", false)?.side).toBe("old");
    expect(commentAt("api", "a.go", del, "h", true)).toBeUndefined();
    expect(
      commentAt("api", "a.go", { kind: "ctx", text: " z", old: 4, new: 6 }, "h", true)?.line,
    ).toBe(6);
    expect(
      commentAt("api", "a.go", { kind: "hunk", text: "@@ -1 +1 @@" }, "h", false),
    ).toBeUndefined();
  });
});

describe("reviewGroups", () => {
  it("splits what needs the CEO, what waits on a lead and the last ones done", () => {
    const at = (id: string, status: string) => ({ id, status }) as Review;
    const all = [
      at("r1", "merged"),
      at("r2", "needs-review"),
      at("r3", "changes-requested"),
      at("r4", "closed"),
      at("r5", "needs-review"),
      at("r6", "merged"),
    ];
    const g = reviewGroups(all, 2);
    expect(g.needsYou.map((v) => v.id)).toEqual(["r2", "r5"]);
    expect(g.waiting.map((v) => v.id)).toEqual(["r3"]);
    expect(g.done.map((v) => v.id)).toEqual(["r1", "r4"]);
  });
});

describe("mergeHeads", () => {
  const diffs = [
    { repo: "api", main: "main", head: "a1b2", since: "", diff: "" },
    { repo: "web", main: "trunk", head: "c3d4", since: "", diff: "" },
    { repo: "docs", main: "main", head: "", since: "", diff: "" },
  ];
  it("names each repo ticked with the head its diff was shown at", () => {
    expect(mergeHeads(diffs, ["api", "web"])).toEqual({ api: "a1b2", web: "c3d4" });
    expect(mergeHeads(diffs, ["web"])).toEqual({ web: "c3d4" });
  });
  it("is nothing to send until every ticked repo has a diff with its head", () => {
    expect(mergeHeads(undefined, ["api"])).toBeUndefined();
    expect(mergeHeads(diffs, ["api", "docs"])).toBeUndefined();
    expect(mergeHeads(diffs, ["api", "gone"])).toBeUndefined();
  });
});
