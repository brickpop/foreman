import type { CleanupBranch, Repo, RepoBranch, Team } from "../api/client";
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
  const r = repos.find((x) => x.name === repo);
  if (!r) return undefined;
  const b = r.branches.find((x) => x.name === branch) ?? r.branches[0];
  return { repo: r.name, branch: b?.name ?? r.defaultBranch };
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
      if (branch.agent) of(branch.agent).branches.push({ repo: repo.name, branch });
    }
    for (const t of repo.idle) {
      of(t.agent).idle.push({ repo: repo.name, uncommitted: t.uncommitted });
    }
  }
  return [...by.values()].sort((a, b) => a.agent.localeCompare(b.agent));
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
    const b = !r.project && r.branches.find((x) => x.name === branch);
    return b ? [{ repo: r.name, ahead: b.ahead, uncommitted: b.uncommitted }] : [];
  });
  const work = (p: BranchPlace) => p.ahead > 0 || p.uncommitted;
  return { changed: places.filter(work), unchanged: places.filter((p) => !work(p)) };
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

/** One group of the branch cleanup: the branches it ticks or unticks at once. */
export type CleanupGroup = {
  key: string;
  /** Its section: the agents' branches, the CEO's, or those in stray worktrees. */
  section: "agents" | "yours" | "stray";
  label: string;
  hint: string;
  /** Ticked at first. */
  on: boolean;
  branches: CleanupBranch[];
};

type Kind = Omit<CleanupGroup, "key" | "branches"> & { state?: string };

const groupKinds: Kind[] = [
  { section: "agents", state: "merged", label: "Merged", hint: "nothing lost", on: true },
  {
    section: "agents",
    state: "superseded",
    label: "Superseded",
    hint: "all of it is in another branch",
    on: true,
  },
  { section: "agents", state: "unmerged", label: "Not merged", hint: "work only here", on: false },
  { section: "yours", state: "merged", label: "Merged", hint: "nothing lost", on: false },
  {
    section: "yours",
    state: "superseded",
    label: "Superseded",
    hint: "all of it is in an agent's branch",
    on: false,
  },
  {
    section: "stray",
    label: "Checked out elsewhere",
    hint: "its worktree goes too, with what git ignores there",
    on: false,
  },
];

/** The section a branch is offered in, if any: one checked out in a stray worktree in its own,
 * whatever its work; one that stays otherwise, none. */
function sectionOf(b: CleanupBranch): CleanupGroup["section"] | undefined {
  if (b.stray) return "stray";
  if (b.stays) return undefined;
  return b.agents ? "agents" : "yours";
}

/** The branch cleanup's groups (wiki/product/features/repos.md#branch-cleanup), in order and
 * without the empty ones: the agents' merged and superseded ticked at first, deleting them
 * loses nothing; their unmerged work, the CEO's own branches and those in stray worktrees off.
 * Branches that can't go are apart, in staying. */
export function cleanupGroups(branches: CleanupBranch[]): {
  groups: CleanupGroup[];
  staying: CleanupBranch[];
} {
  const groups = groupKinds
    .map(({ state, ...k }) => ({
      ...k,
      key: state ? `${k.section}-${state}` : k.section,
      branches: branches.filter(
        (b) => sectionOf(b) === k.section && (state === undefined || b.state === state),
      ),
    }))
    .filter((g) => g.branches.length > 0);
  return { groups, staying: branches.filter((b) => sectionOf(b) === undefined) };
}

/** What the branch cleanup ticks at first: the branches of the groups on. */
export function cleanupPick(groups: CleanupGroup[]): Set<string> {
  return new Set(groups.filter((g) => g.on).flatMap((g) => g.branches.map((b) => b.branch)));
}
