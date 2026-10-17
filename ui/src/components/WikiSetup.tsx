import type { Missing } from "../api/client";
import { useScaffold } from "../app/queries";
import { Button, Empty } from "./ui";

// What Overview and Board show while the project repo lacks scaffold files
// (wiki/product/features/wiki-reader.md#no-wiki-yet): a project made before
// the scaffold had them, or one whose files were deleted. Each creates what
// is missing, never over a file that exists.

/** Instead of the wiki: some missing file is one it can't be read without. */
export function WikiSetup({ project, missing }: { project: string; missing: Missing[] }) {
  const scaffold = useScaffold(project);
  return (
    <Empty>
      <div className="flex flex-col items-center gap-3">
        <span className="font-medium text-tx2">This project's wiki isn't set up yet.</span>
        <span>Foreman reads it once its project repo has:</span>
        <ul className="m-0 flex list-none flex-col gap-0.5 p-0 font-mono text-[13px] text-tx2">
          {missing.map((m) => (
            <li key={m.file}>
              {project}/project/{m.file}
            </li>
          ))}
        </ul>
        <span className="text-[13px]">
          Created from Foreman's starter, next to what is there (nothing is overwritten), and
          committed in the project repo as you.
        </span>
        <Button variant="primary" disabled={scaffold.isPending} onClick={() => scaffold.mutate()}>
          {missing.length === 1 ? "Create it" : "Create them"}
        </Button>
        {scaffold.error && <span className="text-[13px] text-err">{scaffold.error.message}</span>}
      </div>
    </Empty>
  );
}

/** Above the wiki: missing files it reads fine without (AGENTS.md). */
export function ScaffoldNotice({ project, missing }: { project: string; missing: Missing[] }) {
  const scaffold = useScaffold(project);
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 rounded-lg border border-line2 bg-panel px-3.5 py-2.5 text-[13px] text-tx3">
      {missing.map((m) => (
        <span key={m.file}>
          <span className="font-mono text-tx2">{m.file}</span> is missing: {m.why}.
        </span>
      ))}
      <span className="flex-1" />
      <Button disabled={scaffold.isPending} onClick={() => scaffold.mutate()}>
        {missing.length === 1 ? "Create it" : "Create them"}
      </Button>
      {scaffold.error && <span className="text-err">{scaffold.error.message}</span>}
    </div>
  );
}
