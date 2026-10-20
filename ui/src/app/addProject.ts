// The New project dialog's and Project settings' repo helpers
// (wiki/product/features/add-project.md).

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
