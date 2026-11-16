import type { Missing } from "../api/client";
import { useProjectHQ, useScaffold } from "../app/queries";
import { Button, Empty } from "./ui";

// What Wiki and Board show while the HQ lacks scaffold files
// (wiki/product/features/wiki-reader.md#no-wiki-yet): a project made before
// the scaffold had them, or one whose files were deleted. Each creates what
// is missing, never over a file that exists.

/** Instead of the wiki: some missing file is one it can't be read without.
 * A wiki still in its HQ's wiki/ (from before the wiki moved to the HQ's
 * root) is the CEO's to move: nothing is created beside it. */
export function WikiSetup({ project, missing }: { project: string; missing: Missing[] }) {
  const scaffold = useScaffold(project);
  const repo = useProjectHQ(project);
  const from = missing.find((m) => m.from)?.from;
  if (from) return <MoveWiki hq={repo} from={from} />;
  return (
    <Empty>
      <div className="flex flex-col items-center gap-3">
        <span className="font-medium text-tx2">This project's wiki isn't set up yet.</span>
        <span>Foreman reads it once its HQ has:</span>
        <ul className="m-0 flex list-none flex-col gap-0.5 p-0 font-mono text-[13px] text-tx2">
          {missing.map((m) => (
            <li key={m.file}>
              {repo}/{m.file}
            </li>
          ))}
        </ul>
        <span className="text-[13px]">
          Created from Foreman's starter, next to what is there (nothing is overwritten), and
          committed in the HQ as you.
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

/** A wiki still in the HQ's wiki/: how to move it to the HQ's root. */
function MoveWiki({ hq, from }: { hq: string; from: string }) {
  const move = `cd ${hq} && git mv ${from}* . && git commit -m "The wiki at the HQ's root"`;
  return (
    <Empty>
      <div className="flex max-w-[620px] flex-col items-center gap-3">
        <span className="font-medium text-tx2">
          This project's wiki is still in its HQ's {from}.
        </span>
        <span>
          Foreman reads it at the HQ's root. Move it there with your git, while no team is deployed
          on the project; Foreman picks it up:
        </span>
        <code className="font-mono text-[13px] text-tx2 select-text">{move}</code>
        <span className="text-[13px]">
          If its AGENTS.md is Foreman's starter, it still describes the old place: delete it, and
          Foreman offers the current one.
        </span>
      </div>
    </Empty>
  );
}
