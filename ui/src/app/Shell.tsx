import { Outlet, useNavigate, useParams } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { MenuIcon } from "../components/icons";
import { NewProject } from "../components/NewProject";
import { Rail } from "../components/Rail";
import { Toasts, useToast } from "../components/Toasts";
import { noticeFor, notifiesBrowser, showBrowserNotification } from "./notifications";
import { reviewOf, sessionOf } from "./project";
import {
  useInstances,
  useLiveEvents,
  useLook,
  useProjects,
  useSettings,
  useTeams,
} from "./queries";
import { hasUnsentLine } from "./unsentLines";

// The frame every screen lives in (wiki/product/features/app-shell.md): the
// rail, the main area, and the overlays.

export function Shell() {
  // Under 760px the rail hides behind a menu button and opens as an overlay.
  const [menu, setMenu] = useState(false);
  useEffect(() => {
    if (!menu) return;
    const close = (e: KeyboardEvent) => e.key === "Escape" && setMenu(false);
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, [menu]);
  const [newProject, setNewProject] = useState(false);
  const inProject = !!useParams({ strict: false }).project;
  return (
    <Toasts>
      <LiveEvents />
      <UnsentLinesGuard />
      <div className="grid h-screen grid-cols-[44px_minmax(0,1fr)] max-[760px]:grid-cols-[minmax(0,1fr)]">
        <button
          type="button"
          aria-label="Menu"
          aria-expanded={menu}
          onClick={() => setMenu(true)}
          className="fixed top-2 left-3 z-30 hidden rounded-md border border-line2 bg-side p-1.5 text-tx2 max-[760px]:block"
        >
          <MenuIcon />
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
        <RailColumn
          onNewProject={() => setNewProject(true)}
          open={menu}
          onClose={() => setMenu(false)}
        />
        {/* Under 760px the content starts below the menu button; a project's page
            starts with its bar, which leaves the button room on its left instead. */}
        <main className={`min-h-0 min-w-0 overflow-auto ${inProject ? "" : "max-[760px]:pt-12"}`}>
          <Outlet />
        </main>
      </div>
      {/* Mounted only while open: each opening starts fresh. */}
      {newProject && <NewProject onClose={() => setNewProject(false)} />}
    </Toasts>
  );
}

/** The rail, in its column; on narrow screens, an overlay that closes once a link
 * in it is followed. */
function RailColumn({
  onNewProject,
  open,
  onClose,
}: {
  onNewProject: () => void;
  /** Open as an overlay, on narrow screens. */
  open: boolean;
  onClose: () => void;
}) {
  useLook(); // the theme
  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: closes the overlay after a link was followed; the links are the keyboard path
    <aside
      onClick={onClose}
      className={`flex min-h-0 flex-col border-r border-line bg-side max-[760px]:fixed max-[760px]:inset-y-0 max-[760px]:left-0 max-[760px]:z-40 max-[760px]:w-[260px] max-[760px]:px-2 max-[760px]:shadow-overlay ${open ? "" : "max-[760px]:hidden"}`}
    >
      <Rail onNewProject={onNewProject} />
    </aside>
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
  useLook(); // the theme
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
