-- No list of targets (the CEO, 2026-11-06): a merge may go into any branch
-- of the repo but an agent's, so a repo keeps only its default branch
-- (wiki/product/features/pipeline-and-merges.md#where-a-merge-goes).
ALTER TABLE project_repos ADD COLUMN default_branch TEXT NOT NULL DEFAULT 'main';
UPDATE project_repos SET default_branch = coalesce(
    (SELECT name FROM repo_targets t WHERE t.project_id = project_repos.project_id AND t.repo = project_repos.name AND t.is_default = 1),
    'main');
DROP TABLE repo_targets;
