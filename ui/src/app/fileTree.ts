// Paths as a folder tree, as GitHub shows a change's files: folders before
// files, each alphabetically, and a chain of folders holding only one folder
// shown as one row (`ui/src/app`).

/** A folder (its whole path, and its name: one folder, or a chain of them
 * joined by `/`) or one of the items given. */
export type TreeNode<T> =
  | { kind: "folder"; name: string; path: string; children: TreeNode<T>[] }
  | { kind: "file"; name: string; item: T };

type Folder<T> = Extract<TreeNode<T>, { kind: "folder" }>;

/** The items as a tree, by their path (a renamed file's new one). */
export function fileTree<T extends { path: string }>(items: T[]): TreeNode<T>[] {
  const root: Folder<T> = { kind: "folder", name: "", path: "", children: [] };
  for (const item of items) {
    const parts = item.path.split("/");
    let at = root;
    for (const name of parts.slice(0, -1)) {
      const path = at.path ? `${at.path}/${name}` : name;
      let next = at.children.find((c): c is Folder<T> => c.kind === "folder" && c.name === name);
      if (!next) {
        next = { kind: "folder", name, path, children: [] };
        at.children.push(next);
      }
      at = next;
    }
    at.children.push({ kind: "file", name: parts[parts.length - 1], item });
  }
  return shape(root.children);
}

/** Compacts each folder's chain and sorts every level. */
function shape<T>(nodes: TreeNode<T>[]): TreeNode<T>[] {
  return nodes
    .map((n) => {
      if (n.kind === "file") return n;
      let folder = n;
      while (folder.children.length === 1 && folder.children[0].kind === "folder") {
        const only = folder.children[0];
        folder = { ...only, name: `${folder.name}/${only.name}` };
      }
      return { ...folder, children: shape(folder.children) };
    })
    .sort((a, b) =>
      a.kind === b.kind ? a.name.localeCompare(b.name) : a.kind === "folder" ? -1 : 1,
    );
}
