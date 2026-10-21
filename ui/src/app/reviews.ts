import type { NewComment, RepoDiff, Review, ReviewComment } from "../api/client";
import {
  ReviewChangesRequested,
  ReviewClosed,
  ReviewMerged,
  ReviewNeedsYou,
} from "../api/types/workflow";
import type { DiffLine } from "./diff";

// The Reviews screens' logic (wiki/product/features/reviews.md), apart from
// their components so it can be tested.

const statusLabels: Record<string, string> = {
  [ReviewNeedsYou]: "Needs your review",
  [ReviewChangesRequested]: "Changes requested",
  [ReviewMerged]: "Merged",
  [ReviewClosed]: "Closed",
};

/** A review's status as the screens say it. */
export function statusLabel(status: string): string {
  return statusLabels[status] ?? status;
}

/** Whether the CEO can still act on it: comment, merge, request changes, close. */
export function isOpen(v: Review): boolean {
  return v.status === ReviewNeedsYou || v.status === ReviewChangesRequested;
}

/** The repos still open in a review, with the main each merges into. */
export function openRepos(v: Review, mainOf: (repo: string) => string) {
  return v.mergeRequests
    .filter((m) => m.state === "open")
    .map((m) => ({ repo: m.repo, main: mainOf(m.repo), conflicting: m.conflicting }));
}

/** The decision button for the repos ticked: one main for all, else per repo. */
export function mergeLabel(ticked: { main: string }[]): string {
  const mains = new Set(ticked.map((r) => r.main));
  return mains.size === 1 ? `Merge to ${ticked[0].main}` : "Merge";
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

/** The place a diff line holds comments at: a removed line on main's side,
 * the others on the branch's. None for a hunk header. */
export function diffKey(repo: string, path: string, line: DiffLine): string | undefined {
  if (line.kind === "del" && line.old !== undefined) return lineKey(repo, path, "old", line.old);
  if ((line.kind === "add" || line.kind === "ctx") && line.new !== undefined)
    return lineKey(repo, path, "new", line.new);
  return undefined;
}

/**
 * The comments by where they show: on their line (a new-side one at the line
 * it moved to; an old-side one, a removed line, only against main, where
 * main's numbering is the one it was written on), general ones, the outdated
 * ones, whose line changed since, and those whose line the diff shown doesn't
 * have (its repo merged or closed, its file out of the since view).
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

/**
 * The comment a click on a diff line starts: on the branch's side for a line
 * kept or added, main's for a removed one (only against main: since the last
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

/** The Reviews page's sections: what needs the CEO, what waits on a lead,
 * and the last ones done (newest first, as the server lists them). */
export function reviewGroups(reviews: Review[], done = 20) {
  return {
    needsYou: reviews.filter((v) => v.status === ReviewNeedsYou),
    waiting: reviews.filter((v) => v.status === ReviewChangesRequested),
    done: reviews.filter((v) => !isOpen(v)).slice(0, done),
  };
}
