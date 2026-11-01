import type { Repo, RepoBranch, Team } from "../api/client";
import { agentOf } from "./model";

// The Repos section's model (wiki/product/features/repos.md).

/** Whose branch it is, by Foreman's names: wt/<agent>/… is that agent's,
 * lead/<team>/… that team's lead's; anything else is nobody's in particular. */
export function branchOwner(branch: string, teams: Team[]): string | undefined {
  const [kind, who] = branch.split("/");
  if (kind === "wt" && who) return agentOf(teams, who)?.soul.name ?? who;
  if (kind === "lead" && who) {
    const team = teams.find((t) => t.id === who);
    return team ? (team.config.agents[team.config.lead]?.name ?? team.config.lead) : who;
  }
  return undefined;
}

/** The branch a repo's page shows: the one asked for (the URL) when the
 * repo has it, else its first (main, when it has one). Undefined for a repo
 * the project hasn't. */
export function shownAt(
  repos: Repo[],
  repo: string,
  branch?: string,
): { repo: string; branch: string } | undefined {
  const r = repos.find((x) => x.name === repo);
  if (!r) return undefined;
  const b = r.branches.find((x) => x.name === branch) ?? r.branches[0];
  return { repo: r.name, branch: b?.name ?? r.main };
}

export type Worktree = {
  agent: string;
  /** The branch checked out in each of its repos, in the repos' order. */
  branches: { repo: string; branch: RepoBranch }[];
};

/** The agents' worktrees, from the branches checked out in each repo: one
 * per agent, by agent id. */
export function worktrees(repos: Repo[]): Worktree[] {
  const by = new Map<string, Worktree>();
  for (const repo of repos) {
    for (const branch of repo.branches) {
      if (!branch.agent) continue;
      const w = by.get(branch.agent) ?? { agent: branch.agent, branches: [] };
      w.branches.push({ repo: repo.name, branch });
      by.set(branch.agent, w);
    }
  }
  return [...by.values()].sort((a, b) => a.agent.localeCompare(b.agent));
}

/** The branch an agent has checked out: one, since `switch` moves all its
 * repos together; each different one, should they ever differ. "" with no
 * worktree yet. */
export function branchOf(repos: Repo[], agent: string): string {
  const w = worktrees(repos).find((w) => w.agent === agent);
  return [...new Set(w?.branches.map((b) => b.branch.name))].join(", ");
}

/** The repos that have a branch, those with commits beyond their main first:
 * where a work item's branch can be read. */
export function reposWith(repos: Repo[], branch: string): string[] {
  const having = repos.filter((r) => r.branches.some((b) => b.name === branch));
  const ahead = (r: Repo) => (r.branches.find((b) => b.name === branch)?.ahead ?? 0) > 0;
  return [...having.filter(ahead), ...having.filter((r) => !ahead(r))].map((r) => r.name);
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

/** A repo's name as people read it (the CEO, 2026-11-02): hyphens and
 * underscores as spaces, the first letter capital. foreman-ui is "Foreman ui".
 * Only how it shows: its id, folder and URL stay the name. */
export function repoTitle(name: string): string {
  const words = name.replace(/[-_]+/g, " ").trim();
  return words.charAt(0).toUpperCase() + words.slice(1);
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
