import { createRootRoute, createRoute, createRouter } from "@tanstack/react-router";
import { Agent } from "../screens/Agent";
import { ChiefOfStaff } from "../screens/ChiefOfStaff";
import { Dashboard } from "../screens/Dashboard";
import { Project, type ProjectTab } from "../screens/Project";
import { Review } from "../screens/Review";
import { Reviews } from "../screens/Reviews";
import { Sessions } from "../screens/Sessions";
import { Settings } from "../screens/Settings";
import { Team, type TeamTab } from "../screens/Team";
import { cosSearch } from "./chiefOfStaff";
import { Shell } from "./Shell";
import type { SessionsSearch } from "./sessions";
import { settingsSearch } from "./settings";

// One route per screen (wiki/product/features/app-shell.md), with typed path
// and search params: project / tab / entry, team / tab, review / since.

const root = createRootRoute({ component: Shell });

const dashboard = createRoute({ getParentRoute: () => root, path: "/", component: Dashboard });

const sessions = createRoute({
  getParentRoute: () => root,
  path: "sessions",
  validateSearch: (s: Record<string, unknown>): SessionsSearch => ({
    deployment: typeof s.deployment === "string" ? s.deployment : undefined,
    agent: typeof s.agent === "string" ? s.agent : undefined,
    view: s.view === "activity" ? "activity" : undefined,
  }),
  component: Sessions,
});

const reviews = createRoute({ getParentRoute: () => root, path: "reviews", component: Reviews });

const review = createRoute({
  getParentRoute: () => root,
  path: "reviews/$review",
  // since: the change since the CEO last requested changes, instead of against main.
  validateSearch: (s: Record<string, unknown>): { since?: true } =>
    s.since === true || s.since === "1" ? { since: true } : {},
  component: Review,
});

const projectTabs: ProjectTab[] = [
  "overview",
  "board",
  "repositories",
  "worktrees",
  "activity",
  "settings",
];
const text = (v: unknown) => (typeof v === "string" && v !== "" ? v : undefined);
export const projectRoute = createRoute({
  getParentRoute: () => root,
  path: "projects/$project",
  // entry: the wiki entry open on Overview; repo and branch: what Repositories shows.
  validateSearch: (
    s: Record<string, unknown>,
  ): { tab: ProjectTab; entry?: string; repo?: string; branch?: string } => ({
    tab: projectTabs.includes(s.tab as ProjectTab) ? (s.tab as ProjectTab) : "overview",
    entry: text(s.entry),
    repo: text(s.repo),
    branch: text(s.branch),
  }),
  component: Project,
});

const teamTabs: TeamTab[] = ["deployments", "members", "playbook", "file"];
export const teamRoute = createRoute({
  getParentRoute: () => root,
  path: "teams/$team",
  validateSearch: (s: Record<string, unknown>): { tab: TeamTab } => ({
    tab: teamTabs.includes(s.tab as TeamTab) ? (s.tab as TeamTab) : "deployments",
  }),
  component: Team,
});

export const agentRoute = createRoute({
  getParentRoute: () => root,
  path: "teams/$team/agents/$agent",
  component: Agent,
});

const chiefOfStaff = createRoute({
  getParentRoute: () => root,
  path: "chief-of-staff",
  validateSearch: cosSearch,
  component: ChiefOfStaff,
});

const settings = createRoute({
  getParentRoute: () => root,
  path: "settings",
  validateSearch: settingsSearch,
  component: Settings,
});

export const router = createRouter({
  routeTree: root.addChildren([
    dashboard,
    sessions,
    reviews,
    review,
    projectRoute,
    teamRoute,
    agentRoute,
    chiefOfStaff,
    settings,
  ]),
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
