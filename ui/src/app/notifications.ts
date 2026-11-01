import type { Deployment, Instance, Request, Team } from "../api/client";
import type { Event } from "../api/types/events";
import type { Notifications } from "../api/types/home";
import type { Withdrawn } from "../api/types/workflow";
import { agentOf, readable } from "./model";

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
  /** A review's notice opens the review, not the lead's session. */
  review?: string;
  /** The toast's dot (a token). */
  color: string;
};

/** The notice an event deserves, or none. */
export function noticeFor(e: Event, teams: Team[], instances: Instance[]): Notice | undefined {
  const name = (deployment: string, instance: string) =>
    agentOf(teams, deployment, instance)?.soul.name ?? "Foreman";
  const project = (instance: string) => readable(instance.split("@")[1] ?? "");
  if (e.type === "request.created") {
    const r = e.data as Request;
    const who = name(r.deploymentId, r.instanceId);
    const where = r.projectId ? ` on ${readable(r.projectId)}` : "";
    const base = { body: r.summary, instance: r.instanceId, deployment: r.deploymentId };
    if (r.kind === "review") {
      return {
        ...base,
        review: r.reviewId,
        toggle: "mergeRequested",
        title: `${who} requested a merge${where}`,
        color: "var(--ok)",
      };
    }
    return {
      ...base,
      toggle: "needsMe",
      title: `${who} needs you${where}`,
      color: "var(--attn)",
    };
  }
  if (e.type === "review.withdrawn") {
    // A toast only: nothing waits on the CEO. Their drafts on it are gone
    // (the CEO, 2026-10-22: warn that they're lost).
    const w = e.data as Withdrawn;
    const lost = w.lostDrafts === 1 ? "draft comment" : `${w.lostDrafts} draft comments`;
    return {
      title: `${w.by === "foreman" ? "Foreman" : name(w.deploymentId, w.lead)} withdrew ${w.review.branch}`,
      body:
        `${w.review.withdrawnReason}.` +
        (w.lostDrafts > 0 ? ` Your ${lost} on it ${w.lostDrafts === 1 ? "is" : "are"} lost.` : ""),
      instance: w.lead,
      deployment: w.deploymentId,
      review: w.review.id,
      color: w.lostDrafts > 0 ? "var(--err)" : "var(--tx4)",
    };
  }
  if (e.type === "instance.status") {
    const { id, status } = e.data as { id: string; status: string };
    const deployment = instances.find((i) => i.id === id)?.deploymentId ?? "";
    if (status === "queued") {
      // The CEO can act on it (the limit is theirs), but it isn't worth a browser notification.
      return {
        title: `${name(deployment, id)} is waiting for a session slot`,
        body: "Every slot is taken, idle agents included. They start when one frees; the limit is in Settings → Agents.",
        instance: id,
        deployment,
        color: "var(--tx4)",
      };
    }
    if (status !== "done") return undefined;
    return {
      toggle: "finishes",
      title: `${name(deployment, id)} is done on ${project(id)}`,
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
      title: `Foreman stopped ${teams.find((t) => t.id === d.teamId)?.config.name ?? d.teamId} on ${readable(d.projectId)}`,
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

/** Shows a browser notification; its click focuses the tab and opens what
 * it is about (a review, else the session). */
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
