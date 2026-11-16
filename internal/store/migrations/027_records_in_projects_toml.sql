-- A project's portable record (its display name, whether it is hidden, each
-- repo's root commit and default branch) moves to projects.toml in Foreman's
-- HQ, which travels between machines; this database keeps what belongs to
-- this machine: its profile and each repo's path (the CEO, 2026-11-15;
-- wiki/backlog/portable-projects.md). The CEO's records move with the
-- one-off script, not here.
ALTER TABLE projects DROP COLUMN name;
ALTER TABLE projects DROP COLUMN hidden;
ALTER TABLE project_repos DROP COLUMN root_commit;
ALTER TABLE project_repos DROP COLUMN default_branch;
