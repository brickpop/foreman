import { describe, expect, it } from "vitest";
import type { Repo, RepoBranch, Team } from "../api/client";
import { readable } from "./model";
import {
  branchesOf,
  branchOwner,
  bytes,
  cleanupDone,
  cleanupGroups,
  cleanupPick,
  cleanupSections,
  crumbs,
  folderOf,
  folderPage,
  graph,
  matching,
  pickedBranches,
  readmeOf,
  reposWith,
  shownAt,
  uncommittedWork,
  worktreeRows,
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
    const on = "atlas@ledger";
    const owner = (b: string, teams: Team[], d: string | undefined) =>
      branchOwner(b, "ledger", teams, d);
    expect(owner("wt/ledger/tove/LED-44", [atlas], on)).toBe("Tove");
    expect(owner("wt/ledger/tove/a/b", [atlas], on)).toBe("Tove"); // a topic with a slash
    expect(owner("lead/ledger/atlas/integration", [atlas], on)).toBe("Mara");
    expect(owner("wt/ledger/gone/x", [atlas], on)).toBe("gone"); // a soul since removed
    expect(owner("lead/ledger/gone/x", [atlas], on)).toBe("gone"); // a team since removed
    expect(owner("main", [atlas], on)).toBeUndefined();
    expect(owner("feature/x", [atlas], on)).toBeUndefined();
    expect(owner("wt/ledger/tove", [atlas], on)).toBeUndefined(); // no topic
    expect(owner("wt//tove/x", [atlas], on)).toBeUndefined();
    // The names from before branches carried the project: nobody's in particular.
    expect(owner("wt/tove/LED-44", [atlas], on)).toBeUndefined();
    expect(owner("lead/atlas/integration", [atlas], on)).toBeUndefined();
    // Another project's, in a repo they share: its instance id.
    expect(owner("wt/suite/tove/x", [atlas], on)).toBe("tove@suite");
    expect(owner("lead/suite/atlas/integration", [atlas], on)).toBe("mara@suite");
    expect(owner("lead/suite/gone/integration", [atlas], on)).toBe("gone@suite");
    // Never a team on the project: the id as it is.
    expect(owner("wt/ledger/tove/LED-44", [atlas], undefined)).toBe("tove");
    // The team on the project names it, even when another team has a tove too.
    const orion = {
      id: "orion",
      config: { lead: "tove", agents: { tove: { name: "QA" } } },
    } as unknown as Team;
    expect(owner("wt/ledger/tove/LED-44", [atlas, orion], "orion@ledger")).toBe("QA");
    expect(owner("lead/ledger/atlas/integration", [atlas, orion], "orion@ledger")).toBe("Mara");
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
      id: "api",
      hq: false,
      defaultBranch: "main",
      branches: [
        branch("main"),
        branch("wt/tove/LED-44", { agent: "tove", ahead: 2, uncommitted: true }),
        branch("lead/atlas/integration", { agent: "mara" }),
        branch("wt/ines/old", { ahead: 1 }),
      ],
      idle: [{ agent: "ivo", uncommitted: false }],
    },
    {
      id: "web",
      hq: false,
      defaultBranch: "trunk",
      branches: [branch("trunk"), branch("wt/tove/LED-44", { agent: "tove" })],
      idle: [
        { agent: "ivo", uncommitted: true },
        { agent: "mara", uncommitted: false },
      ],
    },
    {
      id: "hq",
      hq: true,
      defaultBranch: "main",
      branches: [branch("main")],
      idle: [],
    },
  ] as Repo[];

  it("gives each agent its checked-out branch per repo, and the repos it has open on none", () => {
    expect(
      worktrees(repos).map((w) => [
        w.agent,
        w.branches.map((b) => `${b.repo}:${b.branch.name}`),
        w.idle.map((i) => `${i.repo}${i.uncommitted ? "*" : ""}`),
      ]),
    ).toEqual([
      // Open only on no branch: an agent with a worktree all the same.
      ["ivo", [], ["api", "web*"]],
      ["mara", ["api:lead/atlas/integration"], ["web"]],
      ["tove", ["api:wt/tove/LED-44", "web:wt/tove/LED-44"], []],
    ]);
  });

  it("names the worktrees holding uncommitted work, on a branch or on none", () => {
    expect(uncommittedWork(worktrees(repos), readable)).toEqual(["ivo's Web", "tove's Api"]);
    // A branch's ahead commits are committed work: not listed.
    const committed = repos.map((r) => ({
      ...r,
      branches: r.branches.map((b) => ({ ...b, uncommitted: false })),
      idle: r.idle.map((t) => ({ ...t, uncommitted: false })),
    }));
    expect(uncommittedWork(worktrees(committed), readable)).toEqual([]);
    expect(uncommittedWork([], readable)).toEqual([]);
  });

  it("gives the branch an agent is on, once however many repos it spans", () => {
    expect(branchesOf(repos, "mara")).toEqual(["lead/atlas/integration"]);
    expect(branchesOf(repos, "ines")).toEqual([]); // a branch but no worktree
    expect(branchesOf(repos, "ivo")).toEqual([]); // idle
    expect(branchesOf([], "tove")).toEqual([]);
    // Repos on different branches (never through switch): each one, in the repos' order.
    const split = [
      repos[0],
      { ...repos[1], branches: [branch("wt/tove/other", { agent: "tove" })] },
    ];
    expect(branchesOf(split, "tove")).toEqual(["wt/tove/LED-44", "wt/tove/other"]);
  });

  it("lists the shown agent's worktrees with a branch checked out, code repos only", () => {
    expect(worktreeRows(repos, "tove")).toEqual([
      { repo: "api", branch: "wt/tove/LED-44" },
      { repo: "web", branch: "wt/tove/LED-44" },
    ]);
    // Open on no branch (web) is no row; a branch no worktree holds (ines) neither.
    expect(worktreeRows(repos, "mara")).toEqual([
      { repo: "api", branch: "lead/atlas/integration" },
    ]);
    expect(worktreeRows(repos, "ivo")).toEqual([]);
    expect(worktreeRows(repos, "ines")).toEqual([]);
    // Never the HQ, even with an agent's branch there.
    const inHq = repos.map((r) =>
      r.hq ? { ...r, branches: [branch("x", { agent: "tove" })] } : r,
    );
    expect(worktreeRows(inHq, "tove").map((w) => w.repo)).toEqual(["api", "web"]);
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

describe("folderPage", () => {
  const file = (name: string) => ({ name, kind: "file" });

  // The other way round from the root's: a folder a link names is a wiki's
  // section, fronted by its index.md.
  it("opens index.md first, then the README, whatever their case", () => {
    expect(folderPage([file("README.md"), file("Index.MD")])).toBe("Index.MD");
    expect(folderPage([file("a.md"), file("readme")])).toBe("readme");
  });

  it("opens nothing without one, and never a folder or a near name", () => {
    expect(folderPage([{ name: "index.md", kind: "dir" }, file("index-old.md")])).toBeUndefined();
    expect(folderPage([])).toBeUndefined();
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

describe("cleanupGroups", () => {
  const b = (
    branch: string,
    agents: boolean,
    state: string,
    stays?: string,
    stray?: string,
    repo = "api",
    free?: string,
  ) => ({ repo, branch, agents, state, legacy: false, stays, stray, free });
  // An inactive agent's branch, free in each repo it has it checked out.
  const free = (branch: string, state: string, repo: string, agent = "ana") =>
    b(branch, true, state, undefined, undefined, repo, agent);
  const keys = (rows: { key: string }[]) => rows.map((r) => r.key);
  const list = [
    b("feature/done", false, "merged"),
    b("feature/done", false, "merged", undefined, undefined, "web"),
    b("feature/base", false, "superseded"),
    b("wt/p/tove/done", true, "merged"),
    b("wt/tove/old", true, "merged"),
    b("wt/p/tove/draft", true, "superseded"),
    b("wt/p/tove/wip", true, "unmerged"),
    b("wt/p/tove/main", true, "merged", "checked out by tove"),
    b("release/2", false, "merged", "an open review merges into it"),
    free("wt/p/ana/done", "merged", "api"),
    free("wt/p/ana/done", "merged", "web"),
    free("wt/p/ana/old", "superseded", "api"),
    b(
      "wt/leady/doc",
      true,
      "merged",
      "checked out in /old/agents/leady/api",
      "/old/agents/leady/api",
    ),
  ];
  it("puts what loses nothing first (the agents' then the CEO's), then inactive agents' branches, unmerged work, strays; those that stay apart", () => {
    const { groups, staying } = cleanupGroups(list);
    expect(groups.map((g) => [g.key, g.on, keys(g.rows)])).toEqual([
      ["agents-merged", true, ["api:wt/p/tove/done", "api:wt/tove/old"]],
      ["agents-superseded", true, ["api:wt/p/tove/draft"]],
      ["yours-merged", true, ["api:feature/done", "web:feature/done"]],
      ["yours-superseded", true, ["api:feature/base"]],
      // Nothing is freed unasked: off at first, one row for every repo ana has it in.
      ["free-merged", false, ["ana:wt/p/ana/done"]],
      ["free-superseded", false, ["ana:wt/p/ana/old"]],
      ["unmerged", false, ["api:wt/p/tove/wip"]],
      // In a stray worktree, merged or not: its worktree goes too, so never ticked at first.
      ["stray", false, ["api:wt/leady/doc"]],
    ]);
    expect(keys(staying)).toEqual(["api:wt/p/tove/main", "api:release/2"]);
  });
  it("sends a free row as every repo it stands for, and other rows as their own", () => {
    const rows = cleanupGroups(list).groups.flatMap((g) => g.rows);
    expect(pickedBranches(rows, new Set(["ana:wt/p/ana/done", "api:feature/base"]))).toEqual([
      { repo: "api", branch: "feature/base" },
      { repo: "api", branch: "wt/p/ana/done" },
      { repo: "web", branch: "wt/p/ana/done" },
    ]);
  });
  it("keeps two agents' frees of one branch name apart", () => {
    const rows = cleanupGroups([
      free("wt/p/x/main", "merged", "api", "ana"),
      free("wt/p/x/main", "merged", "web", "ivo"),
    ]).groups.flatMap((g) => g.rows);
    expect(keys(rows)).toEqual(["ana:wt/p/x/main", "ivo:wt/p/x/main"]);
  });
  it("follows the sections' order, folding the ones that may lose work", () => {
    const order = cleanupSections.map((s) => s.key);
    const { groups } = cleanupGroups(list);
    const seen = groups.map((g) => order.indexOf(g.section));
    expect(seen).toEqual([...seen].sort((x, y) => x - y));
    expect(cleanupSections.filter((s) => s.folded).map((s) => s.key)).toEqual([
      "unmerged",
      "stray",
    ]);
  });
  it("ticks the merged and superseded at first, by repo: the same name in two repos is two picks", () => {
    expect([...cleanupPick(cleanupGroups(list).groups)]).toEqual([
      "api:wt/p/tove/done",
      "api:wt/tove/old",
      "api:wt/p/tove/draft",
      "api:feature/done",
      "web:feature/done",
      "api:feature/base",
    ]);
  });
  it("leaves out empty groups", () => {
    expect(cleanupGroups([b("feature/done", false, "merged")]).groups.map((g) => g.key)).toEqual([
      "yours-merged",
    ]);
    expect(cleanupGroups([])).toEqual({ groups: [], staying: [] });
  });
});

describe("cleanupDone", () => {
  const c = (what: string, repo: string, branch: string, agent = "", why = "") => ({
    agent,
    repo,
    what,
    branch,
    why,
  });
  it("counts the branches only, and says which worktrees went idle, by agent, repos as they show", () => {
    const done = cleanupDone(
      {
        removed: [
          c("detached", "api", "wt/p/tove/x", "tove"),
          c("branch", "api", "wt/p/tove/x"),
          c("detached", "web", "wt/p/tove/x", "tove"),
          c("branch", "web", "wt/p/tove/x"),
          c("detached", "api", "wt/p/ana/y", "ana"),
          c("branch", "api", "wt/p/ana/y"),
          c("worktree", "api", "wt/leady/doc"),
          c("branch", "api", "wt/leady/doc"),
        ],
        kept: [],
      },
      readable,
    );
    expect(done.title).toBe("4 branches deleted");
    expect(done.body).toBe(
      "Left idle at the same commit: tove's Api, Web worktrees; ana's Api worktree.\nForeman keeps a copy of each for 30 days.",
    );
  });
  it("lists what was kept and why, the 30-day copy last", () => {
    const done = cleanupDone(
      {
        removed: [c("branch", "api", "feature/done")],
        kept: [
          c("branch", "api", "wt/p/tove/x", "", "tove started meanwhile"),
          c("branch", "web", "wt/p/tove/x", "", "tove started meanwhile"),
        ],
      },
      readable,
    );
    expect(done.title).toBe("1 branch deleted");
    expect(done.body).toBe(
      "Kept: Api wt/p/tove/x (tove started meanwhile), Web wt/p/tove/x (tove started meanwhile).\nForeman keeps a copy of each for 30 days.",
    );
  });
});
