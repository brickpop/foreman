import { describe, expect, it } from "vitest";
import { diffFiles } from "./diff";
import { fileTree, type TreeNode, treeOrder } from "./fileTree";

/** A tree as nested names, folders ending in `/`, for readable expectations. */
type Shape = string | { [folder: string]: Shape[] };
function shapeOf<T>(nodes: TreeNode<T>[]): Shape[] {
  return nodes.map((n) => (n.kind === "file" ? n.name : { [`${n.name}/`]: shapeOf(n.children) }));
}
const tree = (...paths: string[]) => shapeOf(fileTree(paths.map((path) => ({ path }))));

describe("fileTree", () => {
  it("is empty without files", () => {
    expect(fileTree([])).toEqual([]);
  });

  it("keeps a root-level file at the root", () => {
    expect(tree("README.md")).toEqual(["README.md"]);
  });

  it("nests files under their folders", () => {
    expect(tree("a/x.ts", "a/b/y.ts", "a/b/z.ts", "c/w.ts")).toEqual([
      { "a/": [{ "b/": ["y.ts", "z.ts"] }, "x.ts"] },
      { "c/": ["w.ts"] },
    ]);
  });

  it("compacts a chain of single-folder folders, the chain ending in files", () => {
    expect(tree("ui/src/app/x.ts", "ui/src/app/y.ts")).toEqual([
      { "ui/src/app/": ["x.ts", "y.ts"] },
    ]);
  });

  it("compacts a chain in the middle of the tree, then branches", () => {
    expect(tree("a/b/c/d/x.ts", "a/b/c/e/y.ts", "a/top.ts")).toEqual([
      { "a/": [{ "b/c/": [{ "d/": ["x.ts"] }, { "e/": ["y.ts"] }] }, "top.ts"] },
    ]);
  });

  it("doesn't compact a folder holding files and one subfolder", () => {
    expect(tree("a/b/x.ts", "a/y.ts")).toEqual([{ "a/": [{ "b/": ["x.ts"] }, "y.ts"] }]);
  });

  it("doesn't compact a folder holding two subfolders", () => {
    expect(tree("a/b/x.ts", "a/c/y.ts")).toEqual([
      { "a/": [{ "b/": ["x.ts"] }, { "c/": ["y.ts"] }] },
    ]);
  });

  it("gives each folder its whole path, a compacted one its deepest", () => {
    const [ui] = fileTree([{ path: "ui/src/app/x.ts" }, { path: "ui/src/app/sub/y.ts" }]);
    expect(ui).toMatchObject({ kind: "folder", name: "ui/src/app", path: "ui/src/app" });
    if (ui.kind !== "folder") throw new Error("not a folder");
    expect(ui.children[0]).toMatchObject({ kind: "folder", name: "sub", path: "ui/src/app/sub" });
  });

  it("puts folders before files, each alphabetically, whatever the input order", () => {
    expect(tree("z.ts", "b/x.ts", "A.ts", "a/x.ts", "m.ts")).toEqual([
      { "a/": ["x.ts"] },
      { "b/": ["x.ts"] },
      "A.ts",
      "m.ts",
      "z.ts",
    ]);
  });

  it("keeps each item as given, for its row to use", () => {
    const item = { path: "a/x.ts", extra: 1 };
    const [a] = fileTree([item]);
    expect(a.kind === "folder" && a.children[0]).toEqual({ kind: "file", name: "x.ts", item });
  });

  it("places a renamed file by its new path", () => {
    const files = diffFiles(
      [
        "diff --git a/old/dir/a.ts b/new/place/a.ts",
        "similarity index 100%",
        "rename from old/dir/a.ts",
        "rename to new/place/a.ts",
      ].join("\n"),
    );
    expect(shapeOf(fileTree(files))).toEqual([{ "new/place/": ["a.ts"] }]);
  });

  it("shows a file replaced by a folder of the same name as both", () => {
    expect(tree("a", "a/x.ts")).toEqual([{ "a/": ["x.ts"] }, "a"]);
  });
});

describe("treeOrder", () => {
  it("lists the items as the tree shows them: folders first, at every level", () => {
    // git's order: plain path order, a file before a folder that sorts after it.
    const git = ["README.md", "a.ts", "src/b/y.ts", "src/x.ts", "z/w.ts"].map((path) => ({ path }));
    expect(treeOrder(git).map((f) => f.path)).toEqual([
      "src/b/y.ts",
      "src/x.ts",
      "z/w.ts",
      "a.ts", // names as the tree sorts them: case aside
      "README.md",
    ]);
  });

  it("keeps every item, once, and the same objects", () => {
    const items = [{ path: "x/a" }, { path: "b" }, { path: "x/y/c" }];
    const ordered = treeOrder(items);
    expect(ordered).toHaveLength(3);
    for (const i of items) expect(ordered).toContain(i);
  });

  it("is empty without items", () => {
    expect(treeOrder([])).toEqual([]);
  });
});
