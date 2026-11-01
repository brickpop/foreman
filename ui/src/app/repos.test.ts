import { describe, expect, it } from "vitest";
import type { Repo, RepoBranch, Team } from "../api/client";
import {
  branchOf,
  branchOwner,
  bytes,
  crumbs,
  folderOf,
  graph,
  matching,
  readmeOf,
  reposWith,
  repoTitle,
  shownAt,
  worktrees,
} from "./repos";

const atlas = {
  id: "atlas",
  config: {
    lead: "mara",
    agents: { mara: { name: "Mara" }, tove: { name: "Tove" } },
  },
} as unknown as Team;

describe("the Repos section's model", () => {
  it("names whose branch it is", () => {
    expect(branchOwner("wt/tove/LED-44", [atlas])).toBe("Tove");
    expect(branchOwner("lead/atlas/integration", [atlas])).toBe("Mara");
    expect(branchOwner("wt/gone/x", [atlas])).toBe("gone"); // a soul since removed
    expect(branchOwner("main", [atlas])).toBeUndefined();
    expect(branchOwner("feature/x", [atlas])).toBeUndefined();
  });

  it("breaks a path into crumbs and finds its folder", () => {
    expect(crumbs("src/lib/a.go")).toEqual([
      { name: "src", path: "src" },
      { name: "lib", path: "src/lib" },
      { name: "a.go", path: "src/lib/a.go" },
    ]);
    expect(crumbs("")).toEqual([]);
    expect(folderOf("src/lib/a.go")).toBe("src/lib");
    expect(folderOf("README.md")).toBe("");
  });

  it("tells sizes", () => {
    expect([bytes(12), bytes(3482), bytes(1258291)]).toEqual(["12 B", "3.4 KB", "1.2 MB"]);
  });
});

describe("worktrees", () => {
  const branch = (name: string, more: Partial<RepoBranch> = {}) =>
    ({ name, ahead: 0, uncommitted: false, agent: "", ...more }) as RepoBranch;
  const repos = [
    {
      name: "api",
      project: false,
      main: "main",
      branches: [
        branch("main"),
        branch("wt/tove/LED-44", { agent: "tove", ahead: 2, uncommitted: true }),
        branch("lead/atlas/integration", { agent: "mara" }),
        branch("wt/ines/old", { ahead: 1 }),
      ],
    },
    {
      name: "web",
      project: false,
      main: "trunk",
      branches: [branch("trunk"), branch("wt/tove/LED-44", { agent: "tove" })],
    },
    { name: "project", project: true, main: "main", branches: [branch("main")] },
  ] as Repo[];

  it("gives each agent its checked-out branch per repo", () => {
    expect(
      worktrees(repos).map((w) => [w.agent, w.branches.map((b) => `${b.repo}:${b.branch.name}`)]),
    ).toEqual([
      ["mara", ["api:lead/atlas/integration"]],
      ["tove", ["api:wt/tove/LED-44", "web:wt/tove/LED-44"]],
    ]);
  });

  it("gives the branch an agent is on, once however many repos it spans", () => {
    expect(branchOf(repos, "tove")).toBe("wt/tove/LED-44");
    expect(branchOf(repos, "mara")).toBe("lead/atlas/integration");
    expect(branchOf(repos, "ines")).toBe(""); // a branch but no worktree
    expect(branchOf([], "tove")).toBe("");
    // Repos on different branches (never through switch): each one, in the repos' order.
    const split = [
      repos[0],
      { ...repos[1], branches: [branch("wt/tove/other", { agent: "tove" })] },
    ];
    expect(branchOf(split, "tove")).toBe("wt/tove/LED-44, wt/tove/other");
  });

  it("finds where a branch is read, repos with commits on it first", () => {
    expect(reposWith(repos, "wt/tove/LED-44")).toEqual(["api", "web"]);
    expect(reposWith(repos, "wt/ines/old")).toEqual(["api"]);
    expect(reposWith(repos, "nowhere")).toEqual([]);
    const webFirst = [repos[1], repos[0]];
    expect(reposWith(webFirst, "wt/tove/LED-44")).toEqual(["api", "web"]);
  });

  it("shows the branch asked for, else the repo's first; nothing for a repo it hasn't", () => {
    expect(shownAt(repos, "web", "wt/tove/LED-44")).toEqual({
      repo: "web",
      branch: "wt/tove/LED-44",
    });
    expect(shownAt(repos, "web", "gone")).toEqual({ repo: "web", branch: "trunk" });
    expect(shownAt(repos, "api")).toEqual({ repo: "api", branch: "main" });
    expect(shownAt(repos, "nope", "main")).toBeUndefined();
    expect(shownAt([], "api")).toBeUndefined();
  });
});

describe("the history graph", () => {
  const c = (hash: string, ...parents: string[]) => ({ hash, parents });
  it("keeps a straight history in one lane", () => {
    expect(graph([c("c", "b"), c("b", "a"), c("a")])).toEqual([
      { lane: 0, top: [], bottom: [[0, 0]], width: 1 },
      { lane: 0, top: [[0, 0]], bottom: [[0, 0]], width: 1 },
      { lane: 0, top: [[0, 0]], bottom: [], width: 1 },
    ]);
  });
  it("opens a lane for a merge's second parent and joins it back", () => {
    expect(graph([c("m", "a", "b"), c("b", "a"), c("a")])).toEqual([
      {
        lane: 0,
        top: [],
        bottom: [
          [0, 0],
          [0, 1],
        ],
        width: 2,
      },
      {
        lane: 1,
        top: [
          [0, 0],
          [1, 1],
        ],
        bottom: [
          [0, 0],
          [1, 0],
        ],
        width: 2,
      },
      { lane: 0, top: [[0, 0]], bottom: [], width: 1 },
    ]);
  });
  it("gives each branch tip its own lane, meeting where they part", () => {
    const rows = graph([c("x", "a"), c("y", "a"), c("a", "r"), c("r")]);
    expect(rows.map((r) => r.lane)).toEqual([0, 1, 0, 0]);
    expect(rows[1]).toEqual({
      lane: 1,
      top: [[0, 0]],
      bottom: [
        [0, 0],
        [1, 0],
      ],
      width: 2,
    });
    expect(rows[2].top).toEqual([[0, 0]]); // both lanes waited for a: one line into it
  });
  it("reuses a freed lane rather than widening, and leaves a parent not loaded open", () => {
    const rows = graph([c("m", "a", "b"), c("b", "a"), c("a", "older"), c("n", "q", "p")]);
    // b's lane 1 is free again once b came: the tip n takes it, its second
    // parent the next; a's parent, not loaded, keeps lane 0 open below.
    expect(rows[3]).toEqual({
      lane: 1,
      top: [[0, 0]],
      bottom: [
        [0, 0],
        [1, 1],
        [1, 2],
      ],
      width: 3,
    });
  });
});

describe("repoTitle", () => {
  it("reads a slug as words, only the first capital", () => {
    expect(repoTitle("foreman-ui")).toBe("Foreman ui");
    expect(repoTitle("api")).toBe("Api");
    expect(repoTitle("my__data--set_")).toBe("My data set");
  });
});

describe("readmeOf", () => {
  const file = (name: string) => ({ name, kind: "file" });

  it("opens the README first, whatever its case, then index.md", () => {
    expect(readmeOf([file("index.md"), file("Readme.MD"), file("a.go")])).toBe("Readme.MD");
    expect(readmeOf([file("README")])).toBe("README");
    expect(readmeOf([file("main.go"), file("INDEX.md")])).toBe("INDEX.md");
  });

  it("opens nothing without one, and never a folder or a near name", () => {
    expect(readmeOf([{ name: "README.md", kind: "dir" }, file("readme-old.md")])).toBeUndefined();
    expect(readmeOf([])).toBeUndefined();
  });
});

describe("matching", () => {
  const b = (name: string) => ({ name });

  it("keeps the branches with the query in their name, any case; all for none", () => {
    const all = [b("main"), b("wt/tove/LED-44"), b("lead/atlas/release")];
    expect(matching(all, "led").map((x) => x.name)).toEqual(["wt/tove/LED-44"]);
    expect(matching(all, "  ")).toBe(all);
    expect(matching(all, "nope")).toEqual([]);
  });
});
