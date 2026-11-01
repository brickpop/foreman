// A `git diff` read for the screen (wiki/product/features/reviews.md): split
// per file, each line numbered on its side, so a comment can name its line.

/** One line of a file's diff: a hunk header, or a line kept, added or
 * removed, with its number on main's side (old) and the branch's (new). */
export type DiffLine = {
  kind: "hunk" | "ctx" | "add" | "del" | "meta";
  text: string;
  old?: number;
  new?: number;
};

/** One file of a diff: its path, how it changed, and its numbered lines. */
export type FileDiff = {
  path: string;
  change: "" | "new" | "deleted" | "renamed";
  lines: DiffLine[];
  added: number;
  removed: number;
};

/**
 * A `git diff` split per file. The header lines (`diff --git`, `index`, the
 * modes, `---`/`+++`) become the file's path and change; what follows them
 * (the hunks, or "Binary files … differ") is numbered from each hunk's
 * `@@ -a,b +c,d @@`.
 */
export function diffFiles(diff: string): FileDiff[] {
  const files: FileDiff[] = [];
  let file: FileDiff | undefined;
  let header = false;
  let old = 0;
  let next = 0;
  for (const line of diff.split("\n")) {
    if (line.startsWith("diff --git ")) {
      // `diff --git a/<old> b/<new>`: the new path, until `+++` says better.
      const at = line.lastIndexOf(" b/");
      file = {
        path: at >= 0 ? line.slice(at + 3) : line.slice(11),
        change: "",
        lines: [],
        added: 0,
        removed: 0,
      };
      files.push(file);
      header = true;
      continue;
    }
    if (!file) continue;
    if (header && !line.startsWith("@@") && !line.startsWith("Binary files")) {
      if (line.startsWith("new file mode")) file.change = "new";
      else if (line.startsWith("deleted file mode")) file.change = "deleted";
      else if (line.startsWith("rename from")) file.change = "renamed";
      else if (line.startsWith("+++ b/")) file.path = line.slice(6);
      continue;
    }
    header = false;
    const hunk = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/.exec(line);
    if (hunk) {
      old = Number(hunk[1]);
      next = Number(hunk[2]);
      file.lines.push({ kind: "hunk", text: line });
    } else if (line.startsWith("+")) {
      file.lines.push({ kind: "add", text: line, new: next++ });
      file.added++;
    } else if (line.startsWith("-")) {
      file.lines.push({ kind: "del", text: line, old: old++ });
      file.removed++;
    } else if (line.startsWith(" ")) {
      file.lines.push({ kind: "ctx", text: line, old: old++, new: next++ });
    } else {
      // "\ No newline at end of file", "Binary files … differ", the diff's last newline.
      file.lines.push({ kind: "meta", text: line });
    }
  }
  for (const f of files) {
    const last = f.lines.at(-1);
    if (last?.kind === "meta" && last.text === "") f.lines.pop();
  }
  return files;
}

/** Lock files and the like: written by tools, rarely read in a review. */
const generated =
  /(^|\/)(bun\.lockb?|package-lock\.json|yarn\.lock|pnpm-lock\.yaml|go\.sum|Cargo\.lock|poetry\.lock|uv\.lock|composer\.lock|Gemfile\.lock)$/;

/** More changed lines than a file shows before the CEO opens it. */
export const largeDiff = 500;

/** Why a file starts folded, if it does: a large change, or a generated file
 * (as GitHub folds them). Drawing them all at once is what makes a big
 * review slow; the CEO opens the ones worth reading. */
export function foldedAtFirst(f: FileDiff): string | undefined {
  if (generated.test(f.path)) return "generated: open it to read";
  if (f.added + f.removed > largeDiff) return "large diff: open it to read";
  return undefined;
}
