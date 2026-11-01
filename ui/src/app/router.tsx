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
import { Repos } from "../screens/project/Repos";
import { Review } from "../screens/project/Review";
import { Session } from "../screens/project/Session";
import { Wiki } from "../screens/project/Wiki";
import { Settings } from "../screens/Settings";
import { Team, type TeamTab } from "../screens/Team";
import { cosSearch } from "./chiefOfStaff";
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
  // since: the change since the CEO last requested changes, instead of against main.
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

const reposRoute = createRoute({
  ...section("repos"),
  // repo and branch: what is read.
  validateSearch: (s: Record<string, unknown>): { repo?: string; branch?: string } => ({
    repo: text(s.repo),
    branch: text(s.branch),
  }),
  component: Repos,
});

const deployment = createRoute({ ...section("deployment"), component: Deployment });
const activity = createRoute({ ...section("activity"), component: Activity });
const projectSettings = createRoute({ ...section("settings"), component: ProjectSettings });

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
    projectRoute.addChildren([
      projectHome,
      session,
      inbox,
      review,
      wiki,
      board,
      reposRoute,
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
