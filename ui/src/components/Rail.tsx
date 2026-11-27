import * as Menu from "@radix-ui/react-dropdown-menu";
import { Link, useLocation, useNavigate, useParams } from "@tanstack/react-router";
import { type ReactElement, useEffect, useRef, useState } from "react";
import { byName } from "../app/model";
import { type Section, sectionOf, sections } from "../app/project";
import { useInstances, useProjects, useRequests } from "../app/queries";
import {
  badgeText,
  cycle,
  cycleStep,
  elsewhere,
  groupMark,
  isPopoverShortcut,
  type Mark,
  markLabel,
  markOf,
  needsYou,
  type RailModel,
  type RailProject,
  railModel,
} from "../app/rail";
import { useHoverMenu } from "./hoverMenu";
import {
  BoardIcon,
  Chevron,
  GearIcon,
  InboxIcon,
  RepoIcon,
  SessionIcon,
  SlidersIcon,
  TeamIcon,
  WikiIcon,
} from "./icons";
import { Dot } from "./ui";

// The rail (wiki/product/features/app-shell.md#the-rail): one
// 44px column, each icon one meaning. The Foreman tile goes to the Dashboard and,
// hovered, opens the project popover; under the tile, the open project's sections;
// at the bottom, Foreman's settings. On a narrow screen it opens as a 260px overlay
// (the Shell's), where the names show and the projects list under the tile.

const icons: Record<Section, { Icon: (p: { size?: string }) => ReactElement; name?: string }> = {
  session: { Icon: SessionIcon },
  inbox: { Icon: InboxIcon },
  board: { Icon: BoardIcon },
  repos: { Icon: RepoIcon },
  wiki: { Icon: WikiIcon },
  team: { Icon: TeamIcon },
  settings: { Icon: SlidersIcon, name: "Project settings" },
};

/** Every rail icon's size. */
const iconSize = "size-[18px]";

/** Shown only in the narrow screens' overlay. */
const narrowOnly = "hidden max-[760px]:inline";

// !: links take the accent colour by default (styles.css); the open section (Link's
// data-status) shows raised.
const railItem =
  "relative flex h-9 items-center gap-3 rounded-md px-[9px] !text-tx3 outline-none hover:bg-hover hover:!text-tx2 focus-visible:outline-2 focus-visible:outline-acc data-[status=active]:bg-raised data-[status=active]:!text-tx";

export function Rail({ onNewProject }: { onNewProject: () => void }) {
  const projects = useProjects();
  const instances = useInstances();
  const requests = useRequests();
  const model = railModel(byName(projects.data ?? []), instances.data ?? [], requests.data ?? []);
  const open = useParams({ strict: false }).project;
  const current = [...model.projects, ...model.hidden].find((p) => p.id === open);
  return (
    <>
      <Tile model={model} open={open} onNewProject={onNewProject} />
      {/* The narrow screens' overlay lists the projects inline: no hovering there. */}
      <nav aria-label="Projects" className="hidden flex-col gap-px px-1 pb-2 max-[760px]:flex">
        <ProjectList model={model} open={open} onNewProject={onNewProject} />
      </nav>
      <nav aria-label="Sections" className="flex min-h-0 flex-1 flex-col gap-1 overflow-auto px-1">
        {current && <Sections project={current} />}
      </nav>
      <Link
        to="/settings"
        aria-label="Foreman settings"
        title="Foreman settings"
        className={`${railItem} mx-1 mb-3`}
      >
        <GearIcon size={iconSize} />
        <span className={narrowOnly}>Foreman settings</span>
      </Link>
    </>
  );
}

/** The open project's sections. */
function Sections({ project }: { project: RailProject }) {
  return sections.map((s) => {
    const { Icon, name = s.label } = icons[s.path];
    const mark: Mark =
      s.path === "inbox" && project.inbox > 0
        ? { count: project.inbox }
        : s.path === "session" && project.busy
          ? { status: project.busy }
          : undefined;
    return (
      <Link
        key={s.path}
        to={`/projects/$project/${s.path}`}
        params={{ project: project.id }}
        aria-label={`${name}${markLabel(mark)}`}
        title={name}
        className={railItem}
      >
        <Icon size={iconSize} />
        <span className={narrowOnly}>{name}</span>
        <MarkOn mark={mark} />
      </Link>
    );
  });
}

/** A mark pinned to an icon's top right corner; at the row's right end where the
 * names show (the narrow screens' overlay). */
function MarkOn({ mark }: { mark: Mark }) {
  if (!mark) return null;
  return (
    <span
      aria-hidden="true"
      className="absolute top-0.5 left-[19px] flex max-[760px]:static max-[760px]:ml-auto"
    >
      <MarkView mark={mark} />
    </span>
  );
}

/** A needs-you badge (amber, capped at 9+) or a busy dot; screen readers hear it in
 * the item's name instead. */
function MarkView({ mark }: { mark: Mark }) {
  if (!mark) return null;
  if ("status" in mark) return <Dot status={mark.status} title={mark.status} />;
  return (
    <span
      title={needsYou(mark.count)}
      className="rounded-[9px] bg-attn px-[5px] font-mono text-[10px] leading-[15px] font-semibold text-attnInk"
    >
      {badgeText(mark.count)}
    </span>
  );
}

const narrow = () => window.matchMedia("(width < 760px)").matches;

/** The Foreman tile: a click goes to the Dashboard; resting on it, or focusing it
 * with the keyboard, opens the project popover. Ctrl+Alt+P opens it from anywhere,
 * Ctrl+Alt+↓ / ↑ moves to the next or previous project. */
function Tile({
  model,
  open,
  onNewProject,
}: {
  model: RailModel;
  open?: string;
  onNewProject: () => void;
}) {
  const menu = useHoverMenu();
  const navigate = useNavigate();
  const pathname = useLocation({ select: (l) => l.pathname });
  const tile = useRef<HTMLAnchorElement>(null);
  const content = useRef<HTMLDivElement>(null);
  // How the popover opened, which decides where the keyboard goes: resting the pointer
  // on the tile leaves the focus where it was (in the terminal, say); focusing the tile
  // keeps it there (Enter still goes to the Dashboard); ↓ or Ctrl+Alt+P go into the list.
  const how = useRef<"hover" | "tile" | "list">("hover");
  const before = useRef<HTMLElement | null>(null);
  // Radix focuses the popover as it opens: that first focus is moved on.
  const placing = useRef(false);
  // Set while the tile takes the focus back, so that doesn't reopen anything.
  const returning = useRef(false);
  const count = elsewhere(model, open);

  const intoList = () => {
    how.current = "list";
    const items = content.current?.querySelectorAll<HTMLElement>("[role=menuitem]");
    const at = content.current?.querySelector<HTMLElement>(`[data-project="${open}"]`);
    (at ?? items?.[0])?.focus();
  };
  const toTile = () => {
    returning.current = true;
    tile.current?.focus();
    returning.current = false;
  };
  const opening = (by: typeof how.current) => {
    how.current = by;
    before.current = document.activeElement as HTMLElement | null;
    placing.current = true;
  };
  const openList = () => {
    if (menu.open) return intoList();
    opening("list");
    menu.show();
  };

  // The window's capture phase runs before the terminal, so the agent never gets the keys.
  const keys = useRef<(e: KeyboardEvent) => void>(undefined);
  keys.current = (e) => {
    const step = cycleStep(e);
    if (!isPopoverShortcut(e) && step === 0) return;
    e.preventDefault();
    e.stopPropagation();
    // On a narrow screen the tile is in the closed overlay: nothing to open beside.
    if (step === 0) return narrow() ? undefined : openList();
    const to = cycle(model, open, step);
    if (!to || to === open) return;
    const section = sectionOf(pathname)?.path;
    navigate(
      section
        ? { to: `/projects/$project/${section}`, params: { project: to } }
        : { to: "/projects/$project", params: { project: to } },
    );
  };
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => keys.current?.(e);
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, []);

  return (
    <Menu.Root open={menu.open} onOpenChange={menu.setOpen} modal={false}>
      <Menu.Trigger asChild>
        <Link
          ref={tile}
          to="/"
          aria-label={`Foreman, Dashboard${markLabel(count > 0 ? { count } : undefined, open ? " in other projects" : "")}`}
          activeOptions={{ exact: true }}
          onPointerEnter={(e) => {
            if (narrow()) return;
            if (!menu.open) opening("hover");
            menu.trigger.onPointerEnter(e);
          }}
          onPointerLeave={menu.trigger.onPointerLeave}
          // A click follows the link: it never toggles the menu, nor takes the focus.
          onPointerDown={(e) => e.pointerType === "mouse" && e.preventDefault()}
          onClick={() => menu.setOpen(false)}
          onFocus={(e) => {
            if (returning.current || menu.open || narrow()) return;
            if (!e.currentTarget.matches(":focus-visible")) return;
            opening("tile");
            menu.show();
          }}
          onKeyDown={(e) => {
            // Enter goes to the Dashboard, ↓ into the list; other keys are the menu's.
            if (e.key === "Enter") {
              e.preventDefault();
              menu.setOpen(false);
              navigate({ to: "/" });
            } else if (e.key === "ArrowDown") {
              e.preventDefault();
              openList();
            }
          }}
          // The Dashboard shows raised, as an open section does; the ring is the keyboard's.
          className="group relative mx-1 mt-2 mb-2 flex h-11 items-center gap-[9px] rounded-md px-1 outline-none hover:bg-hover data-[status=active]:bg-raised"
        >
          <span className="grid size-7 shrink-0 place-items-center rounded-md bg-acc font-mono text-xs font-bold text-accInk ring-acc ring-offset-2 ring-offset-side group-focus-visible:ring-2">
            F
          </span>
          <span className={`${narrowOnly} text-[15px] font-semibold !text-tx`}>Foreman</span>
          {/* Inside the rail's 44px: over the tile's top right corner only (its right edge at the rail's), at the row's end when names show. */}
          <span
            aria-hidden="true"
            className="absolute top-1 -right-1 flex max-[760px]:static max-[760px]:ml-auto"
          >
            <MarkView mark={count > 0 ? { count } : undefined} />
          </span>
        </Link>
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Content
          ref={content}
          side="right"
          align="start"
          // Flush with the rail's edge: the tile sits 4px inside it.
          sideOffset={4}
          {...menu.content}
          onFocus={(e) => {
            if (e.target !== e.currentTarget || !placing.current) return;
            placing.current = false;
            // Next frame: Radix moves the keyboard's focus to the first item after this.
            requestAnimationFrame(() => {
              if (how.current === "list") intoList();
              else if (how.current === "tile") toTile();
              else before.current?.focus();
            });
          }}
          // Only the keyboard's popover closes as the focus leaves it; a hovered one
          // closes as the pointer leaves.
          onFocusOutside={(e) => {
            if (how.current === "hover" || e.target === tile.current) e.preventDefault();
          }}
          onCloseAutoFocus={(e) => {
            // Radix would focus the tile. Only when the focus went with the popover (it
            // was in the list): back to the tile for the keyboard (Escape, say), back
            // where it was for the pointer.
            e.preventDefault();
            if (document.activeElement && document.activeElement !== document.body) return;
            if (how.current === "hover") before.current?.focus();
            else toTile();
          }}
          className="z-50 flex max-h-[80vh] min-w-60 flex-col overflow-auto rounded-lg border border-line2 bg-panel p-1 shadow-overlay"
        >
          <ProjectList model={model} open={open} onNewProject={onNewProject} menu />
        </Menu.Content>
      </Menu.Portal>
    </Menu.Root>
  );
}

const entry =
  "flex w-full cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-left text-[13.5px] !text-tx2 outline-none hover:bg-hover data-[highlighted]:bg-hover data-[highlighted]:!text-tx";

/** In the popover, an item of the menu (arrow keys, typing a letter); in the overlay,
 * the plain link or button. */
function Entry({
  menu,
  keepOpen,
  children,
}: {
  menu: boolean;
  keepOpen?: boolean;
  children: ReactElement;
}) {
  if (!menu) return children;
  return (
    <Menu.Item asChild onSelect={keepOpen ? (e) => e.preventDefault() : undefined}>
      {children}
    </Menu.Item>
  );
}

/** Dashboard, the visible projects, the hidden ones folded in More, and + New project. */
function ProjectList({
  model,
  open,
  onNewProject,
  menu = false,
}: {
  model: RailModel;
  open?: string;
  onNewProject: () => void;
  menu?: boolean;
}) {
  const [more, setMore] = useState(false);
  const shown = more || model.hidden.some((p) => p.id === open);
  const folded = groupMark(model.hidden);
  return (
    <>
      {menu && (
        <Entry menu>
          <Link to="/" className={entry}>
            Dashboard
          </Link>
        </Entry>
      )}
      {model.projects.map((p) => (
        <ProjectEntry key={p.id} project={p} open={p.id === open} menu={menu} />
      ))}
      {model.hidden.length > 0 && (
        <Entry menu={menu} keepOpen>
          <button
            type="button"
            aria-expanded={shown}
            aria-label={`More${shown ? "" : markLabel(folded)}`}
            onClick={(e) => {
              e.stopPropagation(); // keeps the narrow screens' overlay open
              setMore(!shown);
            }}
            className={`${entry} !text-tx4 data-[highlighted]:!text-tx3`}
          >
            <Chevron open={shown} />
            <span className="flex-1">More ({model.hidden.length})</span>
            {!shown && <MarkView mark={folded} />}
          </button>
        </Entry>
      )}
      {shown &&
        model.hidden.map((p) => (
          <ProjectEntry key={p.id} project={p} open={p.id === open} menu={menu} />
        ))}
      {menu ? <Menu.Separator className="my-1 h-px bg-line" /> : null}
      <Entry menu={menu}>
        <button
          type="button"
          onClick={onNewProject}
          className={`${entry} !text-tx4 data-[highlighted]:!text-tx3`}
        >
          + New project
        </button>
      </Entry>
    </>
  );
}

/** A project: opens its Session. The open one has an accent bar and "(open)". */
function ProjectEntry({
  project: p,
  open,
  menu,
}: {
  project: RailProject;
  open: boolean;
  menu: boolean;
}) {
  const mark = markOf(p);
  return (
    <Entry menu={menu}>
      <Link
        to="/projects/$project/session"
        params={{ project: p.id }}
        data-project={p.id}
        aria-label={`${p.name}${open ? " (open)" : ""}${markLabel(mark)}`}
        className={`${entry} relative`}
      >
        {open && (
          <span aria-hidden="true" className="absolute inset-y-1 left-0 w-[3px] rounded bg-acc" />
        )}
        <span className={`min-w-0 flex-1 truncate ${open ? "font-medium text-tx" : ""}`}>
          {p.name}
          {open && <span className="font-normal text-tx4"> (open)</span>}
        </span>
        <span aria-hidden="true" className="flex">
          <MarkView mark={mark} />
        </span>
      </Link>
    </Entry>
  );
}
