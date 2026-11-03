// A `git diff` read for the screen (wiki/product/features/reviews.md): split
// per file, each line numbered on its side, so a comment can name its line.

/** One line of a file's diff: a hunk header, or a line kept, added or
 * removed, with its number on the target's side (old) and the branch's (new). */
export type DiffLine = {
  kind: "hunk" | "ctx" | "add" | "del" | "meta";
  text: string;
  old?: number;
  new?: number;
};

/** One file of a diff: its path, how it changed, and its numbered lines. A
 * renamed file has its path on the target in `from`; a pure rename, no lines. */
export type FileDiff = {
  path: string;
  from?: string;
  change: "" | "new" | "deleted" | "renamed";
  lines: DiffLine[];
  added: number;
  removed: number;
};

/**
 * A `git diff` split per file. Of its header lines, `diff --git`, the modes
 * and the renames become the file's path and change; what follows them
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
      // `diff --git a/<path> b/<path>`: unless it's renamed (and `rename to`
      // says where to), the same path twice, so its second half is exact
      // whatever the path holds, " b/" included.
      const both = line.slice(11);
      file = {
        path: unquote(both.slice((both.length + 1) / 2)).slice(2),
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
      else if (line.startsWith("rename from ")) {
        file.change = "renamed";
        file.from = unquote(line.slice(12));
      } else if (line.startsWith("rename to ")) file.path = unquote(line.slice(10));
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

const escapes: Record<string, number> = { a: 7, b: 8, t: 9, n: 10, v: 11, f: 12, r: 13 };

/** A path as git prints it: as is, or quoted C-style when it holds a quote,
 * a backslash, a control or a non-ASCII character (`"caf\303\251.txt"`, its
 * UTF-8 bytes in octal). */
function unquote(path: string): string {
  if (path.length < 2 || !path.startsWith('"') || !path.endsWith('"')) return path;
  const raw = new TextEncoder().encode(path.slice(1, -1));
  const bytes: number[] = [];
  for (let i = 0; i < raw.length; i++) {
    if (raw[i] !== 92) {
      bytes.push(raw[i]);
      continue;
    }
    const e = String.fromCharCode(raw[++i]);
    if (e >= "0" && e <= "7") {
      bytes.push(Number.parseInt(String.fromCharCode(...raw.slice(i, i + 3)), 8));
      i += 2;
    } else bytes.push(escapes[e] ?? raw[i]); // `\"` and `\\`
  }
  return new TextDecoder().decode(new Uint8Array(bytes));
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
