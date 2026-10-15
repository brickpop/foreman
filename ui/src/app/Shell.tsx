import { Link, Outlet, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import type { Instance, Project, Request, Team } from "../api/client";
import { AddProject } from "../components/AddProject";
import { DemoBadge } from "../components/DemoBadge";
import { Drawer } from "../components/Overlays";
import { Toasts, useToast } from "../components/Toasts";
import { CountBadge, Dot } from "../components/ui";
import { ChiefOfStaffContext } from "./chiefOfStaff";
import { noticeFor, notifiesBrowser, showBrowserNotification } from "./notifications";
import {
  useInstances,
  useLiveEvents,
  useProjects,
  useRequests,
  useSettings,
  useTeams,
} from "./queries";

// The frame every screen lives in (wiki/product/features/app-shell.md): the
// sidebar, the main area, and the overlays.

export function Shell() {
  const [chiefOfStaff, setChiefOfStaff] = useState(false);
  // Under ~760px the sidebar hides behind a menu button and opens as an overlay.
  const [menu, setMenu] = useState(false);
  const [addProject, setAddProject] = useState(false);
  return (
    <ChiefOfStaffContext.Provider value={() => setChiefOfStaff(true)}>
      <Toasts>
        <LiveEvents />
        <div className="grid h-screen grid-cols-[220px_minmax(0,1fr)] max-[760px]:grid-cols-[minmax(0,1fr)]">
          <button
            type="button"
            aria-label="Menu"
            onClick={() => setMenu(true)}
            className="fixed top-3 left-3 z-30 hidden rounded-md border border-line2 bg-side px-2.5 py-1 text-tx2 max-[760px]:block"
          >
            ☰
          </button>
          <Sidebar
            onChiefOfStaff={() => setChiefOfStaff(true)}
            onAddProject={() => setAddProject(true)}
            open={menu}
            onClose={() => setMenu(false)}
          />
          <main className="min-h-0 min-w-0 overflow-auto">
            <Outlet />
          </main>
        </div>
        <Drawer
          open={chiefOfStaff}
          onClose={() => setChiefOfStaff(false)}
          title="Chief of Staff"
          width={420}
          layer={45}
        >
          <p className="p-5 text-tx3">The Chief of Staff comes with its own card.</p>
        </Drawer>
        <AddProject open={addProject} onClose={() => setAddProject(false)} />
      </Toasts>
    </ChiefOfStaffContext.Provider>
  );
}

export type SidebarModel = {
  openRequests: number;
  projects: { id: string; needsYou: boolean }[];
  teams: { id: string; name: string; deployedOn: string }[];
};

/** What the sidebar shows, from the workspace, the instances and the requests. */
export function sidebarModel(
  projects: Project[],
  teams: Team[],
  instances: Instance[],
  requests: Request[],
): SidebarModel {
  const open = requests.filter((r) => r.status === "open");
  const deployments = new Set(instances.map((i) => i.deploymentId).filter(Boolean));
  return {
    openRequests: open.length,
    projects: projects.map((p) => ({
      id: p.id,
      needsYou: open.some((r) => r.projectId === p.id),
    })),
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

function Sidebar({
  onChiefOfStaff,
  onAddProject,
  open,
  onClose,
}: {
  onChiefOfStaff: () => void;
  onAddProject: () => void;
  /** Open as an overlay, on narrow screens. */
  open: boolean;
  onClose: () => void;
}) {
  const projects = useProjects();
  const teams = useTeams();
  const instances = useInstances();
  const requests = useRequests();
  const settings = useSettings();
  const model = sidebarModel(
    projects.data ?? [],
    teams.data ?? [],
    instances.data ?? [],
    requests.data ?? [],
  );
  const port = settings.data?.settings.port ?? 8888;
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
            Home <CountBadge count={model.openRequests} />
          </Link>
          <Link to="/sessions" className={navItem} activeProps={navActive}>
            Sessions
          </Link>
        </nav>
        <section className="flex flex-col gap-0.5">
          <Label>Projects</Label>
          {model.projects.map((p) => (
            <Link
              key={p.id}
              to="/projects/$project"
              params={{ project: p.id }}
              search={{ tab: "overview" }}
              className={rowItem}
              activeProps={navActive}
            >
              {p.id}
              {p.needsYou && <Dot title="Needs you" />}
            </Link>
          ))}
          <button
            type="button"
            onClick={onAddProject}
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
            >
              {t.name}
              <span className="text-xs text-tx4">{t.deployedOn}</span>
            </Link>
          ))}
          <button
            type="button"
            onClick={onChiefOfStaff}
            className="px-2.5 py-1.5 text-left text-[13px] text-tx4 hover:text-tx3"
          >
            + New team
          </button>
        </section>
        <div className="px-2.5">
          <DemoBadge areas={["workspace", "agents", "requests"]} />
        </div>
      </div>
      <footer className="flex flex-col gap-1 border-t border-line px-2.5 py-3">
        <button
          type="button"
          onClick={onChiefOfStaff}
          className="flex items-center gap-2.5 rounded-lg border border-accLine bg-accSoft px-2.5 py-[9px] text-left"
        >
          <span className="grid size-6 place-items-center rounded-md bg-acc font-mono text-xs font-bold text-accInk">
            C
          </span>
          <span className="flex flex-col">
            <span className="text-[13px] font-semibold">Chief of Staff</span>
            <span className="text-[11.5px] text-tx3">Hire, set up, anything else</span>
          </span>
        </button>
        <Link to="/settings" className={rowItem} activeProps={navActive}>
          Settings
          <span className="font-mono text-[11.5px] text-tx4">localhost:{port}</span>
        </Link>
      </footer>
    </aside>
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
  useLiveEvents((e) => {
    const n = noticeFor(e, teams.data ?? [], instances.data ?? []);
    if (!n) return;
    const open = () =>
      navigate({
        to: "/sessions",
        search: { deployment: n.deployment || undefined, agent: n.instance || undefined },
      });
    toast({ title: n.title, body: n.body, color: n.color, onOpen: open });
    const toggles = settings.data?.settings.notifications;
    if (toggles && notifiesBrowser(n, toggles, document.hidden)) showBrowserNotification(n, open);
  });
  return null;
}
