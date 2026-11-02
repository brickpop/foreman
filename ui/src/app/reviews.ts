import type {
  Deployment,
  Dismiss,
  Dismissed,
  NewComment,
  RepoDiff,
  Review,
  ReviewComment,
  Team,
} from "../api/client";
import {
  ReviewChangesRequested,
  ReviewDismissed,
  ReviewMerged,
  ReviewNeedsYou,
  ReviewWithdrawn,
} from "../api/types/workflow";
import type { DiffLine } from "./diff";
import { readable } from "./model";

// The Reviews screens' logic (wiki/product/features/reviews.md), apart from
// their components so it can be tested.

const statusLabels: Record<string, string> = {
  [ReviewNeedsYou]: "Needs your review",
  [ReviewChangesRequested]: "Changes requested",
  [ReviewMerged]: "Merged",
  [ReviewDismissed]: "Dismissed",
  [ReviewWithdrawn]: "Withdrawn",
};

/** One way to Dismiss a review: what becomes of its team and branches, and
 * whether the lead hears of it (only while its team keeps working). */
export type DismissChoice = {
  value: "keep" | "stop" | "drop";
  label: string;
  stopTeam: boolean;
  dropBranches: boolean;
  tells: boolean;
};

/** The review's team, by name, and whether its deployment still runs. */
export function reviewTeam(v: Review, active: Deployment[], teams: Team[]) {
  const teamId = v.deploymentId.split("@")[0];
  return {
    name: teams.find((t) => t.id === teamId)?.config.name || teamId,
    live: active.some((d) => d.id === v.deploymentId),
  };
}

/** The ways to Dismiss, the first the default. Branches go only with their
 * team, as its agents work in them (the CEO, 2026-10-21). */
export function dismissChoices(
  team: { name: string; live: boolean },
  project: string,
): DismissChoice[] {
  const aside = "(kept aside, recoverable by hand)";
  const choice = (value: DismissChoice["value"], label: string, stop: boolean, drop: boolean) => ({
    value,
    label,
    stopTeam: stop,
    dropBranches: drop,
    tells: team.live && !stop,
  });
  return team.live
    ? [
        choice(
          "keep",
          `${team.name} keeps working on ${readable(project)}; the branches stay`,
          false,
          false,
        ),
        choice("stop", `Stop ${team.name} on ${readable(project)}; the branches stay`, true, false),
        choice(
          "drop",
          `Stop ${team.name} on ${readable(project)} and drop the branches ${aside}`,
          true,
          true,
        ),
      ]
    : [
        choice("keep", "Keep the branches", false, false),
        choice("drop", `Drop the branches ${aside}`, false, true),
      ];
}

/** The Dismiss to send: the note only when the lead will hear it. */
export function dismissRequest(choice: DismissChoice, note: string): Dismiss {
  return {
    note: choice.tells ? note.trim() : "",
    dropBranches: choice.dropBranches,
    stopTeam: choice.stopTeam,
  };
}

/** Why a note typed won't reach the lead with this choice, if it won't
 * (typed, then a choice that stops the team): the CEO is warned. */
export function lostNote(choice: DismissChoice, note: string, lead: string): string | undefined {
  if (choice.tells || !note.trim()) return undefined;
  return choice.stopTeam
    ? `${lead} won't get your note: the team stops.`
    : `${lead} won't get your note: the team isn't running.`;
}

/** The Dismiss button: it sends only a note someone hears. */
export function dismissLabel(choice: DismissChoice, note: string): string {
  return choice.tells && note.trim() ? "Dismiss and send" : "Dismiss now";
}

/** The branches a dismissal kept, and why, one line each. */
export function keptLines(dismissed: Dismissed): string {
  return dismissed.kept.map((k) => `${k.branch} in ${k.repo}: ${k.why}.`).join("\n");
}

/** A review's status as the screens say it. */
export function statusLabel(status: string): string {
  return statusLabels[status] ?? status;
}

/** Whether the CEO can still act on it: comment, merge, request changes, dismiss. */
export function isOpen(v: Review): boolean {
  return v.status === ReviewNeedsYou || v.status === ReviewChangesRequested;
}

/** The repos still open in a review, with the target each merges into. */
export function openRepos(v: Review) {
  return v.mergeRequests
    .filter((m) => m.state === "open")
    .map((m) => ({ repo: m.repo, target: m.target, conflicting: m.conflicting }));
}

/** How a repo of the review was decided: "merged", "dismissed" or
 * "withdrawn"; "" while it is open. Its diff is then the change as decided. */
export function decidedAs(v: Review, repo: string): string {
  const state = v.mergeRequests.find((m) => m.repo === repo)?.state ?? "open";
  return state === "open" ? "" : state === "rejected" ? "dismissed" : state;
}

/** The decision button for the repos ticked: one target for all, else per repo. */
export function mergeLabel(ticked: { target: string }[]): string {
  const targets = new Set(ticked.map((r) => r.target));
  return targets.size === 1 ? `Merge to ${ticked[0].target}` : "Merge";
}

/** What Merge sends: each ticked repo with the head its diff was shown at,
 * so a commit the CEO hasn't seen is never merged. Undefined while any of
 * them has no diff loaded. */
export function mergeHeads(
  diffs: RepoDiff[] | undefined,
  repos: string[],
): Record<string, string> | undefined {
  const heads: Record<string, string> = {};
  for (const repo of repos) {
    const head = diffs?.find((d) => d.repo === repo)?.head;
    if (!head) return undefined;
    heads[repo] = head;
  }
  return heads;
}

/** Whether there is a "since your last look" to show: the CEO requested
 * changes on an open repo before. */
export function hasSince(v: Review): boolean {
  return v.mergeRequests.some((m) => m.state === "open" && m.reviewedHead !== "");
}

/** Where a line comment sits in the diff: repo, file, side and line. */
export function lineKey(repo: string, path: string, side: "new" | "old", line: number): string {
  return `${repo}\u0000${path}\u0000${side}\u0000${line}`;
}

/** The place a diff line holds comments at: a removed line on the target's side,
 * the others on the branch's. None for a hunk header. */
export function diffKey(repo: string, path: string, line: DiffLine): string | undefined {
  if (line.kind === "del" && line.old !== undefined) return lineKey(repo, path, "old", line.old);
  if ((line.kind === "add" || line.kind === "ctx") && line.new !== undefined)
    return lineKey(repo, path, "new", line.new);
  return undefined;
}

/**
 * The comments by where they show: on their line (a new-side one at the line
 * it moved to; an old-side one, a removed line, only against the target, where
 * the target's numbering is the one it was written on), general ones, the outdated
 * ones, whose line changed since, and those whose line the diff shown doesn't
 * have (its repo merged, dismissed or withdrawn, its file out of the since view).
 */
export function placeComments(comments: ReviewComment[], since: boolean, shown: Set<string>) {
  const at = new Map<string, ReviewComment[]>();
  const general: ReviewComment[] = [];
  const outdated: ReviewComment[] = [];
  const elsewhere: ReviewComment[] = [];
  for (const c of comments) {
    if (c.repo === "") general.push(c);
    else if (c.outdated || (c.side === "old" && since)) outdated.push(c);
    else {
      const side = c.side === "old" ? "old" : "new";
      const key = lineKey(c.repo, c.path, side, side === "old" ? c.line : c.now);
      if (shown.has(key)) at.set(key, [...(at.get(key) ?? []), c]);
      else elsewhere.push(c);
    }
  }
  return { at, general, outdated, elsewhere };
}

/** The draft comments, which Request changes sends. */
export function drafts(comments: ReviewComment[]): number {
  return comments.filter((c) => c.state === "draft").length;
}

/** What Merge asks first while drafts wait: a merge closes the review, and
 * they go with it (the CEO, 2026-11-03). */
export function lostDrafts(n: number): string {
  return n === 1
    ? "1 draft comment won't be sent and will be lost. Merge anyway?"
    : `${n} draft comments won't be sent and will be lost. Merge anyway?`;
}

/**
 * The comment a click on a diff line starts: on the branch's side for a line
 * kept or added, the target's for a removed one (only against it: since the last
 * look, the old side is the branch as it was). None on a hunk header.
 */
export function commentAt(
  repo: string,
  path: string,
  line: DiffLine,
  head: string,
  since: boolean,
): NewComment | undefined {
  const snippet = line.text.slice(1);
  if (line.kind === "add" || line.kind === "ctx")
    return line.new === undefined
      ? undefined
      : { repo, path, side: "new", line: line.new, commit: head, snippet, body: "" };
  if (line.kind === "del" && !since && line.old !== undefined)
    return { repo, path, side: "old", line: line.old, commit: head, snippet, body: "" };
  return undefined;
}

/** The Inbox's reviews: what needs the CEO, what waits on a lead,
 * and the last ones done (newest first, as the server lists them). */
export function reviewGroups(reviews: Review[], done = 20) {
  return {
    needsYou: reviews.filter((v) => v.status === ReviewNeedsYou),
    waiting: reviews.filter((v) => v.status === ReviewChangesRequested),
    done: reviews.filter((v) => !isOpen(v)).slice(0, done),
  };
}
