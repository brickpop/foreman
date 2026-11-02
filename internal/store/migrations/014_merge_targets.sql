-- Merge targets (wiki/product/features/pipeline-and-merges.md#merge-targets).
-- A repo's main branch is its default branch now: the target it starts from
-- and merges go to unless told otherwise.
ALTER TABLE project_repos RENAME COLUMN main TO default_branch;

-- A repo's other targets, made by the lead from any of its commits.
CREATE TABLE repo_targets (
    project_id  TEXT NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    repo        TEXT NOT NULL COLLATE NOCASE,
    name        TEXT NOT NULL,
    made_by     TEXT NOT NULL, -- the lead's instance
    from_commit TEXT NOT NULL, -- the commit it was made at
    created_at  TEXT NOT NULL,
    PRIMARY KEY (project_id, repo, name)
);

-- The branch each merge request merges into: the ones before went to their
-- repo's default branch.
ALTER TABLE merge_requests ADD COLUMN target TEXT NOT NULL DEFAULT '';
UPDATE merge_requests SET target = coalesce(
    (SELECT default_branch FROM project_repos r WHERE r.project_id = merge_requests.project_id AND r.name = merge_requests.repo),
    'main');
