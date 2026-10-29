-- wiki/product/models/project-config.md#the-project-record
-- A project's workspace: the CEO's folder holding its project repo
-- (project/) and its agents' sandboxes (agents/). Empty for a project made
-- before workspaces, still in Foreman's home until it is migrated.
ALTER TABLE projects ADD COLUMN workspace TEXT NOT NULL DEFAULT '';
