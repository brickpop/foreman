import { useDiff } from "../app/queries";
import { Modal } from "./Overlays";

// What a merge request changes: its diff against main, one box per file, one
// colored line per line (wiki/product/features/project-pipeline.md).

/** One file of a diff: its path, how it changed, and its hunks' lines. */
export type FileDiff = {
  path: string;
  change: "" | "new" | "deleted" | "renamed";
  lines: string[];
};

/**
 * A `git diff` split per file. The header lines (`diff --git`, `index`, the
 * modes, `---`/`+++`) become the file's path and change; what follows them
 * (the hunks, or "Binary files … differ") is kept as is.
 */
export function diffFiles(diff: string): FileDiff[] {
  const files: FileDiff[] = [];
  let file: FileDiff | undefined;
  let header = false;
  for (const line of diff.split("\n")) {
    if (line.startsWith("diff --git ")) {
      // `diff --git a/<old> b/<new>`: the new path, until `+++` says better.
      const at = line.lastIndexOf(" b/");
      file = { path: at >= 0 ? line.slice(at + 3) : line.slice(11), change: "", lines: [] };
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
    file.lines.push(line);
  }
  // The diff's last newline leaves an empty line that is no line of the file.
  for (const f of files) if (f.lines.at(-1) === "") f.lines.pop();
  return files;
}

const lineColor = (line: string) =>
  line.startsWith("@@")
    ? "bg-hover text-tx4"
    : line.startsWith("+")
      ? "bg-ok/10 text-ok"
      : line.startsWith("-")
        ? "bg-err/10 text-err"
        : "text-tx3";

export function DiffView({
  mergeRequest,
  title,
  onClose,
}: {
  mergeRequest: string;
  title: string;
  onClose: () => void;
}) {
  const diff = useDiff(mergeRequest);
  const files = diffFiles(diff.data ?? "");
  return (
    <Modal open onClose={onClose} title={title} width={820}>
      <div className="flex max-h-[65vh] flex-col gap-3 overflow-auto font-mono text-[12.5px] leading-[1.6] select-text">
        {diff.error && <p className="m-0 text-err">{diff.error.message}</p>}
        {diff.data !== undefined && files.length === 0 && (
          <p className="m-0 text-tx3">No changes.</p>
        )}
        {files.map((f) => (
          <section key={f.path} className="shrink-0 rounded-[10px] border border-line">
            <header className="sticky top-0 flex items-center gap-2 rounded-t-[10px] border-b border-line bg-side px-3.5 py-2">
              <span className="font-medium text-tx">{f.path}</span>
              {f.change && <span className="text-tx4">{f.change}</span>}
            </header>
            <div className="rounded-b-[10px] bg-term py-1">
              {f.lines.map((line, i) => (
                // biome-ignore lint/suspicious/noArrayIndexKey: a diff line's place is its identity
                <div key={i} className={`px-3.5 whitespace-pre-wrap ${lineColor(line)}`}>
                  {line || " "}
                </div>
              ))}
            </div>
          </section>
        ))}
      </div>
    </Modal>
  );
}
