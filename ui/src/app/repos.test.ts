import { describe, expect, it } from "vitest";
import type { Repo, RepoBranch, Team } from "../api/client";
import {
  branchOf,
  branchOwner,
  bytes,
  crumbs,
  folderOf,
  isMarkdown,
  reposWith,
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

describe("the Repositories tab's model", () => {
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

  it("tells Markdown and sizes", () => {
    expect(isMarkdown("docs/README.MD")).toBe(true);
    expect(isMarkdown("main.go")).toBe(false);
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

  it("shows the repo and branch asked for, else a sensible default", () => {
    expect(shownAt(repos, "web", "wt/tove/LED-44")).toEqual({
      repo: "web",
      branch: "wt/tove/LED-44",
    });
    expect(shownAt(repos, "web", "gone")).toEqual({ repo: "web", branch: "trunk" });
    // An unknown repo falls back to the first; the branch asked for stays when it has it.
    expect(shownAt(repos, "nope", "wt/tove/LED-44")).toEqual({
      repo: "api",
      branch: "wt/tove/LED-44",
    });
    expect(shownAt(repos, "nope", "gone")).toEqual({ repo: "api", branch: "main" });
    expect(shownAt(repos)).toEqual({ repo: "api", branch: "main" });
    expect(shownAt([])).toBeUndefined();
  });
});
