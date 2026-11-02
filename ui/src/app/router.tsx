import { createRootRoute, createRoute, createRouter, redirect } from "@tanstack/react-router";
import { Agent } from "../screens/Agent";
import { ChiefOfStaff } from "../screens/ChiefOfStaff";
import { Dashboard } from "../screens/Dashboard";
import { Project, ProjectHome } from "../screens/Project";
import { Board } from "../screens/project/Board";
import { Inbox } from "../screens/project/Inbox";
import { ProjectSettings } from "../screens/project/ProjectSettings";
import { ProjectTeam } from "../screens/project/ProjectTeam";
import { Repo } from "../screens/project/Repo";
import { Repos } from "../screens/project/Repos";
import { Review } from "../screens/project/Review";
import { Session } from "../screens/project/Session";
import { Wiki } from "../screens/project/Wiki";
import { Settings, SettingsFrame } from "../screens/Settings";
import { Team } from "../screens/Team";
import { cosSearch } from "./chiefOfStaff";
import { type TeamTab, teamTabs } from "./model";
import { type TeamView, teamViews } from "./project";
import { Shell } from "./Shell";
import { settingsTab } from "./settings";

// One route per screen (wiki/product/features/app-shell.md), and one per
// section of a project (wiki/product/features/project-page.md), with typed
// path and search params.

const root = createRootRoute({ component: Shell });

const dashboard = createRoute({ getParentRoute: () => root, path: "/", component: Dashboard });

const text = (v: unknown) => (typeof v === "string" && v !== "" ? v : undefined);

const projectRoute = createRoute({
  getParentRoute: () => root,
  path: "projects/$project",
  component: Project,
});

const section = <P extends string>(path: P) => ({
  getParentRoute: () => projectRoute,
  path,
});

const projectHome = createRoute({ ...section("/"), component: ProjectHome });

const session = createRoute({
  ...section("session"),
  // agent: the one shown, by id (a full instance id is read too); none, the
  // last one shown on the project, else its lead.
  validateSearch: (s: Record<string, unknown>): { agent?: string } => ({ agent: text(s.agent) }),
  component: Session,
});

const inbox = createRoute({ ...section("inbox"), component: Inbox });

const review = createRoute({
  ...section("inbox/$review"),
  // since: the change since the CEO last requested changes, instead of against its target.
  validateSearch: (s: Record<string, unknown>): { since?: true } =>
    s.since === true || s.since === "1" ? { since: true } : {},
  component: Review,
});

const wiki = createRoute({
  ...section("wiki"),
  // entry: the wiki entry open.
  validateSearch: (s: Record<string, unknown>): { entry?: string } => ({ entry: text(s.entry) }),
  component: Wiki,
});

const board = createRoute({ ...section("board"), component: Board });

const reposRoute = createRoute({ ...section("repos"), component: Repos });

const repoRoute = createRoute({
  ...section("repos/$repo"),
  // branch: the one read; path: the file open, or a folder ending in "/" (its
  // page: a link to it); none: the README; history: the branch's history
  // instead of its files.
  validateSearch: (
    s: Record<string, unknown>,
  ): { branch?: string; path?: string; history?: true } => ({
    branch: text(s.branch),
    path: text(s.path),
    ...(s.history === true || s.history === "1" ? { history: true as const } : {}),
  }),
  component: Repo,
});

// The team on the project: its agents, its activity, its work in flight and
// merged (the CEO, 2026-11-03: Deployment and Activity were two sections).
const teamView = (v: unknown) =>
  teamViews.some((t) => t.view === v) ? (v as TeamView) : undefined;
const teamSection = createRoute({
  ...section("team"),
  // No view in the URL: the page picks one from whether there are agents.
  validateSearch: (s: Record<string, unknown>): { view?: TeamView } => ({ view: teamView(s.view) }),
  component: ProjectTeam,
});
// The old sections' links open their view of it.
const oldDeployment = createRoute({
  ...section("deployment"),
  validateSearch: (s: Record<string, unknown>): { view?: TeamView } => ({ view: teamView(s.view) }),
  beforeLoad: ({ params, search }) => {
    throw redirect({ to: "/projects/$project/team", params, search, replace: true });
  },
});
const oldActivity = createRoute({
  ...section("activity"),
  beforeLoad: ({ params }) => {
    throw redirect({
      to: "/projects/$project/team",
      params,
      search: { view: "activity" },
      replace: true,
    });
  },
});
const projectSettings = createRoute({ ...section("settings"), component: ProjectSettings });

// Settings: its tabs, then the teams' and their agents' pages, which live
// there (the CEO, 2026-11-02: the sidebar holds only the projects).
const settingsRoute = createRoute({
  getParentRoute: () => root,
  path: "settings",
  component: SettingsFrame,
});

// /settings opens Teams; an old /settings?tab=<tab> link opens that tab.
const settingsHome = createRoute({
  getParentRoute: () => settingsRoute,
  path: "/",
  validateSearch: (s: Record<string, unknown>): { tab?: unknown } => ({ tab: s.tab }),
  beforeLoad: ({ search }) => {
    throw redirect({
      to: "/settings/$tab",
      params: { tab: settingsTab(search.tab) },
      replace: true,
    });
  },
});

const settingsTabRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: "$tab",
  beforeLoad: ({ params }) => {
    if (settingsTab(params.tab) !== params.tab)
      throw redirect({ to: "/settings/$tab", params: { tab: "teams" }, replace: true });
  },
  component: Settings,
});

// No tab in the URL: the page picks one from whether the team is deployed.
const teamSearch = (s: Record<string, unknown>): { tab?: TeamTab } => ({
  tab: teamTabs.includes(s.tab as TeamTab) ? (s.tab as TeamTab) : undefined,
});

const teamRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: "teams/$team",
  validateSearch: teamSearch,
  component: Team,
});

const agentRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: "teams/$team/agents/$agent",
  component: Agent,
});

const chiefOfStaff = createRoute({
  getParentRoute: () => root,
  path: "chief-of-staff",
  validateSearch: cosSearch,
  component: ChiefOfStaff,
});

// Where the team pages were before they moved into Settings.
const oldTeam = createRoute({
  getParentRoute: () => root,
  path: "teams/$team",
  validateSearch: teamSearch,
  beforeLoad: ({ params, search }) => {
    throw redirect({ to: "/settings/teams/$team", params, search, replace: true });
  },
});

const oldAgent = createRoute({
  getParentRoute: () => root,
  path: "teams/$team/agents/$agent",
  beforeLoad: ({ params }) => {
    throw redirect({ to: "/settings/teams/$team/agents/$agent", params, replace: true });
  },
});

export const router = createRouter({
  routeTree: root.addChildren([
    dashboard,
    projectRoute.addChildren([
      projectHome,
      session,
      inbox,
      review,
      wiki,
      board,
      reposRoute,
      repoRoute,
      teamSection,
      oldDeployment,
      oldActivity,
      projectSettings,
    ]),
    chiefOfStaff,
    settingsRoute.addChildren([settingsHome, settingsTabRoute, teamRoute, agentRoute]),
    oldTeam,
    oldAgent,
  ]),
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
