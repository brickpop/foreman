import { createRootRoute, createRoute, createRouter } from "@tanstack/react-router";
import { Agent } from "../screens/Agent";
import { Home } from "../screens/Home";
import { Project, type ProjectTab } from "../screens/Project";
import { Sessions } from "../screens/Sessions";
import { Settings, type SettingsTab } from "../screens/Settings";
import { Team, type TeamTab } from "../screens/Team";
import { Shell } from "./Shell";
import type { SessionsSearch } from "./sessions";

// One route per screen (wiki/product/features/app-shell.md), with typed path
// and search params: project / tab / entry, team / tab.

const root = createRootRoute({ component: Shell });

const home = createRoute({ getParentRoute: () => root, path: "/", component: Home });

const sessions = createRoute({
  getParentRoute: () => root,
  path: "sessions",
  validateSearch: (s: Record<string, unknown>): SessionsSearch => ({
    deployment: typeof s.deployment === "string" ? s.deployment : undefined,
    agent: typeof s.agent === "string" ? s.agent : undefined,
    panes: typeof s.panes === "string" ? s.panes : undefined,
    view: s.view === "activity" ? "activity" : undefined,
  }),
  component: Sessions,
});

const projectTabs: ProjectTab[] = ["overview", "board", "repositories", "activity", "settings"];
export const projectRoute = createRoute({
  getParentRoute: () => root,
  path: "projects/$project",
  validateSearch: (s: Record<string, unknown>): { tab: ProjectTab; entry?: string } => ({
    tab: projectTabs.includes(s.tab as ProjectTab) ? (s.tab as ProjectTab) : "overview",
    entry: typeof s.entry === "string" ? s.entry : undefined,
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

const settings = createRoute({
  getParentRoute: () => root,
  path: "settings",
  validateSearch: (s: Record<string, unknown>): { tab?: SettingsTab } => ({
    tab: s.tab === "global" ? "global" : undefined,
  }),
  component: Settings,
});

export const router = createRouter({
  routeTree: root.addChildren([home, sessions, projectRoute, teamRoute, agentRoute, settings]),
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
