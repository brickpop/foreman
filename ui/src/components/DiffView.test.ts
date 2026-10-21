import { describe, expect, it } from "vitest";
import { diffFiles } from "./DiffView";

// As `git diff main...branch` prints it.
const diff = [
  "diff --git a/src/app.ts b/src/app.ts",
  "index 1111111..2222222 100644",
  "--- a/src/app.ts",
  "+++ b/src/app.ts",
  "@@ -1,2 +1,2 @@",
  " keep",
  "-old",
  "+new",
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
  it("splits a diff per file, its headers turned into the path and the change", () => {
    expect(diffFiles(diff)).toEqual([
      { path: "src/app.ts", change: "", lines: ["@@ -1,2 +1,2 @@", " keep", "-old", "+new"] },
      {
        path: "docs/new.md",
        change: "new",
        // Inside a hunk a line is a line, even one that looks like a header.
        lines: ["@@ -0,0 +1 @@", "+--- a line that looks like a header, inside a hunk"],
      },
      { path: "gone.txt", change: "deleted", lines: ["@@ -1 +0,0 @@", "-bye"] },
      // A pure rename has no hunks; its path has a space.
      { path: "new name.txt", change: "renamed", lines: [] },
      { path: "logo.png", change: "", lines: ["Binary files a/logo.png and b/logo.png differ"] },
    ]);
  });

  it("has no files for no changes", () => {
    expect(diffFiles("")).toEqual([]);
  });
});
