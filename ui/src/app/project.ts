import type { Deployment, Instance } from "../api/client";

// A project's page (wiki/product/features/project-page.md): its sections,
// listed in the sidebar under the project open, where the project opens, and
// the links into it from elsewhere.

export const sections = [
  { path: "session", label: "Session" },
  { path: "inbox", label: "Inbox" },
  { path: "repos", label: "Repos" },
  { path: "board", label: "Board" },
  { path: "wiki", label: "Wiki" },
  { path: "team", label: "Team" },
  { path: "settings", label: "Settings" },
] as const;

export type Section = (typeof sections)[number]["path"];

/** The deployment of the team on the project, if a team is on it (one at
 * most). The instances listed are the active deployments' only. */
export function teamOn(project: string, instances: Instance[]): string | undefined {
  return instances.find((i) => i.deploymentId.split("@")[1] === project)?.deploymentId;
}

/** Where a project opens: its Session while a team is on it, else its Wiki. */
export function projectHome(project: string, instances: Instance[]): Section {
  return teamOn(project, instances) ? "session" : "wiki";
}

/** An agent's session: its project's Session section, showing it. Takes an
 * instance id (agent@project). */
export function sessionOf(instance: string) {
  const [agent, project = ""] = instance.split("@");
  return { to: "/projects/$project/session", params: { project }, search: { agent } } as const;
}

/** A review, in its project's Inbox. */
export function reviewOf(project: string, review: string) {
  return { to: "/projects/$project/inbox/$review", params: { project, review } } as const;
}

/** The project's deployment: the running one, else the one that stopped last.
 * Its team's agents are the ones a `wt/<project>/<agent>` branch or a card's assignee
 * names: one team works on a project at a time, and a later team's agent of
 * the same id takes over what the earlier one left. */
export function deploymentOn(
  project: string,
  active: Deployment[],
  stopped: Deployment[],
): Deployment | undefined {
  return active.find((d) => d.projectId === project) ?? latestStopped(project, stopped);
}

/** The deployment that stopped last on the project: its team worked there
 * last (the one a Deploy there starts with), and named what it left. */
export function latestStopped(project: string, stopped: Deployment[]): Deployment | undefined {
  return stopped
    .filter((d) => d.projectId === project)
    .reduce<Deployment | undefined>(
      (a, d) => (!a || Date.parse(d.stoppedAt) > Date.parse(a.stoppedAt) ? d : a),
      undefined,
    );
}

/** The projects latest first: by when something last happened on each, as of
 * the page's load (the CEO, 2026-11-01). One added since comes first; with no
 * times yet, the order is left as it is. */
export function latestFirst<T extends { id: string }>(
  projects: T[],
  latest: Record<string, string> | undefined,
): T[] {
  if (!latest) return projects;
  const at = (p: T) => (p.id in latest ? Date.parse(latest[p.id]) : Number.POSITIVE_INFINITY);
  return [...projects].sort((a, b) => at(b) - at(a) || a.id.localeCompare(b.id));
}

export type TeamView = "agents" | "activity" | "inflight" | "merged";
export const teamViews: { view: TeamView; label: string }[] = [
  { view: "agents", label: "Agents" },
  { view: "activity", label: "Activity" },
  { view: "inflight", label: "In flight" },
  { view: "merged", label: "Merged" },
];

/** The Team section's view: the one in the URL, else Agents while there
 * are agents to show (the team's, or an earlier team's worktrees), else In
 * flight. */
export function teamView(asked: unknown, agents: boolean): TeamView {
  const known = teamViews.find((v) => v.view === asked)?.view;
  return known ?? (agents ? "agents" : "inflight");
}
