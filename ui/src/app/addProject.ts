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

/** A new project's name from what the CEO typed: a slug, as the server
 * requires (it is the workspace's folder; readable makes how it shows):
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

/** A new folder's path, name inside folder. */
export function inside(folder: string, name: string): string {
  return `${folder.replace(/\/+$/, "")}/${name}`;
}

/** The "created" toast's body: what became the new project's repo. */
export function createdBody(created: boolean, cloned: boolean, path: string): string {
  if (!created) return `Took over the project repo left at ${path}, as it was.`;
  if (cloned) return `Cloned its project repo into ${path}.`;
  return `Made its project repo at ${path}, with project.toml and a starter wiki.`;
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
