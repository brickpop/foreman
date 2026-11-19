// The New project dialog's and Project settings' repo helpers
// (wiki/product/features/add-project.md).

import type { Branches } from "../api/client";
import { readable, slug } from "./model";

/** A repo's default name, from its folder: the folder's own name as words
 * that make its id (foreman_ui is "Foreman ui"). "" when nothing is left. */
export function nameFromFolder(path: string): string {
  return readable(slug(path.replace(/\/+$/, "").split("/").pop() ?? ""));
}

/** A cloned repo's default name, from its URL: the last part of its path
 * without .git (git@github.com:me/ledger-api.git is "Ledger api"), as
 * nameFromFolder makes it. */
export function cloneName(url: string): string {
  return nameFromFolder(
    url
      .trim()
      .replace(/\/+$/, "")
      .replace(/\.git$/, "")
      .replaceAll(":", "/"),
  );
}

/** What words make a repo's id, for the hints under its Name: "Its id: <id>.
 * It shows as “<words>”." Undefined while they make no id. */
export function repoIdLine(words: string): string | undefined {
  const id = slug(words);
  return id ? `Its id: ${id}. It shows as “${words.trim()}”.` : undefined;
}

/** The rename hint: whether the repo's id changes with the words. Unchanged
 * words (shown, how the repo shows now) change nothing, even for an older id
 * that isn't their slug (Ledger_API showing as "Ledger API"). */
export function renameHint(id: string, words: string, shown: string): string {
  if (words.trim() === shown) return `Its id stays ${id}.`;
  const next = slug(words);
  if (!next) return anyWords;
  if (next === id) return `Its id stays ${id}.`;
  return `Its id becomes ${next}: agents' worktrees and open merge requests follow.`;
}

/** The repo hint while the words make no id yet. */
export const anyWords = "Any words: they become its id, in lowercase with hyphens.";

/** The New project dialog's hint under the name: the id and folder the typed
 * words make, and how the project will show (those words, trimmed). */
export function newProjectHint(typed: string): string {
  const id = slug(typed);
  if (!id) return "Any words: they become its id and its folder, in lowercase with hyphens.";
  return `Its id: ${id}, in projects/${id}/ of Foreman’s folder. It shows as “${typed.trim()}”.`;
}

/** A new folder's path, name inside folder. */
export function inside(folder: string, name: string): string {
  return `${folder.replace(/\/+$/, "")}/${name}`;
}

/** The "created" toast's body: what became the new project's HQ. */
export function createdBody(created: boolean, cloned: boolean, path: string): string {
  if (!created) return `Took over the HQ left at ${path}, as it was.`;
  if (cloned) return `Cloned its HQ into ${path}.`;
  return `Made its HQ at ${path}, with project.toml and a starter wiki.`;
}

/** The default branch to send when adding an existing repo: none when it has
 * a main (the server takes that), else the CEO's pick, by default the branch
 * its checkout is on, or its first. */
export function defaultToSend(b: Branches | undefined, chosen?: string): string | undefined {
  if (!b || b.branches.includes("main")) return undefined;
  if (chosen && b.branches.includes(chosen)) return chosen;
  return b.branches.includes(b.current) ? b.current : b.branches[0];
}

/** The folder browser's breadcrumbs: from the home folder (named as itself)
 * down to path, each with the path it opens; nothing above home. A path
 * outside home (never served) is just itself. */
export function folderCrumbs(path: string, home: string): { name: string; path: string }[] {
  const root = home.replace(/\/+$/, "") || "/";
  const name = (p: string) => p.split("/").filter(Boolean).pop() ?? "/";
  if (path !== root && !path.startsWith(root === "/" ? "/" : `${root}/`)) {
    return [{ name: name(path), path }];
  }
  const crumbs = [{ name: name(root), path: root }];
  let at = root;
  for (const part of path.slice(root.length).split("/").filter(Boolean)) {
    at = at === "/" ? `/${part}` : `${at}/${part}`;
    crumbs.push({ name: part, path: at });
  }
  return crumbs;
}
