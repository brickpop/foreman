import { useScaffold } from "../app/queries";
import { Button, Empty } from "./ui";

// What Overview and Board show while the project repo lacks the files they
// need (wiki/product/features/wiki-reader.md#no-wiki-yet): a project made
// before the scaffold had them, or one whose files were deleted.

export function WikiSetup({ project, missing }: { project: string; missing: string[] }) {
  const scaffold = useScaffold(project);
  return (
    <Empty>
      <div className="flex flex-col items-center gap-3">
        <span className="font-medium text-tx2">This project's wiki isn't set up yet.</span>
        <span>Foreman reads it once its project repo has:</span>
        <ul className="m-0 flex list-none flex-col gap-0.5 p-0 font-mono text-[13px] text-tx2">
          {missing.map((file) => (
            <li key={file}>
              {project}/project/{file}
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
