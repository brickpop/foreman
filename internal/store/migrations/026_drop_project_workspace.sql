-- A project's folders follow from Foreman's folder and its id (the CEO,
-- 2026-11-04): <foreman folder>/projects/<id>/ and worktrees/<id>/, so no
-- project keeps a workspace of its own (wiki/backlog/foreman-folder.md).
ALTER TABLE projects DROP COLUMN workspace;
