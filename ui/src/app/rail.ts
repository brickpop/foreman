import type { Instance, Project, Request } from "../api/client";
import { projectName } from "./model";
import { deploymentStatus } from "./sessions";

// What the rail and its project popover show (wiki/product/features/app-shell.md#the-rail): each project's needs-you count and busy status, the one mark
// each place shows, and the keys that switch projects.

export type RailProject = {
  id: string;
  /** How it shows (projectName). */
  name: string;
  /** What waits on the CEO there: its open requests, reviews included (its Inbox's count). */
  inbox: number;
  /** Its team's status while it is busy (its liveliest agent's: awaiting, running,
   * starting, queued); none at rest. */
  busy?: string;
};

export type RailModel = {
  /** The visible projects, in the order given (by name). */
  projects: RailProject[];
  /** The hidden ones, in the popover's folded More group. */
  hidden: RailProject[];
};

const busy = ["awaiting", "running", "starting", "queued"];

/** Every project's counts, from the projects (sorted by name), instances and requests. */
export function railModel(
  projects: Project[],
  instances: Instance[],
  requests: Request[],
): RailModel {
  const open = requests.filter((r) => r.status === "open");
  const row = (p: Project): RailProject => {
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
    projects: projects.filter((p) => !p.hidden).map(row),
    hidden: projects.filter((p) => p.hidden).map(row),
  };
}

/** One mark per item: the needs-you badge wins over the busy dot. */
export type Mark = { count: number } | { status: string } | undefined;

export function markOf(p: RailProject): Mark {
  if (p.inbox > 0) return { count: p.inbox };
  if (p.busy) return { status: p.busy };
  return undefined;
}

/** The folded More group's one mark: the total badge if any hidden project needs the
 * CEO, else the first busy one's dot. */
export function groupMark(projects: RailProject[]): Mark {
  const count = projects.reduce((n, p) => n + p.inbox, 0);
  if (count > 0) return { count };
  const first = projects.find((p) => p.busy);
  return first?.busy ? { status: first.busy } : undefined;
}

/** The Foreman tile's badge: what needs the CEO in every project but the open one
 * (on the Dashboard, none is open: all of them). */
export function elsewhere(model: RailModel, open?: string): number {
  return [...model.projects, ...model.hidden]
    .filter((p) => p.id !== open)
    .reduce((n, p) => n + p.inbox, 0);
}

/** A badge's text: the count, capped at 9+. */
export function badgeText(count: number): string {
  return count > 9 ? "9+" : String(count);
}

/** A needs-you count in words: "3 requests need you". */
export function needsYou(count: number): string {
  return `${count} ${count === 1 ? "request needs" : "requests need"} you`;
}

/** What a screen reader hears after an item's name, e.g. ". 3 requests need you". */
export function markLabel(mark: Mark, where = ""): string {
  if (!mark) return "";
  if ("status" in mark) return `. ${mark.status}`;
  return `. ${needsYou(mark.count)}${where}`;
}

/** The project `step` (1 next, -1 previous) moves to from `open`, by name among the
 * visible ones, round the ends. From a hidden project or none, the first visible one. */
export function cycle(
  model: RailModel,
  open: string | undefined,
  step: 1 | -1,
): string | undefined {
  const ids = model.projects.map((p) => p.id);
  const at = open === undefined ? -1 : ids.indexOf(open);
  if (at < 0) return ids[0];
  return ids[(at + step + ids.length) % ids.length];
}

type Keys = Pick<KeyboardEvent, "code" | "ctrlKey" | "altKey" | "shiftKey" | "metaKey">;

// By key position (e.code), so any layout works, and Alt's special characters on a Mac too.
const ctrlAlt = (e: Keys) => e.ctrlKey && e.altKey && !e.shiftKey && !e.metaKey;

/** Ctrl+Alt+P opens the project popover from anywhere. */
export function isPopoverShortcut(e: Keys): boolean {
  return ctrlAlt(e) && e.code === "KeyP";
}

/** Ctrl+Alt+↓ / ↑ cycles to the next (1) or previous (-1) project; 0 for any other key. */
export function cycleStep(e: Keys): 1 | -1 | 0 {
  if (!ctrlAlt(e)) return 0;
  return e.code === "ArrowDown" ? 1 : e.code === "ArrowUp" ? -1 : 0;
}
