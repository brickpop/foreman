import type { Deployment, Instance } from "../api/client";

// A project's page (wiki/product/features/project-page.md): its sections,
// listed in the sidebar under the project open, where the project opens, and
// the links into it from elsewhere.

export const sections = [
  { path: "session", label: "Session" },
  { path: "inbox", label: "Inbox" },
  { path: "wiki", label: "Wiki" },
  { path: "board", label: "Board" },
  { path: "repos", label: "Repos" },
  { path: "deployment", label: "Deployment" },
  { path: "activity", label: "Activity" },
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

/** The team that worked on the project last, from the stopped deployments:
 * the one a Deploy there starts with. */
export function lastTeam(project: string, stopped: Deployment[]): string | undefined {
  return stopped
    .filter((d) => d.projectId === project)
    .reduce<Deployment | undefined>(
      (a, d) => (!a || Date.parse(d.stoppedAt) > Date.parse(a.stoppedAt) ? d : a),
      undefined,
    )?.teamId;
}
