import { createRootRoute, createRoute, createRouter } from "@tanstack/react-router";
import { Agent } from "../screens/Agent";
import { ChiefOfStaff } from "../screens/ChiefOfStaff";
import { Dashboard } from "../screens/Dashboard";
import { Project, ProjectHome } from "../screens/Project";
import { Activity } from "../screens/project/Activity";
import { Board } from "../screens/project/Board";
import { Deployment } from "../screens/project/Deployment";
import { Inbox } from "../screens/project/Inbox";
import { ProjectSettings } from "../screens/project/ProjectSettings";
import { Repo } from "../screens/project/Repo";
import { Repos } from "../screens/project/Repos";
import { Review } from "../screens/project/Review";
import { Session } from "../screens/project/Session";
import { Wiki } from "../screens/project/Wiki";
import { Settings } from "../screens/Settings";
import { Team } from "../screens/Team";
import { cosSearch } from "./chiefOfStaff";
import { type TeamTab, teamTabs } from "./model";
import { Shell } from "./Shell";
import { settingsSearch } from "./settings";

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
  // branch: the one read; path: the file open (none: the README); history:
  // the branch's history instead of its files.
  validateSearch: (
    s: Record<string, unknown>,
  ): { branch?: string; path?: string; history?: true } => ({
    branch: text(s.branch),
    path: text(s.path),
    ...(s.history === true || s.history === "1" ? { history: true as const } : {}),
  }),
  component: Repo,
});

const deployment = createRoute({ ...section("deployment"), component: Deployment });
const activity = createRoute({ ...section("activity"), component: Activity });
const projectSettings = createRoute({ ...section("settings"), component: ProjectSettings });

export const teamRoute = createRoute({
  getParentRoute: () => root,
  path: "teams/$team",
  // No tab in the URL: the page picks one from whether the team is deployed.
  validateSearch: (s: Record<string, unknown>): { tab?: TeamTab } => ({
    tab: teamTabs.includes(s.tab as TeamTab) ? (s.tab as TeamTab) : undefined,
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
    projectRoute.addChildren([
      projectHome,
      session,
      inbox,
      review,
      wiki,
      board,
      reposRoute,
      repoRoute,
      deployment,
      activity,
      projectSettings,
    ]),
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
