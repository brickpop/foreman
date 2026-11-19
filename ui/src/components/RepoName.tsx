import { useRepoName } from "../app/queries";

/** A repo as people read it, by its id: its name (repoName), with its name
 * and id as the tooltip. */
export function RepoName({
  project,
  id,
  className,
}: {
  project: string;
  id: string;
  className?: string;
}) {
  const repo = useRepoName(project);
  return (
    <span title={repo.title(id)} className={className}>
      {repo.name(id)}
    </span>
  );
}
