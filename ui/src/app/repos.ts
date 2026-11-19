import type { Cleanup, CleanupBranch, Repo, RepoBranch, Team } from "../api/client";
import { agentOf, plural } from "./model";

// The Repos section's model (wiki/product/features/repos.md).

/** Whose branch it is, by Foreman's names: wt/<project>/<agent>/… is that
 * agent's in the project's deployment (team@project, deploymentOn),
 * lead/<project>/<team>/… that team's lead's; anything else is nobody's in
 * particular (the names from before branches carried the project too).
 * Another project's, in a repo they share, is named agent@project
 * (its instance id): this project's deployment doesn't say whose it is there. */
export function branchOwner(
  branch: string,
  project: string,
  teams: Team[],
  deployment: string | undefined,
): string | undefined {
  const [kind, on, who, topic] = branch.split("/");
  if (!on || !who || !topic) return undefined;
  if (kind === "wt") {
    if (on !== project) return `${who}@${on}`;
    return agentOf(teams, deployment ?? "", who)?.soul.name ?? who;
  }
  if (kind === "lead") {
    const team = teams.find((t) => t.id === who);
    if (on !== project) return `${team?.config.lead ?? who}@${on}`;
    return team ? (team.config.agents[team.config.lead]?.name ?? team.config.lead) : who;
  }
  return undefined;
}

/** The branch a repo's page shows: the one asked for (the URL) when the
 * repo has it, else its first (its default branch, when it has it). Undefined for a repo
 * the project hasn't. */
export function shownAt(
  repos: Repo[],
  repo: string,
  branch?: string,
): { repo: string; branch: string } | undefined {
  const r = repos.find((x) => x.id === repo);
  if (!r) return undefined;
  const b = r.branches.find((x) => x.name === branch) ?? r.branches[0];
  return { repo: r.id, branch: b?.name ?? r.defaultBranch };
}

export type Worktree = {
  agent: string;
  /** The branch checked out in each of its repos, in the repos' order. */
  branches: { repo: string; branch: RepoBranch }[];
  /** Its repos open on no branch, in the repos' order. */
  idle: { repo: string; uncommitted: boolean }[];
};

/** The agents' worktrees, from the branches checked out in each repo and the
 * repos open on none: one per agent, by agent id. An agent with no repo open
 * has none. */
export function worktrees(repos: Repo[]): Worktree[] {
  const by = new Map<string, Worktree>();
  const of = (agent: string) => {
    const w = by.get(agent) ?? { agent, branches: [], idle: [] };
    by.set(agent, w);
    return w;
  };
  for (const repo of repos) {
    for (const branch of repo.branches) {
      if (branch.agent) of(branch.agent).branches.push({ repo: repo.id, branch });
    }
    for (const t of repo.idle) {
      of(t.agent).idle.push({ repo: repo.id, uncommitted: t.uncommitted });
    }
  }
  return [...by.values()].sort((a, b) => a.agent.localeCompare(b.agent));
}

/** The repos where each worktree holds changes not committed, as "agent's
 * repo" (repo as name shows it): what Clean up removes only when asked. In
 * the worktrees' order, each's in its repos'. */
export function uncommittedWork(trees: Worktree[], name: (repo: string) => string): string[] {
  return trees.flatMap((w) =>
    [
      ...w.branches.filter((b) => b.branch.uncommitted).map((b) => b.repo),
      ...w.idle.filter((t) => t.uncommitted).map((t) => t.repo),
    ].map((repo) => `${w.agent}'s ${name(repo)}`),
  );
}

/** The branch an agent has checked out: one, since `switch` moves all its
 * repos together; each different one, should they ever differ. None while
 * idle or with no repo open. */
export function branchesOf(repos: Repo[], agent: string): string[] {
  const w = worktrees(repos).find((w) => w.agent === agent);
  return [...new Set(w?.branches.map((b) => b.branch.name))];
}

export type BranchPlace = { repo: string; ahead: number; uncommitted: boolean };

/** Each code repo that has a branch, for the Session's branch menu, in two
 * groups: those where it holds work (commits beyond the default branch, or
 * changes not committed), and the rest; each in the repos' order. */
export function branchPlaces(
  repos: Repo[],
  branch: string,
): { changed: BranchPlace[]; unchanged: BranchPlace[] } {
  const places = repos.flatMap((r) => {
    const b = !r.hq && r.branches.find((x) => x.name === branch);
    return b ? [{ repo: r.id, ahead: b.ahead, uncommitted: b.uncommitted }] : [];
  });
  const work = (p: BranchPlace) => p.ahead > 0 || p.uncommitted;
  return { changed: places.filter(work), unchanged: places.filter((p) => !work(p)) };
}

/** The repos that have a branch, those with commits beyond their main first:
 * where a work item's branch can be read. */
export function reposWith(repos: Repo[], branch: string): string[] {
  const having = repos.filter((r) => r.branches.some((b) => b.name === branch));
  const ahead = (r: Repo) => (r.branches.find((b) => b.name === branch)?.ahead ?? 0) > 0;
  return [...having.filter(ahead), ...having.filter((r) => !ahead(r))].map((r) => r.id);
}

/** A path's breadcrumbs: each folder (and the file) with the path to it. */
export function crumbs(path: string): { name: string; path: string }[] {
  const parts = path.split("/").filter(Boolean);
  return parts.map((name, i) => ({ name, path: parts.slice(0, i + 1).join("/") }));
}

/** The folder a path sits in ("" at the root). */
export function folderOf(path: string): string {
  return path.split("/").slice(0, -1).join("/");
}

/** The file a folder opens with, as GitHub does: its README, else its
 * index.md (a wiki's), whatever their case; undefined with neither. */
export function readmeOf(entries: { name: string; kind: string }[]): string | undefined {
  const files = entries.filter((e) => e.kind === "file").map((e) => e.name);
  return (
    files.find((n) => /^readme(\.(md|markdown|txt))?$/i.test(n)) ??
    files.find((n) => /^index\.(md|markdown)$/i.test(n))
  );
}

/** The page a folder a link opened shows (the CEO, 2026-11-03): its
 * index.md, else its README; none, and the folder lists what it holds. */
export function folderPage(entries: { name: string; kind: string }[]): string | undefined {
  const files = entries.filter((e) => e.kind === "file").map((e) => e.name);
  return (
    files.find((n) => /^index\.(md|markdown)$/i.test(n)) ??
    files.find((n) => /^readme(\.(md|markdown|txt))?$/i.test(n))
  );
}

/** The branches whose name has the query in it, whatever its case. */
export function matching<T extends { name: string }>(branches: T[], query: string): T[] {
  const q = query.trim().toLowerCase();
  return q ? branches.filter((b) => b.name.toLowerCase().includes(q)) : branches;
}

/** A size as people read it: 12 B, 3.4 KB, 1.2 MB. */
export function bytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

/** One row of the history graph: the commit's lane, and the lines through
 * the row's top half (into the commit, or passing by) and bottom half (to
 * its parents, or passing by), each from one lane to another. */
export type GraphRow = {
  lane: number;
  top: [number, number][];
  bottom: [number, number][];
  /** Lanes the row needs drawn. */
  width: number;
};

/** Lays commits (children before parents, as the log lists them) out in
 * lanes, as `git log --graph` does: a commit takes the lane that waits for it
 * (the leftmost, when several do), its first parent continues that lane, and
 * any other parent takes a free one. A lane is freed when its commit comes,
 * and kept in place otherwise, so lines never jump sideways. */
export function graph(commits: { hash: string; parents: string[] }[]): GraphRow[] {
  const lanes: (string | null)[] = []; // the commit each lane waits for
  const free = () => {
    const i = lanes.indexOf(null);
    return i < 0 ? lanes.length : i;
  };
  return commits.map((c) => {
    const waiting = lanes.indexOf(c.hash);
    const lane = waiting < 0 ? free() : waiting; // < 0: a branch's tip
    const top: [number, number][] = [];
    lanes.forEach((h, i) => {
      if (h !== null) top.push([i, h === c.hash ? lane : i]);
    });
    const passing = lanes.flatMap((h, i) => (h !== null && h !== c.hash ? [i] : []));
    for (let i = 0; i < lanes.length; i++) if (lanes[i] === c.hash) lanes[i] = null;
    const bottom: [number, number][] = passing.map((i) => [i, i]);
    c.parents.forEach((p, n) => {
      let to = lanes.indexOf(p);
      if (to < 0) {
        to = n === 0 ? lane : free();
        lanes[to] = p;
      }
      bottom.push([lane, to]);
    });
    while (lanes.length > 0 && lanes[lanes.length - 1] === null) lanes.pop();
    const used = [lane, ...top.flat(), ...bottom.flat()];
    return { lane, top, bottom, width: Math.max(...used) + 1 };
  });
}

/** What a repo holds on a branch, for its row in the branch menu. */
export function placeLine(p: BranchPlace): string {
  const work = [p.ahead > 0 && plural(p.ahead, "commit"), p.uncommitted && "uncommitted changes"];
  return work.filter(Boolean).join(" · ") || "nothing new";
}

/** One row of the branch cleanup, what the CEO ticks: a branch in one repo;
 * or an inactive agent's branch in every repo that agent has it checked out,
 * which are freed together or not at all. */
export type CleanupRow = {
  /** What a pick holds. */
  key: string;
  /** The branch as its first repo has it: its name, state and why it stays. */
  branch: CleanupBranch;
  /** Every repo it stands for, each sent on Delete. */
  picks: CleanupBranch[];
};

/** The branch cleanup's rows, in the order the branches came: one per branch
 * and repo, except a free one's siblings (the same free agent and branch),
 * which are one row. */
export function cleanupRows(branches: CleanupBranch[]): CleanupRow[] {
  const rows = new Map<string, CleanupRow>();
  for (const b of branches) {
    const key = b.free ? `${b.free}:${b.branch}` : `${b.repo}:${b.branch}`;
    const row = rows.get(key);
    if (row) row.picks.push(b);
    else rows.set(key, { key, branch: b, picks: [b] });
  }
  return [...rows.values()];
}

/** What Delete sends for the rows picked: every repo of each. */
export function pickedBranches(
  rows: CleanupRow[],
  pick: Set<string>,
): { repo: string; branch: string }[] {
  return rows
    .filter((r) => pick.has(r.key))
    .flatMap((r) => r.picks.map(({ repo, branch }) => ({ repo, branch })));
}

/** One section of the branch cleanup: its groups, folded or open at first. */
export type CleanupSection = {
  key: "agents" | "yours" | "free" | "unmerged" | "stray";
  title: string;
  note?: string;
  /** Folded at first: those whose deletion may lose work (the CEO, 2026-11-13). */
  folded: boolean;
};

/** The sections, the safest to delete first (the CEO, 2026-11-13). */
export const cleanupSections: CleanupSection[] = [
  { key: "agents", title: "Agents' branches, merged", folded: false },
  {
    key: "yours",
    title: "Your branches, merged",
    note: "Deleted here only: a copy upstream stays.",
    folded: false,
  },
  {
    key: "free",
    title: "Checked out by agents not at work",
    note: "Picked, the agent's worktrees go idle at the same commit. Nothing in them is lost.",
    folded: false,
  },
  { key: "unmerged", title: "Agents' branches, not merged", folded: true },
  {
    key: "stray",
    title: "In stray worktrees",
    note: "checked out in a folder no agent of any project has (one moved or deleted by hand): picked, that worktree is removed first, unless it holds changes not committed.",
    folded: true,
  },
];

/** One group of the branch cleanup: the branches it ticks or unticks at once. */
export type CleanupGroup = {
  key: string;
  section: CleanupSection["key"];
  label: string;
  hint: string;
  /** Ticked at first. */
  on: boolean;
  rows: CleanupRow[];
};

type Kind = Omit<CleanupGroup, "key" | "rows"> & { state?: string };

// Ticked at first exactly where deleting loses nothing: merged or superseded, in no stray
// worktree, checked out by no agent (the CEO: nothing is freed unasked).
const groupKinds: Kind[] = [
  { section: "agents", state: "merged", label: "Merged", hint: "nothing lost", on: true },
  {
    section: "agents",
    state: "superseded",
    label: "Superseded",
    hint: "all of it is in another branch",
    on: true,
  },
  { section: "yours", state: "merged", label: "Merged", hint: "nothing lost", on: true },
  {
    section: "yours",
    state: "superseded",
    label: "Superseded",
    hint: "all of it is in an agent's branch",
    on: true,
  },
  { section: "free", state: "merged", label: "Merged", hint: "nothing lost", on: false },
  {
    section: "free",
    state: "superseded",
    label: "Superseded",
    hint: "all of it is in another branch",
    on: false,
  },
  { section: "unmerged", label: "Not merged", hint: "work only here", on: false },
  {
    section: "stray",
    label: "Checked out elsewhere",
    hint: "its worktree goes too, with what git ignores there",
    on: false,
  },
];

/** The section a branch is offered in, if any: one checked out in a stray worktree in its own,
 * whatever its work; one an inactive agent of this project can free, in its own; one that
 * stays otherwise, none. The CEO's unmerged ones are never offered. */
function sectionOf(b: CleanupBranch): CleanupSection["key"] | undefined {
  if (b.stray) return "stray";
  if (b.free) return "free";
  if (b.stays) return undefined;
  if (!b.agents) return "yours";
  return b.state === "unmerged" ? "unmerged" : "agents";
}

/** The branch cleanup's groups (wiki/product/features/repos.md#branch-cleanup), in order and
 * without the empty ones: merged and superseded first, the agents' then the CEO's, ticked, as
 * deleting them loses nothing; then those inactive agents have checked out, off; then the
 * agents' unmerged work and those in stray worktrees, off. Branches that can't go are apart,
 * in staying. */
export function cleanupGroups(branches: CleanupBranch[]): {
  groups: CleanupGroup[];
  staying: CleanupRow[];
} {
  const rows = cleanupRows(branches);
  const groups = groupKinds
    .map(({ state, ...k }) => ({
      ...k,
      key: state ? `${k.section}-${state}` : k.section,
      rows: rows.filter(
        (r) =>
          sectionOf(r.branch) === k.section && (state === undefined || r.branch.state === state),
      ),
    }))
    .filter((g) => g.rows.length > 0);
  return { groups, staying: rows.filter((r) => sectionOf(r.branch) === undefined) };
}

/** What the branch cleanup ticks at first: the rows of the groups on. */
export function cleanupPick(groups: CleanupGroup[]): Set<string> {
  return new Set(groups.filter((g) => g.on).flatMap((g) => g.rows.map((r) => r.key)));
}

/** The branch cleanup's toast after a Delete: how many branches went, then, one line each
 * and only those that apply, the worktrees left idle (by agent), what was kept and why, and
 * the 30-day copy; each repo as name shows it. */
export function cleanupDone(
  done: Cleanup,
  name: (repo: string) => string,
): { title: string; body: string } {
  const branches = done.removed.filter((c) => c.what === "branch").length;
  const idle = new Map<string, string[]>();
  for (const c of done.removed.filter((c) => c.what === "detached")) {
    idle.set(c.agent, [...(idle.get(c.agent) ?? []), name(c.repo)]);
  }
  const lines = [
    idle.size > 0 &&
      `Left idle at the same commit: ${[...idle]
        .map(
          ([agent, repos]) =>
            `${agent}'s ${repos.join(", ")} ${repos.length === 1 ? "worktree" : "worktrees"}`,
        )
        .join("; ")}.`,
    done.kept.length > 0 &&
      `Kept: ${done.kept.map((k) => `${name(k.repo)} ${k.branch} (${k.why})`).join(", ")}.`,
    "Foreman keeps a copy of each for 30 days.",
  ];
  return {
    title: `${plural(branches, "branch", "branches")} deleted`,
    body: lines.filter(Boolean).join("\n"),
  };
}
