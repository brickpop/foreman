import { Link, Outlet, useNavigate, useParams } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import type { Instance, Project, Request, Team } from "../api/client";
import { Chevron } from "../components/icons";
import { NewProject } from "../components/NewProject";
import { Toasts, useToast } from "../components/Toasts";
import { CountBadge, Dot } from "../components/ui";
import { useOpenChiefOfStaff } from "./chiefOfStaff";
import { noticeFor, notifiesBrowser, showBrowserNotification } from "./notifications";
import { latestFirst, reviewOf, sections, sessionOf } from "./project";
import {
  useInstances,
  useLatestActivity,
  useLiveEvents,
  usePreferences,
  useProjects,
  useRequests,
  useSettings,
  useTeams,
} from "./queries";

// The frame every screen lives in (wiki/product/features/app-shell.md): the
// sidebar, the main area, and the overlays.

export function Shell() {
  // Under ~760px the sidebar hides behind a menu button and opens as an overlay.
  const [menu, setMenu] = useState(false);
  useEffect(() => {
    if (!menu) return;
    const close = (e: KeyboardEvent) => e.key === "Escape" && setMenu(false);
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, [menu]);
  const [newProject, setNewProject] = useState(false);
  return (
    <Toasts>
      <LiveEvents />
      <div className="grid h-screen grid-cols-[220px_minmax(0,1fr)] max-[760px]:grid-cols-[minmax(0,1fr)]">
        <button
          type="button"
          aria-label="Menu"
          aria-expanded={menu}
          onClick={() => setMenu(true)}
          className="fixed top-3 left-3 z-30 hidden rounded-md border border-line2 bg-side px-2.5 py-1 text-tx2 max-[760px]:block"
        >
          ☰
        </button>
        {menu && (
          <button
            type="button"
            aria-label="Close the menu"
            tabIndex={-1}
            onClick={() => setMenu(false)}
            className="fixed inset-0 z-[35] hidden bg-black/40 max-[760px]:block"
          />
        )}
        <Sidebar
          onNewProject={() => setNewProject(true)}
          open={menu}
          onClose={() => setMenu(false)}
        />
        {/* Under 760px the content starts below the menu button. */}
        <main className="min-h-0 min-w-0 overflow-auto max-[760px]:pt-12">
          <Outlet />
        </main>
      </div>
      {/* Mounted only while open: each opening starts fresh. */}
      {newProject && <NewProject onClose={() => setNewProject(false)} />}
    </Toasts>
  );
}

export type SidebarProject = {
  id: string;
  /** What waits on the CEO there: its open requests, reviews included (its
   * Inbox's count). */
  inbox: number;
};

export type SidebarModel = {
  projects: SidebarProject[];
  /** The archived projects, in a collapsed group of their own. */
  archived: SidebarProject[];
  teams: { id: string; name: string; deployedOn: string }[];
};

/** What the sidebar shows, from the projects, teams, instances and requests. */
export function sidebarModel(
  projects: Project[],
  teams: Team[],
  instances: Instance[],
  requests: Request[],
): SidebarModel {
  const open = requests.filter((r) => r.status === "open");
  const deployments = new Set(instances.map((i) => i.deploymentId).filter(Boolean));
  const row = (p: Project) => ({
    id: p.id,
    inbox: open.filter((r) => r.projectId === p.id).length,
  });
  return {
    projects: projects.filter((p) => !p.archived).map(row),
    archived: projects.filter((p) => p.archived).map(row),
    teams: teams.map((t) => {
      const on = [...deployments]
        .filter((d) => d.startsWith(`${t.id}@`))
        .map((d) => d.split("@")[1]);
      return {
        id: t.id,
        name: t.config.name || t.id,
        deployedOn: on.length === 0 ? "—" : on.length === 1 ? on[0] : `${on.length} projects`,
      };
    }),
  };
}

const navItem =
  "flex items-center justify-between rounded-md px-2.5 py-2 font-medium text-tx2 hover:bg-hover";
const navActive = { className: "bg-raised !text-tx" };
const rowItem =
  "flex items-center justify-between rounded-md px-2.5 py-1.5 text-[13.5px] text-tx2 hover:bg-hover";
const sectionItem =
  "flex items-center justify-between rounded-md px-2.5 py-1 text-[13px] text-tx3 hover:bg-hover hover:text-tx2";

function Sidebar({
  onNewProject,
  open,
  onClose,
}: {
  onNewProject: () => void;
  /** Open as an overlay, on narrow screens. */
  open: boolean;
  onClose: () => void;
}) {
  const openChiefOfStaff = useOpenChiefOfStaff();
  const projects = useProjects();
  const teams = useTeams();
  const instances = useInstances();
  const requests = useRequests();
  usePreferences(); // the theme
  const latest = useLatestActivity();
  const model = sidebarModel(
    latestFirst(projects.data ?? [], latest),
    teams.data ?? [],
    instances.data ?? [],
    requests.data ?? [],
  );
  // The port the browser reached Foreman on: the one it is bound to, whatever
  // settings.toml says (--port wins over it).
  const port = window.location.port;
  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: closes the overlay after a link was followed; the links are the keyboard path
    <aside
      onClick={onClose}
      className={`flex min-h-0 flex-col border-r border-line bg-side max-[760px]:fixed max-[760px]:inset-y-0 max-[760px]:left-0 max-[760px]:z-40 max-[760px]:w-[220px] max-[760px]:shadow-overlay ${open ? "" : "max-[760px]:hidden"}`}
    >
      <div className="flex items-center gap-[9px] px-[18px] pt-5 pb-4">
        <div className="grid size-[22px] place-items-center rounded-[5px] bg-acc font-mono text-xs font-bold text-accInk">
          F
        </div>
        <div className="text-[15px] font-semibold tracking-[-0.01em]">Foreman</div>
      </div>
      <div className="flex min-h-0 flex-1 flex-col gap-[18px] overflow-auto px-2.5 pb-3">
        <nav className="flex flex-col gap-0.5">
          <Link to="/" className={navItem} activeProps={navActive} activeOptions={{ exact: true }}>
            Dashboard
          </Link>
        </nav>
        <section className="flex flex-col gap-0.5">
          <Label>Projects</Label>
          {model.projects.map((p) => (
            <ProjectRow key={p.id} project={p} />
          ))}
          <Archived projects={model.archived} />
          <button
            type="button"
            onClick={onNewProject}
            className="px-2.5 py-1.5 text-left text-[13px] text-tx4 hover:text-tx3"
          >
            + New project
          </button>
        </section>
        <section className="flex flex-col gap-0.5">
          <Label>Teams</Label>
          {model.teams.map((t) => (
            <Link
              key={t.id}
              to="/teams/$team"
              params={{ team: t.id }}
              search={{ tab: "deployments" }}
              className={rowItem}
              activeProps={navActive}
              // Active on every tab, and on its members' Agent pages (paths under it).
              activeOptions={{ includeSearch: false }}
            >
              {t.name}
              <span className="text-xs text-tx4">{t.deployedOn}</span>
            </Link>
          ))}
          <button
            type="button"
            onClick={() => openChiefOfStaff({ start: "create-team" })}
            className="px-2.5 py-1.5 text-left text-[13px] text-tx4 hover:text-tx3"
          >
            + New team
          </button>
        </section>
      </div>
      <footer className="flex flex-col gap-1 border-t border-line px-2.5 py-3">
        <Link to="/settings" className={rowItem} activeProps={navActive}>
          Settings
          <span className="font-mono text-[11.5px] text-tx4">localhost:{port}</span>
        </Link>
      </footer>
    </aside>
  );
}

/** A project in the sidebar; the one open lists its sections under it, the
 * Inbox with its count (the CEO, 2026-10-31: no second row of tabs). A click
 * on the open one's name folds its sections, on another opens it (the CEO,
 * 2026-11-01). */
function ProjectRow({ project }: { project: SidebarProject }) {
  const current = useParams({ strict: false }).project === project.id;
  const [open, setOpen] = useState(true);
  // Opening a project shows its sections again.
  useEffect(() => {
    if (current) setOpen(true);
  }, [current]);
  return (
    <>
      {current ? (
        <button
          type="button"
          aria-expanded={open}
          onClick={(e) => {
            e.stopPropagation(); // keeps the narrow screens' overlay open
            setOpen(!open);
          }}
          className={`${rowItem} text-left font-medium !text-tx`}
        >
          {project.id}
          <span className="flex items-center gap-2">
            {!open && project.inbox > 0 && <Dot title="Needs you" />}
            <Chevron open={open} />
          </span>
        </button>
      ) : (
        <Link to="/projects/$project" params={{ project: project.id }} className={rowItem}>
          {project.id}
          {project.inbox > 0 && <Dot title="Needs you" />}
        </Link>
      )}
      {current && open && (
        <nav className="mb-1 ml-3.5 flex flex-col gap-px border-l border-line2 pl-1.5">
          {sections.map((s) => (
            <Link
              key={s.path}
              to={`/projects/$project/${s.path}`}
              params={{ project: project.id }}
              className={sectionItem}
              activeProps={navActive}
            >
              {s.label}
              {s.path === "inbox" && <CountBadge count={project.inbox} />}
            </Link>
          ))}
        </nav>
      )}
    </>
  );
}

/** The archived projects, collapsed; open while one of them is. */
function Archived({ projects }: { projects: SidebarModel["archived"] }) {
  const [open, setOpen] = useState(false);
  const current = useParams({ strict: false }).project;
  if (projects.length === 0) {
    return null;
  }
  const shown = open || projects.some((p) => p.id === current);
  return (
    <>
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation(); // keeps the narrow screens' overlay open
          setOpen(!shown);
        }}
        aria-expanded={shown}
        className="flex items-center gap-1.5 px-2.5 py-1.5 text-left text-[13px] text-tx4 hover:text-tx3"
      >
        <Chevron open={shown} />
        Archived
      </button>
      {shown && projects.map((p) => <ProjectRow key={p.id} project={p} />)}
    </>
  );
}

function Label({ children }: { children: string }) {
  return <div className="px-2.5 pt-1 pb-1.5 text-xs font-medium text-tx4">{children}</div>;
}

/** Follows the server's events: caches refresh, and what the CEO should hear
 * about becomes a toast, and a browser notification (notifications.md). */
function LiveEvents() {
  const toast = useToast();
  const navigate = useNavigate();
  const teams = useTeams();
  const instances = useInstances();
  const settings = useSettings();
  usePreferences(); // the theme
  useLiveEvents((e) => {
    const n = noticeFor(e, teams.data ?? [], instances.data ?? []);
    if (!n) return;
    // A review opens its page; otherwise who it is about, and with no
    // instance (Foreman itself) the request waits on the Dashboard.
    const open = () =>
      n.review
        ? navigate(reviewOf(n.deployment.split("@")[1] ?? "", n.review))
        : n.instance
          ? navigate(sessionOf(n.instance))
          : navigate({ to: "/" });
    toast({ title: n.title, body: n.body, color: n.color, onOpen: open });
    const toggles = settings.data?.settings.notifications;
    if (toggles && notifiesBrowser(n, toggles, document.hidden)) showBrowserNotification(n, open);
  });
  return null;
}
