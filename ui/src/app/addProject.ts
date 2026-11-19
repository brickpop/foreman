// The New project dialog's and Project settings' repo helpers
// (wiki/product/features/add-project.md).

import type { Branches } from "../api/client";

/** A repo's default name, from its folder: the folder's own name, made to
 * follow the server's rule (letters, digits, dots, hyphens and underscores,
 * starting with a letter or digit, at most 63, no ".."). "" when nothing is left. */
export function repoName(path: string): string {
  const base = path.replace(/\/+$/, "").split("/").pop() ?? "";
  return base
    .replace(/[^A-Za-z0-9._-]+/g, "-")
    .replace(/\.{2,}/g, ".")
    .replace(/^[^A-Za-z0-9]+/, "")
    .slice(0, 63);
}

/** A cloned repo's default name, from its URL: the last part of its path
 * without .git (git@github.com:me/ledger-api.git is ledger-api), as
 * repoName makes it. */
export function cloneName(url: string): string {
  return repoName(
    url
      .trim()
      .replace(/\/+$/, "")
      .replace(/\.git$/, "")
      .replaceAll(":", "/"),
  );
}

/** A new project's name from what the CEO typed: a slug, as the server
 * requires (it names its folders; the typed words are how it shows):
 * lowercase letters and digits, accents dropped, anything else one hyphen,
 * at most 63. "My Café" is my-cafe; "" when nothing is left. */
export function projectSlug(typed: string): string {
  return typed
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+/, "")
    .slice(0, 63)
    .replace(/-+$/, "");
}

/** The New project dialog's hint under the name: the id and folder the typed
 * words make, and how the project will show (those words, trimmed). */
export function newProjectHint(typed: string): string {
  const id = projectSlug(typed);
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
