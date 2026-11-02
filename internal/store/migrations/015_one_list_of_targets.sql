-- A repo's targets are one list, its default branch one of them, flagged
-- (wiki/product/features/pipeline-and-merges.md#merge-targets): changing the
-- default moves the flag, and the old default stays a target.
ALTER TABLE repo_targets ADD COLUMN is_default INTEGER NOT NULL DEFAULT 0;
-- made_by '' and from_commit '': set by the CEO, not made by a lead.
INSERT INTO repo_targets (project_id, repo, name, made_by, from_commit, created_at, is_default)
    SELECT project_id, name, default_branch, '', '', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), 1 FROM project_repos
    WHERE true ON CONFLICT (project_id, repo, name) DO UPDATE SET is_default = 1;
CREATE UNIQUE INDEX repo_targets_one_default ON repo_targets (project_id, repo) WHERE is_default = 1;
ALTER TABLE project_repos DROP COLUMN default_branch;

-- The target a review comment was read against: one on a removed line (the
-- target's side) is outdated once its repo merges somewhere else.
ALTER TABLE review_comments ADD COLUMN target TEXT NOT NULL DEFAULT '';
UPDATE review_comments SET target = coalesce(
    (SELECT target FROM merge_requests m WHERE m.review_id = review_comments.review_id AND m.repo = review_comments.repo),
    '') WHERE repo != '';
