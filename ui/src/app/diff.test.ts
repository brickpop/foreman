import { describe, expect, it } from "vitest";
import { diffFiles } from "./diff";

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
  "diff --git a/old name.txt b/new name.txt",
  "similarity index 90%",
  "rename from old name.txt",
  "rename to new name.txt",
  "diff --git a/logo.png b/logo.png",
  "index 5555555..6666666 100644",
  "Binary files a/logo.png and b/logo.png differ",
  "",
].join("\n");

describe("diffFiles", () => {
  const files = diffFiles(diff);

  it("splits a diff per file, its headers turned into the path and the change", () => {
    expect(files.map((f) => [f.path, f.change, f.added, f.removed])).toEqual([
      ["src/app.ts", "", 2, 1],
      ["docs/new.md", "new", 1, 0],
      ["gone.txt", "deleted", 0, 1],
      ["new name.txt", "renamed", 0, 0], // a pure rename has no hunks; its path has a space
      ["logo.png", "", 0, 0],
    ]);
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
