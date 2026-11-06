import { Link, Outlet, useNavigate, useParams } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import type { Instance, Project, Request } from "../api/client";
import { Chevron } from "../components/icons";
import { NewProject } from "../components/NewProject";
import { Toasts, useToast } from "../components/Toasts";
import { CountBadge, Dot } from "../components/ui";
import { byName, projectName } from "./model";
import { noticeFor, notifiesBrowser, showBrowserNotification } from "./notifications";
import { reviewOf, sections, sessionOf } from "./project";
import {
  useInstances,
  useLiveEvents,
  usePreferences,
  useProjects,
  useRequests,
  useSettings,
  useTeams,
} from "./queries";
import { deploymentStatus } from "./sessions";
import { hasUnsentLine } from "./unsentLines";

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
      <UnsentLinesGuard />
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
  /** How it shows (projectName). */
  name: string;
  /** What waits on the CEO there: its open requests, reviews included (its
   * Inbox's count). */
  inbox: number;
  /** Its team's status while it is busy (its liveliest agent's: awaiting,
   * running, starting, queued), for a dot on its Session item; none at rest. */
  busy?: string;
};

export type SidebarModel = {
  projects: SidebarProject[];
  /** The archived projects, in a collapsed group of their own. */
  archived: SidebarProject[];
};

const busy = ["awaiting", "running", "starting", "queued"];

/** What the sidebar shows, from the projects, instances and requests. */
export function sidebarModel(
  projects: Project[],
  instances: Instance[],
  requests: Request[],
): SidebarModel {
  const open = requests.filter((r) => r.status === "open");
  const row = (p: Project): SidebarProject => {
    const on = instances.filter((i) => i.deploymentId.split("@")[1] === p.id);
    const status = deploymentStatus(on);
    return {
      id: p.id,
      name: projectName([p], p.id),
      inbox: open.filter((r) => r.projectId === p.id).length,
      ...(on.length > 0 && busy.includes(status) && { busy: status }),
    };
  };
  return {
    projects: projects.filter((p) => !p.archived).map(row),
    archived: projects.filter((p) => p.archived).map(row),
  };
}

/** A folded project row's one dot (the CEO, 2026-11-06): needs-you when
 * something waits on the CEO there, else its Session's dot while the team is
 * busy, else none. Unfolded, the dots are on its sections instead. */
export function rowDot(p: SidebarProject): { status?: string; title: string } | undefined {
  if (p.inbox > 0) return { title: "Needs you" };
  if (p.busy) return { status: p.busy, title: p.busy };
  return undefined;
}

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
  const projects = useProjects();
  const instances = useInstances();
  const requests = useRequests();
  usePreferences(); // the theme
  const model = sidebarModel(
    byName(projects.data ?? []),
    instances.data ?? [],
    requests.data ?? [],
  );
  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: closes the overlay after a link was followed; the links are the keyboard path
    <aside
      onClick={onClose}
      className={`flex min-h-0 flex-col border-r border-line bg-side max-[760px]:fixed max-[760px]:inset-y-0 max-[760px]:left-0 max-[760px]:z-40 max-[760px]:w-[220px] max-[760px]:shadow-overlay ${open ? "" : "max-[760px]:hidden"}`}
    >
      {/* The way to the Dashboard (the CEO, 2026-11-02: no menu entry for it). */}
      <Link
        to="/"
        title="Dashboard"
        className="mx-2.5 mt-3 mb-2 flex items-center gap-[9px] rounded-md px-2 py-2 text-tx hover:bg-hover"
      >
        <span className="grid size-[22px] place-items-center rounded-[5px] bg-acc font-mono text-xs font-bold text-accInk">
          F
        </span>
        <span className="text-[15px] font-semibold tracking-[-0.01em]">Foreman</span>
      </Link>
      <nav className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-auto px-2.5 pb-3">
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
      </nav>
      <footer className="flex flex-col gap-1 border-t border-line px-2.5 py-3">
        <Link to="/settings" className={rowItem} activeProps={navActive}>
          Settings
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
  const dot = rowDot(project);
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
          {project.name}
          <span className="flex items-center gap-2">
            {!open && dot && <Dot status={dot.status} title={dot.title} />}
            <Chevron open={open} />
          </span>
        </button>
      ) : (
        <Link to="/projects/$project" params={{ project: project.id }} className={rowItem}>
          {project.name}
          {dot && <Dot status={dot.status} title={dot.title} />}
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
              {s.path === "session" && project.busy && (
                <Dot status={project.busy} title={project.busy} />
              )}
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

/** Follows the server's events: caches refresh, and what the CEO should hear
 * about becomes a toast, and a browser notification (notifications.md). */
/** A reload would lose the lines typed into Line and not sent: the browser asks first. */
function UnsentLinesGuard() {
  const instances = useInstances().data;
  useEffect(() => {
    const ids = (instances ?? []).map((i) => i.id);
    const ask = (e: BeforeUnloadEvent) => {
      if (hasUnsentLine(ids)) e.preventDefault();
    };
    window.addEventListener("beforeunload", ask);
    return () => window.removeEventListener("beforeunload", ask);
  }, [instances]);
  return null;
}

function LiveEvents() {
  const toast = useToast();
  const navigate = useNavigate();
  const teams = useTeams();
  const instances = useInstances();
  const projects = useProjects();
  const settings = useSettings();
  usePreferences(); // the theme
  useLiveEvents((e) => {
    const n = noticeFor(e, teams.data ?? [], instances.data ?? [], projects.data ?? []);
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
