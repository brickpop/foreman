import { describe, expect, it } from "vitest";
import { diffFiles, type FileDiff, foldedAtFirst, largeDiff } from "./diff";

// As `git diff main...branch` prints it.
const diff = [
  "diff --git a/src/app.ts b/src/app.ts",
  "index 1111111..2222222 100644",
  "--- a/src/app.ts",
  "+++ b/src/app.ts",
  "@@ -10,3 +10,3 @@ export function app() {",
  " keep",
  "-old",
  "+new",
  " after",
  "@@ -40 +40,2 @@",
  " forty",
  "+forty-one",
  "\\ No newline at end of file",
  "diff --git a/docs/new.md b/docs/new.md",
  "new file mode 100644",
  "index 0000000..3333333",
  "--- /dev/null",
  "+++ b/docs/new.md",
  "@@ -0,0 +1 @@",
  "+--- a line that looks like a header, inside a hunk",
  "diff --git a/gone.txt b/gone.txt",
  "deleted file mode 100644",
  "index 4444444..0000000",
  "--- a/gone.txt",
  "+++ /dev/null",
  "@@ -1 +0,0 @@",
  "-bye",
  "diff --git a/old name.txt b/to b/new name.txt",
  "similarity index 100%",
  "rename from old name.txt",
  "rename to to b/new name.txt",
  "diff --git a/logo.png b/logo.png",
  "index 5555555..6666666 100644",
  "Binary files a/logo.png and b/logo.png differ",
  "diff --git a/lib/a.ts b/src/a.ts",
  "similarity index 80%",
  "rename from lib/a.ts",
  "rename to src/a.ts",
  "index 7777777..8888888 100644",
  "--- a/lib/a.ts",
  "+++ b/src/a.ts",
  "@@ -1 +1 @@",
  "-x",
  "+y",
  "",
].join("\n");

describe("diffFiles", () => {
  const files = diffFiles(diff);

  it("splits a diff per file, its headers turned into the path and the change", () => {
    expect(files.map((f) => [f.path, f.from, f.change, f.added, f.removed])).toEqual([
      ["src/app.ts", undefined, "", 2, 1],
      ["docs/new.md", undefined, "new", 1, 0],
      ["gone.txt", undefined, "deleted", 0, 1],
      // A pure rename: no hunks, and paths with spaces and a " b/" that
      // `diff --git` alone can't tell apart.
      ["to b/new name.txt", "old name.txt", "renamed", 0, 0],
      ["logo.png", undefined, "", 0, 0],
      ["src/a.ts", "lib/a.ts", "renamed", 1, 1],
    ]);
    expect(files[3].lines).toEqual([]);
    expect(files[4].lines).toEqual([
      { kind: "meta", text: "Binary files a/logo.png and b/logo.png differ" },
    ]);
  });

  it("numbers each line on its side, from each hunk's header", () => {
    expect(files[0].lines).toEqual([
      { kind: "hunk", text: "@@ -10,3 +10,3 @@ export function app() {" },
      { kind: "ctx", text: " keep", old: 10, new: 10 },
      { kind: "del", text: "-old", old: 11 },
      { kind: "add", text: "+new", new: 11 },
      { kind: "ctx", text: " after", old: 12, new: 12 },
      { kind: "hunk", text: "@@ -40 +40,2 @@" },
      { kind: "ctx", text: " forty", old: 40, new: 40 },
      { kind: "add", text: "+forty-one", new: 41 },
      { kind: "meta", text: "\\ No newline at end of file" },
    ]);
    // Inside a hunk a line is a line, even one that looks like a header.
    expect(files[1].lines[1]).toEqual({
      kind: "add",
      text: "+--- a line that looks like a header, inside a hunk",
      new: 1,
    });
    expect(files[2].lines[1]).toEqual({ kind: "del", text: "-bye", old: 1 });
  });

  it("has no files for no changes", () => {
    expect(diffFiles("")).toEqual([]);
  });
});

describe("foldedAtFirst", () => {
  const file = (path: string, added: number, removed = 0): FileDiff => ({
    path,
    change: "",
    lines: [],
    added,
    removed,
  });

  it("folds a change over the limit, counting both sides", () => {
    expect(foldedAtFirst(file("src/a.ts", largeDiff))).toBeUndefined();
    expect(foldedAtFirst(file("src/a.ts", largeDiff, 1))).toMatch(/large/);
  });

  it("folds lock files however small, by their whole name only", () => {
    expect(foldedAtFirst(file("ui/bun.lock", 1))).toMatch(/generated/);
    expect(foldedAtFirst(file("go.sum", 2))).toMatch(/generated/);
    expect(foldedAtFirst(file("docs/not-go.sum.md", 2))).toBeUndefined();
    expect(foldedAtFirst(file("mybun.lock", 2))).toBeUndefined();
  });
});
