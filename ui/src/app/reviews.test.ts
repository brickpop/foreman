import { describe, expect, it } from "vitest";
import type { Deployment, MergeRequest, Review, ReviewComment, Team } from "../api/client";
import {
  commentAt,
  decidedAs,
  diffKey,
  dismissChoices,
  dismissLabel,
  dismissRequest,
  drafts,
  hasSince,
  keptLines,
  lineKey,
  lostDrafts,
  lostNote,
  mergeHeads,
  mergeLabel,
  openRepos,
  placeComments,
  reviewGroups,
  reviewTeam,
  statusLabel,
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

const comment = (more: Partial<ReviewComment>) =>
  ({
    repo: "api",
    path: "a.go",
    side: "new",
    line: 3,
    now: 3,
    nowPath: "a.go",
    outdated: false,
    state: "sent",
    ...more,
  }) as ReviewComment;

describe("Reviews", () => {
  it("names how each repo was decided, and none while open", () => {
    const v = review([
      mr("api", { state: "merged" }),
      mr("web", { state: "rejected" }),
      mr("cli", { state: "withdrawn" }),
      mr("doc"),
    ]);
    expect(["api", "web", "cli", "doc", "gone"].map((r) => decidedAs(v, r))).toEqual([
      "merged",
      "dismissed",
      "withdrawn",
      "",
      "",
    ]);
  });
  it("lists the repos still open, each with its target", () => {
    const v = review([
      mr("api", { target: "main" }),
      mr("web", { target: "release/2.0" }),
      mr("cli", { state: "merged", target: "main" }),
    ]);
    expect(openRepos(v).map((r) => [r.repo, r.target])).toEqual([
      ["api", "main"],
      ["web", "release/2.0"],
    ]);
  });

  it("labels the merge by the targets ticked", () => {
    expect(mergeLabel([{ target: "release/2.0" }, { target: "release/2.0" }])).toBe(
      "Merge to release/2.0",
    );
    expect(mergeLabel([{ target: "main" }, { target: "trunk" }])).toBe("Merge");
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
    // Written on old.go, which the lead renamed a.go since.
    const renamed = comment({ id: "c7", path: "old.go", line: 9, now: 5, nowPath: "a.go" });
    const all = [moved, removed, changed, general, twice, merged, renamed];
    // The lines the diff shows: api's a.go, line 5 kept (it was 3 on the target) and 7 removed.
    const shown = new Set(
      [
        diffKey("api", "a.go", { kind: "ctx", text: " x", old: 3, new: 5 }),
        diffKey("api", "a.go", { kind: "del", text: "-y", old: 7 }),
        diffKey("api", "a.go", { kind: "hunk", text: "@@ -1 +1 @@" }),
      ].filter((k) => k !== undefined),
    );
    expect(shown.size).toBe(2);
    const main = placeComments(all, false, shown);
    expect(main.at.get(lineKey("api", "a.go", "new", 5))?.map((c) => c.id)).toEqual([
      "c1",
      "c5",
      "c7",
    ]);
    expect(main.at.get(lineKey("api", "a.go", "old", 7))?.map((c) => c.id)).toEqual(["c2"]);
    expect(main.at.has(lineKey("api", "a.go", "new", 3))).toBe(false);
    expect(main.general.map((c) => c.id)).toEqual(["c4"]);
    expect(main.outdated.map((c) => c.id)).toEqual(["c3"]);
    // web merged: its diff isn't shown, so its comment is listed apart, not lost.
    expect(main.elsewhere.map((c) => c.id)).toEqual(["c6"]);
    // Since the last look, the target's side isn't shown: a removed line's comment can't sit there.
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
      at("r4", "dismissed"),
      at("r5", "needs-review"),
      at("r6", "withdrawn"),
    ];
    const g = reviewGroups(all, 3);
    expect(g.needsYou.map((v) => v.id)).toEqual(["r2", "r5"]);
    expect(g.waiting.map((v) => v.id)).toEqual(["r3"]);
    expect(g.done.map((v) => v.id)).toEqual(["r1", "r4", "r6"]);
    expect(statusLabel("withdrawn")).toBe("Withdrawn");
  });
});

describe("Dismiss", () => {
  const v = { deploymentId: "atlas@ledger" } as Review;
  const teams = [{ id: "atlas", config: { name: "Atlas" } }] as Team[];
  const atlas = [{ id: "atlas@ledger" }] as Deployment[];
  const running = dismissChoices({ name: "Atlas", live: true }, "Ledger");
  const stopped = dismissChoices({ name: "Atlas", live: false }, "Ledger");
  const shape = (c: (typeof running)[number]) => [c.value, c.stopTeam, c.dropBranches, c.tells];

  it("knows the review's team by name, and whether it still runs", () => {
    expect(reviewTeam(v, atlas, teams)).toEqual({ name: "Atlas", live: true });
    expect(reviewTeam(v, [{ id: "atlas@other" } as Deployment], [])).toEqual({
      name: "atlas",
      live: false,
    });
  });

  it("drops branches only with their team, and tells only a team that keeps working", () => {
    expect(running.map(shape)).toEqual([
      ["keep", false, false, true],
      ["stop", true, false, false],
      ["drop", true, true, false],
    ]);
    // Nobody runs: nothing to stop, nobody to tell.
    expect(stopped.map(shape)).toEqual([
      ["keep", false, false, false],
      ["drop", false, true, false],
    ]);
    expect(running[0].label).toBe("Atlas keeps working on Ledger; the branches stay");
  });

  it("sends the note, and says so, only when someone hears it", () => {
    const [keep, stop] = running;
    expect(dismissRequest(keep, "  not now ")).toEqual({
      note: "not now",
      dropBranches: false,
      stopTeam: false,
    });
    expect(dismissLabel(keep, "not now")).toBe("Dismiss and send");
    expect(dismissLabel(keep, "  ")).toBe("Dismiss now");
    // A note typed, then a choice that stops the team: it goes nowhere.
    expect(dismissRequest(stop, "not now").note).toBe("");
    expect(dismissLabel(stop, "not now")).toBe("Dismiss now");
  });

  it("warns when a note typed won't reach the lead", () => {
    expect(lostNote(running[0], "not now", "Mara")).toBeUndefined();
    expect(lostNote(running[1], "  ", "Mara")).toBeUndefined();
    expect(lostNote(running[1], "not now", "Mara")).toBe(
      "Mara won't get your note: the team stops.",
    );
    expect(lostNote(stopped[0], "not now", "Mara")).toBe(
      "Mara won't get your note: the team isn't running.",
    );
  });

  it("says which branches stayed, and why", () => {
    expect(keptLines({ kept: [] })).toBe("");
    expect(
      keptLines({
        kept: [
          { repo: "api", branch: "lead/ledger/atlas/x", why: "changes in the way" },
          { repo: "web", branch: "wt/ledger/tove/y", why: "busy" },
        ],
      }),
    ).toBe("lead/ledger/atlas/x in api: changes in the way.\nwt/ledger/tove/y in web: busy.");
  });
});

describe("mergeHeads", () => {
  const diffs = [
    {
      repo: "api",
      target: "main",
      head: "a1b2",
      since: "",
      diff: "",
      gone: false,
      targetGone: false,
    },
    {
      repo: "web",
      target: "trunk",
      head: "c3d4",
      since: "",
      diff: "",
      gone: false,
      targetGone: false,
    },
    { repo: "docs", target: "main", head: "", since: "", diff: "", gone: false, targetGone: false },
    {
      repo: "cli",
      target: "release/1.0",
      head: "",
      since: "",
      diff: "",
      gone: false,
      targetGone: true,
    },
  ];
  it("names each repo ticked with the head its diff was shown at", () => {
    expect(mergeHeads(diffs, ["api", "web"])).toEqual({ api: "a1b2", web: "c3d4" });
    expect(mergeHeads(diffs, ["web"])).toEqual({ web: "c3d4" });
  });
  it("is nothing to send until every ticked repo has a diff with its head", () => {
    expect(mergeHeads(undefined, ["api"])).toBeUndefined();
    expect(mergeHeads(diffs, ["api", "docs"])).toBeUndefined();
    expect(mergeHeads(diffs, ["api", "gone"])).toBeUndefined();
    // Its target deleted: no head until the CEO moves it to another target.
    expect(mergeHeads(diffs, ["api", "cli"])).toBeUndefined();
  });
});

describe("lostDrafts", () => {
  it("counts what a merge would lose, one or several", () => {
    expect(lostDrafts(1)).toBe("1 draft comment won't be sent and will be lost. Merge anyway?");
    expect(lostDrafts(3)).toBe("3 draft comments won't be sent and will be lost. Merge anyway?");
  });
});
