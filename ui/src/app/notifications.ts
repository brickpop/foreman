import type { Deployment, Instance, Request, Team } from "../api/client";
import type { Event } from "../api/types/events";
import type { Notifications } from "../api/types/home";
import { agentOf } from "./model";

// What the CEO hears about (wiki/product/features/notifications.md): one
// notice per event worth it, shown as a toast always and as a browser
// notification when its toggle is on and permission is granted.

export type Notice = {
  /** The Settings toggle that gates its browser notification; none: a toast only. */
  toggle?: keyof Notifications;
  title: string;
  body: string;
  /** agent@project: a newer notice replaces an older one from the same instance. */
  instance: string;
  deployment: string;
  /** The toast's dot (a token). */
  color: string;
};

/** The notice an event deserves, or none. */
export function noticeFor(e: Event, teams: Team[], instances: Instance[]): Notice | undefined {
  const name = (instance: string) => agentOf(teams, instance)?.soul.name ?? "Foreman";
  const project = (instance: string) => instance.split("@")[1] ?? "";
  if (e.type === "request.created") {
    const r = e.data as Request;
    const who = name(r.instanceId);
    const where = r.projectId ? ` on ${r.projectId}` : "";
    const base = { body: r.summary, instance: r.instanceId, deployment: r.deploymentId };
    if (r.kind === "review") {
      return {
        ...base,
        toggle: "mergeRequested",
        title: `${who} requested a merge${where}`,
        color: "var(--ok)",
      };
    }
    return {
      ...base,
      toggle: "needsMe",
      title: `${who} needs you${where}`,
      color: "var(--acc)",
    };
  }
  if (e.type === "instance.status") {
    const { id, status } = e.data as { id: string; status: string };
    const deployment = instances.find((i) => i.id === id)?.deploymentId ?? "";
    if (status === "queued") {
      // The CEO can act on it (the limit is theirs), but it isn't worth a browser notification.
      return {
        title: `${name(id)} is waiting for a session slot`,
        body: "Every slot is taken, idle agents included. They start when one frees; the limit is in Settings → Agents.",
        instance: id,
        deployment,
        color: "var(--tx4)",
      };
    }
    if (status !== "done") return undefined;
    return {
      toggle: "finishes",
      title: `${name(id)} is done on ${project(id)}`,
      body: "Open the session to see the hand-off.",
      instance: id,
      deployment,
      color: "var(--info)",
    };
  }
  if (e.type === "deployment.stopped") {
    // Foreman stopped it on its own (its project is gone); the CEO's own Stop says nothing.
    const d = e.data as Deployment;
    if (!d.stopReason) return undefined;
    return {
      title: `Foreman stopped ${teams.find((t) => t.id === d.teamId)?.config.name ?? d.teamId} on ${d.projectId}`,
      body: d.stopReason,
      instance: "",
      deployment: d.id,
      color: "var(--err)",
    };
  }
  return undefined;
}

export type Permission = "on" | "blocked" | "turn on" | "n/a";

/** The browser's notification permission, as Settings shows it. */
export function permission(): Permission {
  if (typeof Notification === "undefined") return "n/a";
  if (Notification.permission === "granted") return "on";
  if (Notification.permission === "denied") return "blocked";
  return "turn on";
}

/** Whether a notice also goes to the browser: its toggle is on, permission is
 * granted, and the CEO isn't looking at Foreman already (the toast covers that). */
export function notifiesBrowser(n: Notice, toggles: Notifications, hidden: boolean): boolean {
  return n.toggle !== undefined && toggles[n.toggle] && permission() === "on" && hidden;
}

/** Shows a browser notification; its click focuses the tab and opens the session. */
export function showBrowserNotification(n: Notice, open: () => void) {
  // The tag collapses duplicates from several open Foreman tabs, and replaces
  // an older notice from the same instance.
  const shown = new Notification(n.title, { body: n.body, tag: n.instance || n.title });
  shown.onclick = () => {
    window.focus();
    open();
    shown.close();
  };
}
